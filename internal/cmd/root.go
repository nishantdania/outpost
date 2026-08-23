package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/nishantdania/ark/internal/remote"

	"github.com/spf13/cobra"

	"github.com/nishantdania/ark/internal/output"
)

type rootOptions struct {
	serverURL string
	token     string
	output    string
	noColor   bool
	ssh       remote.Config
	runner    remote.Runner
	initErr   error
}

func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	home, homeErr := os.UserHomeDir()
	ssh := remote.DefaultConfig(home)
	ssh.User = envString("ARK_SSH_USER", ssh.User)
	ssh.ProxyJump = os.Getenv("ARK_SSH_PROXY_JUMP")
	ssh.IdentityFile = envString("ARK_SSH_IDENTITY", ssh.IdentityFile)
	ssh.KnownHostsFile = envString("ARK_SSH_KNOWN_HOSTS", ssh.KnownHostsFile)
	agentValue := os.Getenv("ARK_SSH_AGENT_FORWARDING")
	var agentErr error
	if agentValue != "" {
		ssh.AgentForwarding, agentErr = strconv.ParseBool(agentValue)
		if agentErr != nil {
			agentErr = fmt.Errorf("parse ARK_SSH_AGENT_FORWARDING: %w", agentErr)
		}
	}
	options := &rootOptions{ssh: ssh, runner: remote.SystemRunner(), initErr: errors.Join(homeErr, agentErr)}
	root := &cobra.Command{
		Use:               "ark",
		Short:             "Create and manage Arks",
		SilenceErrors:     true,
		SilenceUsage:      true,
		PersistentPreRunE: func(*cobra.Command, []string) error { return options.initErr },
	}

	root.PersistentFlags().StringVar(&options.serverURL, "server", envString("ARK_SERVER", "http://127.0.0.1:17890"), "arkd server URL")
	root.PersistentFlags().StringVar(&options.token, "token", os.Getenv("ARK_TOKEN"), "arkd bearer token")
	root.PersistentFlags().StringVarP(&options.output, "output", "o", "table", "Output format: table or json")
	root.PersistentFlags().BoolVar(&options.noColor, "no-color", false, "Disable color output")
	root.PersistentFlags().StringVar(&options.ssh.User, "ssh-user", options.ssh.User, "guest SSH user")
	root.PersistentFlags().StringVar(&options.ssh.ProxyJump, "ssh-proxy-jump", options.ssh.ProxyJump, "SSH ProxyJump host")
	root.PersistentFlags().StringVar(&options.ssh.IdentityFile, "ssh-identity", options.ssh.IdentityFile, "guest SSH identity file")
	root.PersistentFlags().StringVar(&options.ssh.KnownHostsFile, "ssh-known-hosts", options.ssh.KnownHostsFile, "guest SSH known_hosts file")
	root.PersistentFlags().BoolVar(&options.ssh.AgentForwarding, "ssh-agent-forwarding", options.ssh.AgentForwarding, "forward SSH agent to guest")
	root.AddCommand(newCreateCmd(options))
	root.AddCommand(newDeleteCmd(options))
	root.AddCommand(newStartCmd(options))
	root.AddCommand(newStopCmd(options))
	root.AddCommand(newInspectCmd(options))
	root.AddCommand(newListCmd(options))
	root.AddCommand(newSSHCmd(options))
	root.AddCommand(newExecCmd(options))
	root.AddCommand(newCopyCmd(options))
	root.AddCommand(newSyncCmd(options))
	root.AddCommand(newDoctorCmd(options))

	return root
}

func envString(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func newOutputWriter(options *rootOptions, out io.Writer) (*output.Writer, error) {
	return output.New(output.Options{
		Format:  options.output,
		Out:     out,
		NoColor: options.noColor,
	})
}
