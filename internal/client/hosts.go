package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (c *Client) SetHost(ctx context.Context, hostname, name string, port int) (api.Host, error) {
	response, err := c.api.SetHostWithResponse(ctx, hostname, api.SetHostRequest{OutpostName: name, Port: port})
	if err != nil {
		return api.Host{}, fmt.Errorf("register host: %w", err)
	}
	if response.StatusCode() != http.StatusOK {
		for _, failure := range []*api.Error{response.JSON400, response.JSON404, response.JSON409} {
			if failure != nil {
				return api.Host{}, fmt.Errorf("register host: %s", failure.Error)
			}
		}
		return api.Host{}, fmt.Errorf("register host: unexpected response status: %s", response.Status())
	}
	if response.JSON200 == nil {
		return api.Host{}, fmt.Errorf("register host: response did not contain JSON")
	}
	return *response.JSON200, nil
}

func (c *Client) Hosts(ctx context.Context) ([]api.Host, error) {
	response, err := c.api.ListHostsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("list hosts: %w", err)
	}
	if response.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("list hosts: unexpected response status: %s", response.Status())
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("list hosts: response did not contain JSON")
	}
	return *response.JSON200, nil
}

func (c *Client) Host(ctx context.Context, hostname string) (outpost.Host, error) {
	response, err := c.api.GetHostWithResponse(ctx, hostname)
	if err != nil {
		return outpost.Host{}, fmt.Errorf("resolve host: %w", err)
	}
	if response.StatusCode() == http.StatusNotFound {
		return outpost.Host{}, outpost.ErrHostNotFound
	}
	if response.StatusCode() != http.StatusOK {
		return outpost.Host{}, fmt.Errorf("resolve host: unexpected response status: %s", response.Status())
	}
	if response.JSON200 == nil {
		return outpost.Host{}, fmt.Errorf("resolve host: response did not contain JSON")
	}
	host := *response.JSON200
	return outpost.Host{Hostname: host.Hostname, OutpostID: host.OutpostId, OutpostName: host.OutpostName,
		Port: host.Port, GuestIP: host.GuestIp, Status: host.Status, DesiredState: host.DesiredState}, nil
}

func (c *Client) Unhost(ctx context.Context, hostname string) error {
	response, err := c.api.UnhostWithResponse(ctx, hostname)
	if err != nil {
		return fmt.Errorf("remove host: %w", err)
	}
	if response.StatusCode() == http.StatusNoContent {
		return nil
	}
	for _, failure := range []*api.Error{response.JSON400, response.JSON404} {
		if failure != nil {
			return fmt.Errorf("remove host: %s", failure.Error)
		}
	}
	return fmt.Errorf("remove host: unexpected response status: %s", response.Status())
}
