package outpost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Runtime joins launcher-owned bookkeeping with the canonical VM record. It
// stores no second specification or guest IP; those belong to outposts.
type Runtime struct {
	VM        Outpost
	Tap       string
	Gateway   string
	PID       int
	StartTime string
	UFWRoute  bool
}

var runtimeSelect = `SELECT o.` + strings.ReplaceAll(outpostColumns, ", ", ", o.") + `,
 r.tap, r.gateway, r.pid, r.start_time, r.ufw_route
 FROM vm_runtime r JOIN outposts o ON o.id = r.outpost_id`

func scanRuntime(row rowScanner) (Runtime, error) {
	var r Runtime
	a, err := scanOutpost(row, &r.Tap, &r.Gateway, &r.PID, &r.StartTime, &r.UFWRoute)
	r.VM = a
	return r, err
}

func (s *Store) Runtime(ctx context.Context, id string) (Runtime, error) {
	r, err := scanRuntime(s.db.QueryRowContext(ctx, runtimeSelect+` WHERE o.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Runtime{}, ErrNotFound
	}
	if err != nil {
		return Runtime{}, fmt.Errorf("get VM runtime: %w", err)
	}
	return r, nil
}

func (s *Store) Runtimes(ctx context.Context) ([]Runtime, error) {
	rows, err := s.db.QueryContext(ctx, runtimeSelect+` ORDER BY o.id`)
	if err != nil {
		return nil, fmt.Errorf("query VM runtimes: %w", err)
	}
	defer rows.Close()
	runtimes := make([]Runtime, 0)
	for rows.Next() {
		r, err := scanRuntime(rows)
		if err != nil {
			return nil, fmt.Errorf("scan VM runtime: %w", err)
		}
		runtimes = append(runtimes, r)
	}
	return runtimes, rows.Err()
}

// SaveRuntime atomically persists allocations and process identity. Specifications
// are immutable and must already match a VM created by the lifecycle service.
// A launcher request cannot create or overwrite a VM specification.
func (s *Store) SaveRuntime(ctx context.Context, r Runtime) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin runtime update: %w", err)
	}
	defer tx.Rollback()
	a := r.VM
	result, err := tx.ExecContext(ctx, `UPDATE outposts SET guest_ip = ?, updated_at = ?
 WHERE id = ? AND image_id = ? AND vcpus = ? AND memory_mib = ? AND disk_gib = ?`,
		a.GuestIP, timestamp(time.Now()), a.ID, a.ImageID, a.VCPUs, a.MemoryMiB, a.DiskGiB)
	if err != nil {
		return fmt.Errorf("update VM allocation: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrStateMismatch
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO vm_runtime (outpost_id, tap, gateway, pid, start_time, ufw_route)
 VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(outpost_id) DO UPDATE SET
 tap = excluded.tap, gateway = excluded.gateway, pid = excluded.pid,
 start_time = excluded.start_time, ufw_route = excluded.ufw_route`,
		a.ID, r.Tap, r.Gateway, r.PID, r.StartTime, r.UFWRoute)
	if err != nil {
		return fmt.Errorf("save VM runtime: %w", err)
	}
	return tx.Commit()
}

// RemoveRuntime is called only after privileged resources have been removed.
// Until then the FK prevents deletion of their VM record and its image pin.
func (s *Store) RemoveRuntime(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM vm_runtime WHERE outpost_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE outposts SET guest_ip = '', updated_at = ? WHERE id = ?`, timestamp(time.Now()), id); err != nil {
		return err
	}
	return tx.Commit()
}

// ObserveRuntime refreshes stable lifecycle states without racing an in-flight
// create/start/stop/delete transition or erasing an operation's failure.
func (s *Store) ObserveRuntime(ctx context.Context, id string, running bool) error {
	status := StatusStopped
	if running {
		status = StatusRunning
	}
	_, err := s.db.ExecContext(ctx, `UPDATE outposts SET status = ?, updated_at = ?
 WHERE id = ? AND status IN ('running', 'stopped') AND status != ?`, status, timestamp(time.Now()), id, status)
	return err
}

// RecoverLifecycle runs before the launcher accepts requests. An interrupted
// operation must not leave a VM permanently stuck in a transitional state.
func (s *Store) RecoverLifecycle(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE outposts SET status = 'failed',
 failure = 'VM operation interrupted by launcher restart', updated_at = ?
 WHERE status IN ('provisioning', 'stopping', 'deleting')`, timestamp(time.Now())); err != nil {
		return err
	}
	// A crash can occur after runtime cleanup but before lifecycle deletion.
	// Such records must neither claim a running process nor reserve an IP.
	if _, err := tx.ExecContext(ctx, `UPDATE outposts SET guest_ip = '',
 status = CASE WHEN status = 'running' THEN 'stopped' ELSE status END, updated_at = ?
 WHERE id NOT IN (SELECT outpost_id FROM vm_runtime) AND (guest_ip != '' OR status = 'running')`, timestamp(time.Now())); err != nil {
		return err
	}
	return tx.Commit()
}
