package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
	"github.com/nishantdania/ark/internal/output"
)

func newListCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Arks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			arkdClient, err := client.New(options.serverURL)
			if err != nil {
				return fmt.Errorf("create arkd client: %w", err)
			}

			arks, err := arkdClient.ListArks(cmd.Context())
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

			return writer.Write(arks, arkTable(arks))
		},
	}
}

func arkTable(arks []api.Ark) output.Table {
	table := output.Table{
		Headers: []string{"ID", "NAME", "STATUS"},
		Rows:    make([][]string, 0, len(arks)),
		ColumnStyles: map[int]output.ColumnStyle{
			2: output.ColumnStyleStatus,
		},
	}

	for _, ark := range arks {
		table.Rows = append(table.Rows, []string{ark.Id, ark.Name, ark.Status})
	}

	return table
}
