package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nishantdania/outpost/internal/config"
)

const helpText = `Usage:
  outpost <command>

Commands:
  create    Create an Outpost
  help      Show help
`

const createHelpText = `Usage:
  outpost create

Create an Outpost.
`

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0:
		fmt.Fprint(stdout, helpText)
		return 0
	case isHelp(args):
		fmt.Fprint(stdout, helpText)
		return 0
	case len(args) == 2 && args[0] == "create" && isHelp(args[1:]):
		fmt.Fprint(stdout, createHelpText)
		return 0
	case len(args) == 1 && args[0] == "create":
		if err := create(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost create: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", strings.Join(args, " "))
		fmt.Fprint(stderr, helpText)
		return 2
	}
}

func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")
}

func create(ctx context.Context, stdout io.Writer) error {
	cfg, err := config.LoadClient()
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.DaemonURL, "/")+"/outposts", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("call daemon: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}

	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode daemon response: %w", err)
	}
	fmt.Fprintln(stdout, body.Message)
	return nil
}
