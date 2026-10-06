package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCheckoutCmd(options *rootOptions) *cobra.Command {
	var repo, branch, path string
	command := &cobra.Command{Use: "checkout <outpost>", Short: "Transfer an exact local Git commit into a running VM", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if repo == "" {
			return fmt.Errorf("--repo is required")
		}
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || strings.ContainsAny(path, "\x00\r\n") {
			return fmt.Errorf("--path must be a clean absolute guest directory other than /")
		}
		directory, err := os.MkdirTemp("", "outpost-branch-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(directory)
		bundle, sha, err := branchBundle(cmd, options.runner, repo, branch, directory)
		if err != nil {
			return err
		}
		vm, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		if err := checkoutBundle(cmd, options, vm, bundle, sha, path); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s:%s\n", sha, args[0], path)
		return nil
	}}
	command.Flags().StringVar(&repo, "repo", "", "local Git repository (committed code only)")
	command.Flags().StringVar(&branch, "branch", "HEAD", "local branch, ref, or commit; no automatic fetch")
	command.Flags().StringVar(&path, "path", "/workspace", "guest project directory")
	return command
}
