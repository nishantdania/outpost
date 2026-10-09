package service

import (
	"context"

	"github.com/nishantdania/outpost/internal/outpost"
)

func (s *Service) Hosts(ctx context.Context) ([]outpost.Host, error) {
	return s.store.Hosts(ctx)
}

func (s *Service) Host(ctx context.Context, hostname string) (outpost.Host, error) {
	return s.store.Host(ctx, hostname)
}

func (s *Service) SetHost(ctx context.Context, hostname, name string, port int) (outpost.Host, error) {
	return s.store.SetHost(ctx, hostname, name, port)
}

func (s *Service) Unhost(ctx context.Context, hostname string) error {
	return s.store.Unhost(ctx, hostname)
}
