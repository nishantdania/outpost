package assets

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

type Manifest struct {
	Version      string  `json:"version"`
	Architecture string  `json:"architecture"`
	Assets       []Asset `json:"assets"`
}
type Asset struct {
	Name   string `json:"name"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}

var fileName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
var digest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var required = map[string]string{
	"ark": "ark", "arkd": "arkd", "ark-vm-launcher": "ark-vm-launcher", "firecracker": "firecracker", "jailer": "jailer", "vmlinux": "vmlinux", "rootfs.ext4": "rootfs.ext4", "default": "default.oci.tar",
}

func Load(path string) (Manifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var m Manifest
	if err := decoder.Decode(&m); err != nil {
		return m, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return m, fmt.Errorf("trailing manifest data")
	}
	if m.Version == "" || m.Architecture != "amd64" || len(m.Assets) != len(required) {
		return m, fmt.Errorf("invalid asset manifest")
	}
	seenName := map[string]bool{}
	seenFile := map[string]bool{}
	for _, a := range m.Assets {
		u, err := url.Parse(a.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || required[a.Name] != a.File || seenName[a.Name] || seenFile[a.File] || !fileName.MatchString(a.File) || !digest.MatchString(a.SHA256) {
			return m, fmt.Errorf("invalid asset manifest")
		}
		seenName[a.Name], seenFile[a.File] = true, true
	}
	return m, nil
}

func Verify(path, want string) error {
	if !digest.MatchString(want) {
		return fmt.Errorf("invalid checksum")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("checksum mismatch for %s", filepath.Base(path))
	}
	return nil
}
