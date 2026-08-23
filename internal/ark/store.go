package ark

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrNameRequired     = errors.New("ark name is required")
	ErrNameTaken        = errors.New("ark name is already in use")
	ErrNotFound         = errors.New("ark not found")
	ErrInvalidResources = errors.New("ark resources must be positive")
)

type Store struct{ db *sql.DB }

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
	a, err := s.CreateWith(ctx, CreateInput{Name: name, ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: DefaultMemoryMiB, DiskGiB: DefaultDiskGiB})
	if err != nil {
		return Ark{}, err
	}
	return s.SetState(ctx, a.ID, DesiredStopped, StatusStopped, "", "")
}

func (s *Store) CreateWith(ctx context.Context, input CreateInput) (Ark, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ImageID = strings.TrimSpace(input.ImageID)
	if input.Name == "" {
		return Ark{}, ErrNameRequired
	}
	if input.ImageID == "" || input.VCPUs < MinVCPUs || input.VCPUs > MaxVCPUs || input.MemoryMiB < MinMemoryMiB || input.MemoryMiB > MaxMemoryMiB || input.DiskGiB < MinDiskGiB || input.DiskGiB > MaxDiskGiB {
		return Ark{}, ErrInvalidResources
	}
	now := time.Now().UTC()
	a := Ark{ID: uuid.NewString(), Name: input.Name, ImageID: input.ImageID, VCPUs: input.VCPUs, MemoryMiB: input.MemoryMiB, DiskGiB: input.DiskGiB, DesiredState: DesiredRunning, Status: StatusProvisioning, CreatedAt: now, UpdatedAt: now}
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO arks (id, name, image_id, vcpus, memory_mib, disk_gib, desired_state, status, guest_ip, failure, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.ID, a.Name, a.ImageID, a.VCPUs, a.MemoryMiB, a.DiskGiB, a.DesiredState, a.Status, a.GuestIP, a.Failure, timestamp(a.CreatedAt), timestamp(a.UpdatedAt))
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
	return a, nil
}

func (s *Store) Get(ctx context.Context, name string) (Ark, error) {
	a, err := scanArk(s.db.QueryRowContext(ctx, arkSelect+` WHERE name = ? COLLATE NOCASE`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return Ark{}, ErrNotFound
	}
	if err != nil {
		return Ark{}, fmt.Errorf("get ark: %w", err)
	}
	return a, nil
}

func (s *Store) List(ctx context.Context) ([]Ark, error) {
	rows, err := s.db.QueryContext(ctx, arkSelect+` ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query arks: %w", err)
	}
	defer rows.Close()
	arks := make([]Ark, 0)
	for rows.Next() {
		a, err := scanArk(rows)
		if err != nil {
			return nil, fmt.Errorf("scan ark: %w", err)
		}
		arks = append(arks, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate arks: %w", err)
	}
	return arks, nil
}

var ErrStateMismatch = errors.New("ark state changed")

func (s *Store) SetState(ctx context.Context, id, desired, status, guestIP, failure string) (Ark, error) {
	return s.setState(ctx, `UPDATE arks SET desired_state = ?, status = ?, guest_ip = ?, failure = ?, updated_at = ? WHERE id = ? RETURNING `+arkColumns, id, "", desired, status, guestIP, failure)
}

func (s *Store) Transition(ctx context.Context, id, fromStatus, desired, status, guestIP, failure string) (Ark, error) {
	return s.setState(ctx, `UPDATE arks SET desired_state = ?, status = ?, guest_ip = ?, failure = ?, updated_at = ? WHERE id = ? AND status = ? RETURNING `+arkColumns, id, fromStatus, desired, status, guestIP, failure)
}

func (s *Store) setState(ctx context.Context, query, id, fromStatus, desired, status, guestIP, failure string) (Ark, error) {
	now := time.Now().UTC()
	args := []any{desired, status, guestIP, failure, timestamp(now), id}
	if fromStatus != "" {
		args = append(args, fromStatus)
	}
	a, err := scanArk(s.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		if fromStatus != "" {
			return Ark{}, ErrStateMismatch
		}
		return Ark{}, ErrNotFound
	}
	if err != nil {
		return Ark{}, fmt.Errorf("update ark state: %w", err)
	}
	return a, nil
}

func (s *Store) Delete(ctx context.Context, name string) (Ark, error) {
	return s.delete(ctx, `DELETE FROM arks WHERE name = ? COLLATE NOCASE RETURNING `+arkColumns, name)
}

func (s *Store) DeleteByID(ctx context.Context, id string) (Ark, error) {
	return s.delete(ctx, `DELETE FROM arks WHERE id = ? RETURNING `+arkColumns, id)
}

func (s *Store) delete(ctx context.Context, query, value string) (Ark, error) {
	a, err := scanArk(s.db.QueryRowContext(ctx, query, value))
	if errors.Is(err, sql.ErrNoRows) {
		return Ark{}, ErrNotFound
	}
	if err != nil {
		return Ark{}, fmt.Errorf("delete ark: %w", err)
	}
	return a, nil
}

const arkColumns = `id, name, image_id, vcpus, memory_mib, disk_gib, desired_state, status, guest_ip, failure, created_at, updated_at`
const arkSelect = `SELECT ` + arkColumns + ` FROM arks`

type rowScanner interface{ Scan(...any) error }

func scanArk(row rowScanner) (Ark, error) {
	var a Ark
	var createdAt, updatedAt string
	err := row.Scan(&a.ID, &a.Name, &a.ImageID, &a.VCPUs, &a.MemoryMiB, &a.DiskGiB, &a.DesiredState, &a.Status, &a.GuestIP, &a.Failure, &createdAt, &updatedAt)
	if err != nil {
		return Ark{}, err
	}
	var parseErr error
	if createdAt != "" {
		a.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil {
			return Ark{}, fmt.Errorf("parse created time: %w", parseErr)
		}
	}
	if updatedAt != "" {
		a.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updatedAt)
		if parseErr != nil {
			return Ark{}, fmt.Errorf("parse updated time: %w", parseErr)
		}
	}
	return a, nil
}
func timestamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
func (s *Store) Close() error      { return s.db.Close() }
func createDatabaseDirectory(databasePath string) error {
	if databasePath == ":memory:" || strings.HasPrefix(databasePath, "file:") {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o750); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	return nil
}
