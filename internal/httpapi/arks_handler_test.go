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

	createRec := createArk(t, handler, "demo")
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

func TestCreateArkRejectsDuplicateName(t *testing.T) {
	handler := handler{store: newTestStore(t)}

	if rec := createArk(t, handler, "demo"); rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want %d", rec.Code, http.StatusCreated)
	}

	second := createArk(t, handler, "Demo")
	if second.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, want %d", second.Code, http.StatusConflict)
	}

	var response api.Error
	if err := json.Unmarshal(second.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error != ark.ErrNameTaken.Error() {
		t.Fatalf("error = %q, want %q", response.Error, ark.ErrNameTaken)
	}
}

func createArk(t *testing.T, handler handler, name string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/arks", strings.NewReader(`{"name":"`+name+`"}`))
	rec := httptest.NewRecorder()
	handler.CreateArk(rec, req)

	return rec
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
