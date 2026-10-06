package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/remote"
)

func newForkCmd(options *rootOptions) *cobra.Command {
	input := client.CreateOutpostInput{VCPUs: outpost.DefaultVCPUs}
	var repo, branch, path, memory, disk string
	command := &cobra.Command{Use: "fork <snapshot>", Short: "Create an independent VM from an image, optionally checking out a project branch", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if input.Name == "" {
			return fmt.Errorf("--name is required")
		}
		if repo == "" && cmd.Flags().Changed("branch") {
			return fmt.Errorf("--branch requires --repo")
		}
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || strings.ContainsAny(path, "\x00\r\n") {
			return fmt.Errorf("--path must be a clean absolute guest directory other than /")
		}
		var err error
		input.MemoryMiB, err = parseMemoryMiB(memory)
		if err != nil {
			return err
		}
		// Resolve and package the committed code BEFORE creating a VM. The local
		// checkout's working tree, remotes, credentials and hooks are not copied.
		var bundle, sha string
		if repo != "" {
			directory, err := os.MkdirTemp("", "outpost-branch-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(directory)
			bundle, sha, err = branchBundle(cmd, options.runner, repo, branch, directory)
			if err != nil {
				return err
			}
		}
		input.SSHPublicKey, err = options.ssh.EnsureIdentity(cmd.Context(), options.runner)
		if err != nil {
			return err
		}
		c, err := imageClient(options)
		if err != nil {
			return err
		}
		image, err := c.GetImage(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		input.ImageID = image.Digest // Pin the tag before creating the fork.
		if disk == "" {
			input.DiskGiB = int((image.Size + (1 << 30) - 1) >> 30)
			if input.DiskGiB < outpost.DefaultDiskGiB {
				input.DiskGiB = outpost.DefaultDiskGiB
			}
		} else {
			input.DiskGiB, err = parseDiskGiB(disk)
			if err != nil {
				return err
			}
			if int64(input.DiskGiB)<<30 < image.Size {
				return fmt.Errorf("--disk is smaller than the snapshot")
			}
		}
		value, err := c.CreateOutpostWith(cmd.Context(), input)
		if err != nil {
			return err
		}
		if bundle != "" {
			if err := checkoutBundle(cmd, options, value, bundle, sha, path); err != nil {
				return fmt.Errorf("VM %q was created but branch checkout failed (kept for inspection; remove with outpost delete %s): %w", input.Name, input.Name, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Checked out %s in %s:%s\n", sha, input.Name, path)
		}
		writer, err := newOutputWriter(options, cmd.OutOrStdout())
		if err != nil {
			return err
		}
		return writer.Write(value, outpostTable([]api.Outpost{value}))
	}}
	command.Flags().StringVar(&input.Name, "name", "", "new VM name")
	command.Flags().StringVar(&repo, "repo", "", "local Git repository to transfer (committed code only)")
	command.Flags().StringVar(&branch, "branch", "HEAD", "local branch, ref, or commit to check out; no automatic fetch")
	command.Flags().StringVar(&path, "path", "/workspace", "guest project directory; existing clean Git checkouts are updated")
	command.Flags().IntVar(&input.VCPUs, "cpus", input.VCPUs, "virtual CPUs (not inherited from baseline)")
	command.Flags().StringVar(&memory, "memory", "4G", "memory (not inherited from baseline)")
	command.Flags().StringVar(&disk, "disk", "", "disk size; defaults to snapshot size, at least 8 GiB")
	return command
}

func branchBundle(cmd *cobra.Command, runner remote.Runner, repo, branch, directory string) (string, string, error) {
	repo, err := filepath.Abs(repo)
	if err != nil {
		return "", "", err
	}
	var output bytes.Buffer
	if err := runner.Run(cmd.Context(), "git", []string{"-C", repo, "rev-parse", "--verify", "--end-of-options", branch + "^{commit}"}, remote.IO{Stdout: &output, Stderr: cmd.ErrOrStderr()}); err != nil {
		return "", "", fmt.Errorf("resolve branch: %w", err)
	}
	sha := strings.TrimSpace(output.String())
	if !regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`).MatchString(sha) {
		return "", "", fmt.Errorf("Git returned an invalid commit ID")
	}
	bare, bundle := filepath.Join(directory, "source.git"), filepath.Join(directory, "source.bundle")
	objectFormat := "sha1"
	if len(sha) == 64 {
		objectFormat = "sha256"
	}
	for _, args := range [][]string{
		{"init", "--bare", "--object-format=" + objectFormat, bare},
		{"-C", bare, "fetch", "--no-tags", "--no-recurse-submodules", repo, sha},
		{"-C", bare, "update-ref", "refs/heads/workspace", sha},
		{"-C", bare, "bundle", "create", bundle, "refs/heads/workspace"},
	} {
		if err := runner.Run(cmd.Context(), "git", args, remote.IO{Stdout: cmd.ErrOrStderr(), Stderr: cmd.ErrOrStderr()}); err != nil {
			return "", "", fmt.Errorf("package branch: %w", err)
		}
	}
	return bundle, sha, nil
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func checkoutScript(path, destination, sha string) string {
	p, b := shellQuote(path), shellQuote(destination)
	return "set -eu; trap " + shellQuote("rm -f -- "+b) + " EXIT; " +
		"command -v git >/dev/null || { echo 'Install Git in the baseline VM before checking out code' >&2; exit 1; }; " +
		"if git -C " + p + " rev-parse --is-inside-work-tree >/dev/null 2>&1; then " +
		"test -z \"$(git -C " + p + " status --porcelain --untracked-files=no)\" || { echo 'Baseline checkout has tracked changes; refusing to overwrite' >&2; exit 1; }; " +
		"git -C " + p + " fetch --no-tags --no-recurse-submodules " + b + " refs/heads/workspace; " +
		"git -C " + p + " checkout --detach --no-overwrite-ignore " + sha + "; " +
		"else git clone " + b + " " + p + "; git -C " + p + " checkout --detach --no-overwrite-ignore " + sha + "; git -C " + p + " remote remove origin; fi"
}

func checkoutBundle(cmd *cobra.Command, options *rootOptions, vm api.Outpost, bundle, sha, path string) error {
	ssh := options.ssh
	ssh.AgentForwarding = false
	// Each checkout has its own guest filename, even when the same VM is used.
	destination := "/tmp/outpost-checkout-" + uuid.NewString() + ".bundle"
	args, err := ssh.SCPArgs(vm.GuestIp, bundle, destination, true, false)
	if err != nil {
		return err
	}
	if err := options.runner.Run(cmd.Context(), "scp", args, remote.IO{Stdout: cmd.ErrOrStderr(), Stderr: cmd.ErrOrStderr()}); err != nil {
		return err
	}
	args, err = ssh.SSHArgs(vm.GuestIp, []string{"sh", "-c", checkoutScript(path, destination, sha)}, false)
	if err != nil {
		return err
	}
	return options.runner.Run(cmd.Context(), "ssh", args, remote.IO{Stdout: cmd.ErrOrStderr(), Stderr: cmd.ErrOrStderr()})
}
