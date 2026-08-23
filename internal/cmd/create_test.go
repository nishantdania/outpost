package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/ark/internal/api"
)

func TestCreateCommand(t *testing.T) {
	t.Setenv("ARK_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/arks" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/arks")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}

		var request api.CreateArkRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if request.Name != "demo" {
			t.Errorf("name = %q, want %q", request.Name, "demo")
		}
		if request.Vcpus != 2 || request.MemoryMib != 4096 || request.DiskGib != 8 {
			t.Errorf("resources = %#v, want 2 CPU, 4096 MiB, 8 GiB", request)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"demo"}`))
	}))
	defer server.Close()

	root := newRootCmd()
	root.SetArgs([]string{"--server", server.URL, "--output", "json", "create", "--memory", "4G", "--disk", "8G", "demo"})

	var output bytes.Buffer
	root.SetOut(&output)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var created api.Ark
	if err := json.Unmarshal(output.Bytes(), &created); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if created.Id == "" || created.Name != "demo" {
		t.Fatalf("JSON output = %v, want created demo", created)
	}
}
