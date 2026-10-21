package client

import (
	"context"
	"io"
	"net/url"

	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) ImportImage(ctx context.Context, tag string, input io.Reader) (result outpost.Image, err error) {
	err = c.request(ctx, "POST", "/images?tag="+url.QueryEscape(tag), input, "application/octet-stream", 201, &result, true)
	return
}
func (c *Client) ListImages(ctx context.Context) (result []outpost.Image, err error) {
	err = c.request(ctx, "GET", "/images", nil, "", 200, &result, false)
	return
}
func (c *Client) GetImage(ctx context.Context, ref string) (result outpost.Image, err error) {
	err = c.request(ctx, "GET", "/images/"+url.PathEscape(ref), nil, "", 200, &result, false)
	return
}
func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	return c.request(ctx, "DELETE", "/images/"+url.PathEscape(ref), nil, "", 204, nil, false)
}
func (c *Client) GCImages(ctx context.Context) (result []string, err error) {
	err = c.request(ctx, "POST", "/images/gc", nil, "", 200, &result, false)
	return
}
