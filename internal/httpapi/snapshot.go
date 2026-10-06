package httpapi

import (
	"errors"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func (h handler) SnapshotOutpost(w http.ResponseWriter, r *http.Request, name api.OutpostName, params api.SnapshotOutpostParams) {
	image, err := h.service.Snapshot(r.Context(), name, params.Tag)
	switch {
	case errors.Is(err, outpost.ErrNotFound), errors.Is(err, vmapi.ErrNotFound):
		notFound(w)
	case errors.Is(err, outpost.ErrInvalidImage):
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
	case errors.Is(err, service.ErrInvalidState), errors.Is(err, vmapi.ErrConflict):
		writeJSON(w, http.StatusConflict, api.Error{Error: "snapshot requires a stopped VM"})
	case errors.Is(err, service.ErrImagesUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to snapshot outpost"})
	default:
		writeJSON(w, http.StatusCreated, apiImage(image))
	}
}
