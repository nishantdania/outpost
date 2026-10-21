package launcher

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nishantdania/outpost/internal/outpost"
)

// checkRegistry refuses to start a fresh database over existing untracked VM
// resources. There is deliberately no legacy JSON import or silent adoption.
// The only unregistered disk directory allowed is an interrupted initial copy
// for a VM already in SQLite, with no jail or old runtime manifest.
func (r *FirecrackerRuntime) checkRegistry(ctx context.Context, records []runtimeRecord) error {
	known := make(map[string]bool, len(records))
	for _, record := range records {
		known[record.Spec.ID] = true
	}
	jails, err := os.ReadDir(filepath.Join(r.config.JailerBase, filepath.Base(r.config.Firecracker)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range jails {
		if !entry.IsDir() || !known[entry.Name()] {
			return fmt.Errorf("untracked VM jail %q; refusing startup without its SQLite runtime record", entry.Name())
		}
	}
	disks, err := os.ReadDir(filepath.Join(r.config.StateDir, "vms"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range disks {
		id := entry.Name()
		if !entry.IsDir() || !validID(id) {
			return fmt.Errorf("invalid VM state entry %q", id)
		}
		if known[id] {
			continue
		}
		vm, err := r.config.Store.GetByID(ctx, id)
		if err != nil || (vm.Status != outpost.StatusProvisioning && vm.Status != outpost.StatusFailed) {
			return fmt.Errorf("untracked VM disk %q; refusing startup without its SQLite runtime record", id)
		}
		if _, err := os.Lstat(filepath.Join(r.vmPaths(id).stateDir, "runtime.json")); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("legacy or unreadable runtime manifest for %s; automatic conversion is not supported", id)
		}
	}
	return nil
}
