package launcherclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/vmapi"
)

const (
	maxBodyBytes            = 64 << 10
	lifecycleRequestTimeout = 5 * time.Minute
)

type Client struct{ http *http.Client }

func New(socketPath string) *Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
		},
	}
	return &Client{http: &http.Client{Transport: transport, Timeout: lifecycleRequestTimeout}}
}

func (c *Client) Close() { c.http.CloseIdleConnections() }
func (c *Client) Create(ctx context.Context, a ark.Ark) error {
	return c.call(ctx, "/v1/create", vmapi.CreateRequest{Version: vmapi.Version, Spec: vmapi.VMSpec{ID: a.ID, ImageID: a.ImageID, VCPUs: a.VCPUs, MemoryMiB: a.MemoryMiB, DiskGiB: a.DiskGiB}}, nil)
}
func (c *Client) Start(ctx context.Context, id string) (string, error) {
	var response vmapi.StartResponse
	err := c.call(ctx, "/v1/start", vmapi.IDRequest{Version: vmapi.Version, ID: id}, &response)
	return response.GuestIP, err
}
func (c *Client) Inspect(ctx context.Context, id string) (vmapi.InspectResponse, error) {
	var response vmapi.InspectResponse
	err := c.call(ctx, "/v1/inspect", vmapi.IDRequest{Version: vmapi.Version, ID: id}, &response)
	return response, err
}
func (c *Client) List(ctx context.Context) ([]vmapi.RuntimeState, error) {
	var response vmapi.ListResponse
	err := c.call(ctx, "/v1/list", vmapi.VersionRequest{Version: vmapi.Version}, &response)
	return response.VMs, err
}
func (c *Client) Stop(ctx context.Context, id string) error {
	return c.call(ctx, "/v1/stop", vmapi.IDRequest{Version: vmapi.Version, ID: id}, nil)
}
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.call(ctx, "/v1/delete", vmapi.IDRequest{Version: vmapi.Version, ID: id}, nil)
}

func (c *Client) call(ctx context.Context, path string, input, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix"+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call launcher: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return decodeError(response)
	}
	if output == nil {
		return nil
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode launcher response: %w", err)
	}
	return nil
}
func decodeError(response *http.Response) error {
	var value vmapi.ErrorResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxBodyBytes))
	if err := decoder.Decode(&value); err != nil || value.Error == "" {
		return fmt.Errorf("launcher returned HTTP %d", response.StatusCode)
	}
	switch response.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", value.Error, vmapi.ErrNotFound)
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %w", value.Error, vmapi.ErrInvalid)
	case http.StatusConflict:
		return fmt.Errorf("%s: %w", value.Error, vmapi.ErrConflict)
	case http.StatusRequestTimeout:
		return context.DeadlineExceeded
	default:
		return errors.New(value.Error)
	}
}

var _ vmapi.Manager = (*Client)(nil)
