package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	router := newRouter(testHandler(t, newTestStore(t)).service, "test-token")

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{
			name:   "list outposts",
			method: http.MethodGet,
			path:   "/v1/outposts",
			want:   http.StatusOK,
		},
		{
			name:   "wrong method",
			method: http.MethodPut,
			path:   "/v1/outposts",
			want:   http.StatusMethodNotAllowed,
		},
		{
			name:   "unknown route",
			method: http.MethodGet,
			path:   "/unknown",
			want:   http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer test-token")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
