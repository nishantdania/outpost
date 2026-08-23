package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateArk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/arks" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/arks")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"demo"}`))
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	created, err := client.CreateArk(context.Background(), "demo")
	if err != nil {
		t.Fatalf("CreateArk() error = %v", err)
	}

	if created.Id == "" || created.Name != "demo" {
		t.Fatalf("CreateArk() = %v, want created demo", created)
	}
}
