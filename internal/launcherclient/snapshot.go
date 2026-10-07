package launcherclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/nishantdania/outpost/internal/vmapi"
)

// OpenSnapshot streams a private stopped-disk copy over the authenticated Unix
// socket. No disk path or arbitrary host file is exposed to outpostd.
func (c *Client) OpenSnapshot(ctx context.Context, id string) (io.ReadCloser, error) {
	body, err := json.Marshal(vmapi.IDRequest{Version: vmapi.Version, ID: id})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://unix/v1/snapshot", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Keep the shared transport, but let the caller's context bound disk exports.
	httpClient := *c.http
	httpClient.Timeout = 0
	response, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("snapshot launcher disk: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		return nil, decodeError(response)
	}
	if response.Header.Get("Content-Type") != "application/octet-stream" || response.ContentLength <= 0 {
		response.Body.Close()
		return nil, fmt.Errorf("invalid snapshot response")
	}
	return response.Body, nil
}
