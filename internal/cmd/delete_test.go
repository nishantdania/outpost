package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
)

func TestDeleteCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/v1/outposts/demo" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/outposts/demo")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"demo"}`))
	}))
	defer server.Close()

	root := newRootCmd()
	root.SetArgs([]string{"--server", server.URL, "--output", "json", "delete", "demo"})

	var output bytes.Buffer
	root.SetOut(&output)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var deleted api.Outpost
	if err := json.Unmarshal(output.Bytes(), &deleted); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}
	if deleted.Id == "" || deleted.Name != "demo" {
		t.Fatalf("JSON output = %v, want deleted demo", deleted)
	}
}
