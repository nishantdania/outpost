package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
)

func newInspectCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "inspect <name>", Short: "Inspect an Ark", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		arkdClient, err := client.New(options.serverURL, options.token)
		if err != nil {
			return fmt.Errorf("create arkd client: %w", err)
		}
		result, err := arkdClient.GetArk(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		writer, err := newOutputWriter(options, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return writer.Write(result, arkTable([]api.Ark{result}))
	}}
}
