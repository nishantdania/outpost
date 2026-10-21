package manager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/artifact"
	"github.com/nishantdania/outpost/internal/catalog"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type fakeRuntime struct {
	startErr, deleteErr           error
	exportReadErr, exportCloseErr error
	disk                          []byte
	export                        *testExport
	exports                       int
}

func (*fakeRuntime) Create(context.Context, outpost.Outpost) error   { return nil }
func (f *fakeRuntime) Start(context.Context, string) (string, error) { return "", f.startErr }
func (*fakeRuntime) Stop(context.Context, string) error              { return nil }
func (f *fakeRuntime) Delete(context.Context, string) error          { return f.deleteErr }
func (*fakeRuntime) Inspect(context.Context, string) (vmapi.RuntimeState, error) {
	return vmapi.RuntimeState{}, nil
}
func (*fakeRuntime) List(context.Context) ([]vmapi.RuntimeState, error) { return nil, nil }
func (f *fakeRuntime) OpenSnapshot(context.Context, string) (io.ReadCloser, error) {
	f.exports++
	f.export = &testExport{Reader: bytes.NewReader(f.disk), closeErr: f.exportCloseErr}
	if f.exportReadErr != nil {
		f.export.Reader = errorReader{f.exportReadErr}
	}
	return f.export, nil
}

type testExport struct {
	io.Reader
	closed   bool
	closeErr error
}

func (e *testExport) Close() error { e.closed = true; return e.closeErr }

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestSnapshotClosesExportWhenPublicationFails(t *testing.T) {
	m, runtime := setup(t)
	ctx := context.Background()
	if _, err := m.Create(ctx, input("dev", "default")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stop(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	runtime.exportReadErr = errors.New("disk read failed")
	runtime.exportCloseErr = errors.New("export cleanup failed")
	_, err := m.Snapshot(ctx, "dev", "failed")
	if !errors.Is(err, runtime.exportReadErr) || !errors.Is(err, runtime.exportCloseErr) {
		t.Fatalf("lost snapshot errors: %v", err)
	}
	if !runtime.export.closed {
		t.Fatal("failed snapshot leaked export")
	}
	if _, err := m.Image(ctx, "failed"); !errors.Is(err, catalog.ErrImageNotFound) {
		t.Fatalf("registered failed snapshot: %v", err)
	}
}

func setup(t *testing.T) (*Manager, *fakeRuntime) {
	t.Helper()
	ctx := context.Background()
	store, err := outpost.Open(ctx, filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	disks, err := artifact.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runtime := &fakeRuntime{disk: []byte("guest data\x00\x00unchanged")}
	m, err := New(store, runtime, disks)
	if err != nil {
		t.Fatal(err)
	}
	return m, runtime
}
func input(name, image string) outpost.CreateInput {
	return outpost.CreateInput{Name: name, ImageID: image, VCPUs: 2, MemoryMiB: 4096, DiskGiB: 8}
}

func TestLifecycleAndImageProtection(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	image, err := m.ImportImage(ctx, strings.NewReader("base disk"), "base")
	if err != nil {
		t.Fatal(err)
	}
	vm, err := m.Create(ctx, input("dev", "base"))
	if err != nil {
		t.Fatal(err)
	}
	if vm.Status != outpost.StatusRunning || vm.ImageID != image.Digest {
		t.Fatalf("unexpected VM: %+v", vm)
	}
	if _, err := m.Start(ctx, "dev"); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("start running: %v", err)
	}
	if err := m.RemoveImage(ctx, "base"); err != nil {
		t.Fatal(err)
	}
	if err := m.RemoveImage(ctx, image.Digest); !errors.Is(err, catalog.ErrImageProtected) {
		t.Fatalf("remove pinned image: %v", err)
	}
	if ids, err := m.GCImages(ctx); err != nil || len(ids) != 0 {
		t.Fatalf("GC pinned image: %v %v", ids, err)
	}
	if _, err := m.Stop(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Delete(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Image(ctx, image.Digest); err != nil {
		t.Fatalf("VM deletion removed image: %v", err)
	}
	ids, err := m.GCImages(ctx)
	if err != nil || len(ids) != 1 || ids[0] != image.Digest {
		t.Fatalf("GC unpinned: %v %v", ids, err)
	}
	if _, err := m.artifacts.Open(image.Digest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("artifact not removed: %v", err)
	}
}

func TestFailedStartAndDeleteRemainRecoverable(t *testing.T) {
	m, runtime := setup(t)
	ctx := context.Background()
	runtime.startErr = errors.New("firecracker failed")
	if _, err := m.Create(ctx, input("dev", "default")); !errors.Is(err, runtime.startErr) {
		t.Fatal(err)
	}
	vm, err := m.store.Get(ctx, "dev")
	if err != nil || vm.Status != outpost.StatusFailed || !strings.Contains(vm.Failure, "firecracker failed") {
		t.Fatalf("failure not recorded: %+v %v", vm, err)
	}
	runtime.deleteErr = errors.New("cleanup failed")
	if _, err := m.Delete(ctx, "dev"); !errors.Is(err, runtime.deleteErr) {
		t.Fatal(err)
	}
	if _, err := m.store.Get(ctx, "dev"); err != nil {
		t.Fatalf("lost VM after cleanup failure: %v", err)
	}
	runtime.deleteErr = nil
	if _, err := m.Delete(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotRequiresStoppedVMAndPreservesBytes(t *testing.T) {
	m, runtime := setup(t)
	ctx := context.Background()
	if _, err := m.Create(ctx, input("dev", "default")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Snapshot(ctx, "dev", "snapshot"); !errors.Is(err, ErrInvalidState) {
		t.Fatal(err)
	}
	if runtime.exports != 0 {
		t.Fatal("exported running VM")
	}
	if _, err := m.Stop(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	image, err := m.Snapshot(ctx, "dev", "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.export.closed {
		t.Fatal("snapshot export leaked")
	}
	disk, err := m.artifacts.Open(image.Digest)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	data, err := io.ReadAll(disk)
	if err != nil || !bytes.Equal(data, runtime.disk) {
		t.Fatalf("snapshot bytes changed: %q %v", data, err)
	}
	if _, err := m.Delete(ctx, "dev"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Image(ctx, "snapshot"); err != nil {
		t.Fatalf("snapshot lost with source VM: %v", err)
	}
	if _, err := m.Create(ctx, input("clone", "snapshot")); err != nil {
		t.Fatal(err)
	}
}

func TestTagReplacementDoesNotChangeVMImage(t *testing.T) {
	m, _ := setup(t)
	ctx := context.Background()
	first, err := m.ImportImage(ctx, strings.NewReader("first"), "base")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(ctx, input("dev", "base")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ImportImage(ctx, strings.NewReader("second"), "base"); err != nil {
		t.Fatal(err)
	}
	vm, err := m.Get(ctx, "dev")
	if err != nil || vm.ImageID != first.Digest {
		t.Fatalf("tag replacement changed VM: %+v %v", vm, err)
	}
	if ids, err := m.GCImages(ctx); err != nil || len(ids) != 0 {
		t.Fatalf("collected referenced disk: %v %v", ids, err)
	}
}
