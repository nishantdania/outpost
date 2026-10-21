package cmd

import (
	"fmt"

	"github.com/nishantdania/outpost/internal/client"
	"github.com/spf13/cobra"
)

func newSnapshotCmd() *cobra.Command {
	var tag string
	command := &cobra.Command{
		Use:   "snapshot <name> --tag TAG",
		Short: "Save a stopped VM's disk as an image (not live memory)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tag == "" {
				return fmt.Errorf("--tag is required")
			}
			c, err := client.New()
			if err != nil {
				return err
			}
			result, err := c.SnapshotOutpost(cmd.Context(), args[0], tag)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), result.Digest)
			return nil
		},
	}
	command.Flags().StringVarP(&tag, "tag", "t", "", "image tag (existing tags are replaced)")
	return command
}
