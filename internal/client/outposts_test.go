package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
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

func TestListOutposts(t *testing.T) {
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

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	outposts, err := client.ListOutposts(context.Background())
	if err != nil {
		t.Fatalf("ListOutposts() error = %v", err)
	}

	want := []api.Outpost{{
		Id:   "outpost_123",
		Name: "investigate-deploy",
	}}
	if !reflect.DeepEqual(outposts, want) {
		t.Fatalf("ListOutposts() = %v, want %v", outposts, want)
	}
}

func TestListOutpostsReturnsErrorForUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListOutposts(context.Background())
	if err == nil {
		t.Fatal("ListOutposts() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "500 Internal Server Error") {
		t.Fatalf("ListOutposts() error = %q, want status", err)
	}
}

func TestListOutpostsReturnsErrorForInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = client.ListOutposts(context.Background())
	if err == nil {
		t.Fatal("ListOutposts() error = nil, want an error")
	}

	if !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("ListOutposts() error = %q, want decode error", err)
	}
}
