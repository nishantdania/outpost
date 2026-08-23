package client

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/nishantdania/ark/internal/api"
)

const requestTimeout = 10 * time.Second

type Client struct {
	api *api.ClientWithResponses
}

func New(baseURL string) (*Client, error) {
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("server URL must include a scheme and host: %q", baseURL)
	}

	apiClient, err := api.NewClientWithResponses(
		parsedURL.String(),
		api.WithHTTPClient(&http.Client{Timeout: requestTimeout}),
	)
	if err != nil {
		return nil, err
	}

	return &Client{api: apiClient}, nil
}
