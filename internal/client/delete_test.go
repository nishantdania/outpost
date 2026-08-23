package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeleteArk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/v1/arks/demo" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/arks/demo")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"demo"}`))
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	deleted, err := client.DeleteArk(context.Background(), "demo")
	if err != nil {
		t.Fatalf("DeleteArk() error = %v", err)
	}
	if deleted.Id == "" || deleted.Name != "demo" {
		t.Fatalf("DeleteArk() = %v, want deleted demo", deleted)
	}
}
