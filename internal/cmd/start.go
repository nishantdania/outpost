package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/client"
)

func newStartCmd(options *rootOptions) *cobra.Command {
	return newArkNameCmd(options, "start <name>", "Start an Ark", (*client.Client).StartArk)
}
