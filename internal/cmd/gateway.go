package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/gateway"
	"github.com/spf13/cobra"
)

func newGatewayCmd(options *rootOptions) *cobra.Command {
	listen := "127.0.0.1:17891"
	cmd := &cobra.Command{
		Use: "gateway", Short: "Serve registered HTTP hosts through one local gateway", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			host, port, err := net.SplitHostPort(listen)
			ip := net.ParseIP(host)
			value, portErr := strconv.Atoi(port)
			if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || value < 0 || value > 65535 {
				return fmt.Errorf("gateway must listen on a loopback IP and valid port; point a local tunnel connector at it")
			}
			application, err := client.New(options.serverURL, options.token)
			if err != nil {
				return err
			}
			handler := gateway.New(application)
			defer handler.Close()
			listener, err := net.Listen("tcp", listen)
			if err != nil {
				return fmt.Errorf("listen for gateway: %w", err)
			}
			defer listener.Close()
			server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second,
				IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			done := make(chan struct{})
			go func() {
				defer close(done)
				<-ctx.Done()
				shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdown); err != nil {
					_ = server.Close()
				}
			}()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Gateway listening on http://%s; preserve the public Host header when forwarding traffic.\n", listener.Addr())
			err = server.Serve(listener)
			stop()
			<-done
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("serve gateway: %w", err)
		},
	}
	cmd.Flags().StringVar(&listen, "listen", listen, "Loopback HTTP listen address for the tunnel connector")
	return cmd
}
