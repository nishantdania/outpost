package launcher

import (
	"context"

	"github.com/nishantdania/outpost/internal/vmapi"
)

type Runtime interface {
	Create(context.Context, vmapi.VMSpec) error
	Start(context.Context, string) (string, error)
	Stop(context.Context, string) error
	Delete(context.Context, string) error
	Inspect(context.Context, string) (vmapi.RuntimeState, error)
	List(context.Context) ([]vmapi.RuntimeState, error)
}
