package daemon

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/update"
)

type CreateFunc func(context.Context) (outpost.Result, error)
type UpdateFunc func(context.Context) (update.Result, error)

func New(create CreateFunc, version string, applyUpdate UpdateFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /outposts", createHandler(create))
	mux.HandleFunc("GET /version", versionHandler(version))
	mux.HandleFunc("POST /update", updateHandler(applyUpdate))
	return mux
}

type createResponse struct {
	Message string `json:"message"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type updateResponse struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Updated        bool   `json:"updated"`
}

func createHandler(create CreateFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := create(r.Context())
		if err != nil {
			http.Error(w, "create outpost", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(createResponse{Message: result.Message})
	}
}

func versionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(versionResponse{Version: version})
	}
}

func updateHandler(applyUpdate UpdateFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := applyUpdate(r.Context())
		if err != nil {
			http.Error(w, "update outpostd", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(updateResponse{CurrentVersion: result.CurrentVersion, LatestVersion: result.LatestVersion, Updated: result.Updated})
	}
}
