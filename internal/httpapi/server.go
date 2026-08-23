package httpapi

import (
	"context"
	"net/http"
)

type Server struct {
	server *http.Server
}

func NewServer(addr string) *Server {
	return &Server{
		server: &http.Server{
			Addr:    addr,
			Handler: newRouter(),
		},
	}
}

func (s *Server) ListenAndServe() error {
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
