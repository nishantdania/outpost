package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/remote"
)

func TestForkPinsSnapshotDigestAndSizesDisk(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/images/ready":
			json.NewEncoder(w).Encode(api.Image{Digest: digest, SizeBytes: 32 << 30, Tags: []string{"ready"}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/outposts":
			creates++
			var input api.CreateOutpostRequest
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input.ImageId != digest || input.DiskGib != 32 || input.Vcpus != 4 || input.MemoryMib != 8192 {
				t.Errorf("input = %+v", input)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(api.Outpost{Name: input.Name, ImageId: input.ImageId, DiskGib: input.DiskGib})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	dir := t.TempDir()
	os.WriteFile(dir+"/id", []byte("test-private-key"), 0600)
	os.WriteFile(dir+"/id.pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE6h6qf1VbwtwXQs48PeBo5oX5o2r2mR5rZJ6oTQxrTD test"), 0600)
	options := &rootOptions{serverURL: server.URL, output: "json", ssh: remote.Config{User: "root", IdentityFile: dir + "/id", KnownHostsFile: dir + "/known"}, runner: &recordingRunner{}}
	command := newForkCmd(options)
	command.SetArgs([]string{"ready", "--name", "feature", "--cpus", "4", "--memory", "8G"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatal("VM not created")
	}
	var result api.Outpost
	if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.ImageId != digest {
		t.Fatalf("result = %s, %v", output.String(), err)
	}
	command = newForkCmd(options)
	command.SetArgs([]string{"ready", "--name", "too-small", "--disk", "8G"})
	command.SetOut(&bytes.Buffer{})
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "smaller") {
		t.Fatalf("error = %v", err)
	}
	if creates != 1 {
		t.Fatal("created undersized fork")
	}
}

func TestForwardBindsLoopbackAndDisablesAgentForwarding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"feature","status":"running","guest_ip":"172.30.0.2"}`))
	}))
	defer server.Close()
	runner := &recordingRunner{}
	dir := t.TempDir()
	options := &rootOptions{serverURL: server.URL, ssh: remote.Config{User: "root", IdentityFile: dir + "/id", KnownHostsFile: dir + "/known", ProxyJump: "user@server", AgentForwarding: true}, runner: runner}
	command := newForwardCmd(options)
	command.SetArgs([]string{"feature", "3000:8080"})
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"-N", "-L", "127.0.0.1:3000:127.0.0.1:8080", "ExitOnForwardFailure=yes", "user@server", "root@172.30.0.2"} {
		if !contains(runner.args, value) {
			t.Fatalf("missing %q: %v", value, runner.args)
		}
	}
	if contains(runner.args, "-A") {
		t.Fatal("forwarded host SSH agent")
	}
}
