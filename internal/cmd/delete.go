package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
)

func newDeleteCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete an Ark",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arkdClient, err := client.New(options.serverURL)
			if err != nil {
				return fmt.Errorf("create arkd client: %w", err)
			}

			deleted, err := arkdClient.DeleteArk(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			return writer.Write(deleted, arkTable([]api.Ark{deleted}))
		},
	}
}
