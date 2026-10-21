package outpost

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRecoveryAfterDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	vm, err := store.CreateWith(ctx, CreateInput{Name: "interrupted", ImageID: DefaultImageID, VCPUs: 2, MemoryMiB: 4096, DiskGiB: 8})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.RecoverLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByID(ctx, vm.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusFailed || got.Failure == "" {
		t.Fatalf("interrupted VM not recovered: %+v", got)
	}
	if err := store.RecoverLifecycle(ctx); err != nil {
		t.Fatal(err)
	}
	again, err := store.GetByID(ctx, vm.ID)
	if err != nil || again.Failure != got.Failure {
		t.Fatalf("recovery not idempotent: %+v %v", again, err)
	}
}
