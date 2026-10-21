package artifact

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// Open reads a content-addressed disk without creating or changing the store.
func Open(root, digest string) (*os.File, error) {
	return (&Store{root: root}).Open(digest)
}

func (s *Store) Open(digest string) (*os.File, error) {
	if !digestPattern.MatchString(digest) {
		return nil, errors.New("invalid artifact digest")
	}
	file, err := OpenFile(s.path(digest))
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err == nil && fmt.Sprintf("sha256:%x", hash.Sum(nil)) != digest {
		err = errors.New("digest mismatch")
	}
	if err == nil {
		_, err = file.Seek(0, io.SeekStart)
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (s *Store) Verify(digest string) error {
	file, err := s.Open(digest)
	if err != nil {
		return err
	}
	return file.Close()
}

// OpenFile requires a regular disk owned by this process, without unsafe links
// or permissions. Content-addressed disks must use Open for digest verification.
func OpenFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm()&0027 != 0 || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) || stat.Gid != uint32(os.Getegid()) {
		file.Close()
		return nil, errors.New("unsafe artifact")
	}
	return file, nil
}
