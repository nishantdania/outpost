package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/client"
)

func newStartCmd(options *rootOptions) *cobra.Command {
	return newOutpostNameCmd(options, "start <name>", "Start an Outpost", (*client.Client).StartOutpost)
}
