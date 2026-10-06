package launcher

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/nishantdania/outpost/internal/launcherclient"
)

func TestSnapshotStreamsOverAuthenticatedLauncherSocket(t *testing.T) {
	runtime := testRuntime(t, func(c *FirecrackerConfig) {
		c.Runner = runnerFunc(func(context.Context, string, ...string) ([]byte, error) { return nil, nil })
	})
	spec := testSpec()
	seedVM(t, runtime, spec, false)
	socket := filepath.Join(t.TempDir(), "launcher.sock")
	server, err := NewServer(Config{SocketPath: socket, RuntimeDir: filepath.Dir(socket), StateDir: runtime.config.StateDir, SocketGID: -1, AllowedUID: os.Getuid()}, runtime)
	if err != nil {
		t.Fatal(err)
	}
	done := serve(t, server, socket)
	defer stop(t, server, done)
	client := launcherclient.New(socket)
	defer client.Close()
	disk, err := client.OpenSnapshot(t.Context(), spec.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(disk)
	disk.Close()
	if err != nil || string(data) != "disk" {
		t.Fatalf("snapshot = %q, %v", data, err)
	}
	if _, err := client.OpenSnapshot(t.Context(), "../../etc/passwd"); err == nil {
		t.Fatal("accepted arbitrary path")
	}
	matches, _ := filepath.Glob(filepath.Join(runtime.config.StateDir, ".snapshot-*"))
	if len(matches) != 0 {
		t.Fatalf("temporary files remain: %v", matches)
	}
}
