package service

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

func TestLifecycleTransitions(t *testing.T) {
	store := testStore(t)
	manager := &vmapi.FakeManager{StartFunc: func(context.Context, string) (string, error) { return "172.30.0.2", nil }}
	application := New(store, manager)
	created, err := application.Create(context.Background(), input("demo"))
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != outpost.StatusRunning || created.DesiredState != outpost.DesiredRunning || created.GuestIP != "172.30.0.2" {
		t.Fatalf("Create() = %#v, want running Outpost", created)
	}
	stopped, err := application.Stop(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if stopped.Status != outpost.StatusStopped || stopped.DesiredState != outpost.DesiredStopped || stopped.GuestIP != "172.30.0.2" {
		t.Fatalf("Stop() = %#v, want stopped Outpost retaining guest IP", stopped)
	}
	started, err := application.Start(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if started.Status != outpost.StatusRunning || started.Failure != "" {
		t.Fatalf("Start() = %#v, want running Outpost without failure", started)
	}
	deleted, err := application.Delete(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted.ID != created.ID {
		t.Fatalf("Delete() = %#v, want %#v", deleted, created)
	}
	if _, err := application.Get(context.Background(), "demo"); !errors.Is(err, outpost.ErrNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, outpost.ErrNotFound)
	}
	wantCalls := []string{"create:" + created.ID, "start:" + created.ID, "stop:" + created.ID, "start:" + created.ID, "delete:" + created.ID}
	if !reflect.DeepEqual(manager.Calls, wantCalls) {
		t.Fatalf("manager calls = %v, want %v", manager.Calls, wantCalls)
	}
}

func TestFailureIsRetainedAndInvalidTransitionDoesNotCallManager(t *testing.T) {
	store := testStore(t)
	manager := &vmapi.FakeManager{StartFunc: func(context.Context, string) (string, error) { return "", errors.New("boot failed") }}
	application := New(store, manager)
	if _, err := application.Create(context.Background(), input("demo")); err == nil {
		t.Fatal("Create() error = nil, want error")
	}
	failed, err := application.Get(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if failed.Status != outpost.StatusFailed || failed.DesiredState != outpost.DesiredRunning || failed.Failure != "boot failed" {
		t.Fatalf("failed Outpost = %#v", failed)
	}
	if _, err := application.Stop(context.Background(), "demo"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("Stop() error = %v, want %v", err, ErrInvalidState)
	}
	if len(manager.Calls) != 2 {
		t.Fatalf("manager calls = %v, want create and start only", manager.Calls)
	}
}

func TestStaleTransitionIsRejected(t *testing.T) {
	store := testStore(t)
	created, err := store.Create(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Transition(context.Background(), created.ID, outpost.StatusStopped, outpost.DesiredRunning, outpost.StatusProvisioning, "", ""); err != nil {
		t.Fatal(err)
	}
	application := New(store, &vmapi.FakeManager{})
	if _, err := application.transition(context.Background(), created.ID, outpost.StatusStopped, outpost.DesiredRunning, outpost.StatusProvisioning, "", ""); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("transition error = %v, want %v", err, ErrInvalidState)
	}
}

func input(name string) outpost.CreateInput {
	return outpost.CreateInput{Name: name, ImageID: "default", VCPUs: 2, MemoryMiB: 2048, DiskGiB: 20}
}
func testStore(t *testing.T) *outpost.Store {
	t.Helper()
	store, err := outpost.Open(context.Background(), filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
