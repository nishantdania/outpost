package manager

import (
	"context"
	"io"

	"github.com/nishantdania/outpost/internal/catalog"
	"github.com/nishantdania/outpost/internal/outpost"
)

func (s *Manager) Images(ctx context.Context) ([]outpost.Image, error) {
	return s.store.Catalog.ListImages(ctx)
}
func (s *Manager) Image(ctx context.Context, ref string) (outpost.Image, error) {
	return s.store.Catalog.GetImage(ctx, ref)
}

// ImportImage publishes bytes before registering metadata. Imports and disk-only
// snapshots share this path; neither modifies or inspects the guest filesystem.
func (s *Manager) ImportImage(ctx context.Context, disk io.Reader, tag string) (outpost.Image, error) {
	if !catalog.ValidImageTag(tag) || tag == catalog.DefaultImageID {
		return outpost.Image{}, catalog.ErrInvalidImage
	}
	s.imagesMu.Lock()
	defer s.imagesMu.Unlock()
	digest, size, err := s.artifacts.Publish(ctx, disk)
	if err != nil {
		return outpost.Image{}, err
	}
	if err := s.store.Catalog.PutImage(ctx, digest, size, tag); err != nil {
		return outpost.Image{}, err
	}
	return s.store.Catalog.GetImage(ctx, digest)
}

func (s *Manager) RemoveImage(ctx context.Context, ref string) error {
	s.imagesMu.Lock()
	defer s.imagesMu.Unlock()
	image, err := s.store.Catalog.GetImage(ctx, ref)
	if err != nil {
		return err
	}
	if err := s.store.Catalog.RemoveImage(ctx, ref); err != nil {
		return err
	}
	if catalog.ValidDigest(ref) {
		return s.artifacts.Remove(image.Digest)
	}
	return nil
}

func (s *Manager) GCImages(ctx context.Context) ([]string, error) {
	s.imagesMu.Lock()
	defer s.imagesMu.Unlock()
	ids, err := s.store.Catalog.GarbageCollectImages(ctx)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := s.artifacts.Remove(id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// withPin keeps tag resolution + VM reference insertion atomic with respect to
// image deletion/GC. Existing VMs always reference immutable digests, not tags.
func (s *Manager) withPin(ctx context.Context, ref string, pin func(string) error) error {
	s.imagesMu.Lock()
	defer s.imagesMu.Unlock()
	digest, err := s.store.Catalog.ResolveImage(ctx, ref)
	if err != nil {
		return err
	}
	return pin(digest)
}
