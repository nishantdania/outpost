package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
)

func TestCreateArkAndListArks(t *testing.T) {
	store := newTestStore(t)
	handler := handler{store: store}

	createReq := httptest.NewRequest(http.MethodPost, "/v1/arks", strings.NewReader(`{"name":"demo"}`))
	createRec := httptest.NewRecorder()
	handler.CreateArk(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created api.Ark
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Id == "" || created.Name != "demo" {
		t.Fatalf("created Ark = %v, want generated ID and demo name", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/arks", nil)
	listRec := httptest.NewRecorder()
	handler.ListArks(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var arks []api.Ark
	if err := json.Unmarshal(listRec.Body.Bytes(), &arks); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(arks) != 1 || arks[0] != created {
		t.Fatalf("listed Arks = %v, want %v", arks, created)
	}
}

func newTestStore(t *testing.T) *ark.Store {
	t.Helper()

	store, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	return store
}
