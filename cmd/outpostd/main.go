package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/nishantdania/outpost/internal/config"
	"github.com/nishantdania/outpost/internal/daemon"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/update"
)

var version = "dev"

func main() {
	cfg, err := config.LoadDaemon()
	if err != nil {
		log.Fatal(err)
	}

	applyUpdate := func(ctx context.Context) (update.Result, error) {
		executable, err := os.Executable()
		if err != nil {
			return update.Result{}, err
		}
		result, err := update.Apply(ctx, update.Options{Component: "outpostd", CurrentVersion: version, Executable: executable})
		if err != nil || !result.Updated {
			return result, err
		}
		go func() {
			time.Sleep(time.Second)
			if err := exec.Command("systemctl", "--user", "restart", "outpostd.service").Run(); err != nil {
				log.Printf("restart outpostd: %v", err)
			}
		}()
		return result, nil
	}

	server := &http.Server{Addr: cfg.ListenAddr, Handler: daemon.New(outpost.Create, version, applyUpdate)}
	log.Printf("outpostd listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}
