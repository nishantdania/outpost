package daemon

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/update"
)

type CreateFunc func(context.Context, string) (outpost.Record, error)
type ListFunc func(context.Context) ([]outpost.Record, error)
type UpdateFunc func(context.Context) (update.Result, error)
type UninstallFunc func(context.Context) error

func New(create CreateFunc, list ListFunc, version string, applyUpdate UpdateFunc, uninstall UninstallFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /outposts", createHandler(create))
	mux.HandleFunc("GET /outposts", listHandler(list))
	mux.HandleFunc("GET /version", versionHandler(version))
	mux.HandleFunc("POST /update", updateHandler(applyUpdate))
	mux.HandleFunc("POST /uninstall", uninstallHandler(uninstall))
	return mux
}

func createHandler(create CreateFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		record, err := create(r.Context(), body.Name)
		if err != nil {
			http.Error(w, "create outpost", 500)
			return
		}
		write(w, record)
	}
}
func listHandler(list ListFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := list(r.Context())
		if err != nil {
			http.Error(w, "list outposts", 500)
			return
		}
		write(w, struct {
			Outposts []outpost.Record `json:"outposts"`
		}{records})
	}
}
func versionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		write(w, struct {
			Version string `json:"version"`
		}{version})
	}
}
func uninstallHandler(uninstall UninstallFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := uninstall(r.Context()); err != nil {
			http.Error(w, "uninstall outpostd", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
func updateHandler(apply UpdateFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := apply(r.Context())
		if err != nil {
			http.Error(w, "update outpostd", 500)
			return
		}
		write(w, struct {
			CurrentVersion string `json:"current_version"`
			LatestVersion  string `json:"latest_version"`
			Updated        bool   `json:"updated"`
		}{result.CurrentVersion, result.LatestVersion, result.Updated})
	}
}
func write(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
