// Package artifact owns content-addressed disk bytes, not tags or VM state.
package artifact

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

var digestPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type Store struct {
	root string
	mu   sync.Mutex
}

func New(root string) (*Store, error) {
	if !filepath.IsAbs(root) {
		return nil, errors.New("artifact root must be absolute")
	}
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}
func (s *Store) path(digest string) string {
	return filepath.Join(s.root, strings.TrimPrefix(digest, "sha256:"), "rootfs.ext4")
}
func (s *Store) Remove(digest string) error {
	if !digestPattern.MatchString(digest) {
		return errors.New("invalid artifact digest")
	}
	return os.RemoveAll(filepath.Dir(s.path(digest)))
}

// Reconcile removes abandoned publications and unregistered disks at startup.
// The caller must ensure there are no active publishers.
func (s *Store) Reconcile(known map[string]bool) error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".publish-") || (digestPattern.MatchString("sha256:"+name) && !known["sha256:"+name]) {
			if err := os.RemoveAll(filepath.Join(s.root, name)); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) Publish(ctx context.Context, input io.Reader) (string, int64, error) {
	temp, err := os.CreateTemp(s.root, ".publish-")
	if err != nil {
		return "", 0, err
	}
	name := temp.Name()
	defer os.Remove(name)
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(sparseWriter{temp}, hash), contextReader{ctx, input})
	if err == nil {
		err = temp.Truncate(size)
	}
	if err == nil {
		err = temp.Chmod(0640)
	}
	if err == nil {
		err = temp.Sync()
	}
	if err != nil {
		temp.Close()
		return "", 0, err
	}
	digest := fmt.Sprintf("sha256:%x", hash.Sum(nil))
	dir := filepath.Dir(s.path(digest))
	if err := os.MkdirAll(dir, 0750); err != nil {
		temp.Close()
		return "", 0, err
	}
	if err := syncDir(s.root); err != nil {
		temp.Close()
		return "", 0, err
	}
	target := s.path(digest)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Link(name, target); err != nil {
		if !errors.Is(err, os.ErrExist) {
			temp.Close()
			return "", 0, err
		}
		if err := temp.Close(); err != nil {
			return "", 0, err
		}
		if err := verifyPublished(target, digest, size); err != nil {
			return "", 0, fmt.Errorf("existing image artifact is invalid: %w", err)
		}
		return digest, size, nil
	}
	if err := temp.Close(); err != nil {
		return "", 0, err
	}
	if err := syncDir(dir); err != nil {
		return "", 0, err
	}
	if err := os.Remove(name); err != nil {
		return "", 0, err
	}
	if err := syncDir(s.root); err != nil {
		return "", 0, err
	}
	return digest, size, nil
}
func verifyPublished(path, digest string, size int64) error {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0640 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) || info.Size() != size {
		return errors.New("unsafe artifact")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	if fmt.Sprintf("sha256:%x", hash.Sum(nil)) != digest {
		return errors.New("digest mismatch")
	}
	return nil
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Keep zero-filled regions as holes while retaining the logical byte digest.
// This also avoids allocating the unused portion of a large guest disk.
type sparseWriter struct{ file *os.File }

func (w sparseWriter) Write(p []byte) (int, error) {
	if bytes.Count(p, []byte{0}) == len(p) {
		_, err := w.file.Seek(int64(len(p)), io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return w.file.Write(p)
}
