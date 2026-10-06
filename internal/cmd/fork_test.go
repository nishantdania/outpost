package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/remote"
	"github.com/spf13/cobra"
)

func gitTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com", "GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, output, err)
	}
	return strings.TrimSpace(string(output))
}

func TestBranchBundleAndCheckoutPreserveEnvironment(t *testing.T) {
	repo := t.TempDir()
	gitTest(t, repo, "init", "-b", "main")
	os.WriteFile(filepath.Join(repo, "app.txt"), []byte("baseline"), 0600)
	os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("node_modules/\n"), 0600)
	gitTest(t, repo, "add", "app.txt", ".gitignore")
	gitTest(t, repo, "commit", "-m", "baseline")
	baseline := gitTest(t, repo, "rev-parse", "HEAD")
	gitTest(t, repo, "checkout", "-b", "feature")
	os.WriteFile(filepath.Join(repo, "app.txt"), []byte("feature"), 0600)
	gitTest(t, repo, "commit", "-am", "feature")
	feature := gitTest(t, repo, "rev-parse", "HEAD")
	// These host working-tree files and config must not enter the bundle.
	os.WriteFile(filepath.Join(repo, "app.txt"), []byte("uncommitted"), 0600)
	os.WriteFile(filepath.Join(repo, "host-secret.env"), []byte("secret"), 0600)
	gitTest(t, repo, "remote", "add", "origin", "https://secret@example.invalid/repo")
	gitTest(t, repo, "config", "credential.helper", "some-host-helper")
	command := &cobra.Command{}
	command.SetContext(t.Context())
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)
	guest := filepath.Join(t.TempDir(), "workspace ' with spaces;")
	runCheckout := func(branch string, wantError bool) {
		t.Helper()
		bundle, sha, err := branchBundle(command, remote.SystemRunner(), repo, branch, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("sh", "-c", checkoutScript(guest, bundle, sha)).CombinedOutput()
		if (err != nil) != wantError {
			t.Fatalf("checkout error = %v, output = %s", err, output)
		}
		if _, err := os.Stat(bundle); !os.IsNotExist(err) {
			t.Fatal("bundle not cleaned up")
		}
	}
	runCheckout("main", false)
	if got := gitTest(t, guest, "rev-parse", "HEAD"); got != baseline {
		t.Fatalf("baseline = %s", got)
	}
	os.Mkdir(filepath.Join(guest, "node_modules"), 0700)
	os.WriteFile(filepath.Join(guest, "node_modules", "cached"), []byte("dependencies"), 0600)
	runCheckout("feature", false)
	if got := gitTest(t, guest, "rev-parse", "HEAD"); got != feature {
		t.Fatalf("feature = %s", got)
	}
	data, _ := os.ReadFile(filepath.Join(guest, "app.txt"))
	if string(data) != "feature" {
		t.Fatalf("app = %s", data)
	}
	if _, err := os.Stat(filepath.Join(guest, "node_modules", "cached")); err != nil {
		t.Fatal("lost installed dependencies")
	}
	if _, err := os.Stat(filepath.Join(guest, "host-secret.env")); !os.IsNotExist(err) {
		t.Fatal("copied host secret")
	}
	if remotes := gitTest(t, guest, "remote"); remotes != "" {
		t.Fatalf("copied host remotes: %s", remotes)
	}
	config := gitTest(t, guest, "config", "--local", "--list")
	if strings.Contains(config, "credential") || strings.Contains(config, "secret@") {
		t.Fatal("copied host config")
	}
	// A branch that begins tracking an ignored dependency must not replace
	// the baseline's installed file either.
	gitTest(t, repo, "checkout", "-b", "collision")
	os.Mkdir(filepath.Join(repo, "node_modules"), 0700)
	os.WriteFile(filepath.Join(repo, "node_modules", "cached"), []byte("branch file"), 0600)
	gitTest(t, repo, "add", "-f", "node_modules/cached")
	gitTest(t, repo, "commit", "-m", "track conflicting dependency")
	runCheckout("collision", true)
	dependencies, _ := os.ReadFile(filepath.Join(guest, "node_modules", "cached"))
	if string(dependencies) != "dependencies" {
		t.Fatal("overwrote ignored dependency")
	}
	os.WriteFile(filepath.Join(guest, "app.txt"), []byte("dirty baseline"), 0600)
	runCheckout("main", true)
	data, _ = os.ReadFile(filepath.Join(guest, "app.txt"))
	if string(data) != "dirty baseline" {
		t.Fatal("overwrote baseline modifications")
	}
}

func TestForkValidatesBeforeProvisioning(t *testing.T) {
	for _, args := range [][]string{
		{"ready"},
		{"ready", "--name", "feature", "--branch", "feature"},
		{"ready", "--name", "feature", "--path", "/"},
		{"ready", "--name", "feature", "--path", "relative"},
	} {
		command := newForkCmd(&rootOptions{})
		command.SetArgs(args)
		command.SetOut(&bytes.Buffer{})
		command.SetErr(io.Discard)
		if err := command.Execute(); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
