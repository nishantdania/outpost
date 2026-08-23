package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter(t *testing.T) {
	router := newRouter()

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{
			name:   "list arks",
			method: http.MethodGet,
			path:   "/v1/arks",
			want:   http.StatusOK,
		},
		{
			name:   "wrong method",
			method: http.MethodPost,
			path:   "/v1/arks",
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
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
