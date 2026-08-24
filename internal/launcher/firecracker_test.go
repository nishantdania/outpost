package launcher

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nishantdania/outpost/internal/vmapi"
	"golang.org/x/crypto/ssh"
)

type commandCall struct {
	name string
	args []string
}

type runnerFunc func(context.Context, string, ...string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

type exitCodeError int

func (e exitCodeError) Error() string { return fmt.Sprintf("exit %d", e) }
func (e exitCodeError) ExitCode() int { return int(e) }

type fakeProcess struct {
	done     chan struct{}
	onSignal func(os.Signal)
	onWait   func()
	once     sync.Once
}

func newFakeProcess() *fakeProcess {
	return &fakeProcess{done: make(chan struct{})}
}

func (p *fakeProcess) Signal(signal os.Signal) error {
	if p.onSignal != nil {
		p.onSignal(signal)
	}
	return nil
}

func (p *fakeProcess) Wait() error {
	<-p.done
	if p.onWait != nil {
		p.onWait()
	}
	return nil
}

func (p *fakeProcess) exit() {
	p.once.Do(func() { close(p.done) })
}

type fakeLauncher struct {
	start func(string, []string, io.Reader, io.Writer, io.Writer) (LaunchProcess, error)
}

func (l fakeLauncher) Start(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (LaunchProcess, error) {
	return l.start(name, args, stdin, stdout, stderr)
}

type fakeProcesses struct {
	mu       sync.Mutex
	info     map[int]ProcessInfo
	signals  []os.Signal
	onSignal func(int, os.Signal)
}

func (p *fakeProcesses) Inspect(pid int, jailRoot, executable string) (ProcessInfo, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.info[pid], nil
}

func (p *fakeProcesses) Signal(pid int, signal os.Signal) error {
	p.mu.Lock()
	p.signals = append(p.signals, signal)
	onSignal := p.onSignal
	p.mu.Unlock()
	if onSignal != nil {
		onSignal(pid, signal)
	}
	return nil
}

func (p *fakeProcesses) set(pid int, info ProcessInfo) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.info[pid] = info
}

type networkRunner struct {
	mu          sync.Mutex
	calls       []commandCall
	table       bool
	tap         bool
	ready       bool
	failAt      int
	upCalls     int
	deleteTable error
	deleteTap   error
	events      *[]string
}

func (r *networkRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	call := commandCall{name: name, args: append([]string(nil), args...)}
	r.calls = append(r.calls, call)
	if r.events != nil {
		*r.events = append(*r.events, name+" "+strings.Join(args, " "))
	}
	if name == "ssh-keyscan" {
		if r.ready {
			return []byte("key"), nil
		}
		return nil, errors.New("not ready")
	}
	isUp := (name == "ip" && len(args) > 1 && (args[0] == "tuntap" || args[0] == "addr" || args[0] == "link" && args[len(args)-1] == "up")) || name == "nft" && len(args) > 0 && args[0] == "add"
	if isUp {
		current := r.upCalls
		r.upCalls++
		if current == r.failAt {
			return nil, fmt.Errorf("up failure %d", current)
		}
	}
	if name == "ip" && reflect.DeepEqual(args[:min(len(args), 2)], []string{"tuntap", "add"}) {
		r.tap = true
	}
	if name == "nft" && len(args) >= 2 && args[0] == "add" && args[1] == "table" {
		r.table = true
	}
	if name == "nft" && len(args) >= 2 && args[0] == "list" && args[1] == "table" {
		if !r.table {
			return nil, errors.New("not found")
		}
	}
	if name == "nft" && len(args) >= 2 && args[0] == "delete" && args[1] == "table" {
		if r.deleteTable != nil {
			return nil, r.deleteTable
		}
		r.table = false
	}
	if name == "ip" && len(args) >= 3 && args[0] == "link" && args[1] == "show" {
		if !r.tap {
			return nil, errors.New("cannot find device")
		}
	}
	if name == "ip" && len(args) >= 3 && args[0] == "link" && args[1] == "delete" {
		if r.deleteTap != nil {
			return nil, r.deleteTap
		}
		r.tap = false
	}
	return nil, nil
}

func testRuntime(t *testing.T, configure func(*FirecrackerConfig)) *FirecrackerRuntime {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"firecracker", "jailer", "kernel", "rootfs"} {
		mode := os.FileMode(0600)
		if name == "firecracker" || name == "jailer" {
			mode = 0700
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), mode); err != nil {
			t.Fatal(err)
		}
	}
	config := FirecrackerConfig{
		StateDir:      filepath.Join(dir, "state"),
		RuntimeDir:    filepath.Join(dir, "run"),
		JailerBase:    filepath.Join(dir, "jail"),
		Firecracker:   filepath.Join(dir, "firecracker"),
		Jailer:        filepath.Join(dir, "jailer"),
		Kernel:        filepath.Join(dir, "kernel"),
		DefaultRootFS: filepath.Join(dir, "rootfs"),
		Uplink:        "eth0",
		DNS:           "1.1.1.1",
		OutpostVMUID:  os.Getuid(),
		OutpostVMGID:  os.Getgid(),
		PIDTimeout:    30 * time.Millisecond,
		StopTimeout:   20 * time.Millisecond,
		PollInterval:  time.Millisecond,
		SSHTimeout:    20 * time.Millisecond,
	}
	if configure != nil {
		configure(&config)
	}
	runtime, err := NewFirecrackerRuntime(config)
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func testSpec() vmapi.VMSpec {
	return vmapi.VMSpec{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 1, SSHPublicKey: validPublicKey()}
}

