package httpapi

import (
	"net/http"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
)

func newRouter(store *ark.Store) http.Handler {
	return api.Handler(handler{store: store})
}
