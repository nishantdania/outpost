package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/client"
)

func newDeleteCmd(options *rootOptions) *cobra.Command {
	command := newOutpostNameCmd(options, "rm <name>", "Remove a VM and its writable disk, keeping source images", (*client.Client).DeleteOutpost)
	command.Aliases = []string{"delete", "remove"}
	return command
}
