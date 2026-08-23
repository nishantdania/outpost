package launcher

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/launcherclient"
	"github.com/nishantdania/ark/internal/vmapi"
)

func TestServerLifecycleAndSocketPermissions(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "run", "launcher.sock")
	server, err := NewServer(Config{SocketPath: socket, RuntimeDir: filepath.Dir(socket), StateDir: filepath.Join(t.TempDir(), "state"), SocketGID: -1, AllowedUID: os.Getuid()}, NewMemoryRuntime())
	if err != nil {
		t.Fatal(err)
	}
	done := serve(t, server, socket)
	defer func() { stop(t, server, done) }()
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0660 {
		t.Fatalf("socket permissions = %o, want 660", info.Mode().Perm())
	}
	client := launcherclient.New(socket)
	defer client.Close()
	a := testArk()
	if err := client.Create(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if err := client.Create(context.Background(), a); err != nil {
		t.Fatalf("idempotent create: %v", err)
	}
	conflict := a
	conflict.VCPUs++
	if err := client.Create(context.Background(), conflict); !errors.Is(err, vmapi.ErrConflict) {
		t.Fatalf("conflicting Create() error = %v", err)
	}
	listed, err := client.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0].Spec.ID != a.ID {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	ip, err := client.Start(context.Background(), a.ID)
	if err != nil || ip != "172.30.0.2" {
		t.Fatalf("Start() = %q, %v", ip, err)
	}
	if err := client.Stop(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), a.ID); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	if _, err := client.Start(context.Background(), a.ID); !errors.Is(err, vmapi.ErrNotFound) {
		t.Fatalf("Start() error = %v", err)
	}
}

func TestServerRejectsMalformedProtocol(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "launcher.sock")
	server, err := NewServer(Config{SocketPath: socket, RuntimeDir: filepath.Dir(socket), StateDir: filepath.Join(t.TempDir(), "state"), SocketGID: -1, AllowedUID: os.Getuid()}, NewMemoryRuntime())
	if err != nil {
		t.Fatal(err)
	}
	done := serve(t, server, socket)
	defer func() { stop(t, server, done) }()
	a := testArk()
	valid := `{"version":1,"spec":{"id":"` + a.ID + `","image_id":"default","vcpus":2,"memory_mib":1024,"disk_gib":8}}`
	for _, body := range []string{
		valid + ` {}`,
		valid[:len(valid)-1] + `,"unknown":true}`,
		valid + strings.Repeat(" ", maxBodyBytes),
	} {
		if status := post(t, socket, body); status != http.StatusBadRequest {
			t.Fatalf("malformed request status = %d, want %d", status, http.StatusBadRequest)
		}
	}
}

func TestServerRejectsUnauthorizedPeer(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "launcher.sock")
	server, err := NewServer(Config{SocketPath: socket, RuntimeDir: filepath.Dir(socket), StateDir: filepath.Join(t.TempDir(), "state"), SocketGID: -1, Authorize: func(int) bool { return false }}, NewMemoryRuntime())
	if err != nil {
		t.Fatal(err)
	}
	done := serve(t, server, socket)
	defer func() { stop(t, server, done) }()
	client := launcherclient.New(socket)
	defer client.Close()
	if err := client.Create(context.Background(), testArk()); err == nil {
		t.Fatal("unauthorized Create() error = nil")
	}
}

func post(t *testing.T, socket, body string) int {
	t.Helper()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequest(http.MethodPost, "http://unix/v1/create", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func TestRemoveStaleSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err == nil {
		t.Fatal("removeStaleSocket() removed active socket")
	}
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err != nil {
		t.Fatalf("removeStaleSocket() stale socket error = %v", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale socket remains: %v", err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleSocket(path); err == nil {
		t.Fatal("removeStaleSocket() removed regular file")
	}
}

func serve(t *testing.T, server *Server, socket string) chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("launcher did not listen: %v", err)
		default:
		}
		conn, err := net.Dial("unix", socket)
		if err == nil {
			_ = conn.Close()
			return done
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("launcher did not listen")
	return nil
}
func stop(t *testing.T, server *Server, done chan error) {
	t.Helper()
	if err := server.Shutdown(context.Background()); err != nil {
		t.Error(err)
	}
	if err := <-done; err != nil {
		t.Error(err)
	}
	if _, err := os.Lstat(server.config.SocketPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("socket remains after shutdown: %v", err)
	}
}
func testArk() ark.Ark {
	return ark.Ark{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}
}
