package cmd

import "github.com/spf13/cobra"

type rootOptions struct {
	serverURL string
	output    string
	noColor   bool
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	options := &rootOptions{}
	root := &cobra.Command{
		Use:   "ark",
		Short: "Create and manage Arks",
	}

	root.PersistentFlags().StringVar(&options.serverURL, "server", "http://127.0.0.1:17890", "arkd server URL")
	root.PersistentFlags().StringVarP(&options.output, "output", "o", "table", "Output format: table or json")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "Disable color output")
	root.AddCommand(newCreateCmd(options))
	root.AddCommand(newListCmd(options))

	return root
}
