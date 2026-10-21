package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/artifact"
	assetmanifest "github.com/nishantdania/outpost/internal/assets"
	"github.com/nishantdania/outpost/internal/doctor"
	"github.com/nishantdania/outpost/internal/httpapi"
	"github.com/nishantdania/outpost/internal/launcher"
	"github.com/nishantdania/outpost/internal/manager"
	"github.com/nishantdania/outpost/internal/outpost"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config := launcher.DefaultConfig()
	assets := launcher.FirecrackerConfig{StateDir: config.StateDir, JailerBase: "/srv/outpost/jailer", Firecracker: "/usr/local/lib/outpost/launcher/firecracker", Jailer: "/usr/local/lib/outpost/launcher/jailer", Kernel: "/usr/local/lib/outpost/launcher/vmlinux", DefaultRootFS: "/var/lib/outpost/images/default/rootfs.ext4", ImageStore: "/var/lib/outpost/images", Uplink: "eth0"}
	database := flag.String("database", outpost.DefaultDatabasePath, "launcher SQLite database")
	socketGroup := flag.String("socket-group", "outpost", "privileged local client group")
	flag.StringVar(&config.StateDir, "state-dir", config.StateDir, "private VM state directory")
	flag.StringVar(&assets.JailerBase, "jailer-base", assets.JailerBase, "trusted jailer directory")
	flag.StringVar(&assets.Firecracker, "firecracker", assets.Firecracker, "trusted Firecracker executable")
	flag.StringVar(&assets.Jailer, "jailer", assets.Jailer, "trusted jailer executable")
	flag.StringVar(&assets.Kernel, "kernel", assets.Kernel, "trusted kernel")
	flag.StringVar(&assets.DefaultRootFS, "default-rootfs", assets.DefaultRootFS, "trusted default rootfs")
	flag.StringVar(&assets.ImageStore, "image-store", assets.ImageStore, "private image store")
	flag.StringVar(&assets.Uplink, "uplink", assets.Uplink, "host uplink interface")
	doctorMode := flag.Bool("doctor", false, "check server requirements")
	doctorOnline := flag.Bool("doctor-online", false, "check running launcher and socket")
	doctorJSON := flag.Bool("doctor-json", false, "write doctor JSON")
	flag.Parse()
	if *doctorMode {
		return diagnose(config, assets, *socketGroup, *doctorOnline, *doctorJSON)
	}
	if os.Geteuid() != 0 {
		return errors.New("the launcher must run as root")
	}
	syscall.Umask(0o077)
	if !filepath.IsAbs(*database) || filepath.Clean(*database) != *database {
		return errors.New("database must be an absolute clean filesystem path")
	}
	group, err := user.LookupGroup(*socketGroup)
	if err != nil {
		return err
	}
	config.SocketGID, err = strconv.Atoi(group.Gid)
	if err != nil {
		return err
	}
	account, err := user.Lookup("outpostvm")
	if err != nil {
		return err
	}
	assets.OutpostVMUID, err = strconv.Atoi(account.Uid)
	if err != nil {
		return err
	}
	assets.OutpostVMGID, err = strconv.Atoi(account.Gid)
	if err != nil {
		return err
	}
	assets.StateDir = config.StateDir
	for _, path := range []string{filepath.Dir(*database), assets.ImageStore, config.StateDir} {
		if err := privateDir(path, 0700, 0); err != nil {
			return err
		}
	}
	if info, err := os.Lstat(*database); err == nil {
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.Mode().IsRegular() || stat.Uid != 0 || stat.Nlink != 1 || info.Mode().Perm()&0077 != 0 {
			return errors.New("database must be a private root-owned regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	// Lock before opening SQLite or reconciling files. A second launcher must
	// not clean up the first launcher's in-flight publications or snapshots.
	locked := make(map[string]bool)
	for _, directory := range []string{filepath.Dir(*database), assets.ImageStore, config.StateDir} {
		path := filepath.Join(directory, ".launcher.lock")
		if locked[path] {
			continue
		}
		locked[path] = true
		fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		file := os.NewFile(uintptr(fd), path)
		if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			file.Close()
			return fmt.Errorf("another launcher owns %s: %w", directory, err)
		}
		defer file.Close()
	}
	if err := privateDir(config.RuntimeDir, 0750, config.SocketGID); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := outpost.Open(ctx, *database)
	if err != nil {
		return err
	}
	defer store.Close()
	assets.Store = store
	runtime, err := launcher.NewFirecrackerRuntime(assets)
	if err != nil {
		return err
	}
	if err := runtime.Reconcile(ctx); err != nil {
		return err
	}
	if err := store.RecoverLifecycle(ctx); err != nil {
		return err
	}
	disks, err := artifact.New(assets.ImageStore)
	if err != nil {
		return err
	}
	application, err := manager.New(store, launcher.Manager{FirecrackerRuntime: runtime}, disks)
	if err != nil {
		return err
	}
	server, err := launcher.NewServer(config, httpapi.Handler(application))
	if err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	var serveErr error
	select {
	case serveErr = <-errCh:
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		serveErr = server.Shutdown(shutdownCtx)
		cancel()
		if serveErr != nil {
			// A streaming import/export must finish or be disconnected before
			// VM shutdown and database close. Shutdown alone does not cancel it.
			server.Close()
		}
		serveErr = errors.Join(serveErr, <-errCh)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return errors.Join(serveErr, runtime.Shutdown(shutdownCtx))
}

func privateDir(path string, mode os.FileMode, gid int) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("invalid private directory %q", path)
	}
	if err := os.MkdirAll(path, mode); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != 0 {
		return fmt.Errorf("directory %q must be root-owned and not a symlink", path)
	}
	if err := os.Chown(path, 0, gid); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

func diagnose(config launcher.Config, assets launcher.FirecrackerConfig, socketGroup string, online, jsonOutput bool) error {
	manifest, err := assetmanifest.Load("/usr/local/lib/outpost/assets.json")
	if err != nil {
		return err
	}
	files, checksums := map[string]string{}, map[string]string{}
	for _, item := range manifest.Assets {
		path := "/usr/local/lib/outpost/" + item.File
		if item.File == "outpost-vm-launcher" || item.File == "firecracker" || item.File == "jailer" || item.File == "vmlinux" {
			path = "/usr/local/lib/outpost/launcher/" + item.File
		}
		if item.File == "rootfs.ext4" {
			path = assets.DefaultRootFS
		}
		files[item.Name], checksums[item.Name] = path, item.SHA256
	}
	report := doctor.Server(doctor.OSProbe{}, doctor.ServerConfig{StateDir: config.StateDir, JailerDir: assets.JailerBase, RuntimeDir: config.RuntimeDir, Assets: files, Manifest: checksums, Uplink: assets.Uplink, Socket: config.SocketPath, Online: online, Users: []string{"outpostvm"}, Groups: []string{socketGroup, "outpostvm"}, SocketUser: "root", SocketGroup: socketGroup, Directories: []doctor.Directory{{Path: "/var/lib/outpost", Mode: 0700, User: "root", Group: "root"}, {Path: config.StateDir, Mode: 0700, User: "root", Group: "root"}, {Path: assets.JailerBase, Mode: 0750, User: "root", Group: "outpostvm"}, {Path: config.RuntimeDir, Mode: 0750, User: "root", Group: socketGroup}}, Units: []string{"/etc/systemd/system/outpost-vm-launcher.service"}})
	if jsonOutput {
		if err := report.JSON(os.Stdout); err != nil {
			return err
		}
	} else {
		report.Text(os.Stdout)
	}
	if report.Failed() {
		return errors.New("doctor found failing checks")
	}
	return nil
}
