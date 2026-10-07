package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSnapshotCmd(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "snapshot", Short: "Save stopped VM disks as reusable images"}
	var tag string
	create := &cobra.Command{Use: "create <outpost>", Short: "Save a stopped VM disk; no live memory is captured", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if tag == "" {
			return fmt.Errorf("--name is required")
		}
		c, err := imageClient(options)
		if err != nil {
			return err
		}
		result, err := c.SnapshotOutpost(cmd.Context(), args[0], tag)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), result.Digest)
		return nil
	}}
	create.Flags().StringVar(&tag, "name", "", "snapshot image tag (existing tags are replaced)")
	command.AddCommand(create)
	return command
}
