package cmd

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/output"
)

type rootOptions struct {
	serverURL string
	token     string
	output    string
	noColor   bool
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	options := &rootOptions{}
	root := &cobra.Command{
		Use:           "ark",
		Short:         "Create and manage Arks",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVar(&options.serverURL, "server", "http://127.0.0.1:17890", "arkd server URL")
	root.PersistentFlags().StringVar(&options.token, "token", os.Getenv("ARK_TOKEN"), "arkd bearer token")
	root.PersistentFlags().StringVarP(&options.output, "output", "o", "table", "Output format: table or json")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "Disable color output")
	root.AddCommand(newCreateCmd(options))
	root.AddCommand(newDeleteCmd(options))
	root.AddCommand(newStartCmd(options))
	root.AddCommand(newStopCmd(options))
	root.AddCommand(newInspectCmd(options))
	root.AddCommand(newListCmd(options))

	return root
}

func newOutputWriter(options *rootOptions, out io.Writer) (*output.Writer, error) {
	return output.New(output.Options{
		Format:  options.output,
		Out:     out,
		NoColor: options.noColor,
	})
}
