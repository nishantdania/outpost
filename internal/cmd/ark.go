package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
)

type arkNameAction func(*client.Client, context.Context, string) (api.Ark, error)

func newArkNameCmd(options *rootOptions, use, short string, action arkNameAction) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arkdClient, err := client.New(options.serverURL)
			if err != nil {
				return fmt.Errorf("create arkd client: %w", err)
			}

			result, err := action(arkdClient, cmd.Context(), args[0])
			if err != nil {
				return err
			}

			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			return writer.Write(result, arkTable([]api.Ark{result}))
		},
	}
}
