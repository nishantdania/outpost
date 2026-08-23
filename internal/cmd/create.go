package cmd

import (
	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/client"
)

func newCreateCmd(options *rootOptions) *cobra.Command {
	return newArkNameCmd(options, "create <name>", "Create an Ark", (*client.Client).CreateArk)
}
