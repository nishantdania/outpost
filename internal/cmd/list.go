package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/output"
)

func newListCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List Outposts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			outpostdClient, err := client.New(options.serverURL, options.token)
			if err != nil {
				return fmt.Errorf("create outpostd client: %w", err)
			}

			outposts, err := outpostdClient.ListOutposts(cmd.Context())
			if err != nil {
				return err
			}

			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			return writer.Write(outposts, outpostTable(outposts))
		},
	}
}

func outpostTable(outposts []api.Outpost) output.Table {
	table := output.Table{
		Headers: []string{"NAME", "STATUS", "IMAGE", "CPUS", "MEMORY", "DISK", "IP"},
		Rows:    make([][]string, 0, len(outposts)),
	}

	for _, outpost := range outposts {
		table.Rows = append(table.Rows, []string{outpost.Name, string(outpost.Status), outpost.ImageId, fmt.Sprint(outpost.Vcpus), fmt.Sprint(outpost.MemoryMib), fmt.Sprint(outpost.DiskGib), outpost.GuestIp})
	}

	return table
}
