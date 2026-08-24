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

	assetmanifest "github.com/nishantdania/outpost/internal/assets"
	"github.com/nishantdania/outpost/internal/doctor"
	"github.com/nishantdania/outpost/internal/launcher"
)

func main() {
	config := launcher.DefaultConfig()
	assets := launcher.FirecrackerConfig{StateDir: config.StateDir, RuntimeDir: config.RuntimeDir, JailerBase: "/srv/outpost/jailer", Firecracker: "/usr/local/lib/outpost/launcher/firecracker", Jailer: "/usr/local/lib/outpost/launcher/jailer", Kernel: "/usr/local/lib/outpost/launcher/vmlinux", DefaultRootFS: "/var/lib/outpostd/images/default/rootfs.ext4", ImageStore: "/var/lib/outpostd/images", Uplink: "eth0", DNS: "1.1.1.1"}
	flag.StringVar(&config.SocketPath, "socket", config.SocketPath, "launcher Unix socket")
	flag.StringVar(&config.StateDir, "state-dir", config.StateDir, "launcher state directory")
	flag.StringVar(&config.RuntimeDir, "runtime-dir", config.RuntimeDir, "launcher runtime directory")
	flag.IntVar(&config.AllowedUID, "allowed-uid", -1, "authorized outpostd Unix peer UID")
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
		manifest, err := assetmanifest.Load("/usr/local/lib/outpost/assets.json")
		if err != nil {
			log.Print(err)
			os.Exit(1)
		}
		files := map[string]string{}
		checksums := map[string]string{}
		for _, item := range manifest.Assets {
			path := "/usr/local/lib/outpost/" + item.File
			if item.File == "outpost" {
				path = "/usr/local/bin/outpost"
			}
			if item.File == "outpost-vm-launcher" || item.File == "firecracker" || item.File == "jailer" || item.File == "vmlinux" {
				path = "/usr/local/lib/outpost/launcher/" + item.File
			}
			if item.File == "rootfs.ext4" {
				path = "/var/lib/outpostd/images/default/rootfs.ext4"
			}
			files[item.Name] = path
			checksums[item.Name] = item.SHA256
		}
		report := doctor.Server(doctor.OSProbe{}, doctor.ServerConfig{StateDir: config.StateDir, JailerDir: assets.JailerBase, RuntimeDir: config.RuntimeDir, Assets: files, Manifest: checksums, Uplink: assets.Uplink, Socket: config.SocketPath, Online: *doctorOnline, Users: []string{"outpostd", "outpostvm"}, Groups: []string{"outpostd", "outpostvm"}, SocketUser: "root", SocketGroup: "outpostd", Directories: []doctor.Directory{{Path: "/var/lib/outpostd", Mode: 0750, User: "outpostd", Group: "outpostd"}, {Path: config.StateDir, Mode: 0750, User: "root", Group: "outpostd"}, {Path: assets.JailerBase, Mode: 0750, User: "root", Group: "outpostvm"}, {Path: config.RuntimeDir, Mode: 0750, User: "root", Group: "outpostd"}, {Path: "/usr/local/lib/outpost", Mode: 0750, User: "root", Group: "outpostd"}, {Path: "/usr/local/lib/outpost/launcher", Mode: 0750, User: "root", Group: "outpostvm"}}, Units: []string{"/etc/systemd/system/outpostd.service", "/etc/systemd/system/outpost-vm-launcher.service"}})
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
		account, err := user.Lookup("outpostd")
		if err != nil {
			log.Fatal(err)
		}
		config.AllowedUID, err = strconv.Atoi(account.Uid)
		if err != nil {
			log.Fatal(err)
		}
	}
	account, err := user.Lookup("outpostvm")
	if err != nil {
		log.Fatal(err)
	}
	assets.OutpostVMUID, err = strconv.Atoi(account.Uid)
	if err != nil {
		log.Fatal(err)
	}
	group, err := user.LookupGroup("outpostvm")
	if err != nil {
		log.Fatal(err)
	}
	assets.OutpostVMGID, err = strconv.Atoi(group.Gid)
	if err != nil {
		log.Fatal(err)
	}
	outpostdAccount, err := user.Lookup("outpostd")
	if err != nil {
		log.Fatal(err)
	}
	assets.OutpostdUID, err = strconv.Atoi(outpostdAccount.Uid)
	if err != nil {
		log.Fatal(err)
	}
	outpostdGroup, err := user.LookupGroup("outpostd")
	if err != nil {
		log.Fatal(err)
	}
	assets.OutpostdGID, err = strconv.Atoi(outpostdGroup.Gid)
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
