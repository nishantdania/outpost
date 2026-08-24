package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOutpost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/outposts" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/outposts")
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"550e8400-e29b-41d4-a716-446655440000","name":"demo"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "test-token")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	created, err := client.CreateOutpost(context.Background(), "demo")
	if err != nil {
		t.Fatalf("CreateOutpost() error = %v", err)
	}

	if created.Id == "" || created.Name != "demo" {
		t.Fatalf("CreateOutpost() = %v, want created demo", created)
	}
}
