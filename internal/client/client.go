// Package client implements the CLI's local launcher transport.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/nishantdania/outpost/internal/api"
)

const lifecycleRequestTimeout = 5 * time.Minute

type Client struct{ http *http.Client }

func New() (*Client, error) {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", api.SocketPath)
	}}
	return &Client{http: &http.Client{Transport: transport}}, nil
}

// request streams uploads rather than buffering disk images. Ordinary operations
// have a deadline; imports and snapshots use the caller's cancellation instead.
func (c *Client) request(ctx context.Context, method, path string, body io.Reader, contentType string, status int, result any, stream bool) error {
	if !stream {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, lifecycleRequestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://launcher/v1"+path, body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != status {
		var detail api.Error
		if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&detail); err == nil && detail.Error != "" {
			return fmt.Errorf("%s %s: %s", method, path, detail.Error)
		}
		return fmt.Errorf("%s %s: %s", method, path, response.Status)
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(result); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", method, path, err)
	}
	return nil
}

func jsonBody(value any) (io.Reader, error) {
	body, err := json.Marshal(value)
	return bytes.NewReader(body), err
}
