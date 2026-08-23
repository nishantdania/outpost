package ark

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
)

func TestStoreCreatesAndListsArks(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "ark.db")

	store, err := Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	created, err := store.Create(ctx, "demo")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := uuid.Parse(created.ID); err != nil {
		t.Fatalf("created ID = %q, want UUID: %v", created.ID, err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	store, err = Open(ctx, databasePath)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer store.Close()

	arks, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if want := []Ark{created}; !reflect.DeepEqual(arks, want) {
		t.Fatalf("List() = %v, want %v", arks, want)
	}
}

func TestStoreDeletesArkByName(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	created, err := store.Create(context.Background(), "demo")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	deleted, err := store.Delete(context.Background(), "Demo")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleted != created {
		t.Fatalf("Delete() = %v, want %v", deleted, created)
	}

	_, err = store.Delete(context.Background(), "demo")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete() error = %v, want %v", err, ErrNotFound)
	}
}

func TestStoreRejectsDuplicateName(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Create(context.Background(), "demo"); err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	_, err = store.Create(context.Background(), "Demo")
	if !errors.Is(err, ErrNameTaken) {
		t.Fatalf("second Create() error = %v, want %v", err, ErrNameTaken)
	}
}

func TestStoreRejectsMalformedAndMismatchedSSHPublicKeys(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, key := range []string{
		"ssh-ed25519 !!!",
		"ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		_, err := store.CreateWith(context.Background(), CreateInput{Name: "demo", ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: DefaultMemoryMiB, DiskGiB: DefaultDiskGiB, SSHPublicKey: key})
		if !errors.Is(err, ErrInvalidSSHPublicKey) {
			t.Fatalf("key %q error = %v", key, err)
		}
	}
}

func TestStoreRejectsBlankName(t *testing.T) {
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	_, err = store.Create(context.Background(), "   ")
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf("Create() error = %v, want %v", err, ErrNameRequired)
	}
}

func TestSchemaMatchesMigrations(t *testing.T) {
	store, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	got, err := store.Schema(context.Background())
	if err != nil {
		t.Fatalf("Schema() error = %v", err)
	}

	want, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}

	if got != string(want) {
		t.Fatalf("schema.sql is stale; run go generate ./internal/ark\ngot:\n%s\nwant:\n%s", got, want)
	}
}
