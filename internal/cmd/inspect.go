package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
)

func newInspectCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "inspect <name>", Short: "Inspect an Outpost", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		launcherClient, err := client.New()
		if err != nil {
			return fmt.Errorf("create launcher client: %w", err)
		}
		result, err := launcherClient.GetOutpost(cmd.Context(), args[0])
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
