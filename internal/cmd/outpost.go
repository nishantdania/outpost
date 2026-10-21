package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
)

type outpostNameAction func(*client.Client, context.Context, string) (api.Outpost, error)

func newOutpostNameCmd(options *rootOptions, use, short string, action outpostNameAction) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			launcherClient, err := client.New()
			if err != nil {
				return fmt.Errorf("create launcher client: %w", err)
			}

			result, err := action(launcherClient, cmd.Context(), args[0])
			if err != nil {
				return err
			}

			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			return writer.Write(result, outpostTable([]api.Outpost{result}))
		},
	}
}
