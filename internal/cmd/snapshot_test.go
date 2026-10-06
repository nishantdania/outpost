package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
)

func TestSnapshotCreate(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/outposts/dev/snapshot" || r.URL.Query().Get("tag") != "dev-ready" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing authentication")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.Image{Digest: digest, Tags: []string{"dev-ready"}, SizeBytes: 8 << 30})
	}))
	defer server.Close()
	command := newSnapshotCmd(&rootOptions{serverURL: server.URL, token: "secret"})
	command.SetArgs([]string{"create", "dev", "--name", "dev-ready"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if output.String() != digest+"\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestSnapshotCreateValidation(t *testing.T) {
	for _, args := range [][]string{{"create"}, {"create", "dev"}, {"create", "dev", "extra", "--name", "ready"}} {
		command := newSnapshotCmd(&rootOptions{})
		command.SetArgs(args)
		command.SetOut(&bytes.Buffer{})
		command.SetErr(&bytes.Buffer{})
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestSnapshotCreateReportsConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"snapshot requires a stopped VM"}`))
	}))
	defer server.Close()
	command := newSnapshotCmd(&rootOptions{serverURL: server.URL})
	command.SilenceUsage = true
	command.SetArgs([]string{"create", "dev", "--name", "ready"})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&bytes.Buffer{})
	if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "stopped VM") {
		t.Fatalf("error = %v", err)
	}
	if output.Len() != 0 {
		t.Fatalf("unexpected output: %s", output.String())
	}
}
