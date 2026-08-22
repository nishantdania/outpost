package setup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

func Run(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("setup requires Linux")
	}
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return err
	}
	if file, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err != nil {
		return fmt.Errorf("open /dev/kvm: %w", err)
	} else {
		file.Close()
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	assets := filepath.Join(home, ".local", "share", "outpost", "assets")
	if err := os.MkdirAll(assets, 0o700); err != nil {
		return err
	}
	firecracker, version, err := firecrackerBinary(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(assets, "firecracker"), firecracker, 0o755); err != nil {
		return err
	}
	kernel, rootfs, kernelKey, rootfsKey, err := images(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(assets, "vmlinux"), kernel, 0o600); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp("", "outpost-rootfs-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	squashfs := filepath.Join(temporary, "rootfs.squashfs")
	if err := os.WriteFile(squashfs, rootfs, 0o600); err != nil {
		return err
	}
	root := filepath.Join(temporary, "rootfs")
	if err := command(ctx, "sudo", "unsquashfs", "-d", root, squashfs); err != nil {
		return err
	}
	ext4 := filepath.Join(assets, "rootfs.ext4")
	if err := command(ctx, "sudo", "truncate", "-s", "1G", ext4); err != nil {
		return err
	}
	if err := command(ctx, "sudo", "mkfs.ext4", "-d", root, "-F", ext4); err != nil {
		return err
	}
	if err := command(ctx, "sudo", "chown", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), ext4); err != nil {
		return err
	}
	manifest := fmt.Sprintf("{\"firecracker\":%q,\"kernel\":%q,\"rootfs\":%q}\n", version, kernelKey, rootfsKey)
	return os.WriteFile(filepath.Join(assets, "manifest.json"), []byte(manifest), 0o600)
}

func command(ctx context.Context, name string, args ...string) error {
	c := exec.CommandContext(ctx, name, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
func get(ctx context.Context, url string) ([]byte, error) {
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if e != nil {
		return nil, e
	}
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("download %s: %s", url, res.Status)
	}
	return io.ReadAll(res.Body)
}
func firecrackerBinary(ctx context.Context) ([]byte, string, error) {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	base := "https://github.com/firecracker-microvm/firecracker/releases"
	r, e := http.NewRequestWithContext(ctx, http.MethodGet, base+"/latest", nil)
	if e != nil {
		return nil, "", e
	}
	res, e := http.DefaultClient.Do(r)
	if e != nil {
		return nil, "", e
	}
	res.Body.Close()
	parts := strings.Split(strings.TrimRight(res.Request.URL.Path, "/"), "/")
	v := parts[len(parts)-1]
	a, e := get(ctx, fmt.Sprintf("%s/download/%s/firecracker-%s-%s.tgz", base, v, v, arch))
	if e != nil {
		return nil, "", e
	}
	gz, e := gzip.NewReader(bytes.NewReader(a))
	if e != nil {
		return nil, "", e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, "", e
		}
		if strings.Contains(h.Name, "/firecracker-") {
			b, e := io.ReadAll(tr)
			return b, v, e
		}
	}
	return nil, "", fmt.Errorf("firecracker binary missing")
}
func images(ctx context.Context) ([]byte, []byte, string, string, error) {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	s3 := "https://s3.amazonaws.com/spec.ccfc.min"
	data, e := get(ctx, s3+"?list-type=2&prefix=firecracker-ci/&delimiter=/")
	if e != nil {
		return nil, nil, "", "", e
	}
	re := regexp.MustCompile(`firecracker-ci/[0-9]{8}-[^/]+/`)
	p := re.FindAllString(string(data), -1)
	if len(p) == 0 {
		return nil, nil, "", "", fmt.Errorf("no CI artifacts")
	}
	sort.Strings(p)
	prefix := p[len(p)-1]
	find := func(kind string) (string, error) {
		d, e := get(ctx, s3+"?list-type=2&prefix="+prefix+arch+"/"+kind)
		if e != nil {
			return "", e
		}
		r := regexp.MustCompile(prefix + regexp.QuoteMeta(arch) + `/` + kind + `[^<]+`)
		m := r.FindAllString(string(d), -1)
		if len(m) == 0 {
			return "", fmt.Errorf("missing %s", kind)
		}
		sort.Strings(m)
		return m[len(m)-1], nil
	}
	k, e := find("vmlinux-")
	if e != nil {
		return nil, nil, "", "", e
	}
	r, e := find("ubuntu-")
	if e != nil {
		return nil, nil, "", "", e
	}
	kb, e := get(ctx, s3+"/"+k)
	if e != nil {
		return nil, nil, "", "", e
	}
	rb, e := get(ctx, s3+"/"+r)
	return kb, rb, k, r, e
}
