package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/vmapi"
)

var ErrInvalidState = errors.New("ark is not in a state that allows this operation")

type Service struct {
	store   *ark.Store
	manager vmapi.Manager
}

func New(store *ark.Store, manager vmapi.Manager) *Service {
	return &Service{store: store, manager: manager}
}
func (s *Service) List(ctx context.Context) ([]ark.Ark, error) { return s.store.List(ctx) }
func (s *Service) Get(ctx context.Context, name string) (ark.Ark, error) {
	return s.store.Get(ctx, name)
}

func (s *Service) Create(ctx context.Context, input ark.CreateInput) (a ark.Ark, err error) {
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
	return s.transition(ctx, a.ID, ark.StatusProvisioning, ark.DesiredRunning, ark.StatusRunning, guestIP, "")
}
func (s *Service) Start(ctx context.Context, name string) (a ark.Ark, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != ark.StatusStopped {
		return ark.Ark{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, ark.StatusStopped, ark.DesiredRunning, ark.StatusProvisioning, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	guestIP, err := s.manager.Start(ctx, a.ID)
	if err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, ark.StatusProvisioning, ark.DesiredRunning, ark.StatusRunning, guestIP, "")
}
func (s *Service) Stop(ctx context.Context, name string) (a ark.Ark, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != ark.StatusRunning {
		return ark.Ark{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, ark.StatusRunning, ark.DesiredStopped, ark.StatusStopping, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Stop(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, ark.StatusStopping, ark.DesiredStopped, ark.StatusStopped, a.GuestIP, "")
}
func (s *Service) Delete(ctx context.Context, name string) (a ark.Ark, err error) {
	a, err = s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status == ark.StatusProvisioning || a.Status == ark.StatusStopping || a.Status == ark.StatusDeleting {
		return ark.Ark{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, a.Status, ark.DesiredDeleted, ark.StatusDeleting, a.GuestIP, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Delete(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.store.DeleteByID(ctx, a.ID)
}
func (s *Service) transition(ctx context.Context, id, fromStatus, desired, status, guestIP, failure string) (ark.Ark, error) {
	a, err := s.store.Transition(ctx, id, fromStatus, desired, status, guestIP, failure)
	if errors.Is(err, ark.ErrStateMismatch) {
		return ark.Ark{}, ErrInvalidState
	}
	return a, err
}
func (s *Service) fail(ctx context.Context, a ark.Ark, cause error) (ark.Ark, error) {
	failed, stateErr := s.transition(ctx, a.ID, a.Status, a.DesiredState, ark.StatusFailed, a.GuestIP, cause.Error())
	if stateErr != nil {
		return ark.Ark{}, fmt.Errorf("record lifecycle failure: %w", stateErr)
	}
	return failed, fmt.Errorf("manage ark: %w", cause)
}
