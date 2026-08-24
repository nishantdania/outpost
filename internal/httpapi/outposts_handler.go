package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
)

type handler struct{ service *service.Service }

var _ api.ServerInterface = handler{}

func (h handler) ListOutposts(w http.ResponseWriter, r *http.Request) {
	outposts, err := h.service.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to list outposts"})
		return
	}
	writeJSON(w, http.StatusOK, apiOutposts(outposts))
}
func (h handler) GetOutpost(w http.ResponseWriter, r *http.Request, name string) {
	a, err := h.service.Get(r.Context(), name)
	if errors.Is(err, outpost.ErrNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to get outpost"})
		return
	}
	writeJSON(w, http.StatusOK, apiOutpost(a))
}
func (h handler) CreateOutpost(w http.ResponseWriter, r *http.Request) {
	var request api.CreateOutpostRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid request body"})
		return
	}
	created, err := h.service.Create(r.Context(), outpost.CreateInput{Name: request.Name, ImageID: request.ImageId, VCPUs: request.Vcpus, MemoryMiB: request.MemoryMib, DiskGiB: request.DiskGib, SSHPublicKey: stringValue(request.SshPublicKey)})
	if errors.Is(err, outpost.ErrNameRequired) || errors.Is(err, outpost.ErrInvalidResources) || errors.Is(err, outpost.ErrInvalidSSHPublicKey) {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
		return
	}
	if errors.Is(err, outpost.ErrNameTaken) {
		writeJSON(w, http.StatusConflict, api.Error{Error: err.Error()})
		return
	}
	if errors.Is(err, outpost.ErrInvalidImage) || errors.Is(err, outpost.ErrImageNotFound) {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to create outpost"})
		return
	}
	writeJSON(w, http.StatusCreated, apiOutpost(created))
}
func (h handler) ListImages(w http.ResponseWriter, r *http.Request) {
	images, err := h.service.Images(r.Context())
	if errors.Is(err, service.ErrImagesUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, 500, api.Error{Error: "failed to list images"})
		return
	}
	out := make([]api.Image, 0, len(images))
	for _, v := range images {
		out = append(out, apiImage(v))
	}
	writeJSON(w, 200, out)
}
func (h handler) GetImage(w http.ResponseWriter, r *http.Request, ref string) {
	image, err := h.service.Image(r.Context(), ref)
	if errors.Is(err, service.ErrImagesUnavailable) {
		writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
		return
	}
	if errors.Is(err, outpost.ErrImageNotFound) {
		notFound(w)
		return
	}
	if err != nil {
		writeJSON(w, 400, api.Error{Error: "invalid image"})
		return
	}
	writeJSON(w, 200, apiImage(image))
}
func (h handler) BuildImage(w http.ResponseWriter, r *http.Request, p api.BuildImageParams) {
	h.uploadImage(w, r, p.Tag, true)
}
func (h handler) ImportImage(w http.ResponseWriter, r *http.Request, p api.ImportImageParams) {
	h.uploadImage(w, r, p.Tag, false)
}
func (h handler) uploadImage(w http.ResponseWriter, r *http.Request, tag string, build bool) {
	want := "application/octet-stream"
	limit := int64(256 << 20)
	if build {
		want, limit = "application/x-tar", 64<<20
	}
	if r.Header.Get("Content-Type") != want {
		writeJSON(w, http.StatusUnsupportedMediaType, api.Error{Error: "invalid image content type"})
		return
	}
	defer r.Body.Close()
	body := http.MaxBytesReader(w, r.Body, limit+1)
	var image outpost.Image
	var err error
	if build {
		image, err = h.service.BuildImage(r.Context(), body, tag)
	} else {
		image, err = h.service.ImportImage(r.Context(), body, tag)
	}
	if err != nil {
		if errors.Is(err, service.ErrImagesUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
			return
		}
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) || strings.Contains(err.Error(), "exceeds limit") {
			writeJSON(w, http.StatusRequestEntityTooLarge, api.Error{Error: "image input exceeds limit"})
			return
		}
		if errors.Is(err, outpost.ErrInvalidImage) {
			writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "image operation failed"})
		return
	}
	writeJSON(w, http.StatusCreated, apiImage(image))
}
func (h handler) DeleteImage(w http.ResponseWriter, r *http.Request, ref string) {
	if err := h.service.RemoveImage(r.Context(), ref); err != nil {
		if errors.Is(err, service.ErrImagesUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
			return
		}
		if errors.Is(err, outpost.ErrImageNotFound) {
			notFound(w)
			return
		}
		if errors.Is(err, outpost.ErrInvalidImage) {
			writeJSON(w, http.StatusBadRequest, api.Error{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusConflict, api.Error{Error: "image cannot be removed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h handler) GcImages(w http.ResponseWriter, r *http.Request) {
	ids, err := h.service.GCImages(r.Context())
	if err != nil {
		if errors.Is(err, service.ErrImagesUnavailable) {
			writeJSON(w, http.StatusServiceUnavailable, api.Error{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "image GC failed"})
		return
	}
	writeJSON(w, 200, ids)
}
func apiImage(v outpost.Image) api.Image {
	return api.Image{Digest: v.Digest, SizeBytes: int(v.Size), Tags: v.Tags, CreatedAt: v.CreatedAt}
}

func (h handler) StartOutpost(w http.ResponseWriter, r *http.Request, name api.OutpostName) {
	h.lifecycle(w, func() (outpost.Outpost, error) { return h.service.Start(r.Context(), name) }, "start")
}
func (h handler) StopOutpost(w http.ResponseWriter, r *http.Request, name api.OutpostName) {
	h.lifecycle(w, func() (outpost.Outpost, error) { return h.service.Stop(r.Context(), name) }, "stop")
}
func (h handler) DeleteOutpost(w http.ResponseWriter, r *http.Request, name string) {
	h.lifecycle(w, func() (outpost.Outpost, error) { return h.service.Delete(r.Context(), name) }, "delete")
}
func (h handler) lifecycle(w http.ResponseWriter, action func() (outpost.Outpost, error), operation string) {
	a, err := action()
	if errors.Is(err, outpost.ErrNotFound) {
		notFound(w)
		return
	}
	if errors.Is(err, service.ErrInvalidState) {
		writeJSON(w, http.StatusConflict, api.Error{Error: err.Error()})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, api.Error{Error: "failed to " + operation + " outpost"})
		return
	}
	writeJSON(w, http.StatusOK, apiOutpost(a))
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func notFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, api.Error{Error: outpost.ErrNotFound.Error()})
}
func apiOutposts(outposts []outpost.Outpost) []api.Outpost {
	response := make([]api.Outpost, 0, len(outposts))
	for _, a := range outposts {
		response = append(response, apiOutpost(a))
	}
	return response
}
func apiOutpost(a outpost.Outpost) api.Outpost {
	return api.Outpost{Id: a.ID, Name: a.Name, ImageId: a.ImageID, Vcpus: a.VCPUs, MemoryMib: a.MemoryMiB, DiskGib: a.DiskGiB, DesiredState: api.OutpostDesiredState(a.DesiredState), Status: api.OutpostStatus(a.Status), GuestIp: a.GuestIP, Failure: a.Failure, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}
