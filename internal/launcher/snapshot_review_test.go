package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSnapshotRecoveryDoesNotHoldLifecycleLock(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
			close(entered)
			<-release
			return nil, nil
		})
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	done := make(chan error, 1)
	go func() {
		disk, err := r.OpenSnapshot(t.Context(), spec.ID)
		if err == nil {
			err = disk.Close()
		}
		done <- err
	}()
	<-entered
	unlocked := make(chan struct{})
	go func() { r.mu.Lock(); r.mu.Unlock(); close(unlocked) }()
	select {
	case <-unlocked:
	case <-time.After(time.Second):
		close(release)
		<-done
		t.Fatal("recovery holds lifecycle lock")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotCapacityHeldUntilCloseAndWaitIsCancelable(t *testing.T) {
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) { return nil, nil })
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	disk, err := r.OpenSnapshot(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.OpenSnapshot(ctx, spec.ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("capacity wait: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(r.config.StateDir, ".snapshot-*"))
	if len(matches) != 1 {
		t.Fatalf("outstanding exports = %v", matches)
	}
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	// Repeated Close must not release another request's capacity.
	if err := disk.Close(); err != nil {
		t.Fatal(err)
	}
	next, err := r.OpenSnapshot(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}

func TestSnapshotCloseReportsRemovalFailureAndReleasesCapacity(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) { return nil, nil })
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	disk, err := r.OpenSnapshot(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	dir := disk.(*snapshotFile).dir
	t.Cleanup(func() { os.Chmod(dir, 0700) })
	if err := os.Chmod(dir, 0000); err != nil {
		t.Fatal(err)
	}
	if err := disk.Close(); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("missing removal failure: %v", err)
	}
	if len(r.snapshotCapacity) != 0 {
		t.Fatal("cleanup failure leaked capacity")
	}
}

func TestFailedSnapshotSurfacesCleanupFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	failure := errors.New("recovery failure")
	var dir string
	r := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(_ context.Context, _ string, args ...string) ([]byte, error) {
			dir = filepath.Dir(args[len(args)-1])
			if err := os.Chmod(dir, 0000); err != nil {
				return nil, err
			}
			return nil, failure
		})
	})
	spec := testSpec()
	seedVM(t, r, spec, false)
	t.Cleanup(func() {
		if dir != "" {
			os.Chmod(dir, 0700)
		}
	})
	_, err := r.OpenSnapshot(t.Context(), spec.ID)
	if !errors.Is(err, failure) || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("missing recovery or cleanup failure: %v", err)
	}
	if len(r.snapshotCapacity) != 0 {
		t.Fatal("failed snapshot leaked capacity")
	}
}

func TestStartupRemovesAbandonedSnapshots(t *testing.T) {
	r := testRuntime(t, nil)
	dir := filepath.Join(r.config.StateDir, ".snapshot-abandoned")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rootfs.ext4"), []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(r.config.StateDir, "keep")
	if err := os.WriteFile(keep, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFirecrackerRuntime(r.config); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("abandoned copy remains: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityRemovalChecksCommandErrorsAndPostcondition(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		before, after, commandErr error
		wantError                 bool
	}{
		{name: "removed", after: os.ErrNotExist},
		{name: "already absent", before: os.ErrNotExist},
		{name: "process error", commandErr: os.ErrPermission, wantError: true},
		{name: "zero exit command failure", wantError: true},
		{name: "lookup failure", before: os.ErrPermission, wantError: true},
		{name: "postcondition lookup failure", after: os.ErrPermission, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			commands := 0
			r := testRuntime(t, func(c *FirecrackerConfig) {
				c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
					commands++
					// Diagnostic wording is deliberately irrelevant.
					return []byte("localized diagnostic"), tc.commandErr
				})
			})
			lookups := 0
			r.guestStat = func(context.Context, string, string) (os.FileInfo, error) {
				lookups++
				if lookups == 1 {
					return nil, tc.before
				}
				return nil, tc.after
			}
			err := r.removeGuestIdentity(t.Context(), "disk", "/var/lib/dbus/machine-id")
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if tc.before != nil && commands != 0 {
				t.Fatal("command ran without a successful lookup")
			}
		})
	}
}
