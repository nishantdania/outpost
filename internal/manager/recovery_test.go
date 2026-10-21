package manager

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/catalog"
)

func TestStartupReconcilesUnregisteredArtifactsOnly(t *testing.T) {
	m, runtime := setup(t)
	ctx := context.Background()
	kept, err := m.ImportImage(ctx, strings.NewReader("registered disk"), "base")
	if err != nil {
		t.Fatal(err)
	}
	orphan, _, err := m.artifacts.Publish(ctx, strings.NewReader("registration never completed"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(m.store, runtime, m.artifacts); err != nil {
		t.Fatal(err)
	}
	if _, err := m.artifacts.Open(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan survived reconciliation: %v", err)
	}
	file, err := m.artifacts.Open(kept.Digest)
	if err != nil {
		t.Fatalf("registered image lost: %v", err)
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "registered disk" {
		t.Fatalf("registered image changed: %q %v", data, err)
	}
}

func TestCanceledImportDoesNotRegisterImage(t *testing.T) {
	m, _ := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.ImportImage(ctx, strings.NewReader("disk"), "canceled"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ignored cancellation: %v", err)
	}
	if _, err := m.Image(context.Background(), "canceled"); !errors.Is(err, catalog.ErrImageNotFound) {
		t.Fatalf("registered canceled import: %v", err)
	}
}
