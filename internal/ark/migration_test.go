package ark

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationPreservesExistingArks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ark.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE arks (id TEXT PRIMARY KEY, name TEXT NOT NULL); CREATE UNIQUE INDEX arks_name_unique ON arks (name COLLATE NOCASE); CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO schema_migrations (version) VALUES (1), (2); INSERT INTO arks (id, name) VALUES ('old-id', 'old');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	a, err := store.Get(context.Background(), "old")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if a.ImageID != DefaultImageID || a.VCPUs != DefaultVCPUs || a.MemoryMiB != DefaultMemoryMiB || a.DiskGiB != DefaultDiskGiB || a.Status != StatusStopped || a.DesiredState != DesiredStopped || a.CreatedAt.IsZero() || a.UpdatedAt.IsZero() {
		t.Fatalf("migrated Ark = %#v", a)
	}
}
