package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
	"github.com/nishantdania/ark/internal/output"
)

func newCreateCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create an Ark",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arkdClient, err := client.New(options.serverURL)
			if err != nil {
				return fmt.Errorf("create arkd client: %w", err)
			}

			created, err := arkdClient.CreateArk(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			writer, err := output.New(output.Options{
				Format:  options.output,
				Out:     cmd.OutOrStdout(),
				NoColor: options.noColor,
			})
			if err != nil {
				return err
			}

			return writer.Write(created, arkTable([]api.Ark{created}))
		},
	}
}
