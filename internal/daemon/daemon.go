package daemon

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/nishantdania/outpost/internal/outpost"
)

type CreateFunc func(context.Context) (outpost.Result, error)

func New(create CreateFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /outposts", createHandler(create))
	return mux
}

type createResponse struct {
	Message string `json:"message"`
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
