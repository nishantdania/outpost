package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/nishantdania/outpost/internal/config"
)

func execOutpost(ctx context.Context, identifier, script string) error {
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
	remote := "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ~/.local/share/outpost/id_ed25519 root@" + record.IP + " 'bash -lc \"$(printf %s " + encoded + " | base64 -d)\"'"
	command := exec.CommandContext(ctx, "ssh", cfg.SSHHost, remote)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
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
