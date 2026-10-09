package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/output"
	"github.com/spf13/cobra"
)

func newHostCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "host <hostname> <vm:port>",
		Short: "Route an HTTP hostname to a VM port",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname, err := outpost.CanonicalHostname(args[0])
			if err != nil {
				return err
			}
			name, text, err := net.SplitHostPort(args[1])
			if err != nil || name == "" || net.ParseIP(name) != nil {
				return fmt.Errorf("target must be VM_NAME:PORT, not an IP address or URL")
			}
			port, err := strconv.Atoi(text)
			if err != nil || port < 1 || port > 65535 {
				return outpost.ErrInvalidPort
			}
			application, err := client.New(options.serverURL, options.token)
			if err != nil {
				return err
			}
			host, err := application.SetHost(cmd.Context(), hostname, name, port)
			if err != nil {
				return err
			}
			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return writer.Write(host, hostsTable([]api.Host{host}))
		},
	}
}

func newHostsCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "hosts", Short: "List hostname mappings", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			application, err := client.New(options.serverURL, options.token)
			if err != nil {
				return err
			}
			hosts, err := application.Hosts(cmd.Context())
			if err != nil {
				return err
			}
			writer, err := newOutputWriter(options, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			return writer.Write(hosts, hostsTable(hosts))
		},
	}
}

func newUnhostCmd(options *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use: "unhost <hostname>", Short: "Remove a hostname mapping without deleting its VM", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hostname, err := outpost.CanonicalHostname(args[0])
			if err != nil {
				return err
			}
			application, err := client.New(options.serverURL, options.token)
			if err != nil {
				return err
			}
			if err := application.Unhost(cmd.Context(), hostname); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed host %s\n", hostname)
			return err
		},
	}
}

func hostsTable(hosts []api.Host) output.Table {
	table := output.Table{Headers: []string{"HOSTNAME", "TARGET", "STATUS", "ADDRESS"}}
	for _, host := range hosts {
		address := "-"
		if host.GuestIp != "" {
			address = net.JoinHostPort(host.GuestIp, strconv.Itoa(host.Port))
		}
		table.Rows = append(table.Rows, []string{host.Hostname, net.JoinHostPort(host.OutpostName, strconv.Itoa(host.Port)), host.Status, address})
	}
	return table
}
