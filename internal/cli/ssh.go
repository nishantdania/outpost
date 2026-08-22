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

func sshOutpost(ctx context.Context, id string) error {
	response, err := request(ctx, http.MethodGet, "/outposts")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("daemon returned %s", response.Status)
	}
	var body struct {
		Outposts []outpost.Record `json:"outposts"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return err
	}
	var found outpost.Record
	for _, record := range body.Outposts {
		if record.ID == id || record.Name == id {
			found = record
			break
		}
	}
	if found.ID == "" {
		return fmt.Errorf("outpost not found")
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
