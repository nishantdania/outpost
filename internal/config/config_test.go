package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClientFile(t *testing.T) {
	path := writeConfig(t, `{"daemon_url":"http://example.test:8080"}`)

	cfg, err := LoadClientFile(path)
	if err != nil {
		t.Fatalf("LoadClientFile() error = %v", err)
	}
	if cfg.DaemonURL != "http://example.test:8080" {
		t.Errorf("DaemonURL = %q", cfg.DaemonURL)
	}
}

func TestLoadDaemonFile(t *testing.T) {
	path := writeConfig(t, `{"listen_addr":":8080"}`)

	cfg, err := LoadDaemonFile(path)
	if err != nil {
		t.Fatalf("LoadDaemonFile() error = %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q", cfg.ListenAddr)
	}
}

func TestLoadClientFileRequiresDaemonURL(t *testing.T) {
	path := writeConfig(t, `{}`)
	if _, err := LoadClientFile(path); err == nil {
		t.Fatal("LoadClientFile() error = nil, want error")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
