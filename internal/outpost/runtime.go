package outpost

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (service *Service) start(record *Record) error {
	directory := filepath.Join(filepath.Dir(service.path), "instances", record.ID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	rootfs := filepath.Join(directory, "rootfs.ext4")
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		if err := exec.Command("cp", "--reflink=auto", "--sparse=always", filepath.Join(service.assets, "rootfs.ext4"), rootfs).Run(); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	socket := filepath.Join(directory, "firecracker.sock")
	_ = os.Remove(socket)
	bootArgs := "console=ttyS0 reboot=k panic=1"
	config := map[string]any{"boot-source": map[string]any{"kernel_image_path": filepath.Join(service.assets, "vmlinux"), "boot_args": bootArgs}, "drives": []map[string]any{{"drive_id": "rootfs", "path_on_host": rootfs, "is_root_device": true, "is_read_only": false}}, "machine-config": map[string]any{"vcpu_count": 1, "mem_size_mib": 256}}
	if record.Tap != "" {
		config["network-interfaces"] = []map[string]any{{"iface_id": "eth0", "host_dev_name": record.Tap, "guest_mac": record.MAC}}
		config["boot-source"].(map[string]any)["boot_args"] = bootArgs + " ip=" + record.IP + "::172.30.0.1:255.255.255.0::eth0:off nameserver=1.1.1.1"
	}
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "config.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(directory, "firecracker.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	command := exec.Command(filepath.Join(service.assets, "firecracker"), "--api-sock", socket, "--config-file", path)
	command.Stdout = log
	command.Stderr = log
	if err := command.Start(); err != nil {
		log.Close()
		return err
	}
	time.Sleep(time.Second)
	if !alive(command.Process.Pid) {
		return fmt.Errorf("firecracker exited; see %s", filepath.Join(directory, "firecracker.log"))
	}
	record.PID = command.Process.Pid
	record.Socket = socket
	go func() { _ = command.Wait(); _ = log.Close() }()
	return nil
}

func alive(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	parts := strings.Fields(string(data))
	return len(parts) > 2 && parts[2] != "Z"
}

func (service *Service) stopProcess(record Record) {
	if record.PID > 0 {
		if process, err := os.FindProcess(record.PID); err == nil {
			_ = process.Kill()
		}
	}
}

func (service *Service) stop(record Record) {
	service.stopProcess(record)
	_ = os.RemoveAll(filepath.Join(filepath.Dir(service.path), "instances", record.ID))
}
