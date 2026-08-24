package assets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func manifest(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "assets.json")
	if err := os.WriteFile(p, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func valid() string {
	items := []string{}
	for _, asset := range []struct{ name, file, url string }{{"outpost", "outpost", "outpost"}, {"outpostd", "outpostd", "outpostd"}, {"outpost-vm-launcher", "outpost-vm-launcher", "outpost-vm-launcher"}, {"firecracker", "firecracker", "firecracker"}, {"jailer", "jailer", "jailer"}, {"vmlinux", "vmlinux", "vmlinux"}, {"rootfs.ext4", "rootfs.ext4", "rootfs.ext4.zst"}, {"default", "default.oci.tar", "default.oci.tar.zst"}} {
		items = append(items, `{"name":"`+asset.name+`","file":"`+asset.file+`","sha256":"`+strings.Repeat("a", 64)+`","url":"https://example.test/`+asset.url+`"}`)
	}
	return `{"version":"1","architecture":"amd64","assets":[` + strings.Join(items, ",") + `]}`
}
func TestLoadAcceptsExactManifest(t *testing.T) {
	if _, err := Load(manifest(t, valid())); err != nil {
		t.Fatal(err)
	}
}
func TestLoadRejectsMalformedAndTrailing(t *testing.T) {
	for _, body := range []string{"{", valid() + " x", `{"version":"1","architecture":"amd64","assets":[],"x":1}`} {
		if _, err := Load(manifest(t, body)); err == nil {
			t.Fatalf("accepted %q", body)
		}
	}
}
func TestLoadRejectsMissingExtraAndDuplicate(t *testing.T) {
	for _, body := range []string{strings.Replace(valid(), `,"url":"https://example.test/default.oci.tar.zst"}`, `}`, 1), strings.Replace(valid(), `]}`, `,{"name":"extra","file":"extra","sha256":"`+strings.Repeat("a", 64)+`","url":"https://example.test/extra"}]}`, 1), strings.Replace(valid(), `"name":"outpostd"`, `"name":"outpost"`, 1), strings.Replace(valid(), `"file":"outpostd"`, `"file":"outpost"`, 1)} {
		if _, err := Load(manifest(t, body)); err == nil {
			t.Fatal("accepted invalid set")
		}
	}
}
func TestLoadRejectsFields(t *testing.T) {
	for _, body := range []string{strings.Replace(valid(), `"architecture":"amd64"`, `"architecture":"arm64"`, 1), strings.Replace(valid(), `"version":"1"`, `"version":""`, 1), strings.Replace(valid(), `"file":"outpost"`, `"file":"../outpost"`, 1), strings.Replace(valid(), `https://example.test/outpost`, `http://example.test/outpost`, 1), strings.Replace(valid(), strings.Repeat("a", 64), strings.Repeat("A", 64), 1)} {
		if _, err := Load(manifest(t, body)); err == nil {
			t.Fatal("accepted invalid field")
		}
	}
}
func TestVerifyStreamingDigest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "asset")
	if err := os.WriteFile(p, []byte("asset"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(p, "d59386e0ae435e292fbe0ebc0c8b0c4f7e2b1f5f4f2c6b6fd2d5d54d4c7e53d0"); err == nil {
		t.Fatal("accepted wrong digest")
	}
}
