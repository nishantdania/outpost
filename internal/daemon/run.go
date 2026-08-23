package daemon

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/httpapi"
	"github.com/nishantdania/ark/internal/service"
	"github.com/nishantdania/ark/internal/vmapi"
)

var ErrTokenRequired = errors.New("arkd bearer token is required")

const shutdownTimeout = 10 * time.Second

func Run(config Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, config)
}

func run(ctx context.Context, config Config) error {
	if config.Token == "" {
		return ErrTokenRequired
	}

	store, err := ark.Open(ctx, config.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()

	return runServer(ctx, httpapi.NewServer(config.ListenAddr, service.New(store, vmapi.UnavailableManager{}), config.Token))
}

type server interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

func runServer(ctx context.Context, server server) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return serveError(err)
	case <-ctx.Done():
		if err := shutdown(server); err != nil {
			return err
		}

		return serveError(<-errCh)
	}
}

func shutdown(server server) error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return server.Shutdown(ctx)
}

func serveError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
