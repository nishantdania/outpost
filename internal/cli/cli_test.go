package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), nil, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got := stdout.String(); got == "" || stderr.Len() != 0 {
		t.Errorf("help output = stdout %q, stderr %q", got, stderr.String())
	}
}

func TestRunCreateHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create", "--help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got := stdout.String(); got != createHelpText {
		t.Errorf("stdout = %q, want %q", got, createHelpText)
	}
}

func TestRunCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/outposts" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"message":"Hello, World!"}`)
	}))
	defer server.Close()

	setClientConfig(t, server.URL)

	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create"}, &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); got != "Hello, World!\n" {
		t.Errorf("stdout = %q, want %q", got, "Hello, World!\n")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Errorf("output = stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func setClientConfig(t *testing.T, daemonURL string) {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	path := filepath.Join(configHome, "outpost", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte(`{"daemon_url":"` + daemonURL + `"}`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
