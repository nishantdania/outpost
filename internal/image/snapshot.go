package image

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/nishantdania/outpost/internal/outpost"
)

// ImportSnapshot accepts only an internal launcher export, not an arbitrary
// public upload. Unlike container conversion, it preserves installed app data.
func (s *Store) ImportSnapshot(ctx context.Context, input io.Reader, tag string) (outpost.Image, error) {
	s.operations.Lock()
	defer s.operations.Unlock()
	if !outpost.ValidImageTag(tag) || tag == outpost.DefaultImageID {
		return outpost.Image{}, outpost.ErrInvalidImage
	}
	digest, size, err := s.publishLimited(contextReader{ctx, input}, int64(outpost.MaxDiskGiB)<<30)
	if err != nil {
		return outpost.Image{}, err
	}
	if err := s.db.PutImage(ctx, digest, size, tag); err != nil {
		return outpost.Image{}, err
	}
	return s.db.GetImage(ctx, digest)
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Keep zero-filled regions as holes while retaining the logical byte digest.
// This also avoids allocating the unused portion of a large guest disk.
type sparseWriter struct{ file *os.File }

func (w sparseWriter) Write(p []byte) (int, error) {
	if bytes.Count(p, []byte{0}) == len(p) {
		_, err := w.file.Seek(int64(len(p)), io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}
	return w.file.Write(p)
}
