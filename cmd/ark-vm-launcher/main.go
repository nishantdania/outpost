package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/nishantdania/ark/internal/launcher"
)

func main() {
	config := launcher.DefaultConfig()
	flag.StringVar(&config.SocketPath, "socket", config.SocketPath, "launcher Unix socket")
	flag.StringVar(&config.StateDir, "state-dir", config.StateDir, "launcher state directory")
	flag.StringVar(&config.RuntimeDir, "runtime-dir", config.RuntimeDir, "launcher runtime directory")
	flag.IntVar(&config.AllowedUID, "allowed-uid", -1, "authorized arkd Unix peer UID")
	flag.Parse()
	if config.AllowedUID < 0 {
		account, err := user.Lookup("arkd")
		if err != nil {
			log.Fatal(err)
		}
		config.AllowedUID, err = strconv.Atoi(account.Uid)
		if err != nil {
			log.Fatal(err)
		}
	}

	server, err := launcher.NewServer(config, launcher.NewMemoryRuntime())
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
		if err := <-errCh; err != nil {
			log.Fatal(err)
		}
	}
}
