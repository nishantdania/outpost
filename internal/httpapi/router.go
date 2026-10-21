// Package httpapi frames application operations over the local Unix socket.
package httpapi

import (
	"net/http"

	"github.com/nishantdania/outpost/internal/manager"
)

func Handler(application *manager.Manager) http.Handler {
	h := handler{service: application}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/outposts", h.ListOutposts)
	mux.HandleFunc("POST /v1/outposts", h.CreateOutpost)
	mux.HandleFunc("GET /v1/outposts/{name}", func(w http.ResponseWriter, r *http.Request) { h.GetOutpost(w, r, r.PathValue("name")) })
	mux.HandleFunc("DELETE /v1/outposts/{name}", func(w http.ResponseWriter, r *http.Request) { h.DeleteOutpost(w, r, r.PathValue("name")) })
	mux.HandleFunc("POST /v1/outposts/{name}/start", func(w http.ResponseWriter, r *http.Request) { h.StartOutpost(w, r, r.PathValue("name")) })
	mux.HandleFunc("POST /v1/outposts/{name}/stop", func(w http.ResponseWriter, r *http.Request) { h.StopOutpost(w, r, r.PathValue("name")) })
	mux.HandleFunc("POST /v1/outposts/{name}/snapshot", func(w http.ResponseWriter, r *http.Request) {
		h.SnapshotOutpost(w, r, r.PathValue("name"), r.URL.Query().Get("tag"))
	})
	mux.HandleFunc("GET /v1/images", h.ListImages)
	mux.HandleFunc("POST /v1/images", func(w http.ResponseWriter, r *http.Request) { h.ImportImage(w, r, r.URL.Query().Get("tag")) })
	mux.HandleFunc("GET /v1/images/{reference}", func(w http.ResponseWriter, r *http.Request) { h.GetImage(w, r, r.PathValue("reference")) })
	mux.HandleFunc("DELETE /v1/images/{reference}", func(w http.ResponseWriter, r *http.Request) { h.DeleteImage(w, r, r.PathValue("reference")) })
	mux.HandleFunc("POST /v1/images/gc", h.GcImages)
	return mux
}
