package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/outpost"
)

func TestCreateOutpost(t *testing.T) {
	handler := New(func(context.Context) (outpost.Result, error) {
		return outpost.Result{Message: "Hello, World!"}, nil
	})

	request := httptest.NewRequest(http.MethodPost, "/outposts", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Message != "Hello, World!" {
		t.Errorf("message = %q, want %q", body.Message, "Hello, World!")
	}
}

func TestCreateOutpostRejectsOtherMethods(t *testing.T) {
	handler := New(outpost.Create)
	request := httptest.NewRequest(http.MethodGet, "/outposts", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
}
