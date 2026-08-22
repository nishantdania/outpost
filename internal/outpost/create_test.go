package outpost

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCreateResources(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	record, err := service.Create(context.Background(), "build", Resources{VCPUs: 4, MemoryMiB: 8192, DiskGiB: 32})
	if err != nil {
		t.Fatal(err)
	}
	if record.VCPUs != 4 || record.MemoryMiB != 8192 || record.DiskGiB != 32 {
		t.Fatalf("record = %#v", record)
	}
}

func TestCreateRejectsInvalidResources(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	if _, err := service.Create(context.Background(), "bad", Resources{VCPUs: 33}); err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateAndList(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	record, err := service.Create(context.Background(), "test", Resources{})
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "test" || record.ID == "" || record.VCPUs != 2 || record.MemoryMiB != 4096 || record.DiskGiB != 8 {
		t.Fatalf("record = %#v", record)
	}
	if _, err := service.Create(context.Background(), "test", Resources{}); err == nil {
		t.Fatal("expected duplicate name error")
	}
	records, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ID != record.ID {
		t.Fatalf("records = %#v", records)
	}
	deleted, err := service.Delete(context.Background(), record.Name)
	if err != nil || !deleted {
		t.Fatalf("Delete() = %v, %v", deleted, err)
	}
	records, err = service.List(context.Background())
	if err != nil || len(records) != 0 {
		t.Fatalf("records after delete = %#v, %v", records, err)
	}
}
