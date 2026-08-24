package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
)

type CreateOutpostInput struct {
	Name, ImageID, SSHPublicKey string
	VCPUs, MemoryMiB, DiskGiB   int
}

func (c *Client) CreateOutpost(ctx context.Context, name string) (api.Outpost, error) {
	return c.CreateOutpostWith(ctx, CreateOutpostInput{Name: name, ImageID: outpost.DefaultImageID, VCPUs: outpost.DefaultVCPUs, MemoryMiB: outpost.DefaultMemoryMiB, DiskGiB: outpost.DefaultDiskGiB})
}

func (c *Client) CreateOutpostWith(ctx context.Context, input CreateOutpostInput) (api.Outpost, error) {
	response, err := c.api.CreateOutpostWithResponse(ctx, api.CreateOutpostRequest{Name: input.Name, ImageId: input.ImageID, Vcpus: input.VCPUs, MemoryMib: input.MemoryMiB, DiskGib: input.DiskGiB, SshPublicKey: stringPointer(input.SSHPublicKey)})
	if err != nil {
		return api.Outpost{}, fmt.Errorf("create outpost: %w", err)
	}
	return createdResponse(response.StatusCode(), response.Status(), response.JSON201, response.JSON400, response.JSON409)
}
func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (c *Client) DeleteOutpost(ctx context.Context, name string) (api.Outpost, error) {
	response, err := c.api.DeleteOutpostWithResponse(ctx, name)
	if err != nil {
		return api.Outpost{}, fmt.Errorf("delete outpost: %w", err)
	}
	return lifecycleResponse("delete", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
}
func (c *Client) GetOutpost(ctx context.Context, name string) (api.Outpost, error) {
	response, err := c.api.GetOutpostWithResponse(ctx, name)
	if err != nil {
		return api.Outpost{}, fmt.Errorf("get outpost: %w", err)
	}
	return lifecycleResponse("get", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, nil)
}
func (c *Client) StartOutpost(ctx context.Context, name string) (api.Outpost, error) {
	response, err := c.api.StartOutpostWithResponse(ctx, name)
	if err != nil {
		return api.Outpost{}, fmt.Errorf("start outpost: %w", err)
	}
	return lifecycleResponse("start", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
}
func (c *Client) StopOutpost(ctx context.Context, name string) (api.Outpost, error) {
	response, err := c.api.StopOutpostWithResponse(ctx, name)
	if err != nil {
		return api.Outpost{}, fmt.Errorf("stop outpost: %w", err)
	}
	return lifecycleResponse("stop", response.StatusCode(), response.Status(), response.JSON200, response.JSON404, response.JSON409)
}
func (c *Client) ListOutposts(ctx context.Context) ([]api.Outpost, error) {
	response, err := c.api.ListOutpostsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list outposts: %w", err)
	}
	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("list outposts: unexpected response status: %s", response.Status())
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("list outposts: response did not contain JSON")
	}
	return *response.JSON200, nil
}
func createdResponse(status int, statusText string, body *api.Outpost, badRequest, conflict *api.Error) (api.Outpost, error) {
	if status != http.StatusCreated {
		if badRequest != nil {
			return api.Outpost{}, fmt.Errorf("create outpost: %s", badRequest.Error)
		}
		if conflict != nil {
			return api.Outpost{}, fmt.Errorf("create outpost: %s", conflict.Error)
		}
		return api.Outpost{}, fmt.Errorf("create outpost: unexpected response status: %s", statusText)
	}
	if body == nil {
		return api.Outpost{}, fmt.Errorf("create outpost: response did not contain JSON")
	}
	return *body, nil
}
func lifecycleResponse(operation string, status int, statusText string, body *api.Outpost, notFound, conflict *api.Error) (api.Outpost, error) {
	if status != http.StatusOK {
		if notFound != nil {
			return api.Outpost{}, fmt.Errorf("%s outpost: %s", operation, notFound.Error)
		}
		if conflict != nil {
			return api.Outpost{}, fmt.Errorf("%s outpost: %s", operation, conflict.Error)
		}
		return api.Outpost{}, fmt.Errorf("%s outpost: unexpected response status: %s", operation, statusText)
	}
	if body == nil {
		return api.Outpost{}, fmt.Errorf("%s outpost: response did not contain JSON", operation)
	}
	return *body, nil
}
