package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nishantdania/outpost/internal/config"
)

func setup(ctx context.Context) error {
	cfg, err := config.LoadClient()
	if err != nil {
		return err
	}
	if cfg.SSHHost == "" {
		return fmt.Errorf("ssh_host is required in client configuration")
	}
	var command *exec.Cmd
	if cfg.SSHHost == "local" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		command = exec.CommandContext(ctx, filepath.Join(home, ".local", "bin", "outpostd"), "--setup")
	} else {
		command = exec.CommandContext(ctx, "ssh", "-tt", cfg.SSHHost, "~/.local/bin/outpostd --setup")
	}
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
