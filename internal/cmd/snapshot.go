package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/spf13/cobra"
)

func newSnapshotCmd(options *rootOptions) *cobra.Command {
	command := &cobra.Command{Use: "snapshot", Short: "Save stopped VM disks as reusable images"}
	var tag, profileFile string
	create := &cobra.Command{Use: "create <outpost>", Short: "Save a stopped VM disk; no live memory is captured", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if tag == "" {
			return fmt.Errorf("--name is required")
		}
		c, err := imageClient(options)
		if err != nil {
			return err
		}
		var profile *credentials.Profile
		if profileFile != "" {
			f, err := os.Open(profileFile)
			if err != nil {
				return fmt.Errorf("open credential profile: %w", err)
			}
			defer f.Close()
			data, err := io.ReadAll(io.LimitReader(f, 65537))
			if err != nil {
				return fmt.Errorf("read credential profile")
			}
			p, err := credentials.Parse(data)
			if err != nil {
				return err
			}
			profile = &p
		}
		result, err := c.SnapshotOutpostWithCredentials(cmd.Context(), args[0], tag, profile)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), result.Digest)
		return nil
	}}
	create.Flags().StringVar(&profileFile, "credentials", "", "JSON host credential bindings to inherit into snapshot forks")
	create.Flags().StringVar(&tag, "name", "", "snapshot image tag (existing tags are replaced)")
	command.AddCommand(create)
	return command
}
