package assets

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
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
	"outpost": "outpost", "outpostd": "outpostd", "outpost-vm-launcher": "outpost-vm-launcher", "firecracker": "firecracker", "jailer": "jailer", "vmlinux": "vmlinux", "rootfs.ext4": "rootfs.ext4", "default": "default.oci.tar",
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
