package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nishantdania/outpost/internal/client"
	"github.com/spf13/cobra"
)

func newImageCmd() *cobra.Command {
	image := &cobra.Command{Use: "image", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
	var tag string
	importCmd := &cobra.Command{Use: "import TAG PATH|-", Short: "Import a disk image under a reusable tag", Args: cobra.RangeArgs(1, 2), RunE: func(c *cobra.Command, args []string) error {
		path := args[0]
		if len(args) == 2 {
			if tag != "" {
				return fmt.Errorf("specify either TAG PATH or --tag TAG PATH")
			}
			tag, path = args[0], args[1]
		}
		if tag == "" {
			return fmt.Errorf("provide TAG PATH or --tag TAG PATH")
		}
		var input io.Reader
		var close func() error
		if path == "-" {
			input = c.InOrStdin()
			close = func() error { return nil }
		} else {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			input = f
			close = f.Close
		}
		defer close()
		api, err := client.New()
		if err != nil {
			return err
		}
		result, err := api.ImportImage(c.Context(), tag, input)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.OutOrStdout(), result.Digest)
		return nil
	}}
	importCmd.Flags().StringVarP(&tag, "tag", "t", "", "image tag")
	list := &cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		api, err := client.New()
		if err != nil {
			return err
		}
		v, err := api.ListImages(c.Context())
		if err != nil {
			return err
		}
		for _, i := range v {
			fmt.Fprintf(c.OutOrStdout(), "%s\t%s\n", i.Digest, strings.Join(i.Tags, ","))
		}
		return nil
	}}
	inspect := &cobra.Command{Use: "inspect IMAGE", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		api, err := client.New()
		if err != nil {
			return err
		}
		v, err := api.GetImage(c.Context(), args[0])
		if err != nil {
			return err
		}
		fmt.Fprintf(c.OutOrStdout(), "%s\t%d\t%s\n", v.Digest, v.Size, strings.Join(v.Tags, ","))
		return nil
	}}
	remove := &cobra.Command{Use: "rm IMAGE", Aliases: []string{"remove", "delete"}, Short: "Remove an image tag; existing VMs are kept", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		api, err := client.New()
		if err != nil {
			return err
		}
		return api.RemoveImage(c.Context(), args[0])
	}}
	gc := &cobra.Command{Use: "gc", Short: "Reclaim untagged images not referenced by VMs", Args: cobra.NoArgs, RunE: func(c *cobra.Command, _ []string) error {
		api, err := client.New()
		if err != nil {
			return err
		}
		v, err := api.GCImages(c.Context())
		if err != nil {
			return err
		}
		for _, d := range v {
			fmt.Fprintln(c.OutOrStdout(), d)
		}
		return nil
	}}
	image.AddCommand(importCmd, list, inspect, remove, gc)
	return image
}
