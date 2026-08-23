package cmd

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/client"
	"github.com/nishantdania/ark/internal/remote"
)

func resolveRemote(cmd *cobra.Command, options *rootOptions, name string) (api.Ark, error) {
	if options.initErr != nil {
		return api.Ark{}, options.initErr
	}
	if err := options.ssh.Prepare(); err != nil {
		return api.Ark{}, err
	}
	arkd, err := client.New(options.serverURL, options.token)
	if err != nil {
		return api.Ark{}, fmt.Errorf("create arkd client: %w", err)
	}
	value, err := arkd.GetArk(cmd.Context(), name)
	if err != nil {
		return api.Ark{}, err
	}
	if string(value.Status) != "running" {
		return api.Ark{}, fmt.Errorf("ark %q is not running", name)
	}
	if value.GuestIp == "" || net.ParseIP(value.GuestIp) == nil {
		return api.Ark{}, fmt.Errorf("ark %q has no valid guest IP", name)
	}
	return value, nil
}

func newSSHCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "ssh <ark>", Short: "Open an SSH session to an Ark", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ark, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		argv, err := options.ssh.SSHArgs(ark.GuestIp, nil, true)
		if err != nil {
			return err
		}
		return options.runner.Run(cmd.Context(), "ssh", argv, streams(cmd))
	}}
}

func newExecCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "exec <ark> -- <program> [args...]", Short: "Run a program in an Ark", Args: func(cmd *cobra.Command, args []string) error {
		if cmd.ArgsLenAtDash() != 1 || len(args) < 2 {
			return fmt.Errorf("requires an Ark and program after --")
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		ark, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		argv, err := options.ssh.SSHArgs(ark.GuestIp, args[1:], false)
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
		return "", "", false, fmt.Errorf("invalid Ark endpoint %q", value)
	}
	name, path := value[:index], value[index+1:]
	if path == "" || strings.ContainsAny(name, "/\\\x00\r\n") {
		return "", "", false, fmt.Errorf("invalid Ark endpoint %q", value)
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
		return fmt.Errorf("copy requires exactly one Ark endpoint")
	}
	name, local, remotePath, upload := rightName, leftPath, rightPath, true
	if leftRemote {
		name, local, remotePath, upload = leftName, rightPath, leftPath, false
	}
	ark, err := resolveRemote(cmd, options, name)
	if err != nil {
		return err
	}
	var argv []string
	if sync {
		argv, err = options.ssh.RsyncArgs(ark.GuestIp, local, remotePath, upload)
	} else {
		argv, err = options.ssh.SCPArgs(ark.GuestIp, local, remotePath, upload, true)
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
	return &cobra.Command{Use: "copy <source> <destination>", Short: "Copy files to or from an Ark", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error { return transfer(cmd, options, args, false) }}
}
func newSyncCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "sync <source> <destination>", Short: "Synchronize files to or from an Ark", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error { return transfer(cmd, options, args, true) }}
}
