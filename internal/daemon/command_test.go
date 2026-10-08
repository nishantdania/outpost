package daemon

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParseConfigToken(t *testing.T) {
	const secret = "secret-token-from-environment"
	t.Setenv("OUTPOSTD_TOKEN", secret)
	config, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if config.Token != secret {
		t.Fatal("token environment default was not preserved")
	}
	config, err = parseConfig([]string{"--token", "override-token"})
	if err != nil {
		t.Fatal(err)
	}
	if config.Token != "override-token" {
		t.Fatal("explicit token override was not preserved")
	}

	for _, arg := range []string{"--help", "--unknown"} {
		t.Run(arg, func(t *testing.T) {
			output, err := os.CreateTemp(t.TempDir(), "usage")
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			stderr := os.Stderr
			os.Stderr = output
			defer func() { os.Stderr = stderr }()
			_, err = parseConfig([]string{arg})
			if err == nil || (arg == "--help" && !errors.Is(err, flag.ErrHelp)) {
				t.Fatalf("unexpected parse error: %v", err)
			}
			data, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), secret) {
				t.Fatal("help/usage exposed the token")
			}
			if !strings.Contains(string(data), "-token") || !strings.Contains(string(data), "OUTPOSTD_TOKEN") {
				t.Fatal("help must still document the token flag and environment variable")
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantAddr string
		wantDB   string
		wantErr  bool
	}{
		{
			name:     "default configuration",
			wantAddr: "127.0.0.1:17890",
			wantDB:   "./outpost.db",
		},
		{
			name:     "custom configuration",
			args:     []string{"--listen", ":8080", "--database", "data/outpost.db"},
			wantAddr: ":8080",
			wantDB:   "data/outpost.db",
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := parseConfig(tt.args)

			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfig() error = %v, want error = %t", err, tt.wantErr)
			}

			if err == nil && config.ListenAddr != tt.wantAddr {
				t.Fatalf("ListenAddr = %q, want %q", config.ListenAddr, tt.wantAddr)
			}

			if err == nil && config.DatabasePath != tt.wantDB {
				t.Fatalf("DatabasePath = %q, want %q", config.DatabasePath, tt.wantDB)
			}
		})
	}
}
