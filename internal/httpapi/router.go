package httpapi

import "net/http"

func newRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/arks", listArks)

	return mux
}
