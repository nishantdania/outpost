package daemon

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/httpapi"
	"github.com/nishantdania/ark/internal/launcher"
	"github.com/nishantdania/ark/internal/launcherclient"
	"github.com/nishantdania/ark/internal/service"
)

func TestArkdLifecycleCrossesLauncherUnixSocket(t *testing.T) {
	directory := t.TempDir()
	socket := filepath.Join(directory, "launcher.sock")
	launcherServer, err := launcher.NewServer(launcher.Config{SocketPath: socket, RuntimeDir: directory, StateDir: filepath.Join(directory, "state"), SocketGID: -1, Authorize: func(int) bool { return true }}, launcher.NewMemoryRuntime())
	if err != nil {
		t.Fatal(err)
	}
	launcherDone := make(chan error, 1)
	go func() { launcherDone <- launcherServer.ListenAndServe() }()
	waitForSocket(t, socket)
	defer func() {
		_ = launcherServer.Shutdown(context.Background())
		if err := <-launcherDone; err != nil {
			t.Error(err)
		}
	}()

	store, err := ark.Open(context.Background(), filepath.Join(directory, "ark.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := launcherclient.New(socket)
	defer manager.Close()
	api := httptest.NewServer(httpapi.NewServer("", service.New(store, manager), "token").Handler)
	defer api.Close()
	client := api.Client()
	for _, request := range []struct{ method, path, body string }{
		{"POST", "/v1/arks", `{"name":"demo","image_id":"default","vcpus":2,"memory_mib":1024,"disk_gib":8}`},
		{"POST", "/v1/arks/demo/stop", ""},
		{"POST", "/v1/arks/demo/start", ""},
		{"DELETE", "/v1/arks/demo", ""},
	} {
		doAuthorized(t, client, api.URL+request.path, request.method, request.body)
	}
}

func doAuthorized(t *testing.T, client *http.Client, url, method, body string) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer token")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("%s %s status = %d", method, url, response.StatusCode)
	}
}

func waitForSocket(t *testing.T, socket string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socket); err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("launcher socket was not created")
}
