package launcher

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func TestMemoryRuntimeCreateCancellationAndConflict(t *testing.T) {
	runtime := NewMemoryRuntime()
	spec := vmapi.VMSpec{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runtime.Create(ctx, spec); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Create() error = %v", err)
	}
	if err := runtime.Create(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	if err := runtime.Create(context.Background(), spec); err != nil {
		t.Fatalf("idempotent Create() error = %v", err)
	}
	spec.VCPUs++
	if err := runtime.Create(context.Background(), spec); !errors.Is(err, vmapi.ErrConflict) {
		t.Fatalf("conflicting Create() error = %v", err)
	}
}

func TestMemoryRuntimeHonorsCancellationAndMissingVMs(t *testing.T) {
	runtime := NewMemoryRuntime()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := runtime.Start(ctx, "missing"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Start() error = %v", err)
	}
	if err := runtime.Stop(context.Background(), "missing"); !errors.Is(err, vmapi.ErrNotFound) {
		t.Fatalf("missing Stop() error = %v", err)
	}
}
