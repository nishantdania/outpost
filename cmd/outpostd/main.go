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
	cfg, err := config.LoadDaemon()
	if err != nil {
		log.Fatal(err)
	}
	if len(os.Args) == 2 && os.Args[1] == "--setup" {
		if err := setup.Run(context.Background(), cfg.FirecrackerVersion); err != nil {
			log.Fatal(err)
		}
		return
	}

	path, err := config.OutpostsPath()
	if err != nil {
		log.Fatal(err)
	}
	service := outpost.NewWithRuntime(path, filepath.Join(filepath.Dir(path), "assets"))

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
		dataPath, err := config.OutpostsPath()
		if err != nil {
			return err
		}
		records, _ := service.List(context.Background())
		for _, record := range records {
			_, _ = service.Delete(context.Background(), record.ID)
		}
		go func() {
			time.Sleep(time.Second)
			if err := exec.Command("sudo", "-n", "/usr/local/lib/outpost/uninstall-network").Run(); err != nil {
				log.Printf("remove VM networking: %v", err)
			}
			if err := os.RemoveAll(filepath.Dir(dataPath)); err != nil {
				log.Printf("remove data: %v", err)
			}
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

	server := &http.Server{Addr: cfg.ListenAddr, Handler: daemon.New(service.Create, service.List, service.Delete, service.Start, service.Stop, version, applyUpdate, uninstall, doctor.Run)}
	log.Printf("outpostd listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}
