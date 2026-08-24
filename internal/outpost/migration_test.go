package outpost

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationPreservesExistingOutposts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outpost.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE outposts (id TEXT PRIMARY KEY, name TEXT NOT NULL); CREATE UNIQUE INDEX outposts_name_unique ON outposts (name COLLATE NOCASE); CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP); INSERT INTO schema_migrations (version) VALUES (1), (2); INSERT INTO outposts (id, name) VALUES ('old-id', 'old');`)
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
		t.Fatalf("migrated Outpost = %#v", a)
	}
}
