package service

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/nishantdania/outpost/internal/image"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

var ErrInvalidState = errors.New("outpost is not in a state that allows this operation")
var ErrImagesUnavailable = errors.New("image operations are unavailable")

type Service struct {
	store   *outpost.Store
	manager vmapi.Manager
	images  *image.Store
}

func New(store *outpost.Store, manager vmapi.Manager) *Service {
	return &Service{store: store, manager: manager}
}
func (s *Service) WithImages(images *image.Store) *Service { s.images = images; return s }
func (s *Service) Images(ctx context.Context) ([]outpost.Image, error) {
	if s.images == nil {
		return nil, ErrImagesUnavailable
	}
	return s.store.ListImages(ctx)
}
func (s *Service) Image(ctx context.Context, ref string) (outpost.Image, error) {
	if s.images == nil {
		return outpost.Image{}, ErrImagesUnavailable
	}
	return s.store.GetImage(ctx, ref)
}
func (s *Service) BuildImage(ctx context.Context, input io.Reader, tag string) (outpost.Image, error) {
	if s.images == nil {
		return outpost.Image{}, ErrImagesUnavailable
	}
	return s.images.Build(ctx, input, tag)
}
func (s *Service) ImportImage(ctx context.Context, input io.Reader, tag string) (outpost.Image, error) {
	if s.images == nil {
		return outpost.Image{}, ErrImagesUnavailable
	}
	return s.images.Import(ctx, input, tag)
}
func (s *Service) RemoveImage(ctx context.Context, ref string) error {
	if s.images == nil {
		return ErrImagesUnavailable
	}
	return s.images.Remove(ctx, ref)
}
func (s *Service) GCImages(ctx context.Context) ([]string, error) {
	if s.images == nil {
		return nil, ErrImagesUnavailable
	}
	return s.images.GC(ctx)
}
func (s *Service) List(ctx context.Context) ([]outpost.Outpost, error) { return s.store.List(ctx) }
func (s *Service) Get(ctx context.Context, name string) (outpost.Outpost, error) {
	return s.store.Get(ctx, name)
}

func (s *Service) Create(ctx context.Context, input outpost.CreateInput) (a outpost.Outpost, err error) {
	input.ImageID, err = s.store.ResolveImage(ctx, input.ImageID)
	if err != nil {
		return a, err
	}
	a, err = s.store.CreateWith(ctx, input)
	if err != nil {
		return a, err
	}
	if err = s.manager.Create(ctx, a); err != nil {
		return s.fail(ctx, a, err)
	}
	guestIP, err := s.manager.Start(ctx, a.ID)
	if err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusProvisioning, outpost.DesiredRunning, outpost.StatusRunning, guestIP, "")
}
func (s *Service) Start(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != outpost.StatusStopped {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, outpost.StatusStopped, outpost.DesiredRunning, outpost.StatusProvisioning, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	guestIP, err := s.manager.Start(ctx, a.ID)
	if err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusProvisioning, outpost.DesiredRunning, outpost.StatusRunning, guestIP, "")
}
func (s *Service) Stop(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != outpost.StatusRunning {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, outpost.StatusRunning, outpost.DesiredStopped, outpost.StatusStopping, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Stop(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusStopping, outpost.DesiredStopped, outpost.StatusStopped, a.GuestIP, "")
}
func (s *Service) Delete(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status == outpost.StatusProvisioning || a.Status == outpost.StatusStopping || a.Status == outpost.StatusDeleting {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, a.Status, outpost.DesiredDeleted, outpost.StatusDeleting, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Delete(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.store.DeleteByID(ctx, a.ID)
}
func (s *Service) transition(ctx context.Context, id, fromStatus, desired, status, guestIP, failure string) (outpost.Outpost, error) {
	a, err := s.store.Transition(ctx, id, fromStatus, desired, status, guestIP, failure)
	if errors.Is(err, outpost.ErrStateMismatch) {
		return outpost.Outpost{}, ErrInvalidState
	}
	return a, err
}
func (s *Service) fail(ctx context.Context, a outpost.Outpost, cause error) (outpost.Outpost, error) {
	failed, stateErr := s.transition(ctx, a.ID, a.Status, a.DesiredState, outpost.StatusFailed, a.GuestIP, cause.Error())
	if stateErr != nil {
		return outpost.Outpost{}, fmt.Errorf("record lifecycle failure: %w", stateErr)
	}
	return failed, fmt.Errorf("manage outpost: %w", cause)
}
