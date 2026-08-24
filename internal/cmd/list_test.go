package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
)

func TestListCommandWritesJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}
		if r.URL.Path != "/v1/outposts" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/outposts")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"outpost_123","name":"investigate-deploy"}]`))
	}))
	defer server.Close()

	root := newRootCmd()
	root.SetArgs([]string{"--server", server.URL, "--output", "json", "list"})

	var output bytes.Buffer
	root.SetOut(&output)

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var outposts []api.Outpost
	if err := json.Unmarshal(output.Bytes(), &outposts); err != nil {
		t.Fatalf("decode JSON output: %v", err)
	}

	if len(outposts) != 1 || outposts[0].Id != "outpost_123" {
		t.Fatalf("JSON output = %v, want outpost_123", outposts)
	}
}