func validPublicKey() string {
	key, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " test@outpost"
}

func createRunner(t *testing.T, calls *[]commandCall, e2fsckExit int, failName string) CommandRunner {
	t.Helper()
	return runnerFunc(func(ctx context.Context, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, commandCall{name: name, args: append([]string(nil), args...)})
		if name == failName {
			return nil, errors.New("injected failure")
		}
		switch name {
		case "cp":
			if err := os.WriteFile(args[len(args)-1], []byte("disk"), 0600); err != nil {
				t.Fatal(err)
			}
		case "ssh-keygen":
			for i, arg := range args {
				if arg == "-f" {
					if err := os.WriteFile(args[i+1], []byte("private"), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(args[i+1]+".pub", []byte("public"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		case "e2fsck":
			if e2fsckExit != 0 {
				return []byte("filesystem modified"), exitCodeError(e2fsckExit)
			}
		}
		return nil, nil
	})
}

func seedVM(t *testing.T, r *FirecrackerRuntime, spec vmapi.VMSpec, withAllocation bool) manifest {
	t.Helper()
	paths := r.vmPaths(spec.ID)
	if err := os.MkdirAll(paths.stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.stateDisk, []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	m := manifest{Spec: spec}
	if err := r.save(spec.ID, m); err != nil {
		t.Fatal(err)
	}
	if withAllocation {
		var err error
		m.Tap, m.Gateway, m.GuestIP, err = r.allocate(spec.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.save(spec.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func TestCustomRootFSValidationAndHeldCopy(t *testing.T) {
	r := testRuntime(t, nil)
	content := []byte("verified image bytes")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(content))
	path := filepath.Join(r.config.ImageStore, strings.TrimPrefix(digest, "sha256:"), "rootfs.ext4")
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0640); err != nil {
		t.Fatal(err)
	}
	file, err := r.rootfs(digest)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, []byte("replacement"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "disk")
	if err := copySparse(t.Context(), file, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(content) {
		t.Fatalf("held copy = %q, %v", got, err)
	}
	r.config.OutpostdUID = os.Getuid() + 1
	if rejected, err := r.rootfs(digest); err == nil {
		rejected.Close()
		t.Fatal("wrong owner accepted")
	}
	r.config.OutpostdUID = -1
	for _, test := range []struct {
		name   string
		change func()
	}{
		{"mode", func() { os.Chmod(path, 0644) }},
		{"hardlink", func() { os.Chmod(path, 0640); os.Link(path, path+".link") }},
		{"oversize", func() { os.Remove(path + ".link"); os.Truncate(path, r.config.MaxImageBytes+1) }},
		{"symlink", func() { os.Truncate(path, int64(len(content))); os.Remove(path); os.Symlink(target, path) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.change()
			if file, err := r.rootfs(digest); err == nil {
				file.Close()
				t.Fatal("invalid custom image accepted")
			}
		})
	}
}

func TestCreateAcceptsE2fsckExitOneAndNeverCreatesJail(t *testing.T) {
	var calls []commandCall
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = createRunner(t, &calls, 1, "")
	})
	spec := testSpec()
	if err := r.Create(t.Context(), spec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(r.vmPaths(spec.ID).jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Create made jail: %v", err)
	}
	found := false
	for _, call := range calls {
		if call.name == "e2fsck" {
			found = true
		}
	}
	if !found {
		t.Fatal("e2fsck was not run")
	}
}

func TestCreateCommandSequenceInodeMetadataAndRollback(t *testing.T) {
	t.Run("sequence and inode metadata", func(t *testing.T) {
		var calls []commandCall
		r := testRuntime(t, func(config *FirecrackerConfig) {
			config.Runner = createRunner(t, &calls, 0, "")
		})
		spec := testSpec()
		if err := r.Create(t.Context(), spec); err != nil {
			t.Fatal(err)
		}
		paths := r.vmPaths(spec.ID)
		wantPrefix := []string{"truncate", "e2fsck", "resize2fs"}
		for i, want := range wantPrefix {
			if calls[i].name != want {
				t.Fatalf("command %d = %s, want %s", i, calls[i].name, want)
			}
		}
		info, err := os.Stat(paths.stateDisk)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("rootfs mode = %o", info.Mode().Perm())
		}
		stat := info.Sys().(*syscall.Stat_t)
		if int(stat.Uid) != r.config.OutpostVMUID || int(stat.Gid) != r.config.OutpostVMGID {
			t.Fatalf("rootfs owner = %d:%d", stat.Uid, stat.Gid)
		}
		commands := make(map[string]bool)
		for _, call := range calls {
			if call.name == "debugfs" && len(call.args) >= 3 {
				commands[call.args[2]] = true
			}
		}
		pathsAndModes := map[string]string{
			"/etc/resolv.conf":                  "0100644",
			"/etc/ssh/ssh_host_rsa_key":         "0100600",
			"/etc/ssh/ssh_host_rsa_key.pub":     "0100644",
			"/etc/ssh/ssh_host_ecdsa_key":       "0100600",
			"/etc/ssh/ssh_host_ecdsa_key.pub":   "0100644",
			"/etc/ssh/ssh_host_ed25519_key":     "0100600",
			"/etc/ssh/ssh_host_ed25519_key.pub": "0100644",
			"/root/.ssh/authorized_keys":        "0100600",
		}
		for path, mode := range pathsAndModes {
			for _, command := range []string{"set_inode_field " + path + " mode " + mode, "set_inode_field " + path + " uid 0", "set_inode_field " + path + " gid 0"} {
				if !commands[command] {
					t.Errorf("missing %q", command)
				}
			}
		}
	})
	t.Run("rollback", func(t *testing.T) {
		var calls []commandCall
		r := testRuntime(t, func(config *FirecrackerConfig) {
			config.Runner = createRunner(t, &calls, 0, "truncate")
		})
		spec := testSpec()
		if err := r.Create(t.Context(), spec); err == nil {
			t.Fatal("Create succeeded")
		}
		if _, err := os.Stat(r.vmPaths(spec.ID).stateDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("state was not rolled back: %v", err)
		}
		if len(calls) != 1 || calls[0].name != "truncate" {
			t.Fatalf("calls = %#v", calls)
		}
	})
}

func TestJailPathsLayoutConfigAndExactCleanup(t *testing.T) {
	network := &networkRunner{failAt: -1, ready: true}
	processes := &fakeProcesses{info: make(map[int]ProcessInfo)}
	process := newFakeProcess()
	var launchedName string
	var launchedArgs []string
	var r *FirecrackerRuntime
	r = testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = network
		config.Processes = processes
		config.Launcher = fakeLauncher{start: func(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (LaunchProcess, error) {
			launchedName = name
			launchedArgs = append([]string(nil), args...)
			paths := r.vmPaths(args[1])
			if err := os.WriteFile(paths.jailPIDFile, []byte("321\n"), 0600); err != nil {
				t.Fatal(err)
			}
			processes.set(321, ProcessInfo{Exists: true, Verified: true, StartTime: "44"})
			return process, nil
		}}
	})
	spec := testSpec()
	m := seedVM(t, r, spec, true)
	ip, err := r.Start(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ip != m.GuestIP {
		t.Fatalf("IP = %s", ip)
	}
	paths := r.vmPaths(spec.ID)
	if paths.jailIDDir != filepath.Join(r.config.JailerBase, filepath.Base(r.config.Firecracker), spec.ID) || paths.jailRoot != filepath.Join(paths.jailIDDir, "root") {
		t.Fatalf("paths = %#v", paths)
	}
	for path, mode := range map[string]os.FileMode{paths.jailKernel: 0400, paths.jailDisk: 0600, paths.jailConfig: 0400} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
		if info.Mode().Perm() != mode {
			t.Fatalf("mode of %s = %o", path, info.Mode().Perm())
		}
	}
	if _, err := os.Stat(filepath.Join(paths.jailIDDir, "config.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config placed outside jail root: %v", err)
	}
	data, err := os.ReadFile(paths.jailConfig)
	if err != nil {
		t.Fatal(err)
	}
	var config firecrackerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.BootSource.KernelImagePath != "/vmlinux" || config.Drives[0].PathOnHost != "/rootfs.ext4" || config.NetworkInterfaces[0].HostDevice != m.Tap || config.MachineConfig.VCPUs != spec.VCPUs || !strings.Contains(config.BootSource.BootArgs, "ip="+m.GuestIP+"::"+m.Gateway) {
		t.Fatalf("config = %#v", config)
	}
	if launchedName != r.config.Jailer || !reflect.DeepEqual(launchedArgs, r.jailerArgs(spec.ID)) {
		t.Fatalf("launch = %s %#v", launchedName, launchedArgs)
	}
	processes.onSignal = func(pid int, signal os.Signal) {
		processes.set(pid, ProcessInfo{})
		process.exit()
	}
	if err := r.Stop(t.Context(), spec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail ID directory remains: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(paths.jailIDDir)); err != nil {
		t.Fatalf("executable jail directory was removed: %v", err)
	}
}

func TestNetworkUpUsesExactArguments(t *testing.T) {
	runner := &networkRunner{failAt: -1}
	r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
	m := manifest{Tap: "outpost0001", Gateway: "172.30.0.5", GuestIP: "172.30.0.6"}
	if err := r.networkUp(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	want := []commandCall{
		{name: "ip", args: []string{"tuntap", "add", "dev", "outpost0001", "mode", "tap", "user", fmt.Sprint(os.Getuid())}},
		{name: "ip", args: []string{"addr", "add", "172.30.0.5/30", "dev", "outpost0001"}},
		{name: "ip", args: []string{"link", "set", "dev", "outpost0001", "up"}},
		{name: "nft", args: []string{"add", "table", "inet", "outpost0001"}},
		{name: "nft", args: []string{"add", "chain", "inet", "outpost0001", "forward", "{", "type", "filter", "hook", "forward", "priority", "filter", ";", "policy", "accept", ";", "}"}},
		{name: "nft", args: []string{"add", "chain", "inet", "outpost0001", "postrouting", "{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}"}},
		{name: "nft", args: []string{"add", "rule", "inet", "outpost0001", "forward", "iifname", "outpost0001", "oifname", "eth0", "accept"}},
		{name: "nft", args: []string{"add", "rule", "inet", "outpost0001", "forward", "iifname", "eth0", "oifname", "outpost0001", "ct", "state", "established,related", "accept"}},
		{name: "nft", args: []string{"add", "rule", "inet", "outpost0001", "postrouting", "ip", "saddr", "172.30.0.6", "oifname", "eth0", "masquerade"}},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls =\n%#v\nwant\n%#v", runner.calls, want)
	}
}

func TestNetworkUpRollsBackEveryPartialFailure(t *testing.T) {
	m := manifest{Tap: "outpost0001", Gateway: "172.30.0.5", GuestIP: "172.30.0.6"}
	for failure := 0; failure < 9; failure++ {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			runner := &networkRunner{failAt: failure}
			r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
			if err := r.networkUp(t.Context(), m); err == nil {
				t.Fatal("networkUp succeeded")
			}
			var nftProbe, tapProbe bool
			for _, call := range runner.calls {
				nftProbe = nftProbe || call.name == "nft" && len(call.args) > 0 && call.args[0] == "list"
				tapProbe = tapProbe || call.name == "ip" && len(call.args) > 1 && call.args[0] == "link" && call.args[1] == "show"
			}
			if !nftProbe || !tapProbe {
				t.Fatalf("rollback probes missing: %#v", runner.calls)
			}
		})
	}
}

func TestNetworkDownNoopAbsenceAndDeletionFailure(t *testing.T) {
	m := manifest{Tap: "outpost0001", Gateway: "172.30.0.5", GuestIP: "172.30.0.6"}
	t.Run("empty no-op", func(t *testing.T) {
		runner := &networkRunner{failAt: -1}
		r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
		if err := r.networkDown(t.Context(), manifest{}); err != nil {
			t.Fatal(err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("absent probes", func(t *testing.T) {
		runner := &networkRunner{failAt: -1}
		r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
		if err := r.networkDown(t.Context(), m); err != nil {
			t.Fatal(err)
		}
		if len(runner.calls) != 2 || runner.calls[0].args[0] != "list" || runner.calls[1].args[1] != "show" {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
	t.Run("deletion failure", func(t *testing.T) {
		runner := &networkRunner{failAt: -1, table: true, tap: true, deleteTable: errors.New("permission denied")}
		r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
		if err := r.networkDown(t.Context(), m); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("error = %v", err)
		}
		if len(runner.calls) != 4 || runner.calls[3].args[1] != "delete" {
			t.Fatalf("calls = %#v", runner.calls)
		}
	})
}

func TestAllocationPersistsAndAvoidsHashCollision(t *testing.T) {
	r := testRuntime(t, nil)
	seen := make(map[string]string)
	var first, second string
	for i := 0; i < 100000 && second == ""; i++ {
		id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprint(i))).String()
		tap, _, _, err := r.allocate(id)
		if err != nil {
			t.Fatal(err)
		}
		if previous := seen[tap]; previous != "" {
			first, second = previous, id
		} else {
			seen[tap] = id
		}
	}
	if second == "" {
		t.Fatal("no collision found")
	}
	firstSpec := testSpec()
	firstSpec.ID = first
	firstManifest := seedVM(t, r, firstSpec, true)
	secondSpec := testSpec()
	secondSpec.ID = second
	secondManifest := seedVM(t, r, secondSpec, true)
	if firstManifest.GuestIP == secondManifest.GuestIP || firstManifest.Tap == secondManifest.Tap {
		t.Fatalf("collision was persisted: %#v %#v", firstManifest, secondManifest)
	}
	loaded, err := r.load(first)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Tap != firstManifest.Tap || loaded.GuestIP != firstManifest.GuestIP {
		t.Fatalf("allocation changed: %#v", loaded)
	}
}

func TestAllocationValidationRejectsMismatchedFields(t *testing.T) {
	valid := manifest{Tap: "outpost0001", Gateway: "172.30.0.5", GuestIP: "172.30.0.6"}
	if !allocationValid(valid) {
		t.Fatal("valid allocation rejected")
	}
	for _, m := range []manifest{
		{Tap: "outpost0002", Gateway: valid.Gateway, GuestIP: valid.GuestIP},
		{Tap: valid.Tap, Gateway: "172.30.0.1", GuestIP: valid.GuestIP},
		{Tap: valid.Tap, Gateway: valid.Gateway, GuestIP: "172.31.0.6"},
		{Tap: valid.Tap, Gateway: valid.Gateway, GuestIP: "172.30.0.7"},
	} {
		if allocationValid(m) {
			t.Fatalf("invalid allocation accepted: %#v", m)
		}
	}
}

func TestReconcileRejectsCorruptManifestWithoutCleanup(t *testing.T) {
	runner := &networkRunner{failAt: -1, table: true, tap: true}
	r := testRuntime(t, func(config *FirecrackerConfig) { config.Runner = runner })
	id := uuid.NewString()
	paths := r.vmPaths(id)
	if err := os.MkdirAll(paths.jailRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.stateDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.manifest, []byte(`{"spec":`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Reconcile(t.Context()); err == nil {
		t.Fatal("Reconcile succeeded")
	}
	if _, err := os.Stat(paths.jailRoot); err != nil {
		t.Fatalf("jail was removed: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("network changed: %#v", runner.calls)
	}
}

func TestReconcileCleansStaleDeadVM(t *testing.T) {
	runner := &networkRunner{failAt: -1, table: true, tap: true}
	processes := &fakeProcesses{info: make(map[int]ProcessInfo)}
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = processes
	})
	spec := testSpec()
	m := seedVM(t, r, spec, true)
	m.PID = 123
	m.StartTime = "9"
	if err := r.save(spec.ID, m); err != nil {
		t.Fatal(err)
	}
	paths := r.vmPaths(spec.ID)
	if err := os.MkdirAll(paths.jailRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.jailPIDFile, []byte("123"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Reconcile(t.Context()); err != nil {
		t.Fatal(err)
	}
	loaded, err := r.load(spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PID != 0 || loaded.StartTime != "" {
		t.Fatalf("stale identity remains: %#v", loaded)
	}
	if _, err := os.Stat(paths.jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail remains: %v", err)
	}
	if runner.table || runner.tap {
		t.Fatal("network remains")
	}
}

func TestLifecycleRefusesUnverifiedPIDAndPreservesResources(t *testing.T) {
	for _, operation := range []string{"start", "stop", "delete", "reconcile"} {
		t.Run(operation, func(t *testing.T) {
			runner := &networkRunner{failAt: -1, table: true, tap: true}
			processes := &fakeProcesses{info: map[int]ProcessInfo{777: {Exists: true, Verified: false, StartTime: "10"}}}
			r := testRuntime(t, func(config *FirecrackerConfig) {
				config.Runner = runner
				config.Processes = processes
			})
			spec := testSpec()
			m := seedVM(t, r, spec, true)
			m.PID = 777
			m.StartTime = "10"
			if err := r.save(spec.ID, m); err != nil {
				t.Fatal(err)
			}
			paths := r.vmPaths(spec.ID)
			if err := os.MkdirAll(paths.jailRoot, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(paths.jailPIDFile, []byte("777"), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "start":
				_, err = r.Start(t.Context(), spec.ID)
			case "stop":
				err = r.Stop(t.Context(), spec.ID)
			case "delete":
				err = r.Delete(t.Context(), spec.ID)
			case "reconcile":
				err = r.Reconcile(t.Context())
			}
			if err == nil || !strings.Contains(err.Error(), "unverified") {
				t.Fatalf("error = %v", err)
			}
			loaded, loadErr := r.load(spec.ID)
			if loadErr != nil || loaded.PID != 777 {
				t.Fatalf("manifest changed: %#v %v", loaded, loadErr)
			}
			if _, statErr := os.Stat(paths.jailRoot); statErr != nil {
				t.Fatalf("jail changed: %v", statErr)
			}
			if !runner.table || !runner.tap || len(runner.calls) != 0 {
				t.Fatalf("network changed: %#v", runner.calls)
			}
		})
	}
}

func TestStartLaunchFailureCleansJailAndNetwork(t *testing.T) {
	runner := &networkRunner{failAt: -1}
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = &fakeProcesses{info: make(map[int]ProcessInfo)}
		config.Launcher = fakeLauncher{start: func(string, []string, io.Reader, io.Writer, io.Writer) (LaunchProcess, error) {
			return nil, errors.New("start failed")
		}}
	})
	spec := testSpec()
	seedVM(t, r, spec, true)
	if _, err := r.Start(t.Context(), spec.ID); err == nil || !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(r.vmPaths(spec.ID).jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail remains: %v", err)
	}
	if runner.table || runner.tap {
		t.Fatal("network remains")
	}
}

func TestStartProcessFailureReapsBeforeCleanup(t *testing.T) {
	var events []string
	runner := &networkRunner{failAt: -1, events: &events}
	process := newFakeProcess()
	process.onWait = func() { events = append(events, "wait reaped") }
	process.exit()
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = &fakeProcesses{info: make(map[int]ProcessInfo)}
		config.Launcher = fakeLauncher{start: func(string, []string, io.Reader, io.Writer, io.Writer) (LaunchProcess, error) {
			events = append(events, "launch started")
			return process, nil
		}}
	})
	spec := testSpec()
	seedVM(t, r, spec, true)
	if _, err := r.Start(t.Context(), spec.ID); err == nil || !strings.Contains(err.Error(), "jailer exited") {
		t.Fatalf("error = %v", err)
	}
	waitIndex, cleanupIndex := -1, -1
	for i, event := range events {
		if event == "wait reaped" {
			waitIndex = i
		}
		if strings.HasPrefix(event, "nft list") && i > 2 {
			cleanupIndex = i
			break
		}
	}
	if waitIndex < 0 || cleanupIndex < 0 || waitIndex > cleanupIndex {
		t.Fatalf("events = %#v", events)
	}
	if _, err := os.Stat(r.vmPaths(spec.ID).jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail remains: %v", err)
	}
}

func TestStartCancellationExplicitlyTerminatesIndependentProcess(t *testing.T) {
	runner := &networkRunner{failAt: -1}
	process := newFakeProcess()
	var signals []os.Signal
	process.onSignal = func(signal os.Signal) {
		signals = append(signals, signal)
		process.exit()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = &fakeProcesses{info: make(map[int]ProcessInfo)}
		config.Launcher = fakeLauncher{start: func(string, []string, io.Reader, io.Writer, io.Writer) (LaunchProcess, error) {
			cancel()
			return process, nil
		}}
	})
	spec := testSpec()
	seedVM(t, r, spec, true)
	if _, err := r.Start(ctx, spec.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	if len(signals) != 1 || signals[0] != syscall.SIGTERM {
		t.Fatalf("signals = %#v", signals)
	}
	if _, err := os.Stat(r.vmPaths(spec.ID).jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail remains: %v", err)
	}
}

func TestStopUsesTermThenKillAndVerifiesExit(t *testing.T) {
	runner := &networkRunner{failAt: -1, table: true, tap: true}
	processes := &fakeProcesses{info: map[int]ProcessInfo{99: {Exists: true, Verified: true, StartTime: "8"}}}
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = processes
		config.StopTimeout = 3 * time.Millisecond
	})
	spec := testSpec()
	m := seedVM(t, r, spec, true)
	m.PID = 99
	m.StartTime = "8"
	if err := r.save(spec.ID, m); err != nil {
		t.Fatal(err)
	}
	paths := r.vmPaths(spec.ID)
	if err := os.MkdirAll(paths.jailRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.jailPIDFile, []byte("99"), 0600); err != nil {
		t.Fatal(err)
	}
	processes.onSignal = func(pid int, signal os.Signal) {
		if signal == syscall.SIGKILL {
			processes.set(pid, ProcessInfo{})
		}
	}
	if err := r.Stop(t.Context(), spec.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(processes.signals, []os.Signal{syscall.SIGTERM, syscall.SIGKILL}) {
		t.Fatalf("signals = %#v", processes.signals)
	}
	if _, err := os.Stat(paths.jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("jail remains: %v", err)
	}
}

func TestShutdownContinuesAfterUnsafeVM(t *testing.T) {
	runner := &networkRunner{failAt: -1}
	processes := &fakeProcesses{info: map[int]ProcessInfo{700: {Exists: true, Verified: false, StartTime: "1"}}}
	r := testRuntime(t, func(config *FirecrackerConfig) {
		config.Runner = runner
		config.Processes = processes
	})
	unsafeSpec := testSpec()
	unsafe := seedVM(t, r, unsafeSpec, true)
	unsafe.PID = 700
	unsafe.StartTime = "1"
	if err := r.save(unsafeSpec.ID, unsafe); err != nil {
		t.Fatal(err)
	}
	unsafePaths := r.vmPaths(unsafeSpec.ID)
	if err := os.MkdirAll(unsafePaths.jailRoot, 0700); err != nil {
		t.Fatal(err)
	}
	staleSpec := testSpec()
	seedVM(t, r, staleSpec, true)
	stalePaths := r.vmPaths(staleSpec.ID)
	if err := os.MkdirAll(stalePaths.jailRoot, 0700); err != nil {
		t.Fatal(err)
	}
	if err := r.Shutdown(t.Context()); err == nil {
		t.Fatal("Shutdown succeeded")
	}
	if _, err := os.Stat(unsafePaths.jailRoot); err != nil {
		t.Fatalf("unsafe jail removed: %v", err)
	}
	if _, err := os.Stat(stalePaths.jailIDDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale jail remains: %v", err)
	}
}

func TestOSProcessInspectTreatsProcDisappearanceAsMissing(t *testing.T) {
	jail := t.TempDir()
	executable := filepath.Join(jail, "firecracker")
	if err := os.WriteFile(executable, []byte("x"), 0700); err != nil {
		t.Fatal(err)
	}
	procRoot, procExe := "/proc/77/root", "/proc/77/exe"
	for _, missing := range []string{procRoot, procExe} {
		controller := OSProcessController{
			ReadFile: func(string) ([]byte, error) { return []byte("77 (firecracker) R " + strings.Repeat("0 ", 20)), nil },
			Stat: func(name string) (os.FileInfo, error) {
				if name == missing {
					return nil, os.ErrNotExist
				}
				if name == procRoot {
					return os.Stat(jail)
				}
				if name == procExe {
					return os.Stat(executable)
				}
				return os.Stat(name)
			},
		}
		info, err := controller.Inspect(77, jail, "firecracker")
		if err != nil || info.Exists || info.Verified {
			t.Fatalf("%s = %+v, %v", missing, info, err)
		}
	}
}
func TestOSProcessInspectKeepsPermissionAndReuseUnverified(t *testing.T) {
	jail := t.TempDir()
	executable := filepath.Join(jail, "firecracker")
	if err := os.WriteFile(executable, []byte("x"), 0700); err != nil {
		t.Fatal(err)
	}
	procRoot, procExe := "/proc/77/root", "/proc/77/exe"
	for _, permission := range []bool{true, false} {
		controller := OSProcessController{
			ReadFile: func(string) ([]byte, error) { return []byte("77 (firecracker) R " + strings.Repeat("0 ", 20)), nil },
			Stat: func(name string) (os.FileInfo, error) {
				if name == procRoot && permission {
					return nil, os.ErrPermission
				}
				if name == procRoot {
					return os.Stat(jail)
				}
				if name == procExe && !permission {
					return os.Stat(jail)
				}
				if name == procExe {
					return os.Stat(executable)
				}
				return os.Stat(name)
			},
		}
		info, err := controller.Inspect(77, jail, "firecracker")
		if err != nil || !info.Exists || info.Verified || info.StartTime == "" {
			t.Fatalf("permission %v = %+v, %v", permission, info, err)
		}
	}
}

func TestFirecrackerRuntimeAcceptsVersionedExecutableName(t *testing.T) {
	r := testRuntime(t, nil)
	path := filepath.Join(filepath.Dir(r.config.Firecracker), "firecracker-v1.16.1")
	if err := os.Rename(r.config.Firecracker, path); err != nil {
		t.Fatal(err)
	}
	config := r.config
	config.Firecracker = path
	if _, err := NewFirecrackerRuntime(config); err != nil {
		t.Fatal(err)
	}
}

func TestFirecrackerRuntimeRejectsUnsafeConfig(t *testing.T) {
	_, err := NewFirecrackerRuntime(FirecrackerConfig{})
	if !errors.Is(err, vmapi.ErrInvalid) {
		t.Fatalf("error = %v", err)
	}
}
