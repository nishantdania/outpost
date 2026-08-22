package outpost

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
)

func (service *Service) start(record *Record) error {
	directory := filepath.Join(filepath.Dir(service.path), "instances", record.ID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	rootfs := filepath.Join(directory, "rootfs.ext4")
	if err := exec.Command("cp", "--reflink=auto", "--sparse=always", filepath.Join(service.assets, "rootfs.ext4"), rootfs).Run(); err != nil {
		return err
	}
	socket := filepath.Join(directory, "firecracker.sock")
	config := map[string]any{"boot-source": map[string]any{"kernel_image_path": filepath.Join(service.assets, "vmlinux"), "boot_args": "console=ttyS0 reboot=k panic=1"}, "drives": []map[string]any{{"drive_id": "rootfs", "path_on_host": rootfs, "is_root_device": true, "is_read_only": false}}, "machine-config": map[string]any{"vcpu_count": 1, "mem_size_mib": 256}}
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
	record.PID = command.Process.Pid
	record.Socket = socket
	return nil
}

func (service *Service) stop(record Record) {
	if record.PID > 0 {
		if process, err := os.FindProcess(record.PID); err == nil {
			_ = process.Kill()
		}
	}
	_ = os.RemoveAll(filepath.Join(filepath.Dir(service.path), "instances", record.ID))
}
