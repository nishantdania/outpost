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
