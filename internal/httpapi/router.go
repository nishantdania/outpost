package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/service"
)

func newRouter(application *service.Service, token string) http.Handler {
	return authenticate(token, api.Handler(handler{service: application}))
}

func authenticate(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		credential, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		provided := sha256.Sum256([]byte(credential))
		expected := sha256.Sum256([]byte(token))
		if !ok || credential == "" || subtle.ConstantTimeCompare(provided[:], expected[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeJSON(w, http.StatusUnauthorized, api.Error{Error: "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
