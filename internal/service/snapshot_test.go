package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/nishantdania/outpost/internal/image"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/testutil"
)

func TestSnapshotRequiresImagesAndSnapshotCapableManager(t *testing.T) {
	db, err := outpost.Open(t.Context(), filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	app := New(db, &testutil.FakeManager{})
	if _, err := app.Snapshot(t.Context(), "dev", "ready"); !errors.Is(err, ErrImagesUnavailable) {
		t.Fatalf("without images: %v", err)
	}
	images, err := image.New(filepath.Join(t.TempDir(), "images"), db, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.WithImages(images)
	for _, tag := range []string{"", "default", "../escape"} {
		if _, err := app.Snapshot(t.Context(), "dev", tag); !errors.Is(err, outpost.ErrInvalidImage) {
			t.Fatalf("tag %q: %v", tag, err)
		}
	}
	if _, err := app.Snapshot(t.Context(), "missing", "ready"); !errors.Is(err, outpost.ErrNotFound) {
		t.Fatalf("missing VM: %v", err)
	}
	if _, err := app.Create(t.Context(), outpost.CreateInput{Name: "dev", ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Snapshot(t.Context(), "dev", "ready"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("running VM: %v", err)
	}
	if _, err := app.Stop(t.Context(), "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Snapshot(t.Context(), "dev", "ready"); !errors.Is(err, ErrImagesUnavailable) {
		t.Fatalf("unsupported manager: %v", err)
	}
}
