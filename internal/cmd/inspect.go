package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
)

func newInspectCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "inspect <name>", Short: "Inspect an Outpost", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		outpostdClient, err := client.New(options.serverURL, options.token)
		if err != nil {
			return fmt.Errorf("create outpostd client: %w", err)
		}
		result, err := outpostdClient.GetOutpost(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		writer, err := newOutputWriter(options, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return writer.Write(result, outpostTable([]api.Outpost{result}))
	}}
}
