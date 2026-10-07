package image

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nishantdania/outpost/internal/outpost"
)

func TestSnapshotPublicationPreservesBytesAndSparseTail(t *testing.T) {
	db, err := outpost.Open(t.Context(), filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 4<<20)
	copy(data, "application dependencies and database")
	image, err := store.ImportSnapshot(t.Context(), bytes.NewReader(data), "my-app-ready")
	if err != nil {
		t.Fatal(err)
	}
	if image.Digest != fmt.Sprintf("sha256:%x", sha256.Sum256(data)) || image.Size != int64(len(data)) {
		t.Fatalf("image = %+v", image)
	}
	saved, err := os.ReadFile(store.imagePath(image.Digest))
	if err != nil || !bytes.Equal(saved, data) {
		t.Fatal("snapshot bytes changed")
	}
	info, _ := os.Stat(store.imagePath(image.Digest))
	if info.Mode().Perm() != 0640 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	// Publication is idempotent and supports a second tag without hard links.
	if _, err := store.ImportSnapshot(t.Context(), bytes.NewReader(data), "another-tag"); err != nil {
		t.Fatal(err)
	}
	if err := verifyPublished(store.imagePath(image.Digest), image.Digest, image.Size); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"default", "../escape", ""} {
		if _, err := store.ImportSnapshot(t.Context(), bytes.NewReader(data), tag); err == nil {
			t.Fatalf("accepted %q", tag)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := store.ImportSnapshot(ctx, bytes.NewReader(data), "cancelled"); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, _, err := store.publishLimited(bytes.NewReader(data), 1024); err == nil {
		t.Fatal("ignored size limit")
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestInterruptedSnapshotDoesNotPublish(t *testing.T) {
	db, err := outpost.Open(t.Context(), filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store, err := New(filepath.Join(t.TempDir(), "images"), db, localRunner{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ImportSnapshot(t.Context(), failingReader{}, "broken"); err == nil {
		t.Fatal("accepted interrupted stream")
	}
	entries, _ := os.ReadDir(store.root)
	if len(entries) != 0 {
		t.Fatal("failed publication left artifacts")
	}
}
