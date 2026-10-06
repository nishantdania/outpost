package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/image"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
	"github.com/nishantdania/outpost/internal/testutil"
)

type snapshotManager struct {
	testutil.FakeManager
	opens int
}

func (m *snapshotManager) OpenSnapshot(context.Context, string) (io.ReadCloser, error) {
	m.opens++
	return io.NopCloser(strings.NewReader("prepared project disk")), nil
}

func TestSnapshotAPIAndClientReuseImageLifecycle(t *testing.T) {
	db := newTestStore(t)
	images, err := image.New(filepath.Join(t.TempDir(), "images"), db, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager := &snapshotManager{}
	app := service.New(db, manager).WithImages(images)
	if _, err := app.Create(t.Context(), outpost.CreateInput{Name: "baseline", ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newRouter(app, "token"))
	defer server.Close()
	c, err := client.New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.SnapshotOutpost(t.Context(), "baseline", "ready"); err == nil {
		t.Fatal("snapshotted running VM")
	}
	if manager.opens != 0 {
		t.Fatal("exported running VM")
	}
	if _, err := app.Stop(t.Context(), "baseline"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := c.SnapshotOutpost(t.Context(), "baseline", "ready")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Size != int64(len("prepared project disk")) || len(snapshot.Tags) != 1 || snapshot.Tags[0] != "ready" {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if _, err := app.Delete(t.Context(), "baseline"); err != nil {
		t.Fatal(err)
	}
	// Deleting the source does not remove the independent saved disk image.
	fork, err := c.CreateOutpostWith(t.Context(), client.CreateOutpostInput{Name: "feature", ImageID: "ready", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8})
	if err != nil {
		t.Fatal(err)
	}
	if fork.ImageId != snapshot.Digest {
		t.Fatalf("fork image = %s", fork.ImageId)
	}
	if err := c.RemoveImage(t.Context(), "ready"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveImage(t.Context(), snapshot.Digest); err == nil {
		t.Fatal("removed image referenced by fork")
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/outposts/feature/snapshot?tag=stolen", nil)
	response := httptest.NewRecorder()
	newRouter(app, "token").ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated response %d", response.Code)
	}
}
