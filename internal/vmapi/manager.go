package vmapi

import (
	"context"
	"errors"

	"github.com/nishantdania/outpost/internal/outpost"
)

var (
	ErrNotFound = errors.New("VM not found")
	ErrInvalid  = errors.New("invalid VM request")
	ErrConflict = errors.New("VM specification conflicts with existing VM")
)

type Manager interface {
	Create(context.Context, outpost.Outpost) error
	Start(context.Context, string) (string, error)
	Stop(context.Context, string) error
	Delete(context.Context, string) error
}
