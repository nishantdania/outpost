package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nishantdania/outpost/internal/api"
)

const lifecycleRequestTimeout = 5 * time.Minute

type Client struct{ api *api.ClientWithResponses }

func New(baseURL string, tokens ...string) (*Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("server URL must include a scheme and host: %q", baseURL)
	}
	token := ""
	if len(tokens) > 0 {
		token = tokens[0]
	}
	apiClient, err := api.NewClientWithResponses(parsedURL.String(), api.WithHTTPClient(&http.Client{Timeout: lifecycleRequestTimeout}), api.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		return nil
	}))
	if err != nil {
		return nil, err
	}
	return &Client{api: apiClient}, nil
}
