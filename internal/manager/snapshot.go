package manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/nishantdania/outpost/internal/catalog"
	"github.com/nishantdania/outpost/internal/outpost"
)

// Snapshot saves a stopped VM's disk as a reusable image, not memory/device state.
func (s *Manager) Snapshot(ctx context.Context, name, tag string) (_ outpost.Image, resultErr error) {
	if !catalog.ValidImageTag(tag) || tag == catalog.DefaultImageID {
		return outpost.Image{}, catalog.ErrInvalidImage
	}
	vm, err := s.Get(ctx, name)
	if err != nil {
		return outpost.Image{}, err
	}
	if vm.Status != outpost.StatusStopped {
		return outpost.Image{}, ErrInvalidState
	}
	// Host execution rechecks stopped state under its lifecycle lock, and returns
	// a private copy so a later start/delete cannot change the captured bytes.
	disk, err := s.manager.OpenSnapshot(ctx, vm.ID)
	if err != nil {
		return outpost.Image{}, err
	}
	defer func() {
		if err := disk.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close snapshot export: %w", err))
		}
	}()
	return s.ImportImage(ctx, disk, tag)
}
