package client

import (
	"context"
	"net/url"

	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) SnapshotOutpost(ctx context.Context, name, tag string) (result outpost.Image, err error) {
	err = c.request(ctx, "POST", "/outposts/"+url.PathEscape(name)+"/snapshot?tag="+url.QueryEscape(tag), nil, "", 201, &result, true)
	return
}
