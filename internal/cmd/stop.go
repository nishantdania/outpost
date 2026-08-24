package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/client"
)

func newStopCmd(options *rootOptions) *cobra.Command {
	return newOutpostNameCmd(options, "stop <name>", "Stop an Outpost", (*client.Client).StopOutpost)
}
