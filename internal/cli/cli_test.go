package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), nil, "v0.0.2", &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if got := stdout.String(); got == "" || stderr.Len() != 0 {
		t.Errorf("help output = stdout %q, stderr %q", got, stderr.String())
	}
}

func TestRunCommandHelpDoesNotRunCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create", "--help"}, "v0.0.2", &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d", code)
	}
	if got := stdout.String(); got != "Usage: outpost create [name]\n\nCreate and start a new Outpost.\n\nExamples:\n  outpost create dev\n" || stderr.Len() != 0 {
		t.Fatalf("output = stdout %q, stderr %q", got, stderr.String())
	}
}

func TestRunAliasHelpUsesCanonicalCommand(t *testing.T) {
	for _, test := range []struct{ alias, usage string }{{"cp", "Usage: outpost copy"}, {"ls", "Usage: outpost list"}} {
		var stdout, stderr bytes.Buffer
		if code := Run(context.Background(), []string{test.alias, "--help"}, "v0.0.2", &stdout, &stderr); code != 0 {
			t.Fatalf("Run(%s) code = %d", test.alias, code)
		}
		if !strings.HasPrefix(stdout.String(), test.usage) || stderr.Len() != 0 {
			t.Fatalf("Run(%s) output = stdout %q, stderr %q", test.alias, stdout.String(), stderr.String())
		}
	}
}

func TestRunCreateRejectsOptionAsName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create", "--unknown"}, "v0.0.2", &stdout, &stderr); code != 1 {
		t.Fatalf("Run() code = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() == 0 {
		t.Fatalf("output = stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestRunCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/outposts" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"id","name":"name","status":"created"}`)
	}))
	defer server.Close()

	setClientConfig(t, server.URL)

	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create"}, "v0.0.2", &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); got != "Created name (id)\n" {
		t.Errorf("stdout = %q, want %q", got, "Created name (id)\n")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"nope"}, "v0.0.2", &stdout, &stderr); code != 2 {
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
