package outpost

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCreateAndList(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	record, err := service.Create(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	if record.Name != "test" || record.ID == "" {
		t.Fatalf("record = %#v", record)
	}
	if _, err := service.Create(context.Background(), "test"); err == nil {
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
