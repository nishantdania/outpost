package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/client"
)

func newStopCmd(options *rootOptions) *cobra.Command {
	return newArkNameCmd(options, "stop <name>", "Stop an Ark", (*client.Client).StopArk)
}
