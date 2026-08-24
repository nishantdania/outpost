package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterRequiresBearerToken(t *testing.T) {
	router := newRouter(testHandler(t, newTestStore(t)).service, "test-token")
	for _, authorization := range []string{"", "Bearer wrong", "Bearer a-much-longer-wrong-token", "Bearer", "Basic test-token"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/outposts", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("Authorization %q: status = %d, want %d", authorization, rec.Code, http.StatusUnauthorized)
		}
		if rec.Header().Get("WWW-Authenticate") != "Bearer" {
			t.Fatalf("Authorization %q: WWW-Authenticate = %q", authorization, rec.Header().Get("WWW-Authenticate"))
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/outposts", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid authorization: status = %d, want %d", rec.Code, http.StatusOK)
	}
}
