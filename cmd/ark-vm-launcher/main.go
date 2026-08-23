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

	assetmanifest "github.com/nishantdania/ark/internal/assets"
	"github.com/nishantdania/ark/internal/doctor"
	"github.com/nishantdania/ark/internal/launcher"
)

func main() {
	config := launcher.DefaultConfig()
	assets := launcher.FirecrackerConfig{StateDir: config.StateDir, RuntimeDir: config.RuntimeDir, JailerBase: "/srv/ark/jailer", Firecracker: "/usr/local/lib/ark/launcher/firecracker", Jailer: "/usr/local/lib/ark/launcher/jailer", Kernel: "/usr/local/lib/ark/launcher/vmlinux", DefaultRootFS: "/var/lib/arkd/images/default/rootfs.ext4", ImageStore: "/var/lib/arkd/images", Uplink: "eth0", DNS: "1.1.1.1"}
	flag.StringVar(&config.SocketPath, "socket", config.SocketPath, "launcher Unix socket")
	flag.StringVar(&config.StateDir, "state-dir", config.StateDir, "launcher state directory")
	flag.StringVar(&config.RuntimeDir, "runtime-dir", config.RuntimeDir, "launcher runtime directory")
	flag.IntVar(&config.AllowedUID, "allowed-uid", -1, "authorized arkd Unix peer UID")
	flag.StringVar(&assets.JailerBase, "jailer-base", assets.JailerBase, "trusted jailer directory")
	flag.StringVar(&assets.Firecracker, "firecracker", assets.Firecracker, "trusted Firecracker executable")
	flag.StringVar(&assets.Jailer, "jailer", assets.Jailer, "trusted jailer executable")
	flag.StringVar(&assets.Kernel, "kernel", assets.Kernel, "trusted kernel")
	flag.StringVar(&assets.DefaultRootFS, "default-rootfs", assets.DefaultRootFS, "trusted default rootfs")
	flag.StringVar(&assets.ImageStore, "image-store", assets.ImageStore, "trusted custom image store")
	flag.StringVar(&assets.Uplink, "uplink", assets.Uplink, "host uplink interface")
	flag.StringVar(&assets.DNS, "dns", assets.DNS, "guest DNS server")
	doctorMode := flag.Bool("doctor", false, "check server requirements")
	doctorOnline := flag.Bool("doctor-online", false, "check running services and launcher socket")
	doctorJSON := flag.Bool("doctor-json", false, "write doctor JSON")
	flag.Parse()
	if *doctorMode {
		manifest, err := assetmanifest.Load("/usr/local/lib/ark/assets.json")
		if err != nil {
			log.Print(err)
			os.Exit(1)
		}
		files := map[string]string{}
		checksums := map[string]string{}
		for _, item := range manifest.Assets {
			path := "/usr/local/lib/ark/" + item.File
			if item.File == "ark" {
				path = "/usr/local/bin/ark"
			}
			if item.File == "ark-vm-launcher" || item.File == "firecracker" || item.File == "jailer" || item.File == "vmlinux" {
				path = "/usr/local/lib/ark/launcher/" + item.File
			}
			if item.File == "rootfs.ext4" {
				path = "/var/lib/arkd/images/default/rootfs.ext4"
			}
			files[item.Name] = path
			checksums[item.Name] = item.SHA256
		}
		report := doctor.Server(doctor.OSProbe{}, doctor.ServerConfig{StateDir: config.StateDir, JailerDir: assets.JailerBase, RuntimeDir: config.RuntimeDir, Assets: files, Manifest: checksums, Uplink: assets.Uplink, Socket: config.SocketPath, Online: *doctorOnline, Users: []string{"arkd", "arkvm"}, Groups: []string{"arkd", "arkvm"}, SocketUser: "root", SocketGroup: "arkd", Directories: []doctor.Directory{{Path: "/var/lib/arkd", Mode: 0750, User: "arkd", Group: "arkd"}, {Path: config.StateDir, Mode: 0750, User: "root", Group: "arkd"}, {Path: assets.JailerBase, Mode: 0750, User: "root", Group: "arkvm"}, {Path: config.RuntimeDir, Mode: 0750, User: "root", Group: "arkd"}, {Path: "/usr/local/lib/ark", Mode: 0750, User: "root", Group: "arkd"}, {Path: "/usr/local/lib/ark/launcher", Mode: 0750, User: "root", Group: "arkvm"}}, Units: []string{"/etc/systemd/system/arkd.service", "/etc/systemd/system/ark-vm-launcher.service"}})
		if *doctorJSON {
			err = report.JSON(os.Stdout)
		} else {
			report.Text(os.Stdout)
		}
		if err != nil || report.Failed() {
			os.Exit(1)
		}
		return
	}
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
	account, err := user.Lookup("arkvm")
	if err != nil {
		log.Fatal(err)
	}
	assets.ArkVMUID, err = strconv.Atoi(account.Uid)
	if err != nil {
		log.Fatal(err)
	}
	group, err := user.LookupGroup("arkvm")
	if err != nil {
		log.Fatal(err)
	}
	assets.ArkVMGID, err = strconv.Atoi(group.Gid)
	if err != nil {
		log.Fatal(err)
	}
	arkdAccount, err := user.Lookup("arkd")
	if err != nil {
		log.Fatal(err)
	}
	assets.ArkdUID, err = strconv.Atoi(arkdAccount.Uid)
	if err != nil {
		log.Fatal(err)
	}
	arkdGroup, err := user.LookupGroup("arkd")
	if err != nil {
		log.Fatal(err)
	}
	assets.ArkdGID, err = strconv.Atoi(arkdGroup.Gid)
	if err != nil {
		log.Fatal(err)
	}
	assets.StateDir, assets.RuntimeDir = config.StateDir, config.RuntimeDir
	runtime, err := launcher.NewFirecrackerRuntime(assets)
	if err != nil {
		log.Fatal(err)
	}
	if err := runtime.Reconcile(context.Background()); err != nil {
		log.Fatal(err)
	}
	server, err := launcher.NewServer(config, runtime)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	select {
	case err := <-errCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		shutdownErr := runtime.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		if shutdownErr != nil {
			log.Fatal(shutdownErr)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
		if err := <-errCh; err != nil {
			log.Fatal(err)
		}
		cancel()
		vmShutdownCtx, vmCancel := context.WithTimeout(context.Background(), 15*time.Second)
		if err := runtime.Shutdown(vmShutdownCtx); err != nil {
			vmCancel()
			log.Fatal(err)
		}
		vmCancel()
	}
}
