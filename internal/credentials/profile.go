// Package credentials implements host-held, snapshot-inherited HTTP credentials.
package credentials

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
)

const Port = 18443

var ErrInvalid = errors.New("invalid credential profile")
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
var secretPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var hostPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,252}$`)
var headerPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)
var methodPattern = regexp.MustCompile(`^[A-Z]{1,16}$`)

type Operation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type Binding struct {
	Env        string      `json:"env"`
	Secret     string      `json:"secret"`
	Host       string      `json:"host"`
	Header     string      `json:"header"`
	Encoding   string      `json:"encoding"`
	Prefix     string      `json:"prefix,omitempty"` // Optional literal prefix for raw auth schemes.
	Username   string      `json:"username,omitempty"`
	Operations []Operation `json:"operations,omitempty"`
}
type Profile struct {
	// nil permits public HTTPS; an explicit list restricts additional egress.
	// Keep nil/null distinct from [] when persisting snapshot policy.
	AllowedHosts []string  `json:"allowed_hosts"`
	Credentials  []Binding `json:"credentials"`
}

func Parse(data []byte) (Profile, error) {
	var p Profile
	if len(data) > 65536 {
		return p, ErrInvalid
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if d.Decode(&p) != nil {
		return p, ErrInvalid
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Profile{}, ErrInvalid
	}
	return p, p.Validate()
}

func (p Profile) Validate() error {
	if len(p.Credentials) == 0 || len(p.Credentials) > 64 || len(p.AllowedHosts) > 128 {
		return ErrInvalid
	}
	seen := map[string]string{}
	slots := map[string]bool{}
	for _, host := range p.AllowedHosts {
		if !validHost(host) {
			return ErrInvalid
		}
	}
	for _, b := range p.Credentials {
		if !namePattern.MatchString(b.Env) || !secretPattern.MatchString(b.Secret) || !validHost(b.Host) || !headerPattern.MatchString(b.Header) {
			return ErrInvalid
		}
		if existing, ok := seen[b.Env]; ok && existing != b.Secret {
			return ErrInvalid
		}
		seen[b.Env] = b.Secret
		slot := b.Host + "\x00" + http.CanonicalHeaderKey(b.Header) + "\x00" + b.Value(Placeholder(b.Secret, b.Env))
		if slots[slot] {
			return ErrInvalid
		}
		slots[slot] = true
		switch http.CanonicalHeaderKey(b.Header) {
		case "Host", "Content-Length", "Transfer-Encoding", "Connection", "Upgrade", "Trailer", "Accept-Encoding", "Proxy-Authorization":
			return ErrInvalid
		}
		if b.Prefix != "" {
			if b.Encoding != "raw" || len(b.Prefix) > 64 {
				return ErrInvalid
			}
			for _, c := range []byte(b.Prefix) {
				if c < 32 || c > 126 {
					return ErrInvalid
				}
			}
		}
		switch b.Encoding {
		case "bearer", "raw":
			if b.Username != "" {
				return ErrInvalid
			}
		case "basic":
			if strings.ContainsAny(b.Username, ":\r\n") || len(b.Username) > 128 {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		if len(b.Operations) > 128 {
			return ErrInvalid
		}
		for _, op := range b.Operations {
			if !methodPattern.MatchString(op.Method) || op.Method == "CONNECT" || !strings.HasPrefix(op.Path, "/") || strings.ContainsAny(op.Path, "?#\r\n") || len(op.Path) > 2048 {
				return ErrInvalid
			}
		}
	}
	return nil
}
func validHost(host string) bool {
	return hostPattern.MatchString(host) && strings.Contains(host, ".") && !strings.Contains(host, "..") && !strings.HasSuffix(host, ".") && net.ParseIP(host) == nil
}
func (p Profile) JSON() string { b, _ := json.Marshal(p); return string(b) }
func Placeholder(secretName, env string) string {
	sum := sha256.Sum256([]byte(secretName + "\x00" + env))
	return fmt.Sprintf("outpost_ref_%x", sum[:])
}

// References are stable across snapshot forks, including references persisted in
// application data. Authorization is the host-owned VM/TAP boundary, not secrecy
// of a reference. Rotating the host value does not change guest references.
func (p Profile) Environment() string {
	var lines []string
	seen := map[string]bool{}
	for _, b := range p.Credentials {
		if seen[b.Env] {
			continue
		}
		seen[b.Env] = true
		lines = append(lines, b.Env+"="+Placeholder(b.Secret, b.Env))
	}
	return strings.Join(lines, "\n") + "\n"
}
func (b Binding) Value(value string) string {
	switch b.Encoding {
	case "bearer":
		return "Bearer " + value
	case "basic":
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(b.Username+":"+value))
	default:
		return b.Prefix + value
	}
}
func (b Binding) Allows(r *http.Request) bool {
	if len(b.Operations) == 0 {
		return true
	}
	for _, op := range b.Operations {
		if op.Method == r.Method && op.Path == r.URL.EscapedPath() {
			return true
		}
	}
	return false
}
