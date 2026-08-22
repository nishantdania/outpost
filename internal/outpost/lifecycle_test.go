package outpost

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStopByName(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	record, err := service.Create(context.Background(), "dev", Resources{})
	if err != nil {
		t.Fatal(err)
	}
	stopped, err := service.Stop(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if stopped.ID != record.ID || stopped.Status != "stopped" {
		t.Fatalf("record = %#v", stopped)
	}
}

func TestStartByName(t *testing.T) {
	service := New(filepath.Join(t.TempDir(), "outposts.json"))
	record := Record{ID: "id", Name: "dev", Status: "running", PID: os.Getpid(), Resources: defaultResources(Resources{}), CreatedAt: time.Now()}
	if err := service.save([]Record{record}); err != nil {
		t.Fatal(err)
	}
	started, err := service.Start(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if started.ID != record.ID {
		t.Fatalf("record = %#v", started)
	}
}
