package launcher

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"

	backendfile "github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/ext4"
)

// statGuestFile uses structured directory entries, not localized CLI diagnostics.
// Walk one component at a time so absence has the stable fs.ErrNotExist identity.
// Do not follow symlinks in identity paths; unexpected layouts fail closed.
func statGuestFile(ctx context.Context, disk, name string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := os.Open(disk)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	filesystem, err := ext4.Read(backendfile.New(file, true), info.Size(), 0, 512)
	if err != nil {
		return nil, fmt.Errorf("read guest filesystem: %w", err)
	}
	parent := "."
	components := strings.Split(strings.TrimPrefix(path.Clean(name), "/"), "/")
	for i, component := range components {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := filesystem.ReadDir(parent)
		if err != nil {
			return nil, fmt.Errorf("read guest directory %s: %w", parent, err)
		}
		var found fs.DirEntry
		for _, entry := range entries {
			if entry.Name() == component {
				found = entry
				break
			}
		}
		if found == nil {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
		}
		if i == len(components)-1 {
			return found.Info()
		}
		if !found.IsDir() {
			return nil, fmt.Errorf("guest identity parent %s is not a directory", path.Join(parent, component))
		}
		parent = path.Join(parent, component)
	}
	return nil, fs.ErrInvalid
}

func (r *FirecrackerRuntime) removeGuestIdentity(ctx context.Context, disk, name string) error {
	_, err := r.guestStat(ctx, disk, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := r.run(ctx, "debugfs", "-w", "-R", "rm "+name, disk); err != nil {
		return err
	}
	// debugfs can return exit status zero for failed commands. Reopen the
	// filesystem to verify the mutation rather than interpreting its output.
	_, err = r.guestStat(ctx, disk, name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("guest identity %s remains after removal", name)
}
