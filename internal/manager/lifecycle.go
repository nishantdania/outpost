package manager

import (
	"context"
	"errors"
	"fmt"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

var ErrInvalidState = errors.New("outpost is not in a state that allows this operation")

func (s *Manager) List(ctx context.Context) ([]outpost.Outpost, error) {
	// The launcher verifies processes and refreshes stable states in the same
	// database before the API reads its canonical VM records.
	if _, err := s.manager.List(ctx); err != nil {
		return nil, err
	}
	return s.store.List(ctx)
}
func (s *Manager) Get(ctx context.Context, name string) (outpost.Outpost, error) {
	a, err := s.store.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if _, err := s.manager.Inspect(ctx, a.ID); err != nil && !errors.Is(err, vmapi.ErrNotFound) {
		return outpost.Outpost{}, err
	}
	return s.store.GetByID(ctx, a.ID)
}

func (s *Manager) Create(ctx context.Context, input outpost.CreateInput) (a outpost.Outpost, err error) {
	pin := func(digest string) error {
		input.ImageID = digest
		var createErr error
		a, createErr = s.store.CreateWith(ctx, input)
		return createErr
	}
	err = s.withPin(ctx, input.ImageID, pin)
	if err != nil {
		return a, err
	}
	if err = s.manager.Create(ctx, a); err != nil {
		return s.fail(ctx, a, err)
	}
	_, err = s.manager.Start(ctx, a.ID)
	if err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusProvisioning, outpost.DesiredRunning, outpost.StatusRunning, "")
}
func (s *Manager) Start(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != outpost.StatusStopped {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, outpost.StatusStopped, outpost.DesiredRunning, outpost.StatusProvisioning, "")
	if err != nil {
		return a, err
	}
	_, err = s.manager.Start(ctx, a.ID)
	if err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusProvisioning, outpost.DesiredRunning, outpost.StatusRunning, "")
}
func (s *Manager) Stop(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status != outpost.StatusRunning {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, outpost.StatusRunning, outpost.DesiredStopped, outpost.StatusStopping, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Stop(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.transition(ctx, a.ID, outpost.StatusStopping, outpost.DesiredStopped, outpost.StatusStopped, "")
}
func (s *Manager) Delete(ctx context.Context, name string) (a outpost.Outpost, err error) {
	a, err = s.Get(ctx, name)
	if err != nil {
		return a, err
	}
	if a.Status == outpost.StatusProvisioning || a.Status == outpost.StatusStopping || a.Status == outpost.StatusDeleting {
		return outpost.Outpost{}, ErrInvalidState
	}
	a, err = s.transition(ctx, a.ID, a.Status, outpost.DesiredDeleted, outpost.StatusDeleting, "")
	if err != nil {
		return a, err
	}
	if err = s.manager.Delete(ctx, a.ID); err != nil {
		return s.fail(ctx, a, err)
	}
	return s.store.DeleteByID(ctx, a.ID)
}
func (s *Manager) transition(ctx context.Context, id, fromStatus, desired, status, failure string) (outpost.Outpost, error) {
	a, err := s.store.Transition(ctx, id, fromStatus, desired, status, failure)
	if errors.Is(err, outpost.ErrStateMismatch) {
		return outpost.Outpost{}, ErrInvalidState
	}
	return a, err
}
func (s *Manager) fail(ctx context.Context, a outpost.Outpost, cause error) (outpost.Outpost, error) {
	failed, stateErr := s.transition(ctx, a.ID, a.Status, a.DesiredState, outpost.StatusFailed, cause.Error())
	if stateErr != nil {
		return outpost.Outpost{}, fmt.Errorf("record lifecycle failure: %w", stateErr)
	}
	return failed, fmt.Errorf("manage outpost: %w", cause)
}
