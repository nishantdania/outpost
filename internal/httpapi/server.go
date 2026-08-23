package httpapi

import (
	"net/http"

	"github.com/nishantdania/ark/internal/ark"
)

func NewServer(addr string, store *ark.Store) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: newRouter(store),
	}
}
