package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func (h handler) SnapshotOutpost(w http.ResponseWriter, r *http.Request, name api.OutpostName, params api.SnapshotOutpostParams) {
	var profile *credentials.Profile
	if r.Body != nil {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid credential profile"})
			return
		}
		if len(data) > 0 {
			contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				writeJSON(w, http.StatusUnsupportedMediaType, api.Error{Error: "credential profile requires application/json"})
				return
			}
			p, err := credentials.Parse(data)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid credential profile"})
				return
			}
			profile = &p
		}
	}
	image, err := h.service.SnapshotWithCredentials(r.Context(), name, params.Tag, profile)
	switch {
	case errors.Is(err, outpost.ErrNotFound), errors.Is(err, vmapi.ErrNotFound):
		notFound(w)
	case errors.Is(err, credentials.ErrInvalid), errors.Is(err, credentials.ErrUnavailable):
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
	case errors.Is(err, outpost.ErrInvalidImage):
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
	case errors.Is(err, outpost.ErrCredentialConflict):
		writeJSON(w, http.StatusConflict, api.Error{Error: err.Error()})
	case errors.Is(err, service.ErrInvalidState), errors.Is(err, vmapi.ErrConflict):
		writeJSON(w, http.StatusConflict, api.Error{Error: "snapshot requires a stopped VM"})
	case errors.Is(err, service.ErrImagesUnavailable), errors.Is(err, service.ErrCredentialsUnavailable):
		writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to snapshot outpost"})
	default:
		writeJSON(w, http.StatusCreated, apiImage(image))
	}
}
