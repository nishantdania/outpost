package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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
	command := exec.CommandContext(ctx, "ssh", "-tt", cfg.SSHHost, "~/.local/bin/outpostd --setup")
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	return command.Run()
}
