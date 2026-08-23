package image

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nishantdania/ark/internal/ark"
)

type localRunner struct{}

func (localRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	if name == "podman" && len(args) > 0 && args[0] == "unshare" {
		name, args = args[1], args[2:]
		if name == "tar" {
			filtered := args[:0]
			for _, arg := range args {
				if arg != "--same-owner" {
					filtered = append(filtered, arg)
				}
			}
			args = filtered
		}
	}
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
func (localRunner) RunIO(ctx context.Context, input io.Reader, name string, args ...string) error {
	command := exec.CommandContext(ctx, name, args...)
	command.Stdin = input
	return command.Run()
}

func rootfsTar(t *testing.T) *bytes.Reader {
	t.Helper()
	var data bytes.Buffer
	tw := tar.NewWriter(&data)
	for _, item := range []struct{ name, body string }{{".dockerenv", "bad"}, {"etc/machine-id", "bad"}, {"root/.ssh/authorized_keys", "bad"}, {"usr/lib/systemd/systemd", "systemd"}, {"usr/lib/systemd/system/multi-user.target", "target"}, {"usr/lib/systemd/system/ssh.service", "ssh"}, {"usr/bin/rsync", "rsync"}, {"usr/sbin/sshd", "sshd"}, {"etc/ssh/sshd_config", "PermitRootLogin prohibit-password\nPasswordAuthentication no\n"}} {
		name, body := item.name, item.body
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct{ name, target string }{{"sbin/init", "/usr/lib/systemd/systemd"}, {"etc/systemd/system/default.target", "/usr/lib/systemd/system/multi-user.target"}, {"etc/systemd/system/multi-user.target.wants/ssh.service", "/usr/lib/systemd/system/ssh.service"}} {
		if err := tw.WriteHeader(&tar.Header{Name: item.name, Linkname: item.target, Typeflag: tar.TypeSymlink, Mode: 0777}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data.Bytes())
}

func TestConvertCreatesJournaledBootableRootFS(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	rootfs, err := s.convert(context.Background(), rootfsTar(t))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(rootfs)
	out, err := exec.Command("debugfs", "-R", "stats", rootfs).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "has_journal") {
		t.Fatalf("journal missing: %v %s", err, out)
	}
	for _, path := range []string{"/.dockerenv", "/etc/machine-id", "/root/.ssh/authorized_keys"} {
		out, _ = exec.Command("debugfs", "-R", "stat "+path, rootfs).CombinedOutput()
		if !strings.Contains(string(out), "not found") {
			t.Fatalf("scrubbed path remains %s: %s", path, out)
		}
	}
}

func TestConvertIsDeterministic(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.convert(context.Background(), rootfsTar(t))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(first)
	time.Sleep(3 * time.Second)
	second, err := s.convert(context.Background(), rootfsTar(t))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(second)
	a, err := os.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := os.Open(second)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ha, hb := sha256.New(), sha256.New()
	io.Copy(ha, a)
	io.Copy(hb, b)
	if !bytes.Equal(ha.Sum(nil), hb.Sum(nil)) {
		out, _ := exec.Command("cmp", "-l", first, second).Output()
		t.Fatalf("identical rootfs did not produce identical ext4: %s", string(out[:min(len(out), 200)]))
	}
}

func TestNewReconcilesOnlyKnownTemporaryPrefixes(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := filepath.Join(t.TempDir(), "images")
	if err := os.MkdirAll(filepath.Join(root, ".unknown"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".context-stale"), 0750); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root, db, localRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".unknown")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".context-stale")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary entry remains: %v", err)
	}
}

func TestNewReconcilesOrphanArtifact(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	root := filepath.Join(t.TempDir(), "images")
	orphan := filepath.Join(root, strings.Repeat("a", 64))
	if err := os.MkdirAll(orphan, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "rootfs.ext4"), []byte("orphan"), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err := New(root, db, localRunner{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan remains: %v", err)
	}
}

func TestPublishDeduplicatesConcurrentWrites(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("x"), 4096)
	var first string
	var wg sync.WaitGroup
	var mu sync.Mutex
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			digest, _, err := s.publish(bytes.NewReader(data))
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if first == "" {
				first = digest
			} else if first != digest {
				t.Errorf("digest = %s, want %s", digest, first)
			}
		}()
	}
	wg.Wait()
	info, err := os.Stat(s.imagePath(first))
	if err != nil || info.Size() != int64(len(data)) {
		t.Fatalf("published artifact = %v, %v", info, err)
	}
}

