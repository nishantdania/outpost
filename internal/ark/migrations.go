package ark

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
		return fmt.Errorf("create migration table: %w", err)
	}

	for _, migration := range migrations {
		var applied bool
		err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", migration.version).Scan(&applied)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", migration.name, err)
		}
		if applied {
			continue
		}

		if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", migration.version); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.name, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}

func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}

	migrations := make([]migration, 0, len(entries))
	versions := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		if existing, found := versions[version]; found {
			return nil, fmt.Errorf("migrations %s and %s have the same version", existing, entry.Name())
		}
		versions[version] = entry.Name()

		contents, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    entry.Name(),
			sql:     string(contents),
		})
	}

	return migrations, nil
}

func migrationVersion(name string) (int, error) {
	versionText, _, found := strings.Cut(name, "_")
	if !found || !strings.HasSuffix(name, ".sql") {
		return 0, fmt.Errorf("migration filename %q must be VERSION_name.sql", name)
	}

	version, err := strconv.Atoi(versionText)
	if err != nil || version <= 0 {
		return 0, fmt.Errorf("migration filename %q has invalid version", name)
	}

	return version, nil
}
