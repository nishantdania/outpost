package outpost

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// schemaApplicationID distinguishes this baseline from the retired migration chain.
const schemaApplicationID = 0x4f555450 // "OUTP"

// ErrUnsupportedSchema rejects other schema lineages and unknown migration histories.
var ErrUnsupportedSchema = errors.New("unsupported database schema; automatic conversion is not supported")

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

	var applicationID int
	if err := tx.QueryRowContext(ctx, "PRAGMA application_id").Scan(&applicationID); err != nil {
		return fmt.Errorf("read schema identity: %w", err)
	}
	fresh := applicationID == 0
	if fresh {
		var objects, userVersion int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE name NOT GLOB 'sqlite_*'`).Scan(&objects); err != nil {
			return fmt.Errorf("inspect database: %w", err)
		}
		if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&userVersion); err != nil {
			return fmt.Errorf("read database version: %w", err)
		}
		if objects != 0 || userVersion != 0 {
			return fmt.Errorf("%w: unrecognized existing database; preserve it and use a fresh database", ErrUnsupportedSchema)
		}
		if _, err := tx.ExecContext(ctx, `CREATE TABLE schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
)`); err != nil {
			return fmt.Errorf("create migration table: %w", err)
		}
	} else if applicationID != schemaApplicationID {
		return fmt.Errorf("%w: unrecognized application identity %d", ErrUnsupportedSchema, applicationID)
	}

	var count, first, latest int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MIN(version), 0), COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&count, &first, &latest); err != nil {
		return fmt.Errorf("%w: invalid migration ledger: %v", ErrUnsupportedSchema, err)
	}
	if (!fresh && count == 0) || (count > 0 && (first != 1 || latest != count)) || latest > len(migrations) {
		return fmt.Errorf("%w: migration ledger is incomplete or newer than this launcher", ErrUnsupportedSchema)
	}

	for _, migration := range migrations {
		if migration.version <= latest {
			continue
		}

		if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES (?)", migration.version); err != nil {
			return fmt.Errorf("record migration %s: %w", migration.name, err)
		}
	}

	if fresh {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA application_id = %d", schemaApplicationID)); err != nil {
			return fmt.Errorf("record schema identity: %w", err)
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
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		version, err := migrationVersion(entry.Name())
		if err != nil {
			return nil, err
		}
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

	if len(migrations) == 0 {
		return nil, errors.New("no database migrations embedded")
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	for i, migration := range migrations {
		if migration.version != i+1 {
			return nil, fmt.Errorf("migration %s must have version %d", migration.name, i+1)
		}
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
