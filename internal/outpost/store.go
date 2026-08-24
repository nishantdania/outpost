package outpost

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
	ErrNameRequired        = errors.New("outpost name is required")
	ErrNameTaken           = errors.New("outpost name is already in use")
	ErrNotFound            = errors.New("outpost not found")
	ErrInvalidResources    = errors.New("outpost resources must be positive")
	ErrInvalidSSHPublicKey = errors.New("SSH public key is invalid")
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

func (s *Store) Create(ctx context.Context, name string) (Outpost, error) {
	a, err := s.CreateWith(ctx, CreateInput{Name: name, ImageID: DefaultImageID, VCPUs: DefaultVCPUs, MemoryMiB: DefaultMemoryMiB, DiskGiB: DefaultDiskGiB})
	if err != nil {
		return Outpost{}, err
	}
	return s.SetState(ctx, a.ID, DesiredStopped, StatusStopped, "", "")
}

func (s *Store) CreateWith(ctx context.Context, input CreateInput) (Outpost, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ImageID = strings.TrimSpace(input.ImageID)
	if input.Name == "" {
		return Outpost{}, ErrNameRequired
	}
	if err := ValidateSSHPublicKey(input.SSHPublicKey); err != nil {
		return Outpost{}, err
	}
	if input.VCPUs < MinVCPUs || input.VCPUs > MaxVCPUs || input.MemoryMiB < MinMemoryMiB || input.MemoryMiB > MaxMemoryMiB || input.DiskGiB < MinDiskGiB || input.DiskGiB > MaxDiskGiB {
		return Outpost{}, ErrInvalidResources
	}
	resolved, err := s.ResolveImage(ctx, input.ImageID)
	if err != nil {
		return Outpost{}, err
	}
	input.ImageID = resolved
	now := time.Now().UTC()
	a := Outpost{ID: uuid.NewString(), Name: input.Name, ImageID: input.ImageID, VCPUs: input.VCPUs, MemoryMiB: input.MemoryMiB, DiskGiB: input.DiskGiB, SSHPublicKey: input.SSHPublicKey, DesiredState: DesiredRunning, Status: StatusProvisioning, CreatedAt: now, UpdatedAt: now}
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO outposts (id, name, image_id, vcpus, memory_mib, disk_gib, desired_state, status, guest_ip, failure, ssh_public_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, a.ID, a.Name, a.ImageID, a.VCPUs, a.MemoryMiB, a.DiskGiB, a.DesiredState, a.Status, a.GuestIP, a.Failure, a.SSHPublicKey, timestamp(a.CreatedAt), timestamp(a.UpdatedAt))
	if err != nil {
		return Outpost{}, fmt.Errorf("insert outpost: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return Outpost{}, fmt.Errorf("check inserted outpost: %w", err)
	}
	if inserted == 0 {
		return Outpost{}, ErrNameTaken
	}
	return a, nil
}

func (s *Store) Get(ctx context.Context, name string) (Outpost, error) {
	a, err := scanOutpost(s.db.QueryRowContext(ctx, outpostSelect+` WHERE name = ? COLLATE NOCASE`, name))
	if errors.Is(err, sql.ErrNoRows) {
		return Outpost{}, ErrNotFound
	}
	if err != nil {
		return Outpost{}, fmt.Errorf("get outpost: %w", err)
	}
	return a, nil
}

func (s *Store) List(ctx context.Context) ([]Outpost, error) {
	rows, err := s.db.QueryContext(ctx, outpostSelect+` ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("query outposts: %w", err)
	}
	defer rows.Close()
	outposts := make([]Outpost, 0)
	for rows.Next() {
		a, err := scanOutpost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan outpost: %w", err)
		}
		outposts = append(outposts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outposts: %w", err)
	}
	return outposts, nil
}

var ErrStateMismatch = errors.New("outpost state changed")

func (s *Store) SetState(ctx context.Context, id, desired, status, guestIP, failure string) (Outpost, error) {
	return s.setState(ctx, `UPDATE outposts SET desired_state = ?, status = ?, guest_ip = ?, failure = ?, updated_at = ? WHERE id = ? RETURNING `+outpostColumns, id, "", desired, status, guestIP, failure)
}

func (s *Store) Transition(ctx context.Context, id, fromStatus, desired, status, guestIP, failure string) (Outpost, error) {
	return s.setState(ctx, `UPDATE outposts SET desired_state = ?, status = ?, guest_ip = ?, failure = ?, updated_at = ? WHERE id = ? AND status = ? RETURNING `+outpostColumns, id, fromStatus, desired, status, guestIP, failure)
}

func (s *Store) setState(ctx context.Context, query, id, fromStatus, desired, status, guestIP, failure string) (Outpost, error) {
	now := time.Now().UTC()
	args := []any{desired, status, guestIP, failure, timestamp(now), id}
	if fromStatus != "" {
		args = append(args, fromStatus)
	}
	a, err := scanOutpost(s.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		if fromStatus != "" {
			return Outpost{}, ErrStateMismatch
		}
		return Outpost{}, ErrNotFound
	}
	if err != nil {
		return Outpost{}, fmt.Errorf("update outpost state: %w", err)
	}
	return a, nil
}

func (s *Store) Delete(ctx context.Context, name string) (Outpost, error) {
	return s.delete(ctx, `DELETE FROM outposts WHERE name = ? COLLATE NOCASE RETURNING `+outpostColumns, name)
}

func (s *Store) DeleteByID(ctx context.Context, id string) (Outpost, error) {
	return s.delete(ctx, `DELETE FROM outposts WHERE id = ? RETURNING `+outpostColumns, id)
}

func (s *Store) delete(ctx context.Context, query, value string) (Outpost, error) {
	a, err := scanOutpost(s.db.QueryRowContext(ctx, query, value))
	if errors.Is(err, sql.ErrNoRows) {
		return Outpost{}, ErrNotFound
	}
	if err != nil {
		return Outpost{}, fmt.Errorf("delete outpost: %w", err)
	}
	return a, nil
}

const outpostColumns = `id, name, image_id, vcpus, memory_mib, disk_gib, desired_state, status, guest_ip, failure, ssh_public_key, created_at, updated_at`
const outpostSelect = `SELECT ` + outpostColumns + ` FROM outposts`

type rowScanner interface{ Scan(...any) error }

func scanOutpost(row rowScanner) (Outpost, error) {
	var a Outpost
	var createdAt, updatedAt string
	err := row.Scan(&a.ID, &a.Name, &a.ImageID, &a.VCPUs, &a.MemoryMiB, &a.DiskGiB, &a.DesiredState, &a.Status, &a.GuestIP, &a.Failure, &a.SSHPublicKey, &createdAt, &updatedAt)
	if err != nil {
		return Outpost{}, err
	}
	var parseErr error
	if createdAt != "" {
		a.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil {
			return Outpost{}, fmt.Errorf("parse created time: %w", parseErr)
		}
	}
	if updatedAt != "" {
		a.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updatedAt)
		if parseErr != nil {
			return Outpost{}, fmt.Errorf("parse updated time: %w", parseErr)
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
