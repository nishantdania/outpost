package daemon

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/httpapi"
	"github.com/nishantdania/outpost/internal/image"
	"github.com/nishantdania/outpost/internal/launcherclient"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
)

var ErrTokenRequired = errors.New("outpostd bearer token is required")

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

	store, err := outpost.Open(ctx, config.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	manager := launcherclient.New(config.LauncherSocket)
	defer manager.Close()
	application := service.New(store, manager)
	if _, lookupErr := exec.LookPath("podman"); lookupErr == nil {
		images, imageErr := image.New(config.ImageStore, store, nil)
		if imageErr != nil {
			return imageErr
		}
		available := true
		if config.DefaultOCI != "" {
			if _, statErr := os.Stat(config.DefaultOCI); statErr == nil {
				if importErr := images.ImportDefault(ctx, config.DefaultOCI); importErr != nil {
					available = false
					log.Printf("outpostd image capability disabled: %s", boundedError(importErr))
				}
			} else {
				available = false
				log.Printf("outpostd image capability disabled: default OCI archive unavailable")
			}
		}
		if available {
			application.WithImages(images)
		}
	}
	if config.CredentialStore == "" {
		// Managed create/start is rejected by Service. Existing managed launcher
		// rules still route to a closed proxy, never to unrestricted NAT. Keep the
		// API available for inspection/stop/delete even when mediation is disabled.
		return runServer(ctx, httpapi.NewServer(config.ListenAddr, application, config.Token))
	}
	host, err := credentials.NewHost(config.CredentialStore, config.EgressState)
	if err != nil {
		return err
	}
	application.WithCredentials(host)
	proxy := credentials.NewProxy(host, service.CredentialAuthorizer(store)).Server()
	listener, err := net.Listen("tcp4", "0.0.0.0:"+strconv.Itoa(credentials.Port))
	if err != nil {
		return err
	}
	defer listener.Close()
	proxyErrors := make(chan error, 1)
	go func() { proxyErrors <- proxy.ServeTLS(listener, "", "") }()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	apiErrors := make(chan error, 1)
	go func() { apiErrors <- runServer(child, httpapi.NewServer(config.ListenAddr, application, config.Token)) }()
	select {
	case err := <-proxyErrors:
		cancel()
		<-apiErrors
		return err
	case err := <-apiErrors:
		_ = proxy.Close()
		<-proxyErrors
		return err
	}
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

func boundedError(err error) string {
	value := err.Error()
	if len(value) > 256 {
		return value[len(value)-256:]
	}
	return value
}

func serveError(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}
