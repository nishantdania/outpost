package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newForwardCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{Use: "forward <outpost> <local-port>:<guest-port>", Short: "Forward a localhost port to the guest app (foreground; Ctrl-C to close)", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		ports := strings.Split(args[1], ":")
		if len(ports) != 2 {
			return fmt.Errorf("expected local-port:guest-port")
		}
		for _, value := range ports {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > 65535 {
				return fmt.Errorf("ports must be between 1 and 65535")
			}
		}
		vm, err := resolveRemote(cmd, options, args[0])
		if err != nil {
			return err
		}
		ssh := options.ssh
		ssh.AgentForwarding = false
		argv, err := ssh.SSHArgs(vm.GuestIp, nil, false)
		if err != nil {
			return err
		}
		// Insert options before the destination. Remote apps can bind guest loopback.
		destination := argv[len(argv)-1]
		argv = append(argv[:len(argv)-1], "-N", "-o", "ExitOnForwardFailure=yes", "-L", "127.0.0.1:"+ports[0]+":127.0.0.1:"+ports[1], destination)
		fmt.Fprintf(cmd.ErrOrStderr(), "Forwarding http://127.0.0.1:%s to %s:%s (Ctrl-C to close)\n", ports[0], args[0], ports[1])
		return options.runner.Run(cmd.Context(), "ssh", argv, streams(cmd))
	}}
}
