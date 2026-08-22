package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
)

type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func Run(ctx context.Context) []Check {
	home, _ := os.UserHomeDir()
	assets := filepath.Join(home, ".local", "share", "outpost", "assets")
	checks := []Check{}
	for _, name := range []string{"/dev/kvm", filepath.Join(assets, "firecracker"), filepath.Join(assets, "vmlinux"), filepath.Join(assets, "rootfs.ext4"), filepath.Join(assets, "manifest.json")} {
		_, err := os.Stat(name)
		checks = append(checks, Check{Name: name, OK: err == nil, Message: message(err)})
	}
	if file, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0); err != nil {
		checks[0].OK = false
		checks[0].Message = err.Error()
	} else {
		file.Close()
	}
	if err := exec.CommandContext(ctx, "e2fsck", "-fn", filepath.Join(assets, "rootfs.ext4")).Run(); err != nil {
		checks = append(checks, Check{Name: "rootfs validity", OK: false, Message: err.Error()})
	} else {
		checks = append(checks, Check{Name: "rootfs validity", OK: true, Message: "ok"})
	}
	return checks
}
func message(err error) string {
	if err != nil {
		return err.Error()
	}
	return "ok"
}
