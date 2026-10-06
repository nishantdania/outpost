package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/nishantdania/outpost/internal/vmapi"
)

// OpenSnapshot captures a stopped disk while holding the lifecycle lock. The
// caller gets a private, recovered copy, never a handle to the mutable VM disk.
func (r *FirecrackerRuntime) OpenSnapshot(ctx context.Context, id string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, vmapi.ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load(id)
	if errors.Is(err, os.ErrNotExist) {
		return nil, vmapi.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	state, err := r.runtimeProcess(m)
	if err != nil {
		return nil, err
	}
	if state != processMissing {
		return nil, fmt.Errorf("snapshot requires a stopped VM: %w", vmapi.ErrConflict)
	}
	path := r.vmPaths(id).stateDisk
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	source := os.NewFile(uintptr(fd), path)
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(m.Spec.DiskGiB)<<30 {
		return nil, vmapi.ErrInvalid
	}
	dir, err := os.MkdirTemp(r.config.StateDir, ".snapshot-")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			os.RemoveAll(dir)
		}
	}()
	target := filepath.Join(dir, "rootfs.ext4")
	if err := copySparse(ctx, source, target); err != nil {
		return nil, err
	}
	// Stop currently terminates Firecracker rather than shutting down the guest.
	// Recover the COPY so that snapshotting never alters the baseline disk.
	if err := r.e2fsck(ctx, target); err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	success = true
	return &snapshotFile{File: file, dir: dir}, nil
}

type snapshotFile struct {
	*os.File
	dir string
}

func (f *snapshotFile) Close() error { return errors.Join(f.File.Close(), os.RemoveAll(f.dir)) }
