package launcher

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nishantdania/outpost/internal/vmapi"
)

func TestSnapshotRecoversPrivateCopyAndKeepsBaseline(t *testing.T) {
	var baseline string
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(ctx context.Context, name string, args ...string) ([]byte, error) {
			if name != "e2fsck" {
				t.Fatalf("unexpected command %s", name)
			}
			target := args[len(args)-1]
			if target == baseline {
				t.Fatal("recovery mutated baseline")
			}
			return nil, os.WriteFile(target, []byte("recovered"), 0600)
		})
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	baseline = r.vmPaths(spec.ID).stateDisk
	disk, err := r.OpenSnapshot(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Source deletion after capture must not invalidate the snapshot handle.
	if err := os.Remove(baseline); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(disk)
	if err != nil || string(data) != "recovered" {
		t.Fatalf("snapshot = %q, %v", data, err)
	}
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(r.config.StateDir, ".snapshot-*"))
	if len(matches) != 0 {
		t.Fatalf("temporary snapshots remain: %v", matches)
	}
}

func TestSnapshotRefusesLiveAndUnverifiedVMs(t *testing.T) {
	for _, verified := range []bool{true, false} {
		t.Run(map[bool]string{true: "verified", false: "unverified"}[verified], func(t *testing.T) {
			processes := &fakeProcesses{info: map[int]ProcessInfo{123: {Exists: true, Verified: verified, StartTime: "10"}}}
			r := testRuntime(t, func(c *FirecrackerConfig) { c.Processes = processes })
			spec := testSpec()
			m := seedVM(t, r, spec, true)
			m.PID, m.StartTime = 123, "10"
			if err := r.save(spec.ID, m); err != nil {
				t.Fatal(err)
			}
			if _, err := r.OpenSnapshot(t.Context(), spec.ID); !errors.Is(err, vmapi.ErrConflict) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSnapshotRejectsSymlinkDiskAndCleansFailedRecovery(t *testing.T) {
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("broken filesystem") })
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	if _, err := r.OpenSnapshot(t.Context(), spec.ID); err == nil {
		t.Fatal("accepted failed recovery")
	}
	matches, _ := filepath.Glob(filepath.Join(r.config.StateDir, ".snapshot-*"))
	if len(matches) != 0 {
		t.Fatal("failed snapshot leaked")
	}
	path := r.vmPaths(spec.ID).stateDisk
	os.Remove(path)
	if err := os.Symlink(r.config.DefaultRootFS, path); err != nil {
		t.Fatal(err)
	}
	if _, err := r.OpenSnapshot(t.Context(), spec.ID); err == nil {
		t.Fatal("accepted symlink disk")
	}
}
