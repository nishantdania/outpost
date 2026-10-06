package service

import (
	"context"
	"io"

	"github.com/nishantdania/outpost/internal/outpost"
)

func (s *Service) Snapshot(ctx context.Context, name, tag string) (outpost.Image, error) {
	if s.images == nil {
		return outpost.Image{}, ErrImagesUnavailable
	}
	if !outpost.ValidImageTag(tag) || tag == outpost.DefaultImageID {
		return outpost.Image{}, outpost.ErrInvalidImage
	}
	a, err := s.store.Get(ctx, name)
	if err != nil {
		return outpost.Image{}, err
	}
	if a.Status != outpost.StatusStopped {
		return outpost.Image{}, ErrInvalidState
	}
	manager, ok := s.manager.(interface {
		OpenSnapshot(context.Context, string) (io.ReadCloser, error)
	})
	if !ok {
		return outpost.Image{}, ErrImagesUnavailable
	}
	disk, err := manager.OpenSnapshot(ctx, a.ID)
	if err != nil {
		return outpost.Image{}, err
	}
	defer disk.Close()
	return s.images.ImportSnapshot(ctx, disk, tag)
}
