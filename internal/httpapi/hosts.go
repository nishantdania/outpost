package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (h handler) ListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.service.Hosts(r.Context())
	if err != nil {
		hostError(w, err)
		return
	}
	response := make([]api.Host, 0, len(hosts))
	for _, host := range hosts {
		response = append(response, apiHost(host))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h handler) GetHost(w http.ResponseWriter, r *http.Request, hostname string) {
	host, err := h.service.Host(r.Context(), hostname)
	if err != nil {
		hostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiHost(host))
}

func (h handler) SetHost(w http.ResponseWriter, r *http.Request, hostname string) {
	var input api.SetHostRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid host request body"})
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, api.Error{Error: "invalid host request body"})
		return
	}
	host, err := h.service.SetHost(r.Context(), hostname, input.OutpostName, input.Port)
	if err != nil {
		hostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiHost(host))
}

func (h handler) Unhost(w http.ResponseWriter, r *http.Request, hostname string) {
	if err := h.service.Unhost(r.Context(), hostname); err != nil {
		hostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func hostError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "host mapping operation failed"
	switch {
	case errors.Is(err, outpost.ErrInvalidHostname), errors.Is(err, outpost.ErrInvalidPort):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, outpost.ErrHostTaken):
		status, message = http.StatusConflict, err.Error()
	case errors.Is(err, outpost.ErrHostNotFound), errors.Is(err, outpost.ErrNotFound):
		status, message = http.StatusNotFound, err.Error()
	}
	writeJSON(w, status, api.Error{Error: message})
}

func apiHost(host outpost.Host) api.Host {
	return api.Host{Hostname: host.Hostname, OutpostId: host.OutpostID, OutpostName: host.OutpostName,
		Port: host.Port, GuestIp: host.GuestIP, Status: host.Status, DesiredState: host.DesiredState}
}
