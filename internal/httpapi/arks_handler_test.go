package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/ark/internal/api"
)

func TestListArks(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/arks", nil)
	rec := httptest.NewRecorder()

	handler{}.ListArks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", got, "application/json")
	}

	var arks []api.Ark
	if err := json.Unmarshal(rec.Body.Bytes(), &arks); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(arks) != 0 {
		t.Fatalf("arks = %v, want empty list", arks)
	}
}
