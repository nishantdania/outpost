package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedLifecycleRoutes(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.Create(t.Context(), "demo"); err != nil {
		t.Fatal(err)
	}
	router := newRouter(testHandler(t, store).service, "test-token")
	request := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := request(http.MethodGet, "/v1/arks/demo"); got != http.StatusOK {
		t.Fatalf("get status = %d", got)
	}
	if got := request(http.MethodPost, "/v1/arks/demo/start"); got != http.StatusOK {
		t.Fatalf("start status = %d", got)
	}
	if got := request(http.MethodPost, "/v1/arks/demo/start"); got != http.StatusConflict {
		t.Fatalf("duplicate start status = %d", got)
	}
	if got := request(http.MethodPost, "/v1/arks/demo/stop"); got != http.StatusOK {
		t.Fatalf("stop status = %d", got)
	}
	if got := request(http.MethodPost, "/v1/arks/demo/stop"); got != http.StatusConflict {
		t.Fatalf("duplicate stop status = %d", got)
	}
	for _, path := range []string{"/v1/arks/missing", "/v1/arks/missing/start", "/v1/arks/missing/stop"} {
		method := http.MethodGet
		if path != "/v1/arks/missing" {
			method = http.MethodPost
		}
		if got := request(method, path); got != http.StatusNotFound {
			t.Fatalf("%s %s status = %d", method, path, got)
		}
	}
}
