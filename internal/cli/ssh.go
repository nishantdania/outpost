package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/nishantdania/outpost/internal/config"
	"github.com/nishantdania/outpost/internal/outpost"
)

func findOutpost(ctx context.Context, identifier string) (outpost.Record, error) {
	response, err := request(ctx, http.MethodGet, "/outposts")
	if err != nil {
		return outpost.Record{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return outpost.Record{}, fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		Outposts []outpost.Record `json:"outposts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return outpost.Record{}, err
	}
	for _, record := range body.Outposts {
		if record.ID == identifier || record.Name == identifier {
			return record, nil
		}
	}
	return outpost.Record{}, fmt.Errorf("outpost not found")
}

func sshOutpost(ctx context.Context, identifier string) error {
	found, err := findOutpost(ctx, identifier)
	if err != nil {
		return err
	}
	if found.Status != "running" || found.IP == "" {
		return fmt.Errorf("outpost is not reachable")
	}
	cfg, err := config.LoadClient()
	if err != nil {
		return err
	}
	remote := "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i ~/.local/share/outpost/id_ed25519 root@" + found.IP
	command := exec.CommandContext(ctx, "ssh", "-t", cfg.SSHHost, remote)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}
