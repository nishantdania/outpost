package ark

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestStoreValidatesResourceBounds(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, input := range []CreateInput{
		{Name: "minimum", ImageID: DefaultImageID, VCPUs: MinVCPUs, MemoryMiB: MinMemoryMiB, DiskGiB: MinDiskGiB},
		{Name: "maximum", ImageID: DefaultImageID, VCPUs: MaxVCPUs, MemoryMiB: MaxMemoryMiB, DiskGiB: MaxDiskGiB},
	} {
		if _, err := store.CreateWith(context.Background(), input); err != nil {
			t.Fatalf("CreateWith(%#v) error = %v", input, err)
		}
	}
	for _, input := range []CreateInput{
		{Name: "cpu-low", ImageID: DefaultImageID, VCPUs: MinVCPUs - 1, MemoryMiB: DefaultMemoryMiB, DiskGiB: DefaultDiskGiB},
		{Name: "cpu-high", ImageID: DefaultImageID, VCPUs: MaxVCPUs + 1, MemoryMiB: DefaultMemoryMiB, DiskGiB: DefaultDiskGiB},
		{Name: "memory-low", ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: MinMemoryMiB - 1, DiskGiB: DefaultDiskGiB},
		{Name: "memory-high", ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: MaxMemoryMiB + 1, DiskGiB: DefaultDiskGiB},
		{Name: "disk-low", ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: DefaultMemoryMiB, DiskGiB: MinDiskGiB - 1},
		{Name: "disk-high", ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: DefaultMemoryMiB, DiskGiB: MaxDiskGiB + 1},
	} {
		if _, err := store.CreateWith(context.Background(), input); !errors.Is(err, ErrInvalidResources) {
			t.Fatalf("CreateWith(%#v) error = %v, want %v", input, err, ErrInvalidResources)
		}
	}
}
