package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/nishantdania/outpost/internal/config"
	"github.com/nishantdania/outpost/internal/daemon"
	"github.com/nishantdania/outpost/internal/doctor"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/setup"
	"github.com/nishantdania/outpost/internal/update"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--setup" {
		if err := setup.Run(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}
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

	uninstall := func(context.Context) error {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		go func() {
			time.Sleep(time.Second)
			if err := exec.Command("systemctl", "--user", "disable", "outpostd.service").Run(); err != nil {
				log.Printf("disable outpostd: %v", err)
			}
			if err := os.Remove(executable); err != nil {
				log.Printf("remove outpostd: %v", err)
			}
			if configDirectory, err := os.UserConfigDir(); err != nil {
				log.Printf("find configuration directory: %v", err)
			} else if err := os.RemoveAll(filepath.Join(configDirectory, "outpost")); err != nil {
				log.Printf("remove configuration: %v", err)
			}
			if home, err := os.UserHomeDir(); err != nil {
				log.Printf("find home directory: %v", err)
			} else if err := os.Remove(filepath.Join(home, ".config", "systemd", "user", "outpostd.service")); err != nil && !os.IsNotExist(err) {
				log.Printf("remove service unit: %v", err)
			}
			if err := exec.Command("systemctl", "--user", "stop", "outpostd.service").Run(); err != nil {
				log.Printf("stop outpostd: %v", err)
			}
		}()
		return nil
	}

	path, err := config.OutpostsPath()
	if err != nil {
		log.Fatal(err)
	}
	service := outpost.New(path)
	server := &http.Server{Addr: cfg.ListenAddr, Handler: daemon.New(service.Create, service.List, service.Delete, version, applyUpdate, uninstall, doctor.Run)}
	log.Printf("outpostd listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}
