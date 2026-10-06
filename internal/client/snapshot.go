package client

import (
	"context"
	"fmt"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) SnapshotOutpost(ctx context.Context, name, tag string) (outpost.Image, error) {
	response, err := c.api.SnapshotOutpostWithResponse(ctx, name, &api.SnapshotOutpostParams{Tag: tag})
	if err != nil {
		return outpost.Image{}, err
	}
	if response.JSON201 == nil {
		return outpost.Image{}, fmt.Errorf("snapshot outpost: %s: %s", response.Status(), response.Body)
	}
	return image(*response.JSON201), nil
}
