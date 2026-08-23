package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nishantdania/ark/internal/service"
	"github.com/nishantdania/ark/internal/vmapi"
)

func TestImageRoutesRequireAuthAndAreUnavailableWithoutPodman(t *testing.T) {
	router := newRouter(service.New(newTestStore(t), &vmapi.FakeManager{}), "token")
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/v1/images", nil),
		httptest.NewRequest(http.MethodPost, "/v1/images?tag=coding:latest", strings.NewReader("x")),
		httptest.NewRequest(http.MethodPost, "/v1/images/build?tag=coding:latest", strings.NewReader("x")),
		httptest.NewRequest(http.MethodGet, "/v1/images/coding:latest", nil),
	} {
		if request.Method == http.MethodPost {
			if strings.Contains(request.URL.Path, "/build") {
				request.Header.Set("Content-Type", "application/x-tar")
			} else {
				request.Header.Set("Content-Type", "application/octet-stream")
			}
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, request)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s = %d", request.URL, rec.Code)
		}
		request.Header.Set("Authorization", "Bearer token")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, request)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("unavailable %s = %d", request.URL, rec.Code)
		}
		if !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("content type = %q", rec.Header().Get("Content-Type"))
		}
	}
}

func TestImageUploadRejectsExactWrongContentType(t *testing.T) {
	router := newRouter(service.New(newTestStore(t), &vmapi.FakeManager{}), "token")
	request := httptest.NewRequest(http.MethodPost, "/v1/images?tag=coding:latest", strings.NewReader("x"))
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/octet-stream; charset=binary")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, request)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d", rec.Code)
	}
}
