package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
)

type handler struct {
	store *ark.Store
}

var _ api.ServerInterface = handler{}

func (h handler) ListArks(w http.ResponseWriter, r *http.Request) {
	arks, err := h.store.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to list arks"})
		return
	}

	writeJSON(w, http.StatusOK, apiArks(arks))
}

func (h handler) CreateArk(w http.ResponseWriter, r *http.Request) {
	var request api.CreateArkRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid request body"})
		return
	}

	created, err := h.store.Create(r.Context(), request.Name)
	if errors.Is(err, ark.ErrNameRequired) {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
		return
	}
	if errors.Is(err, ark.ErrNameTaken) {
		writeJSON(w, http.StatusConflict, api.Error{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to create ark"})
		return
	}

	writeJSON(w, http.StatusCreated, apiArk(created))
}

func apiArks(arks []ark.Ark) []api.Ark {
	response := make([]api.Ark, 0, len(arks))
	for _, ark := range arks {
		response = append(response, apiArk(ark))
	}

	return response
}

func apiArk(ark ark.Ark) api.Ark {
	return api.Ark{
		Id:   ark.ID,
		Name: ark.Name,
	}
}
