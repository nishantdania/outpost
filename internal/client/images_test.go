package client

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestTarDirectoryAllowsManyEntries(t *testing.T) {
	dir := t.TempDir()
	for i := range 10001 {
		if err := os.Mkdir(filepath.Join(dir, strconv.Itoa(i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarDirectory(io.Discard, dir); err != nil {
		t.Fatalf("context over former 10,000 entry cap rejected: %v", err)
	}
}

func TestTarDirectoryAllowsLargeContext(t *testing.T) {
	dir := t.TempDir()
	file, err := os.Create(filepath.Join(dir, "large-file"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(64<<20 + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := tarDirectory(io.Discard, dir); err != nil {
		t.Fatalf("context over former 64 MiB and 32 MiB file caps rejected: %v", err)
	}
}
