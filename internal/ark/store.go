package ark

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrNameRequired = errors.New("ark name is required")
	ErrNameTaken    = errors.New("ark name is already in use")
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if err := createDatabaseDirectory(databasePath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Create(ctx context.Context, name string) (Ark, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Ark{}, ErrNameRequired
	}

	ark := Ark{
		ID:   uuid.NewString(),
		Name: name,
	}
	result, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO arks (id, name) VALUES (?, ?)", ark.ID, ark.Name)
	if err != nil {
		return Ark{}, fmt.Errorf("insert ark: %w", err)
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return Ark{}, fmt.Errorf("check inserted ark: %w", err)
	}
	if inserted == 0 {
		return Ark{}, ErrNameTaken
	}

	return ark, nil
}

func (s *Store) List(ctx context.Context) ([]Ark, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name FROM arks ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("query arks: %w", err)
	}
	defer rows.Close()

	arks := make([]Ark, 0)
	for rows.Next() {
		var ark Ark
		if err := rows.Scan(&ark.ID, &ark.Name); err != nil {
			return nil, fmt.Errorf("scan ark: %w", err)
		}
		arks = append(arks, ark)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate arks: %w", err)
	}

	return arks, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func createDatabaseDirectory(databasePath string) error {
	if databasePath == ":memory:" || strings.HasPrefix(databasePath, "file:") {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}

	return nil
}
