package cmd

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/output"
)

type rootOptions struct {
	output  string
	noColor bool
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	options := &rootOptions{}
	root := &cobra.Command{
		Use:           "outpost",
		Args:          cobra.NoArgs,
		RunE:          func(c *cobra.Command, _ []string) error { return c.Help() },
		Short:         "Create and manage Outposts",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVarP(&options.output, "output", "o", "table", "Output format: table or json")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "Disable color output")
	root.AddCommand(newCreateCmd(options))
	root.AddCommand(newDeleteCmd(options))
	root.AddCommand(newStartCmd(options))
	root.AddCommand(newStopCmd(options))
	root.AddCommand(newInspectCmd(options))
	root.AddCommand(newListCmd(options))
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newImageCmd())
	root.AddCommand(newSnapshotCmd())

	return root
}

func newOutputWriter(options *rootOptions, out io.Writer) (*output.Writer, error) {
	return output.New(output.Options{
		Format:  options.output,
		Out:     out,
		NoColor: options.noColor,
	})
}
