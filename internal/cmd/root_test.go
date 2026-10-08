package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandTokenHelp(t *testing.T) {
	const secret = "secret-token-from-environment"
	t.Setenv("OUTPOST_TOKEN", secret)
	for _, args := range [][]string{{"--help"}, {"list", "--help"}, {"image", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := newRootCmd()
			if got, _ := root.PersistentFlags().GetString("token"); got != secret {
				t.Fatal("token environment default was not preserved")
			}
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if err := root.Usage(); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), secret) {
				t.Fatal("help/usage exposed the token")
			}
			if !strings.Contains(output.String(), "--token") || !strings.Contains(output.String(), "OUTPOST_TOKEN") {
				t.Fatal("help must still document the token flag and environment variable")
			}
			if err := root.PersistentFlags().Set("token", "override-token"); err != nil {
				t.Fatal(err)
			}
			if got, _ := root.PersistentFlags().GetString("token"); got != "override-token" {
				t.Fatal("explicit token override was not preserved")
			}
		})
	}
}

func TestRootCommandUsesServerEnvironment(t *testing.T) {
	t.Setenv("OUTPOST_SERVER", "https://handoff.example")
	root := newRootCmd()
	if got, _ := root.PersistentFlags().GetString("server"); got != "https://handoff.example" {
		t.Fatalf("server = %q", got)
	}
	if err := root.PersistentFlags().Set("server", "https://override.example"); err != nil {
		t.Fatal(err)
	}
	if got, _ := root.PersistentFlags().GetString("server"); got != "https://override.example" {
		t.Fatalf("server override = %q", got)
	}
}

func TestRootCommandHasUninstallHelp(t *testing.T) {
	root := newRootCmd()
	uninstall, args, err := root.Find([]string{"uninstall"})
	if err != nil {
		t.Fatalf("find uninstall command: %v", err)
	}
	if len(args) != 0 {
		t.Fatalf("args = %v, want none", args)
	}
	if uninstall.Flags().Lookup("client-only") == nil || uninstall.Flags().Lookup("yes") == nil {
		t.Fatal("uninstall flags are missing")
	}
}

func TestRootCommandHasListCommand(t *testing.T) {
	root := newRootCmd()

	list, args, err := root.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list command: %v", err)
	}

	if len(args) != 0 {
		t.Fatalf("args = %v, want none", args)
	}

	if list.Name() != "list" {
		t.Fatalf("command name = %q, want %q", list.Name(), "list")
	}
}
