package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
)

func TestDeleteOutpost(t *testing.T) {
	store := newTestStore(t)
	created, err := store.Create(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/v1/outposts/demo", nil)
	rec := httptest.NewRecorder()
	testHandler(t, store).DeleteOutpost(rec, req, "demo")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var deleted api.Outpost
	if err := json.Unmarshal(rec.Body.Bytes(), &deleted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deleted.Id != created.ID || deleted.Name != created.Name {
		t.Fatalf("deleted Outpost = %v, want %v", deleted, created)
	}
}

func TestDeleteOutpostReturnsNotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodDelete, "/v1/outposts/missing", nil)
	rec := httptest.NewRecorder()
	testHandler(t, newTestStore(t)).DeleteOutpost(rec, req, "missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
