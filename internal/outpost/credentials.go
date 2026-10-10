package outpost

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nishantdania/outpost/internal/credentials"
)

var ErrCredentialConflict = errors.New("image digest already has a different credential profile")

func (s *Store) ImageCredentials(ctx context.Context, ref string) (string, error) {
	digest, err := s.ResolveImage(ctx, ref)
	if err != nil {
		return "", err
	}
	if digest == DefaultImageID {
		return "", nil
	}
	var config string
	err = s.db.QueryRowContext(ctx, `SELECT credential_config FROM images WHERE digest=?`, digest).Scan(&config)
	return config, err
}

// PutSnapshotImage publishes both the profile and the tag in one transaction.
// A digest's profile is immutable: aliases cannot silently change authorization.
func (s *Store) PutSnapshotImage(ctx context.Context, digest string, size int64, tag, config string) error {
	if !ValidDigest(digest) || !ValidImageTag(tag) || size < 0 {
		return ErrInvalidImage
	}
	if config != "" {
		if _, err := credentials.Parse([]byte(config)); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT credential_config FROM images WHERE digest=?`, digest).Scan(&existing)
	if err == nil && existing != config {
		return ErrCredentialConflict
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO images(digest,size_bytes,created_at,credential_config) VALUES(?,?,?,?)`, digest, size, timestamp(time.Now()), config); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO image_tags(tag,digest,updated_at) VALUES(?,?,?) ON CONFLICT(tag) DO UPDATE SET digest=excluded.digest,updated_at=excluded.updated_at`, tag, digest, timestamp(time.Now())); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CredentialGuest(ctx context.Context, ip string) (Outpost, error) {
	a, err := scanOutpost(s.db.QueryRowContext(ctx, outpostSelect+` WHERE guest_ip=? AND status='running' AND desired_state='running' AND credential_config!=''`, ip))
	if errors.Is(err, sql.ErrNoRows) {
		return Outpost{}, ErrNotFound
	}
	return a, err
}
