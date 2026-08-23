package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/service"
)

type handler struct{ service *service.Service }

var _ api.ServerInterface = handler{}

func (h handler) ListArks(w http.ResponseWriter, r *http.Request) {
	arks, err := h.service.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to list arks"})
		return
	}
	writeJSON(w, http.StatusOK, apiArks(arks))
}
func (h handler) GetArk(w http.ResponseWriter, r *http.Request, name string) {
	a, err := h.service.Get(r.Context(), name)
	if errors.Is(err, ark.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to get ark"})
		return
	}
	writeJSON(w, http.StatusOK, apiArk(a))
}
func (h handler) CreateArk(w http.ResponseWriter, r *http.Request) {
	var request api.CreateArkRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid request body"})
		return
	}
	created, err := h.service.Create(r.Context(), ark.CreateInput{Name: request.Name, ImageID: request.ImageId, VCPUs: request.Vcpus, MemoryMiB: request.MemoryMib, DiskGiB: request.DiskGib})
	if errors.Is(err, ark.ErrNameRequired) || errors.Is(err, ark.ErrInvalidResources) {
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
func (h handler) StartArk(w http.ResponseWriter, r *http.Request, name api.ArkName) {
	h.lifecycle(w, func() (ark.Ark, error) { return h.service.Start(r.Context(), name) }, "start")
}
func (h handler) StopArk(w http.ResponseWriter, r *http.Request, name api.ArkName) {
	h.lifecycle(w, func() (ark.Ark, error) { return h.service.Stop(r.Context(), name) }, "stop")
}
func (h handler) DeleteArk(w http.ResponseWriter, r *http.Request, name string) {
	h.lifecycle(w, func() (ark.Ark, error) { return h.service.Delete(r.Context(), name) }, "delete")
}
func (h handler) lifecycle(w http.ResponseWriter, action func() (ark.Ark, error), operation string) {
	a, err := action()
	if errors.Is(err, ark.ErrNotFound) {
		notFound(w)
		return
	}
	if errors.Is(err, service.ErrInvalidState) {
		writeJSON(w, http.StatusConflict, api.Error{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to " + operation + " ark"})
		return
	}
	writeJSON(w, http.StatusOK, apiArk(a))
}
func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, api.Error{Error: ark.ErrNotFound.Error()})
}
func apiArks(arks []ark.Ark) []api.Ark {
	response := make([]api.Ark, 0, len(arks))
	for _, a := range arks {
		response = append(response, apiArk(a))
	}
	return response
}
func apiArk(a ark.Ark) api.Ark {
	return api.Ark{Id: a.ID, Name: a.Name, ImageId: a.ImageID, Vcpus: a.VCPUs, MemoryMib: a.MemoryMiB, DiskGib: a.DiskGiB, DesiredState: api.ArkDesiredState(a.DesiredState), Status: api.ArkStatus(a.Status), GuestIp: a.GuestIP, Failure: a.Failure, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
