package ark

import (
	"context"
	"path/filepath"
	"testing"
)

func TestImageTagsAndReferences(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	if err := s.PutImage(ctx, a, 1, "coding:latest"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ResolveImage(ctx, "coding:latest"); err != nil || got != a {
		t.Fatalf("resolve = %q, %v", got, err)
	}
	if err := s.PutImage(ctx, b, 2, "coding:latest"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.ResolveImage(ctx, "coding:latest"); err != nil || got != b {
		t.Fatalf("replacement = %q, %v", got, err)
	}
	if err := s.RemoveImage(ctx, a); err != nil {
		t.Fatalf("untagged digest remove: %v", err)
	}
	if err := s.RemoveImage(ctx, b); err == nil {
		t.Fatal("tagged digest removed")
	}
	if err := s.RemoveImage(ctx, "coding:latest"); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveImage(ctx, b); err != nil {
		t.Fatal(err)
	}
}

func TestImageDigestBlockedByArkAndGC(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	d := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	if err := s.PutImage(ctx, d, 1, ""); err != nil {
		t.Fatal(err)
	}
	a, err := s.CreateWith(ctx, CreateInput{Name: "uses-image", ImageID: d, VCPUs: 1, MemoryMiB: 128, DiskGiB: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveImage(ctx, d); err == nil {
		t.Fatal("in-use digest removed")
	}
	if _, err := s.Transition(ctx, a.ID, StatusProvisioning, DesiredDeleted, StatusStopped, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveImage(ctx, d); err != nil {
		t.Fatalf("deleted ark still blocked digest: %v", err)
	}
	if err := s.PutImage(ctx, d, 1, ""); err != nil {
		t.Fatal(err)
	}
	ids, err := s.GarbageCollectImages(ctx)
	if err != nil || len(ids) != 1 || ids[0] != d {
		t.Fatalf("gc = %v, %v", ids, err)
	}
}
