package launcher

import (
	"context"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

// Manager adapts lifecycle creation input to the in-process Firecracker runtime.
// All other operations, including snapshot export, are direct method calls.
type Manager struct{ *FirecrackerRuntime }

var _ vmapi.Manager = Manager{}

func (m Manager) Create(ctx context.Context, vm outpost.Outpost) error {
	return m.FirecrackerRuntime.Create(ctx, runtimeView(outpost.Runtime{VM: vm}).Spec)
}
