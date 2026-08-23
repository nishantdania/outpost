package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/remote"
)

type recordingRunner struct {
	name string
	args []string
	io   remote.IO
}

func (r *recordingRunner) Run(_ context.Context, name string, args []string, streams remote.IO) error {
	r.name, r.args, r.io = name, args, streams
	return nil
}

func TestEndpoint(t *testing.T) {
	name, path, ok, err := endpoint("work:relative")
	if err != nil || !ok || name != "work" || path != "relative" {
		t.Fatalf("endpoint = %q %q %v %v", name, path, ok, err)
	}
	if _, _, _, err := endpoint("work:"); err == nil {
		t.Fatal("empty remote path accepted")
	}
	if _, _, _, err := endpoint(":/path"); err == nil {
		t.Fatal("empty endpoint accepted")
	}
	if _, _, _, err := endpoint("bad/name:/path"); err == nil {
		t.Fatal("malformed endpoint accepted")
	}
}

func TestTransferRejectsInvalidEndpoints(t *testing.T) {
	runner := &recordingRunner{}
	options := &rootOptions{runner: runner}
	for _, operands := range [][]string{{"", "work:/x"}, {"local", ""}, {"local", "other"}, {"work:/x", "other:/x"}, {":/x", "local"}, {"bad/name:/x", "local"}} {
		command := &cobra.Command{}
		if err := transfer(command, options, operands, false); err == nil {
			t.Fatalf("transfer accepted %#v", operands)
		}
		if runner.name != "" {
			t.Fatalf("runner invoked for %#v", operands)
		}
	}
}

func TestRemoteTransfers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"work","status":"running","guest_ip":"172.30.0.2"}`))
	}))
	defer server.Close()
	for _, test := range []struct {
		name    string
		args    []string
		sync    bool
		program string
		tail    []string
	}{
		{"upload", []string{"local dir/", "work:/remote dir/"}, false, "scp", []string{"--", "local dir/", "root@172.30.0.2:/remote dir/"}},
		{"download", []string{"work:/remote dir/", "local dir/"}, false, "scp", []string{"--", "root@172.30.0.2:/remote dir/", "local dir/"}},
		{"sync upload", []string{"local/", "work:remote/"}, true, "rsync", []string{"--", "local/", "root@172.30.0.2:remote/"}},
		{"sync download", []string{"work:remote/", "local/"}, true, "rsync", []string{"--", "root@172.30.0.2:remote/", "local/"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner, dir := &recordingRunner{}, t.TempDir()
			options := &rootOptions{serverURL: server.URL, ssh: remote.Config{User: "root", IdentityFile: dir + "/id", KnownHostsFile: dir + "/known"}, runner: runner}
			command := &cobra.Command{RunE: func(command *cobra.Command, _ []string) error {
				return transfer(command, options, test.args, test.sync)
			}}
			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if runner.name != test.program || !reflect.DeepEqual(runner.args[len(runner.args)-3:], test.tail) {
				t.Fatalf("call = %q %#v", runner.name, runner.args)
			}
			if !test.sync && !contains(runner.args, "-r") {
				t.Fatalf("copy was not recursive: %#v", runner.args)
			}
			if test.sync && !contains(runner.args, "-s") {
				t.Fatalf("sync lacks protected args: %#v", runner.args)
			}
		})
	}
}
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func TestRemoteExecWiresStreams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"work","status":"running","guest_ip":"172.30.0.2"}`))
	}))
	defer server.Close()
	runner := &recordingRunner{}
	dir := t.TempDir()
	options := &rootOptions{serverURL: server.URL, token: "token", ssh: remote.Config{User: "root", IdentityFile: dir + "/id", KnownHostsFile: dir + "/known"}, runner: runner}
	withoutDash := newExecCmd(options)
	withoutDash.SetArgs([]string{"work", "printf"})
	if err := withoutDash.Execute(); err == nil {
		t.Fatal("exec without -- succeeded")
	}
	command := newExecCmd(options)
	input, output, errors := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	command.SetIn(input)
	command.SetOut(output)
	command.SetErr(errors)
	command.SetArgs([]string{"work", "--", "printf", "a b"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.name != "ssh" || !reflect.DeepEqual(runner.args[len(runner.args)-1:], []string{"'printf' 'a b'"}) {
		t.Fatalf("call = %q %#v", runner.name, runner.args)
	}
	if runner.io.Stdin != input || runner.io.Stdout != output || runner.io.Stderr != errors {
		t.Fatal("streams were not preserved")
	}
}
