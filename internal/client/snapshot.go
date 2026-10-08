package client

import (
	"context"
	"fmt"
	"strings"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) SnapshotOutpost(ctx context.Context, name, tag string) (outpost.Image, error) {
	return c.SnapshotOutpostWithCredentials(ctx, name, tag, nil)
}

func (c *Client) SnapshotOutpostWithCredentials(ctx context.Context, name, tag string, profile *credentials.Profile) (outpost.Image, error) {
	var body *strings.Reader
	if profile == nil {
		body = strings.NewReader("")
	} else {
		if err := profile.Validate(); err != nil {
			return outpost.Image{}, err
		}
		body = strings.NewReader(profile.JSON())
	}
	response, err := c.snapshotAPI.SnapshotOutpostWithBodyWithResponse(ctx, name, &api.SnapshotOutpostParams{Tag: tag}, "application/json", body)
	if err != nil {
		return outpost.Image{}, err
	}
	if response.JSON201 == nil {
		return outpost.Image{}, fmt.Errorf("snapshot outpost: %s: %s", response.Status(), response.Body)
	}
	if profile != nil && (response.JSON201.CredentialProfile == nil || response.JSON201.CredentialProfile.JSON() != profile.JSON()) {
		return outpost.Image{}, fmt.Errorf("server did not persist the requested credential profile; snapshot may have been created, inspect it and upgrade the server before use")
	}
	return image(*response.JSON201), nil
}
