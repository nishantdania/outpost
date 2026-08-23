package httpapi

import (
	"net/http"

	"github.com/nishantdania/ark/internal/api"
)

type handler struct{}

var _ api.ServerInterface = handler{}

func (handler) ListArks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []api.Ark{})
}
