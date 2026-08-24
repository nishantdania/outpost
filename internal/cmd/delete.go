package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/client"
)

func newDeleteCmd(options *rootOptions) *cobra.Command {
	return newOutpostNameCmd(options, "delete <name>", "Delete an Outpost", (*client.Client).DeleteOutpost)
}
