package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Model structured lookups for command-sequence tests using a non-ext4 fixture.
func fakeGuestStat(t *testing.T, calls *[]commandCall) func(context.Context, string, string) (os.FileInfo, error) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0600); err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, _ string, name string) (os.FileInfo, error) {
		for i := len(*calls) - 1; i >= 0; i-- {
			call := (*calls)[i]
			if call.name == "debugfs" && len(call.args) > 2 && strings.HasPrefix(call.args[2], "write ") && strings.HasSuffix(call.args[2], " "+name) {
				return os.Stat(empty)
			}
		}
		return nil, os.ErrNotExist
	}
}

func TestIdentityResetOnRealExt4(t *testing.T) {
	for _, command := range []string{"mkfs.ext4", "debugfs", "ssh-keygen"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("%s unavailable", command)
		}
	}
	r := testRuntime(t, nil)
	spec := testSpec()
	paths := r.vmPaths(spec.ID)
	if err := os.MkdirAll(paths.stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	disk, err := os.Create(paths.stateDisk)
	if err != nil {
		t.Fatal(err)
	}
	if err := disk.Truncate(32 << 20); err != nil {
		t.Fatal(err)
	}
	disk.Close()
	if _, err := r.run(t.Context(), "mkfs.ext4", "-F", paths.stateDisk); err != nil {
		t.Fatal(err)
	}
	inherited := filepath.Join(paths.stateDir, "inherited")
	if err := os.WriteFile(inherited, []byte("inherited identity"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"mkdir /etc", "mkdir /var", "mkdir /var/lib", "mkdir /var/lib/dbus", "mkdir /var/lib/systemd", "mkdir /root", "mkdir /root/.ssh", "write " + inherited + " /etc/machine-id", "write " + inherited + " /var/lib/dbus/machine-id", "write " + inherited + " /var/lib/systemd/random-seed", "write " + inherited + " /root/.ssh/authorized_keys"} {
		if _, err := r.run(t.Context(), "debugfs", "-w", "-R", command, paths.stateDisk); err != nil {
			t.Fatal(err)
		}
	}
	// A zero-exit debugfs mutation that does nothing must still fail.
	r.config.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) {
		return []byte("arbitrary localized failure"), nil
	})
	if err := r.removeGuestIdentity(t.Context(), paths.stateDisk, "/var/lib/dbus/machine-id"); err == nil {
		t.Fatal("accepted a failed removal with exit status zero")
	}
	r.config.Runner = OSRunner{}
	if err := r.installGuestFiles(t.Context(), paths, spec); err != nil {
		t.Fatal(err)
	}
	// Lookup is idempotent when files or their parent directories are absent.
	for _, name := range []string{"/var/lib/dbus/machine-id", "/var/lib/systemd/random-seed", "/missing/parent/identity"} {
		if _, err := statGuestFile(t.Context(), paths.stateDisk, name); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("lookup %s: %v", name, err)
		}
		if err := r.removeGuestIdentity(t.Context(), paths.stateDisk, name); err != nil {
			t.Fatal(err)
		}
	}
}
