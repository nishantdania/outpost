package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nishantdania/outpost/internal/config"
	"github.com/nishantdania/outpost/internal/update"
)

const helpText = `Usage:
  outpost <command>

Commands:
  create    Create an Outpost
  list      List Outposts
  update    Update Outpost
  uninstall Remove Outpost
  version   Show versions
  help      Show help
`

func Run(ctx context.Context, args []string, version string, stdout, stderr io.Writer) int {
	switch {
	case len(args) == 0 || isHelp(args):
		fmt.Fprint(stdout, helpText)
		return 0
	case (len(args) == 1 || len(args) == 2) && args[0] == "create":
		name := ""
		if len(args) == 2 {
			name = args[1]
		}
		if err := create(ctx, name, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost create: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 1 && args[0] == "list":
		if err := list(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost list: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 1 && args[0] == "version":
		if err := versions(ctx, version, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost version: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "version" && args[1] == "local":
		fmt.Fprintf(stdout, "outpost %s\n", version)
		return 0
	case len(args) == 2 && args[0] == "version" && args[1] == "server":
		if err := serverVersion(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost version server: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 1 && args[0] == "update":
		if err := serverUpdate(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost update: %v\n", err)
			return 1
		}
		if err := localUpdate(ctx, version, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost update: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 1 && args[0] == "uninstall":
		if err := serverUninstall(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost uninstall: %v\n", err)
			return 1
		}
		if err := localUninstall(stdout); err != nil {
			fmt.Fprintf(stderr, "outpost uninstall: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "uninstall" && args[1] == "local":
		if err := localUninstall(stdout); err != nil {
			fmt.Fprintf(stderr, "outpost uninstall local: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "uninstall" && args[1] == "server":
		if err := serverUninstall(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost uninstall server: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "update" && args[1] == "local":
		if err := localUpdate(ctx, version, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost update local: %v\n", err)
			return 1
		}
		return 0
	case len(args) == 2 && args[0] == "update" && args[1] == "server":
		if err := serverUpdate(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "outpost update server: %v\n", err)
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

func daemonURL() (string, error) {
	cfg, err := config.LoadClient()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(cfg.DaemonURL, "/"), nil
}
func request(ctx context.Context, method, path string) (*http.Response, error) {
	base, err := daemonURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, nil)
	if err != nil {
		return nil, err
	}
	return (&http.Client{Timeout: 30 * time.Second}).Do(req)
}

func create(ctx context.Context, name string, stdout io.Writer) error {
	base, err := daemonURL()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(struct {
		Name string `json:"name"`
	}{name})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/outposts", strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Created %s (%s)\n", body.Name, body.ID)
	return nil
}

func list(ctx context.Context, stdout io.Writer) error {
	response, err := request(ctx, http.MethodGet, "/outposts")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		Outposts []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"outposts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	for _, record := range body.Outposts {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", record.ID, record.Name, record.Status)
	}
	return nil
}
func versions(ctx context.Context, version string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Local:  %s\n", version)
	return serverVersion(ctx, stdout)
}
func serverVersion(ctx context.Context, stdout io.Writer) error {
	response, err := request(ctx, http.MethodGet, "/version")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Server: %s\n", body.Version)
	return nil
}
func localUpdate(ctx context.Context, version string, stdout io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	result, err := update.Apply(ctx, update.Options{Component: "outpost", CurrentVersion: version, Executable: executable})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Local:  %s → %s\n", result.CurrentVersion, result.LatestVersion)
	return nil
}
func serverUninstall(ctx context.Context, stdout io.Writer) error {
	response, err := request(ctx, http.MethodPost, "/uninstall")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	fmt.Fprintln(stdout, "Server: uninstalled")
	return nil
}

func localUninstall(stdout io.Writer) error {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(configDirectory, "outpost")); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Remove(executable); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "Local:  uninstalled")
	return nil
}

func serverUpdate(ctx context.Context, stdout io.Writer) error {
	response, err := request(ctx, http.MethodPost, "/update")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		CurrentVersion string `json:"current_version"`
		LatestVersion  string `json:"latest_version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Server: %s → %s\n", body.CurrentVersion, body.LatestVersion)
	return nil
}
