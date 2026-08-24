package cmd

import (
	"fmt"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/spf13/cobra"
	"io"
	"os"
	"strings"
)

func imageClient(options *rootOptions) (*client.Client, error) {
	return client.New(options.serverURL, options.token)
}
func newImageCmd(options *rootOptions) *cobra.Command {
	image := &cobra.Command{Use: "image"}
	var tag string
	build := &cobra.Command{Use: "build DIR", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		if tag == "" {
			return fmt.Errorf("--tag is required")
		}
		api, err := imageClient(options)
		if err != nil {
			return err
		}
		v, err := api.BuildImage(c.Context(), tag, args[0])
		if err != nil {
			return err
		}
		fmt.Fprintln(c.OutOrStdout(), v.Digest)
		return nil
	}}
	build.Flags().StringVarP(&tag, "tag", "t", "", "image tag")
	importCmd := &cobra.Command{Use: "import PATH|-", Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		if tag == "" {
			return fmt.Errorf("--tag is required")
		}
		var input io.Reader
		var close func() error
		if args[0] == "-" {
			input = c.InOrStdin()
			close = func() error { return nil }
		} else {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			input = f
			close = f.Close
		}
		defer close()
		api, err := imageClient(options)
		if err != nil {
			return err
		}
		v, err := api.ImportImage(c.Context(), tag, input)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.OutOrStdout(), v.Digest)
		return nil
	}}
	importCmd.Flags().StringVarP(&tag, "tag", "t", "", "image tag")
	list := &cobra.Command{Use: "list", RunE: func(c *cobra.Command, _ []string) error {
		api, err := imageClient(options)
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
		api, err := imageClient(options)
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
	remove := &cobra.Command{Use: "remove IMAGE", Aliases: []string{"delete"}, Args: cobra.ExactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		api, err := imageClient(options)
		if err != nil {
			return err
		}
		return api.RemoveImage(c.Context(), args[0])
	}}
	gc := &cobra.Command{Use: "gc", RunE: func(c *cobra.Command, _ []string) error {
		api, err := imageClient(options)
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
	image.AddCommand(build, importCmd, list, inspect, remove, gc)
	return image
}
