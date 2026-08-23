package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/nishantdania/ark/internal/api"
)

func TestNewRejectsInvalidServerURL(t *testing.T) {
	for _, serverURL := range []string{"", "127.0.0.1:17890"} {
		t.Run(serverURL, func(t *testing.T) {
			_, err := New(serverURL)
			if err == nil {
				t.Fatal("New() error = nil, want an error")
			}
		})
	}
}

func TestListArks(t *testing.T) {
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

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	arks, err := client.ListArks(context.Background())
	if err != nil {
		t.Fatalf("ListArks() error = %v", err)
	}

	want := []api.Ark{{
		Id:     "ark_123",
		Name:   "investigate-deploy",
		Status: "running",
	}}
	if !reflect.DeepEqual(arks, want) {
		t.Fatalf("ListArks() = %v, want %v", arks, want)
	}
}

func TestListArksReturnsErrorForUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListArks(context.Background())
	if err == nil {
		t.Fatal("ListArks() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Fatalf("ListArks() error = %q, want status", err)
	}
}

func TestListArksReturnsErrorForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListArks(context.Background())
	if err == nil {
		t.Fatal("ListArks() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("ListArks() error = %q, want decode error", err)
	}
}
