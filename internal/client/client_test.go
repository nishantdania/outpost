package client

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/artifact"
	"github.com/nishantdania/outpost/internal/httpapi"
	"github.com/nishantdania/outpost/internal/manager"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type runtimeStub struct{}

func (runtimeStub) Create(context.Context, outpost.Outpost) error { return nil }
func (runtimeStub) Start(context.Context, string) (string, error) { return "", nil }
func (runtimeStub) Stop(context.Context, string) error            { return nil }
func (runtimeStub) Delete(context.Context, string) error          { return nil }
func (runtimeStub) Inspect(context.Context, string) (vmapi.RuntimeState, error) {
	return vmapi.RuntimeState{}, nil
}
func (runtimeStub) List(context.Context) ([]vmapi.RuntimeState, error) { return nil, nil }
func (runtimeStub) OpenSnapshot(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("snapshot bytes")), nil
}

func localClient(t *testing.T) *Client {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	store, err := outpost.Open(ctx, filepath.Join(dir, "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	disks, err := artifact.New(filepath.Join(dir, "images"))
	if err != nil {
		t.Fatal(err)
	}
	application, err := manager.New(store, runtimeStub{}, disks)
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "api.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: httpapi.Handler(application)}
	done := make(chan struct{})
	go func() { defer close(done); server.Serve(listener) }()
	t.Cleanup(func() { server.Close(); <-done })
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	return &Client{http: &http.Client{Transport: transport}}
}

func TestUnixSocketLifecycle(t *testing.T) {
	c := localClient(t)
	ctx := context.Background()
	image, err := c.ImportImage(ctx, "team/base:latest", strings.NewReader("imported disk"))
	if err != nil {
		t.Fatal(err)
	}
	if image.Size != int64(len("imported disk")) {
		t.Fatalf("wrong image: %+v", image)
	}
	if _, err := c.GetImage(ctx, "team/base:latest"); err != nil {
		t.Fatal(err)
	}
	name := "dev with spaces"
	vm, err := c.CreateOutpostWith(ctx, CreateOutpostInput{Name: name, ImageID: "team/base:latest", VCPUs: 2, MemoryMiB: 4096, DiskGiB: 8})
	if err != nil || vm.Status != "running" {
		t.Fatalf("create: %+v %v", vm, err)
	}
	if _, err := c.SnapshotOutpost(ctx, name, "saved"); err == nil || !strings.Contains(err.Error(), "state") {
		t.Fatalf("expected running snapshot conflict: %v", err)
	}
	if _, err := c.StopOutpost(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SnapshotOutpost(ctx, name, "team/saved:latest"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartOutpost(ctx, name); err != nil {
		t.Fatal(err)
	}
	if vms, err := c.ListOutposts(ctx); err != nil || len(vms) != 1 {
		t.Fatalf("list: %v %v", vms, err)
	}
	if err := c.RemoveImage(ctx, image.Digest); err == nil || !strings.Contains(err.Error(), "tag") {
		t.Fatalf("expected protected image error: %v", err)
	}
	if err := c.RemoveImage(ctx, "team/base:latest"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteOutpost(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetOutpost(ctx, name); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not found: %v", err)
	}
	if ids, err := c.GCImages(ctx); err != nil || len(ids) != 1 {
		t.Fatalf("GC: %v %v", ids, err)
	}
	if _, err := c.GetImage(ctx, "team/saved:latest"); err != nil {
		t.Fatal(err)
	}
}

func TestRejectMalformedRequests(t *testing.T) {
	c := localClient(t)
	for _, body := range []string{`{}`, `{"unknown":true}`, `{} {}`, `null`, strings.Repeat("x", 65<<10)} {
		err := c.request(context.Background(), "POST", "/outposts", strings.NewReader(body), "application/json", 201, new(any), false)
		if err == nil {
			t.Fatalf("accepted malformed request: %.30s", body)
		}
	}
	if err := c.request(context.Background(), "POST", "/images?tag=base", strings.NewReader("disk"), "text/plain", 201, new(any), true); err == nil {
		t.Fatal("accepted wrong disk content type")
	}
}

func TestCanceledRequest(t *testing.T) {
	c := localClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ListOutposts(ctx); err == nil {
		t.Fatal("ignored cancellation")
	}
}
