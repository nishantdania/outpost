package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nishantdania/outpost/internal/config"
)

func execOutpost(ctx context.Context, identifier, script string, interactive bool) error {
	record, err := findOutpost(ctx, identifier)
	if err != nil {
		return err
	}
	if record.Status != "running" || record.IP == "" {
		return fmt.Errorf("outpost is not reachable")
	}
	cfg, err := config.LoadClient()
	if err != nil {
		return err
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(script))
	guest := "bash -lc \"$(printf %s " + encoded + " | base64 -d)\""
	var command *exec.Cmd
	if cfg.SSHHost == "local" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		key := filepath.Join(home, ".local", "share", "outpost", "id_ed25519")
		command = exec.CommandContext(ctx, "ssh", "-o", "LogLevel=ERROR", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", key, "root@"+record.IP, guest)
	} else {
		remote := "ssh -o LogLevel=ERROR -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ~/.local/share/outpost/id_ed25519 root@" + record.IP + " " + shellQuote(guest)
		command = exec.CommandContext(ctx, "ssh", cfg.SSHHost, remote)
	}
	if interactive {
		command.Stdin = os.Stdin
	}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	return command.Run()
}

func runExec(err error, stderr io.Writer) int {
	if err == nil {
		return 0
	}
	fmt.Fprintf(stderr, "outpost exec: %v\n", err)
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return exitError.ExitCode()
	}
	return 1
}
