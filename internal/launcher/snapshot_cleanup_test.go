package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSnapshotCloseRemovesPrivateCopyAndReleasesCapacityOnce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".snapshot-test")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(filepath.Join(dir, "rootfs.ext4"))
	if err != nil {
		t.Fatal(err)
	}
	releases := 0
	export := &snapshotFile{File: file, dir: dir, release: func() { releases++ }}
	if err := export.Close(); err != nil {
		t.Fatal(err)
	}
	if err := export.Close(); err != nil {
		t.Fatal(err)
	}
	if releases != 1 {
		t.Fatalf("released capacity %d times", releases)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("snapshot directory leaked: %v", err)
	}
}
