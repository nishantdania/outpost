package httpapi

import (
	"net/http"

	"github.com/nishantdania/outpost/internal/service"
)

func NewServer(addr string, application *service.Service, token string) *http.Server {
	return &http.Server{Addr: addr, Handler: newRouter(application, token)}
}
