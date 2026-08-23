package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/ark/internal/api"
)

func TestDeleteArk(t *testing.T) {
	store := newTestStore(t)
	created, err := store.Create(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/arks/demo", nil)
	rec := httptest.NewRecorder()
	handler{store: store}.DeleteArk(rec, req, "demo")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var deleted api.Ark
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deleted.Id != created.ID || deleted.Name != created.Name {
		t.Fatalf("deleted Ark = %v, want %v", deleted, created)
	}
}

func TestDeleteArkReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/v1/arks/missing", nil)
	rec := httptest.NewRecorder()
	handler{store: newTestStore(t)}.DeleteArk(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
