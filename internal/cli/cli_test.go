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

func TestSelectHost(t *testing.T) {
	t.Setenv("OUTPOST_HOST", "default")
	args, restore, err := selectHost([]string{"--host", "local", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 1 || args[0] != "list" || os.Getenv("OUTPOST_HOST") != "local" {
		t.Fatalf("args = %#v, host = %q", args, os.Getenv("OUTPOST_HOST"))
	}
	restore()
	if os.Getenv("OUTPOST_HOST") != "default" {
		t.Fatalf("restored host = %q", os.Getenv("OUTPOST_HOST"))
	}
}

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
	if got := stdout.String(); got != "Usage: outpost create [name] [--cpus N] [--memory SIZE] [--disk SIZE]\n\nCreate and start a new Outpost. Defaults: 2 vCPU, 4 GiB RAM, 8 GiB disk.\n\nExamples:\n  outpost create dev\n  outpost create build --cpus 4 --memory 8G --disk 32G\n" || stderr.Len() != 0 {
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

func TestParseCreateResources(t *testing.T) {
	options, err := parseCreateArgs([]string{"build", "--cpus", "4", "--memory", "8G", "--disk", "32G"})
	if err != nil {
		t.Fatal(err)
	}
	if options.Name != "build" || options.VCPUs != 4 || options.MemoryMiB != 8192 || options.DiskGiB != 32 {
		t.Fatalf("options = %#v", options)
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
		_, _ = io.WriteString(w, `{"id":"id","name":"name","status":"created","vcpus":2,"memory_mib":4096,"disk_gib":8}`)
	}))
	defer server.Close()

	setClientConfig(t, server.URL)

	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"create"}, "v0.0.2", &stdout, &stderr); code != 0 {
		t.Fatalf("Run() code = %d, stderr = %s", code, stderr.String())
	}
	if got := stdout.String(); got != "Created name (id): 2 vCPU, 4096 MiB RAM, 8 GiB disk\n" {
		t.Errorf("stdout = %q", got)
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
