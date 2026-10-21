package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/catalog"
	"github.com/nishantdania/outpost/internal/manager"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type handler struct{ service *manager.Manager }

func respond(w http.ResponseWriter, status int, value any, err error) {
	if err == nil {
		if status == http.StatusNoContent {
			w.WriteHeader(status)
		} else {
			writeJSON(w, status, value)
		}
		return
	}
	message := err.Error()
	switch {
	case errors.Is(err, outpost.ErrNotFound), errors.Is(err, outpost.ErrImageNotFound), errors.Is(err, vmapi.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, outpost.ErrNameRequired), errors.Is(err, outpost.ErrInvalidResources), errors.Is(err, outpost.ErrInvalidImage):
		status = http.StatusBadRequest
	case errors.Is(err, outpost.ErrNameTaken), errors.Is(err, manager.ErrInvalidState), errors.Is(err, vmapi.ErrConflict), errors.Is(err, catalog.ErrImageProtected):
		status = http.StatusConflict
	default:
		status, message = http.StatusInternalServerError, "operation failed"
	}
	writeJSON(w, status, api.Error{Error: message})
}

func (h handler) ListOutposts(w http.ResponseWriter, r *http.Request) {
	vms, err := h.service.List(r.Context())
	result := make([]api.Outpost, 0, len(vms))
	for _, vm := range vms {
		result = append(result, apiOutpost(vm))
	}
	respond(w, http.StatusOK, result, err)
}
func (h handler) GetOutpost(w http.ResponseWriter, r *http.Request, name string) {
	vm, err := h.service.Get(r.Context(), name)
	respond(w, http.StatusOK, apiOutpost(vm), err)
}
func (h handler) CreateOutpost(w http.ResponseWriter, r *http.Request) {
	var request api.CreateOutpostRequest
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid request body"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "expected a single JSON request"})
		return
	}
	vm, err := h.service.Create(r.Context(), outpost.CreateInput{Name: request.Name, ImageID: request.ImageId, VCPUs: request.Vcpus, MemoryMiB: request.MemoryMib, DiskGiB: request.DiskGib})
	respond(w, http.StatusCreated, apiOutpost(vm), err)
}
func (h handler) StartOutpost(w http.ResponseWriter, r *http.Request, name string) {
	vm, err := h.service.Start(r.Context(), name)
	respond(w, http.StatusOK, apiOutpost(vm), err)
}
func (h handler) StopOutpost(w http.ResponseWriter, r *http.Request, name string) {
	vm, err := h.service.Stop(r.Context(), name)
	respond(w, http.StatusOK, apiOutpost(vm), err)
}
func (h handler) DeleteOutpost(w http.ResponseWriter, r *http.Request, name string) {
	vm, err := h.service.Delete(r.Context(), name)
	respond(w, http.StatusOK, apiOutpost(vm), err)
}
func (h handler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.service.Images(r.Context())
	respond(w, http.StatusOK, images, err)
}
func (h handler) GetImage(w http.ResponseWriter, r *http.Request, ref string) {
	image, err := h.service.Image(r.Context(), ref)
	respond(w, http.StatusOK, image, err)
}
func (h handler) ImportImage(w http.ResponseWriter, r *http.Request, tag string) {
	if r.Header.Get("Content-Type") != "application/octet-stream" {
		writeJSON(w, http.StatusUnsupportedMediaType, api.Error{Error: "expected application/octet-stream"})
		return
	}
	defer r.Body.Close()
	image, err := h.service.ImportImage(r.Context(), r.Body, tag)
	respond(w, http.StatusCreated, image, err)
}
func (h handler) DeleteImage(w http.ResponseWriter, r *http.Request, ref string) {
	err := h.service.RemoveImage(r.Context(), ref)
	respond(w, http.StatusNoContent, nil, err)
}
func (h handler) GcImages(w http.ResponseWriter, r *http.Request) {
	ids, err := h.service.GCImages(r.Context())
	if ids == nil {
		ids = []string{}
	}
	respond(w, http.StatusOK, ids, err)
}
func (h handler) SnapshotOutpost(w http.ResponseWriter, r *http.Request, name, tag string) {
	image, err := h.service.Snapshot(r.Context(), name, tag)
	respond(w, http.StatusCreated, image, err)
}
func apiOutpost(a outpost.Outpost) api.Outpost {
	return api.Outpost{Id: a.ID, Name: a.Name, ImageId: a.ImageID, Vcpus: a.VCPUs, MemoryMib: a.MemoryMiB, DiskGib: a.DiskGiB, DesiredState: a.DesiredState, Status: a.Status, GuestIp: a.GuestIP, Failure: a.Failure, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
