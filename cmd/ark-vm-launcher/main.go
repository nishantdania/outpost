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
	assets := launcher.FirecrackerConfig{StateDir: config.StateDir, RuntimeDir: config.RuntimeDir, JailerBase: "/srv/ark/jailer", Firecracker: "/usr/local/lib/ark/firecracker", Jailer: "/usr/local/lib/ark/jailer", Kernel: "/usr/local/lib/ark/vmlinux", DefaultRootFS: "/var/lib/arkd/images/default/rootfs.ext4", Uplink: "eth0", DNS: "1.1.1.1"}
	flag.StringVar(&config.SocketPath, "socket", config.SocketPath, "launcher Unix socket")
	flag.StringVar(&config.StateDir, "state-dir", config.StateDir, "launcher state directory")
	flag.StringVar(&config.RuntimeDir, "runtime-dir", config.RuntimeDir, "launcher runtime directory")
	flag.IntVar(&config.AllowedUID, "allowed-uid", -1, "authorized arkd Unix peer UID")
	flag.StringVar(&assets.JailerBase, "jailer-base", assets.JailerBase, "trusted jailer directory")
	flag.StringVar(&assets.Firecracker, "firecracker", assets.Firecracker, "trusted Firecracker executable")
	flag.StringVar(&assets.Jailer, "jailer", assets.Jailer, "trusted jailer executable")
	flag.StringVar(&assets.Kernel, "kernel", assets.Kernel, "trusted kernel")
	flag.StringVar(&assets.DefaultRootFS, "default-rootfs", assets.DefaultRootFS, "trusted default rootfs")
	flag.StringVar(&assets.Uplink, "uplink", assets.Uplink, "host uplink interface")
	flag.StringVar(&assets.DNS, "dns", assets.DNS, "guest DNS server")
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
