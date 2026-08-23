package httpapi

import (
	"net/http"

	"github.com/nishantdania/ark/internal/api"
)

func newRouter() http.Handler {
	return api.Handler(handler{})
}
