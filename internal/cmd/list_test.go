package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/ark/internal/api"
)

func TestListCommandWritesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/arks" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/arks")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"ark_123","name":"investigate-deploy","status":"running"}]`))
	}))
	defer server.Close()

	root := newRootCmd()
	root.SetArgs([]string{"--server", server.URL, "--output", "json", "list"})

	var output bytes.Buffer
	root.SetOut(&output)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var arks []api.Ark
	if err := json.Unmarshal(output.Bytes(), &arks); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	if len(arks) != 1 || arks[0].Id != "ark_123" {
		t.Fatalf("JSON output = %v, want ark_123", arks)
	}
}
