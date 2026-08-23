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
)

const shutdownTimeout = 10 * time.Second

func Run(config Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return run(ctx, config)
}

func run(ctx context.Context, config Config) error {
	store, err := ark.Open(ctx, config.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()

	return runServer(ctx, httpapi.NewServer(config.ListenAddr, store))
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
