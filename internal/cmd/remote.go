package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/remote"
)

func resolveRemote(cmd *cobra.Command, options *rootOptions, name string) (api.Outpost, error) {
	if options.initErr != nil {
		return api.Outpost{}, options.initErr
	}
	if err := options.ssh.Prepare(); err != nil {
		return api.Outpost{}, err
	}
	outpostd, err := client.New(options.serverURL, options.token)
	if err != nil {
		return api.Outpost{}, fmt.Errorf("create outpostd client: %w", err)
	}
	value, err := outpostd.GetOutpost(cmd.Context(), name)
	if err != nil {
		return api.Outpost{}, err
	}
	if string(value.Status) != "running" {
		return api.Outpost{}, fmt.Errorf("outpost %q is not running", name)
	}
	if value.GuestIp == "" || net.ParseIP(value.GuestIp) == nil {
		return api.Outpost{}, fmt.Errorf("outpost %q has no valid guest IP", name)
	}
	return value, nil
}

func newSSHCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "ssh <outpost>", Short: "Open an SSH session to an Outpost", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		outpost, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		argv, err := options.ssh.SSHArgs(outpost.GuestIp, nil, true)
		if err != nil {
			return err
		}
		return options.runner.Run(cmd.Context(), "ssh", argv, streams(cmd))
	}}
}

func newExecCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "exec <outpost> -- <program> [args...]", Short: "Run a program in an Outpost", Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
			return fmt.Errorf("requires an Outpost and program after --")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		outpost, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		argv, err := options.ssh.SSHArgs(outpost.GuestIp, args[1:], false)
		if err != nil {
			return err
		}
		return options.runner.Run(cmd.Context(), "ssh", argv, streams(cmd))
	}}
}

func streams(cmd *cobra.Command) remote.IO {
	return remote.IO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}
}

func endpoint(value string) (string, string, bool, error) {
	index := strings.IndexByte(value, ':')
	if index < 0 {
		return "", value, false, nil
	}
	if index == 0 {
		return "", "", false, fmt.Errorf("invalid Outpost endpoint %q", value)
	}
	name, path := value[:index], value[index+1:]
	if path == "" || strings.ContainsAny(name, "/\\\x00\r\n") {
		return "", "", false, fmt.Errorf("invalid Outpost endpoint %q", value)
	}
	return name, path, true, nil
}

func transfer(cmd *cobra.Command, options *rootOptions, args []string, sync bool) error {
	if args[0] == "" || args[1] == "" {
		return fmt.Errorf("copy endpoints must not be empty")
	}
	leftName, leftPath, leftRemote, err := endpoint(args[0])
	if err != nil {
		return err
	}
	rightName, rightPath, rightRemote, err := endpoint(args[1])
	if err != nil {
		return err
	}
	if leftRemote == rightRemote {
		return fmt.Errorf("copy requires exactly one Outpost endpoint")
	}
	name, local, remotePath, upload := rightName, leftPath, rightPath, true
	if leftRemote {
		name, local, remotePath, upload = leftName, rightPath, leftPath, false
	}
	outpost, err := resolveRemote(cmd, options, name)
	if err != nil {
		return err
	}
	var argv []string
	if sync {
		argv, err = options.ssh.RsyncArgs(outpost.GuestIp, local, remotePath, upload)
	} else {
		argv, err = options.ssh.SCPArgs(outpost.GuestIp, local, remotePath, upload, true)
	}
	if err != nil {
		return err
	}
	name = "scp"
	if sync {
		name = "rsync"
	}
	return options.runner.Run(cmd.Context(), name, argv, streams(cmd))
}

func newCopyCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "copy <source> <destination>", Short: "Copy files to or from an Outpost", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error { return transfer(cmd, options, args, false) }}
}
func newSyncCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "sync <source> <destination>", Short: "Synchronize files to or from an Outpost", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error { return transfer(cmd, options, args, true) }}
}
