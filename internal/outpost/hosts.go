package outpost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
)

var (
	ErrInvalidHostname = errors.New("hostname must be a fully qualified ASCII DNS name without a scheme, wildcard or port")
	ErrInvalidPort     = errors.New("port must be between 1 and 65535")
	ErrHostTaken       = errors.New("hostname is already mapped to a different VM or port; unhost it before changing the mapping")
	ErrHostNotFound    = errors.New("host mapping not found")
)

// Host stores a VM identity, never a pinned address. Address/state come from the
// current VM record, so stop/start and IP changes need no routing configuration reload.
type Host struct {
	Hostname     string
	OutpostID    string
	OutpostName  string
	Port         int
	GuestIP      string
	Status       string
	DesiredState string
}

func CanonicalHostname(value string) (string, error) {
	hostname := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if len(hostname) > 253 || net.ParseIP(hostname) != nil {
		return "", ErrInvalidHostname
	}
	labels := strings.Split(hostname, ".")
	if len(labels) < 2 {
		return "", ErrInvalidHostname
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", ErrInvalidHostname
		}
		for _, char := range label {
			if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
				return "", ErrInvalidHostname
			}
		}
	}
	return hostname, nil
}

func (s *Store) Host(ctx context.Context, hostname string) (Host, error) {
	hostname, err := CanonicalHostname(hostname)
	if err != nil {
		return Host{}, err
	}
	host, err := scanHost(s.db.QueryRowContext(ctx, hostSelect+` WHERE h.hostname = ?`, hostname))
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, ErrHostNotFound
	}
	if err != nil {
		return Host{}, fmt.Errorf("get host mapping: %w", err)
	}
	return host, nil
}

func (s *Store) Hosts(ctx context.Context) ([]Host, error) {
	rows, err := s.db.QueryContext(ctx, hostSelect+` ORDER BY h.hostname`)
	if err != nil {
		return nil, fmt.Errorf("list host mappings: %w", err)
	}
	defer rows.Close()
	hosts := make([]Host, 0)
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			return nil, fmt.Errorf("scan host mapping: %w", err)
		}
		hosts = append(hosts, host)
	}
	return hosts, rows.Err()
}

// SetHost is idempotent for the same VM ID and port. A competing mapping never
// replaces the existing one; registration and name resolution are one transaction.
func (s *Store) SetHost(ctx context.Context, hostname, name string, port int) (Host, error) {
	hostname, err := CanonicalHostname(hostname)
	if err != nil {
		return Host{}, err
	}
	if port < 1 || port > 65535 {
		return Host{}, ErrInvalidPort
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Host{}, fmt.Errorf("begin host registration: %w", err)
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM outposts WHERE name = ? COLLATE NOCASE`, name).Scan(&id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Host{}, ErrNotFound
		}
		return Host{}, fmt.Errorf("resolve host VM: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO hosts (hostname, outpost_id, port) VALUES (?, ?, ?) ON CONFLICT(hostname) DO NOTHING`, hostname, id, port); err != nil {
		return Host{}, fmt.Errorf("register host: %w", err)
	}
	host, err := scanHost(tx.QueryRowContext(ctx, hostSelect+` WHERE h.hostname = ?`, hostname))
	if err != nil {
		return Host{}, fmt.Errorf("read registered host: %w", err)
	}
	if host.OutpostID != id || host.Port != port {
		return Host{}, ErrHostTaken
	}
	if err := tx.Commit(); err != nil {
		return Host{}, fmt.Errorf("commit host registration: %w", err)
	}
	return host, nil
}

func (s *Store) Unhost(ctx context.Context, hostname string) error {
	hostname, err := CanonicalHostname(hostname)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM hosts WHERE hostname = ?`, hostname)
	if err != nil {
		return fmt.Errorf("remove host mapping: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check removed host mapping: %w", err)
	}
	if count == 0 {
		return ErrHostNotFound
	}
	return nil
}

const hostSelect = `SELECT h.hostname, h.outpost_id, a.name, h.port, a.guest_ip, a.status, a.desired_state FROM hosts h JOIN outposts a ON a.id = h.outpost_id`

func scanHost(row rowScanner) (Host, error) {
	var host Host
	err := row.Scan(&host.Hostname, &host.OutpostID, &host.OutpostName, &host.Port, &host.GuestIP, &host.Status, &host.DesiredState)
	return host, err
}
