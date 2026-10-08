package launcher

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func credentialSpec(t *testing.T) vmapi.VMSpec {
	t.Helper()
	directory, state := t.TempDir(), t.TempDir()
	os.Chmod(directory, 0700)
	os.Chmod(state, 0700)
	host, err := credentials.NewHost(directory, state)
	if err != nil {
		t.Fatal(err)
	}
	spec := testSpec()
	spec.EgressCA = host.PublicCA
	spec.CredentialEnv = (credentials.Profile{Credentials: []credentials.Binding{{Env: "API_KEY", Secret: "key", Host: "api.example.com", Header: "Authorization", Encoding: "bearer"}}}).Environment()
	return spec
}

func TestManagedNetworkAtomicAndNoPermissiveFallback(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "nft failure"}[fail], func(t *testing.T) {
			var calls []commandCall
			var rules string
			r := testRuntime(t, func(c *FirecrackerConfig) {
				c.Runner = runnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
					calls = append(calls, commandCall{name: name, args: append([]string(nil), args...)})
					if name == "nft" && len(args) > 1 && args[0] == "-f" {
						data, err := os.ReadFile(args[1])
						if err != nil {
							t.Fatal(err)
						}
						rules = string(data)
						if fail {
							return nil, errors.New("synthetic nft failure")
						}
					}
					return nil, nil
				})
			})
			m := seedVM(t, r, credentialSpec(t), true)
			err := r.networkUp(t.Context(), m)
			if (err != nil) != fail {
				t.Fatalf("network result: %v", err)
			}
			for _, expected := range []string{"dnat ip to " + m.Gateway + ":18443", "meta nfproto ipv6 drop", "ip saddr != " + m.GuestIP + " drop", "iifname != \"" + m.Tap + "\" drop", "udp dport 53 accept", "iifname \"" + m.Tap + "\" drop"} {
				if !strings.Contains(rules, expected) {
					t.Fatalf("missing confinement rule %q", expected)
				}
			}
			for _, call := range calls {
				if call.name == "ufw" || call.name == "nft" && len(call.args) > 0 && call.args[0] == "add" {
					t.Fatalf("permissive network fallback: %v", call)
				}
			}
			if _, err := os.Stat(filepath.Join(r.vmPaths(m.Spec.ID).stateDir, "egress.nft")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary network script retained")
			}
		})
	}
}

func TestCredentialMaterialOnRealExt4(t *testing.T) {
	for _, command := range []string{"mkfs.ext4", "debugfs"} {
		if _, err := exec.LookPath(command); err != nil {
			t.Skip(command + " unavailable")
		}
	}
	r := testRuntime(t, nil)
	spec := credentialSpec(t)
	paths := r.vmPaths(spec.ID)
	os.MkdirAll(paths.stateDir, 0700)
	f, err := os.Create(paths.stateDisk)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(32 << 20); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := r.run(t.Context(), "mkfs.ext4", "-F", paths.stateDisk); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"/etc", "/etc/systemd", "/etc/systemd/system", "/usr", "/usr/local", "/usr/local/share"} {
		if _, err := r.run(t.Context(), "debugfs", "-w", "-R", "mkdir "+dir, paths.stateDisk); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := r.installCredentials(t.Context(), paths, spec); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"/etc/outpost-credentials.env", "/etc/profile.d/outpost-credentials.sh", "/usr/local/share/ca-certificates/outpost-egress.crt", "/etc/systemd/system/ssh.service.d/outpost-egress.conf"} {
		info, err := r.guestStat(t.Context(), paths.stateDisk, name)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("guest material %s: info=%+v err=%v", name, info, err)
		}
		// diskfs directory FileInfo omits permission bits; use debugfs only as test evidence.
		stat, err := r.run(t.Context(), "debugfs", "-R", "stat "+name, paths.stateDisk)
		if err != nil || !strings.Contains(string(stat), "0644") {
			t.Fatalf("guest file mode %s: %s (%v)", name, stat, err)
		}
	}
	output, err := r.run(t.Context(), "debugfs", "-R", "cat /etc/outpost-credentials.env", paths.stateDisk)
	if err != nil || !strings.Contains(string(output), spec.CredentialEnv) {
		t.Fatal("placeholder env not installed")
	}
	output, err = r.run(t.Context(), "debugfs", "-R", "cat /usr/local/share/ca-certificates/outpost-egress.crt", paths.stateDisk)
	if err != nil || !strings.Contains(string(output), spec.EgressCA) || strings.Contains(string(output), "PRIVATE KEY") {
		t.Fatal("guest did not receive only public CA")
	}
}
