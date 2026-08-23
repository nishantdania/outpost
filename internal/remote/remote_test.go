package remote

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

type call struct {
	name string
	args []string
}
type fakeRunner struct {
	calls []call
	err   error
}

func (f *fakeRunner) Run(_ context.Context, name string, args []string, streams IO) error {
	f.calls = append(f.calls, call{name, args})
	if streams.Stdout != nil {
		_, _ = streams.Stdout.Write([]byte("ssh-ed25519 AAAA test\n"))
	}
	return f.err
}

func TestSSHArgs(t *testing.T) {
	config := Config{User: "root", ProxyJump: "me@host", IdentityFile: "/key", KnownHostsFile: "/known", AgentForwarding: true}
	got, err := config.SSHArgs("172.30.0.2", []string{"echo", "a b"}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=/known", "-o", "IdentitiesOnly=yes", "-i", "/key", "-J", "me@host", "-A", "root@172.30.0.2", "'echo' 'a b'"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}
func TestSSHArgsRejectsNewline(t *testing.T) {
	config := Config{User: "root", IdentityFile: "/key", KnownHostsFile: "/known"}
	if _, err := config.SSHArgs("172.30.0.2", []string{"echo\nbad"}, false); err == nil {
		t.Fatal("newline command accepted")
	}
}
func TestQuote(t *testing.T) {
	got := Quote([]string{"a b", "x'y", "", "$(bad); rm -rf /"})
	want := "'a b' 'x'\"'\"'y' '' '$(bad); rm -rf /'"
	if got != want {
		t.Fatalf("Quote() = %q, want %q", got, want)
	}
}
func TestTransferArgs(t *testing.T) {
	config := Config{User: "root", IdentityFile: "/key", KnownHostsFile: "/known"}
	got, err := config.SCPArgs("172.30.0.2", "local", "/remote", true, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-o", "StrictHostKeyChecking=accept-new", "-o", "UserKnownHostsFile=/known", "-o", "IdentitiesOnly=yes", "-i", "/key", "-r", "--", "local", "root@172.30.0.2:/remote"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scp = %#v, want %#v", got, want)
	}
	rsync, err := config.RsyncArgs("172.30.0.2", "local/", "dest/", false)
	if err != nil {
		t.Fatal(err)
	}
	if rsync[len(rsync)-2] != "root@172.30.0.2:dest/" || rsync[len(rsync)-1] != "local/" {
		t.Fatalf("rsync = %#v", rsync)
	}
	if rsync[3] != "'ssh' '-o' 'StrictHostKeyChecking=accept-new' '-o' 'UserKnownHostsFile=/known' '-o' 'IdentitiesOnly=yes' '-i' '/key'" || rsync[4] != "--" {
		t.Fatalf("transport = %#v", rsync)
	}
}
func TestPrepareAndIdentity(t *testing.T) {
	dir := t.TempDir()
	config := Config{User: "root", IdentityFile: filepath.Join(dir, "keys", "id"), KnownHostsFile: filepath.Join(dir, "known_hosts")}
	if err := config.Prepare(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(config.KnownHostsFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("known_hosts mode = %o", info.Mode().Perm())
	}
	if err := os.WriteFile(config.IdentityFile, []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	if _, err := config.EnsureIdentity(context.Background(), runner); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 || !reflect.DeepEqual(runner.calls[0].args, []string{"-y", "-f", config.IdentityFile}) {
		t.Fatalf("calls = %#v", runner.calls)
	}
	info, err = os.Stat(config.IdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("identity mode = %o", info.Mode().Perm())
	}
}
func TestConfigRejectsMalformedValues(t *testing.T) {
	config := Config{User: "bad user", IdentityFile: "key", KnownHostsFile: "known"}
	if err := config.Validate(); err == nil {
		t.Fatal("Validate unexpectedly succeeded")
	}
	for _, value := range []string{"host\nother", "-host", "user@-host", "a..b", "-a.example", "a-.example", "host:0", "host:65536"} {
		config = Config{User: "root", IdentityFile: "/key", KnownHostsFile: "/known", ProxyJump: value}
		if err := config.Validate(); err == nil {
			t.Fatalf("Validate accepted %q", value)
		}
	}
}
func TestProxyJumpAcceptsValidHosts(t *testing.T) {
	for _, value := range []string{"host.example", "user@host.example:22", "192.0.2.1:2222", "user@[2001:db8::1]:22"} {
		if err := validateJump(value); err != nil {
			t.Fatalf("validateJump(%q): %v", value, err)
		}
	}
}
func TestRunnerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := SystemRunner().Run(ctx, "sh", []string{"-c", "exit 0"}, IO{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
}
func TestRunnerMidFlightCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- SystemRunner().Run(ctx, "sh", []string{"-c", "sleep 10"}, IO{}) }()
	cancel()
	if err := <-done; err == nil {
		t.Fatal("runner succeeded after cancellation")
	}
}
