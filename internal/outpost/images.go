package outpost

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	ErrImageNotFound = errors.New("image not found")
	ErrInvalidImage  = errors.New("image reference is invalid")
)

var digestID = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var imageTag = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,127}(:[a-z0-9][a-z0-9._-]{0,127})?$`)

type Image struct {
	Digest    string    `json:"digest"`
	Size      int64     `json:"size_bytes"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

func ValidDigest(v string) bool   { return digestID.MatchString(v) }
func ValidImageTag(v string) bool { return imageTag.MatchString(v) }

func (s *Store) ResolveImage(ctx context.Context, reference string) (string, error) {
	if reference == DefaultImageID {
		return reference, nil
	}
	if ValidDigest(reference) {
		var found bool
		err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM images WHERE digest = ?)`, reference).Scan(&found)
		if err != nil {
			return "", fmt.Errorf("find image: %w", err)
		}
		if found {
			return reference, nil
		}
		return "", ErrImageNotFound
	}
	if !ValidImageTag(reference) {
		return "", ErrInvalidImage
	}
	var digest string
	err := s.db.QueryRowContext(ctx, `SELECT digest FROM image_tags WHERE tag = ?`, reference).Scan(&digest)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrImageNotFound
	}
	if err != nil {
		return "", fmt.Errorf("resolve image: %w", err)
	}
	return digest, nil
}

func (s *Store) PutImage(ctx context.Context, digest string, size int64, tag string) error {
	if !ValidDigest(digest) || size < 0 || (tag != "" && !ValidImageTag(tag)) {
		return ErrInvalidImage
	}
	now := timestamp(time.Now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO images(digest,size_bytes,created_at) VALUES(?,?,?)`, digest, size, now); err != nil {
		return fmt.Errorf("insert image: %w", err)
	}
	if tag != "" {
		if _, err = tx.ExecContext(ctx, `INSERT INTO image_tags(tag,digest,updated_at) VALUES(?,?,?) ON CONFLICT(tag) DO UPDATE SET digest=excluded.digest,updated_at=excluded.updated_at`, tag, digest, now); err != nil {
			return fmt.Errorf("tag image: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) ListImages(ctx context.Context) ([]Image, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT i.digest,i.size_bytes,i.created_at,COALESCE(group_concat(t.tag, char(31)), '') FROM images i LEFT JOIN image_tags t ON t.digest=i.digest GROUP BY i.digest ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Image{{Digest: DefaultImageID, Tags: []string{DefaultImageID}}}
	for rows.Next() {
		var image Image
		var created, tags string
		if err := rows.Scan(&image.Digest, &image.Size, &created, &tags); err != nil {
			return nil, err
		}
		image.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, err
		}
		if tags != "" {
			image.Tags = strings.Split(tags, "\x1f")
		}
		result = append(result, image)
	}
	return result, rows.Err()
}
func (s *Store) GetImage(ctx context.Context, ref string) (Image, error) {
	d, err := s.ResolveImage(ctx, ref)
	if err != nil {
		return Image{}, err
	}
	if d == DefaultImageID {
		return Image{Digest: d, Tags: []string{d}}, nil
	}
	images, err := s.ListImages(ctx)
	if err != nil {
		return Image{}, err
	}
	for _, i := range images {
		if i.Digest == d {
			return i, nil
		}
	}
	return Image{}, ErrImageNotFound
}
func (s *Store) RemoveImage(ctx context.Context, ref string) error {
	if ref == DefaultImageID {
		return ErrInvalidImage
	}
	if !ValidDigest(ref) && ValidImageTag(ref) {
		result, err := s.db.ExecContext(ctx, `DELETE FROM image_tags WHERE tag = ?`, ref)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return ErrImageNotFound
		}
		return nil
	}
	d, err := s.ResolveImage(ctx, ref)
	if err != nil {
		return err
	}
	if d == DefaultImageID {
		return ErrInvalidImage
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tagged, used bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM image_tags WHERE digest=?)`, d).Scan(&tagged); err != nil {
		return err
	}
	if tagged {
		return fmt.Errorf("image has tags")
	}
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM outposts WHERE image_id=? AND desired_state != 'deleted')`, d).Scan(&used); err != nil {
		return err
	}
	if used {
		return fmt.Errorf("image is in use")
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM images WHERE digest=?`, d)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrImageNotFound
	}
	return tx.Commit()
}
func (s *Store) GarbageCollectImages(ctx context.Context) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT digest FROM images WHERE digest NOT IN (SELECT digest FROM image_tags) AND digest NOT IN (SELECT image_id FROM outposts WHERE desired_state != 'deleted')`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var d string
		if err = rows.Scan(&d); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, d)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	for _, d := range ids {
		if _, err = tx.ExecContext(ctx, `DELETE FROM images WHERE digest=?`, d); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return ids, nil
}
