package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nishantdania/ark/internal/api"
)

func (c *Client) ListArks(ctx context.Context) ([]api.Ark, error) {
	response, err := c.api.ListArksWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list arks: %w", err)
	}

	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("list arks: unexpected response status: %s", response.Status())
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf("list arks: response did not contain JSON")
	}

	return *response.JSON200, nil
}
