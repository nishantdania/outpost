package vmapi

import (
	"crypto/x509"
	"encoding/pem"
	"regexp"
	"strings"
)

func validEgressMaterial(spec VMSpec) bool {
	if spec.EgressCA == "" && spec.CredentialEnv == "" {
		return true
	}
	if len(spec.EgressCA) > 8192 || len(spec.CredentialEnv) > 16384 {
		return false
	}
	block, rest := pem.Decode([]byte(spec.EgressCA))
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !cert.IsCA {
		return false
	}
	lines := strings.Split(strings.TrimSuffix(spec.CredentialEnv, "\n"), "\n")
	if len(lines) == 0 || len(lines) > 64 {
		return false
	}
	seen := map[string]bool{}
	for _, line := range lines {
		name, value, ok := strings.Cut(line, "=")
		if !ok || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`).MatchString(name) || seen[name] || !regexp.MustCompile(`^outpost_ref_[a-f0-9]{64}$`).MatchString(value) {
			return false
		}
		seen[name] = true
	}
	return true
}
