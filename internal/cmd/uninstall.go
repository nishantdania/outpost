package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Outpost",
		Long:  "Remove the configured server and local client, or remove only local client files when the server is unavailable.",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return errors.New("uninstall must be run through the installed Outpost client")
		},
	}
	command.Flags().Bool("yes", false, "Skip confirmation")
	command.Flags().Bool("client-only", false, "Remove local client files without contacting the server")
	return command
}
