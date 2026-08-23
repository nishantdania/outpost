package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nishantdania/ark/internal/api"
)

func (c *Client) CreateArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.CreateArkWithResponse(ctx, api.CreateArkRequest{Name: name})
	if err != nil {
		return api.Ark{}, fmt.Errorf("create ark: %w", err)
	}

	if response.StatusCode() != http.StatusCreated {
		if response.JSON400 != nil {
			return api.Ark{}, fmt.Errorf("create ark: %s", response.JSON400.Error)
		}
		if response.JSON409 != nil {
			return api.Ark{}, fmt.Errorf("create ark: %s", response.JSON409.Error)
		}
		return api.Ark{}, fmt.Errorf("create ark: unexpected response status: %s", response.Status())
	}

	if response.JSON201 == nil {
		return api.Ark{}, fmt.Errorf("create ark: response did not contain JSON")
	}

	return *response.JSON201, nil
}

func (c *Client) DeleteArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.DeleteArkWithResponse(ctx, name)
	if err != nil {
		return api.Ark{}, fmt.Errorf("delete ark: %w", err)
	}

	if response.StatusCode() != http.StatusOK {
		if response.JSON404 != nil {
			return api.Ark{}, fmt.Errorf("delete ark: %s", response.JSON404.Error)
		}
		return api.Ark{}, fmt.Errorf("delete ark: unexpected response status: %s", response.Status())
	}

	if response.JSON200 == nil {
		return api.Ark{}, fmt.Errorf("delete ark: response did not contain JSON")
	}

	return *response.JSON200, nil
}

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
