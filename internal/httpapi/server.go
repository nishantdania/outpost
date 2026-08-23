package httpapi

import "net/http"

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
