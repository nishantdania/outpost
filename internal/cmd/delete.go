package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/client"
)

func newDeleteCmd(options *rootOptions) *cobra.Command {
	return newArkNameCmd(options, "delete <name>", "Delete an Ark", (*client.Client).DeleteArk)
}
