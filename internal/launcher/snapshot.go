package launcher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/nishantdania/outpost/internal/vmapi"
)

// OpenSnapshot captures a stopped disk while holding the lifecycle lock. The
// caller gets a private, recovered copy, never a handle to the mutable VM disk.
func (r *FirecrackerRuntime) OpenSnapshot(ctx context.Context, id string) (_ io.ReadCloser, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validID(id) {
		return nil, vmapi.ErrInvalid
	}
	// Bound private copies through recovery, streaming, and publication waits.
	// Wait without holding the lifecycle lock and honor request cancellation.
	select {
	case r.snapshotCapacity <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	success := false
	defer func() {
		if !success {
			<-r.snapshotCapacity
		}
	}()
	r.mu.Lock()
	locked := true
	defer func() {
		if locked {
			r.mu.Unlock()
		}
	}()
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
	defer func() {
		if !success {
			if err := os.RemoveAll(dir); err != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("remove failed snapshot export %s: %w", dir, err))
			}
		}
	}()
	target := filepath.Join(dir, "rootfs.ext4")
	if err := copySparse(ctx, source, target); err != nil {
		return nil, err
	}
	r.mu.Unlock()
	locked = false
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
	return &snapshotFile{File: file, dir: dir, release: func() { <-r.snapshotCapacity }}, nil
}

type snapshotFile struct {
	*os.File
	dir      string
	release  func()
	once     sync.Once
	closeErr error
}

func (f *snapshotFile) Close() error {
	f.once.Do(func() {
		f.closeErr = errors.Join(f.File.Close(), os.RemoveAll(f.dir))
		if f.release != nil {
			f.release()
		}
	})
	return f.closeErr
}
