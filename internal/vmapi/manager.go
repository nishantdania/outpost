package vmapi

import (
	"context"
	"errors"

	"github.com/nishantdania/outpost/internal/outpost"
)

var (
	ErrUnavailable = errors.New("VM manager is not configured")
	ErrNotFound    = errors.New("VM not found")
	ErrInvalid     = errors.New("invalid VM request")
	ErrConflict    = errors.New("VM specification conflicts with existing VM")
)

type Manager interface {
	Create(context.Context, outpost.Outpost) error
	Start(context.Context, string) (string, error)
	Stop(context.Context, string) error
	Delete(context.Context, string) error
}

type UnavailableManager struct{}

func (UnavailableManager) Create(context.Context, outpost.Outpost) error { return ErrUnavailable }
func (UnavailableManager) Start(context.Context, string) (string, error) { return "", ErrUnavailable }
func (UnavailableManager) Stop(context.Context, string) error            { return ErrUnavailable }
func (UnavailableManager) Delete(context.Context, string) error          { return ErrUnavailable }

type FakeManager struct {
	CreateFunc func(context.Context, outpost.Outpost) error
	StartFunc  func(context.Context, string) (string, error)
	StopFunc   func(context.Context, string) error
	DeleteFunc func(context.Context, string) error
	Calls      []string
}

func (m *FakeManager) Create(ctx context.Context, a outpost.Outpost) error {
	m.Calls = append(m.Calls, "create:"+a.ID)
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, a)
	}
	return nil
}
func (m *FakeManager) Start(ctx context.Context, id string) (string, error) {
	m.Calls = append(m.Calls, "start:"+id)
	if m.StartFunc != nil {
		return m.StartFunc(ctx, id)
	}
	return "", nil
}
func (m *FakeManager) Stop(ctx context.Context, id string) error {
	m.Calls = append(m.Calls, "stop:"+id)
	if m.StopFunc != nil {
		return m.StopFunc(ctx, id)
	}
	return nil
}
func (m *FakeManager) Delete(ctx context.Context, id string) error {
	m.Calls = append(m.Calls, "delete:"+id)
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}