func TestBootContractResolvesOnlyInsideGuest(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{"usr/lib/systemd/systemd": "x", "usr/bin/rsync": "x", "usr/sbin/sshd": "x", "usr/lib/systemd/system/multi-user.target": "x", "usr/lib/systemd/system/ssh.service": "x", "etc/ssh/sshd_config": "PermitRootLogin prohibit-password\nPasswordAuthentication no\n"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"sbin/init": "/usr/lib/systemd/systemd", "etc/systemd/system/default.target": "/usr/lib/systemd/system/multi-user.target", "etc/systemd/system/multi-user.target.wants/ssh.service": "/usr/lib/systemd/system/ssh.service"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
	}
	s := &Store{}
	if err := s.bootContract(context.Background(), root); err != nil {
		t.Fatalf("realistic links rejected: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "sbin/init")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/usr/bin/sh", filepath.Join(root, "sbin/init")); err != nil {
		t.Fatal(err)
	}
	if err := s.bootContract(context.Background(), root); err == nil {
		t.Fatal("host absolute symlink satisfied contract")
	}
}

type cleanupRunner struct{ calls [][]string }

func (r *cleanupRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil, nil
}
func (r *cleanupRunner) RunIO(context.Context, io.Reader, string, ...string) error { return nil }
func TestExtractedCleanupRemovesNestedFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".rootfs-test")
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "file"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	(&Store{runner: localRunner{}, podman: "podman"}).cleanupExtracted(dir)
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("extracted tree remains: %v", err)
	}
}
func TestExtractedCleanupUsesRootlessNamespace(t *testing.T) {
	r := &cleanupRunner{}
	(&Store{runner: r, podman: "podman"}).cleanupExtracted("/images/.rootfs-test")
	if len(r.calls) != 1 || strings.Join(r.calls[0], " ") != "podman unshare rm -rf -- /images/.rootfs-test" {
		t.Fatalf("%v", r.calls)
	}
}

type defaultRunner struct {
	calls     [][]string
	canonical bool
}

func (r *defaultRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "image exists ark/default:1") && r.canonical {
		return nil, nil
	}
	if strings.Contains(joined, "image exists") {
		return nil, errors.New("missing")
	}
	if strings.Contains(joined, "load") {
		return []byte("Loaded image: localhost/ark/default:0.1.0-amd64\n"), nil
	}
	if strings.Contains(joined, "inspect") {
		return []byte("sha256:immutable\n"), nil
	}
	return nil, nil
}
func (r *defaultRunner) RunIO(context.Context, io.Reader, string, ...string) error { return nil }
func TestImportDefaultTagsCanonicalReference(t *testing.T) {
	r := &defaultRunner{}
	s := &Store{runner: r, podman: "podman"}
	if err := s.ImportDefault(context.Background(), "archive"); err != nil {
		t.Fatal(err)
	}
	calls := strings.Join(func() []string {
		out := make([]string, len(r.calls))
		for i, call := range r.calls {
			out[i] = strings.Join(call, " ")
		}
		return out
	}(), "\n")
	if !strings.Contains(calls, "inspect --format {{.Id}} localhost/ark/default:0.1.0-amd64") || !strings.Contains(calls, "tag sha256:immutable ark/default:1") || !strings.Contains(calls, "inspect --format {{.Id}} ark/default:1") {
		t.Fatalf("canonical tag was not verified: %s", calls)
	}
	r.canonical = true
	before := len(r.calls)
	if err := s.ImportDefault(context.Background(), "archive"); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != before+1 {
		t.Fatal("existing canonical image was reloaded")
	}
}

func TestArchiveAllowsGuestLinksAndRejectsEscapes(t *testing.T) {
	makeArchive := func(headers ...*tar.Header) string {
		var data bytes.Buffer
		tw := tar.NewWriter(&data)
		for _, header := range headers {
			if err := tw.WriteHeader(header); err != nil {
				t.Fatal(err)
			}
			if header.Size > 0 {
				tw.Write(bytes.Repeat([]byte("x"), int(header.Size)))
			}
		}
		tw.Close()
		file := filepath.Join(t.TempDir(), "rootfs.tar")
		os.WriteFile(file, data.Bytes(), 0600)
		return file
	}
	valid := makeArchive(&tar.Header{Name: "usr/bin/tool", Typeflag: tar.TypeReg, Mode: 0755, Size: 1}, &tar.Header{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "usr/bin"}, &tar.Header{Name: "etc/mtab", Typeflag: tar.TypeSymlink, Linkname: "/proc/mounts"}, &tar.Header{Name: "usr/bin/tool-copy", Typeflag: tar.TypeLink, Linkname: "usr/bin/tool"})
	if err := validateArchive(valid, maxArchiveBytes); err != nil {
		t.Fatalf("valid links rejected: %v", err)
	}
	escape := makeArchive(&tar.Header{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "../../outside"})
	if err := validateArchive(escape, maxArchiveBytes); err == nil {
		t.Fatal("escape symlink accepted")
	}
	through := makeArchive(&tar.Header{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "usr/bin"}, &tar.Header{Name: "bin/tool", Typeflag: tar.TypeReg, Size: 1})
	if err := validateArchive(through, maxArchiveBytes); err == nil {
		t.Fatal("write through symlink accepted")
	}
}

func TestContextRejectsUnsafeArchive(t *testing.T) {
	db, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	tw := tar.NewWriter(&data)
	tw.WriteHeader(&tar.Header{Name: "../Dockerfile", Mode: 0644, Size: 1})
	tw.Write([]byte("x"))
	tw.Close()
	if _, err := s.context(context.Background(), bytes.NewReader(data.Bytes())); err == nil {
		t.Fatal("unsafe archive was accepted")
	}
}
