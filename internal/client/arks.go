package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
)

type CreateArkInput struct {
	Name, ImageID             string
	VCPUs, MemoryMiB, DiskGiB int
}

func (c *Client) CreateArk(ctx context.Context, name string) (api.Ark, error) {
	return c.CreateArkWith(ctx, CreateArkInput{Name: name, ImageID: ark.DefaultImageID, VCPUs: ark.DefaultVCPUs, MemoryMiB: ark.DefaultMemoryMiB, DiskGiB: ark.DefaultDiskGiB})
}

func (c *Client) CreateArkWith(ctx context.Context, input CreateArkInput) (api.Ark, error) {
	response, err := c.api.CreateArkWithResponse(ctx, api.CreateArkRequest{Name: input.Name, ImageId: input.ImageID, Vcpus: input.VCPUs, MemoryMib: input.MemoryMiB, DiskGib: input.DiskGiB})
	if err != nil {
		return api.Ark{}, fmt.Errorf("create ark: %w", err)
	}
	return createdResponse(response.StatusCode(), response.Status(), response.JSON201, response.JSON400, response.JSON409)
}
func (c *Client) DeleteArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.DeleteArkWithResponse(ctx, name)
	if err != nil {
		return api.Ark{}, fmt.Errorf("delete ark: %w", err)
	}
	return lifecycleResponse("delete", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
}
func (c *Client) GetArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.GetArkWithResponse(ctx, name)
	if err != nil {
		return api.Ark{}, fmt.Errorf("get ark: %w", err)
	}
	return lifecycleResponse("get", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, nil)
}
func (c *Client) StartArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.StartArkWithResponse(ctx, name)
	if err != nil {
		return api.Ark{}, fmt.Errorf("start ark: %w", err)
	}
	return lifecycleResponse("start", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
}
func (c *Client) StopArk(ctx context.Context, name string) (api.Ark, error) {
	response, err := c.api.StopArkWithResponse(ctx, name)
	if err != nil {
		return api.Ark{}, fmt.Errorf("stop ark: %w", err)
	}
	return lifecycleResponse("stop", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
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
func createdResponse(status int, statusText string, body *api.Ark, badRequest, conflict *api.Error) (api.Ark, error) {
	if status != http.StatusCreated {
		if badRequest != nil {
			return api.Ark{}, fmt.Errorf("create ark: %s", badRequest.Error)
		}
		if conflict != nil {
			return api.Ark{}, fmt.Errorf("create ark: %s", conflict.Error)
		}
		return api.Ark{}, fmt.Errorf("create ark: unexpected response status: %s", statusText)
	}
	if body == nil {
		return api.Ark{}, fmt.Errorf("create ark: response did not contain JSON")
	}
	return *body, nil
}
func lifecycleResponse(operation string, status int, statusText string, body *api.Ark, notFound, conflict *api.Error) (api.Ark, error) {
	if status != http.StatusOK {
		if notFound != nil {
			return api.Ark{}, fmt.Errorf("%s ark: %s", operation, notFound.Error)
		}
		if conflict != nil {
			return api.Ark{}, fmt.Errorf("%s ark: %s", operation, conflict.Error)
		}
		return api.Ark{}, fmt.Errorf("%s ark: unexpected response status: %s", operation, statusText)
	}
	if body == nil {
		return api.Ark{}, fmt.Errorf("%s ark: response did not contain JSON", operation)
	}
	return *body, nil
}
