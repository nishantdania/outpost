package client

import (
	"context"
	"net/url"

	"github.com/nishantdania/outpost/internal/api"
)

type CreateOutpostInput struct {
	Name, ImageID             string
	VCPUs, MemoryMiB, DiskGiB int
}

func (c *Client) CreateOutpostWith(ctx context.Context, input CreateOutpostInput) (result api.Outpost, err error) {
	body, err := jsonBody(api.CreateOutpostRequest{Name: input.Name, ImageId: input.ImageID, Vcpus: input.VCPUs, MemoryMib: input.MemoryMiB, DiskGib: input.DiskGiB})
	if err != nil {
		return result, err
	}
	err = c.request(ctx, "POST", "/outposts", body, "application/json", 201, &result, false)
	return
}

func (c *Client) DeleteOutpost(ctx context.Context, name string) (result api.Outpost, err error) {
	err = c.request(ctx, "DELETE", "/outposts/"+url.PathEscape(name), nil, "", 200, &result, false)
	return
}
func (c *Client) GetOutpost(ctx context.Context, name string) (result api.Outpost, err error) {
	err = c.request(ctx, "GET", "/outposts/"+url.PathEscape(name), nil, "", 200, &result, false)
	return
}
func (c *Client) StartOutpost(ctx context.Context, name string) (result api.Outpost, err error) {
	err = c.request(ctx, "POST", "/outposts/"+url.PathEscape(name)+"/start", nil, "", 200, &result, false)
	return
}
func (c *Client) StopOutpost(ctx context.Context, name string) (result api.Outpost, err error) {
	err = c.request(ctx, "POST", "/outposts/"+url.PathEscape(name)+"/stop", nil, "", 200, &result, false)
	return
}
func (c *Client) ListOutposts(ctx context.Context) (result []api.Outpost, err error) {
	err = c.request(ctx, "GET", "/outposts", nil, "", 200, &result, false)
	return
}
