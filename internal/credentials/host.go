package credentials

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var ErrUnavailable = errors.New("host credential unavailable")

type Host struct {
	directory string
	ca        *x509.Certificate
	key       *ecdsa.PrivateKey
	PublicCA  string
}

// NewHost requires pre-provisioned private directories owned by the daemon user.
// The CA is persisted separately from guest disks and snapshot metadata.
func NewHost(directory, state string) (*Host, error) {
	for _, dir := range []string{directory, state} {
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return nil, ErrUnavailable
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || stat.Uid != uint32(os.Geteuid()) {
			return nil, ErrUnavailable
		}
	}
	path := filepath.Join(state, "ca.pem")
	data, err := privateRead(path, 16384)
	if errors.Is(err, os.ErrNotExist) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, e
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			return nil, e
		}
		template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Outpost credential egress"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().AddDate(5, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
		der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if e != nil {
			return nil, e
		}
		encoded, e := x509.MarshalECPrivateKey(key)
		if e != nil {
			return nil, e
		}
		data = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: encoded})...)
		// O_EXCL prevents silent CA rotation or concurrent replacement.
		f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, ErrUnavailable
		}
		_, e = f.Write(data)
		if e == nil {
			e = f.Sync()
		}
		closeErr := f.Close()
		if e != nil || closeErr != nil {
			return nil, ErrUnavailable
		}
		dir, e := os.Open(state)
		if e != nil {
			return nil, ErrUnavailable
		}
		e = dir.Sync()
		closeErr = dir.Close()
		if e != nil || closeErr != nil {
			return nil, ErrUnavailable
		}
	} else if err != nil {
		return nil, ErrUnavailable
	}
	pair, err := tls.X509KeyPair(data, data)
	if err != nil || len(pair.Certificate) != 1 {
		return nil, ErrUnavailable
	}
	ca, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || !ca.IsCA || time.Now().After(ca.NotAfter) {
		return nil, ErrUnavailable
	}
	key, ok := pair.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, ErrUnavailable
	}
	return &Host{directory: directory, ca: ca, key: key, PublicCA: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}))}, nil
}

func privateRead(path string, limit int64) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 || info.Size() <= 0 || info.Size() > limit {
		return nil, ErrUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrUnavailable
	}
	return data, nil
}
func (h *Host) Secret(name string) (string, error) {
	if !secretPattern.MatchString(name) {
		return "", ErrUnavailable
	}
	data, err := privateRead(filepath.Join(h.directory, name), 4096)
	if err != nil {
		return "", ErrUnavailable
	}
	// Permit the final newline written by common secret-file tools, never header injection.
	value := string(data)
	if len(value) > 0 && value[len(value)-1] == '\n' {
		value = value[:len(value)-1]
	}
	if value == "" {
		return "", ErrUnavailable
	}
	for _, c := range []byte(value) {
		if c < 32 || c > 126 {
			return "", ErrUnavailable
		}
	}
	return value, nil
}
func (h *Host) Check(p Profile) error {
	if p.Validate() != nil {
		return ErrInvalid
	}
	for _, b := range p.Credentials {
		if _, err := h.Secret(b.Secret); err != nil {
			return err
		}
	}
	return nil
}
func (h *Host) certificate(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	until := time.Now().Add(24 * time.Hour)
	if h.ca.NotAfter.Before(until) {
		until = h.ca.NotAfter
	}
	template := &x509.Certificate{SerialNumber: serial, DNSNames: []string{host}, NotBefore: time.Now().Add(-time.Minute), NotAfter: until, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, h.ca, &key.PublicKey, h.key)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{Certificate: [][]byte{der, h.ca.Raw}, PrivateKey: key}, nil
}
