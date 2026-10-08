package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/credentials"
)

func TestSnapshotCredentialProfileFlag(t *testing.T) {
	profile := credentials.Profile{Credentials: []credentials.Binding{{Env: "API_KEY", Secret: "named-host-key", Host: "api.example.com", Header: "X-Api-Key", Encoding: "raw"}}}
	file := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(file, []byte(profile.JSON()), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var includeProfile atomic.Bool
	includeProfile.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("missing JSON content type")
		}
		var body credentials.Profile
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.JSON() != profile.JSON() {
			t.Error("profile not sent")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		var echo *credentials.Profile
		if includeProfile.Load() {
			echo = &profile
		}
		json.NewEncoder(w).Encode(api.Image{Digest: "sha256:" + strings.Repeat("a", 64), Tags: []string{"ready"}, SizeBytes: 1, CredentialProfile: echo})
	}))
	defer server.Close()
	run := func() error {
		command := newSnapshotCmd(&rootOptions{serverURL: server.URL, token: "test"})
		command.SetArgs([]string{"create", "dev", "--name", "ready", "--credentials", file})
		command.SetOut(&bytes.Buffer{})
		command.SetErr(&bytes.Buffer{})
		return command.Execute()
	}
	if err := run(); err != nil {
		t.Fatal(err)
	}
	includeProfile.Store(false)
	if err := run(); err == nil || !strings.Contains(err.Error(), "did not persist") {
		t.Fatal("older server silently ignored requested credentials")
	}
	os.WriteFile(file, []byte(`{"credentials":[],"value":"invalid-data"}`), 0600)
	if err := run(); err == nil || strings.Contains(err.Error(), "invalid-data") {
		t.Fatal("invalid profile was accepted or echoed")
	}
	if calls.Load() != 2 {
		t.Fatal("invalid profile reached server")
	}
}
