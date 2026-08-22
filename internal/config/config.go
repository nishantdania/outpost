package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ClientHost struct {
	DaemonURL string `json:"daemon_url"`
	SSHHost   string `json:"ssh_host"`
}

type Client struct {
	DaemonURL   string                `json:"daemon_url,omitempty"`
	SSHHost     string                `json:"ssh_host,omitempty"`
	DefaultHost string                `json:"default_host,omitempty"`
	Hosts       map[string]ClientHost `json:"hosts,omitempty"`
	Host        string                `json:"-"`
}

type Daemon struct {
	ListenAddr         string `json:"listen_addr"`
	FirecrackerVersion string `json:"firecracker_version"`
}

func ClientPath() (string, error) {
	return configPath("config.json")
}

func DaemonPath() (string, error) {
	return configPath("daemon.json")
}

func OutpostsPath() (string, error) {
	directory := os.Getenv("XDG_DATA_HOME")
	if directory == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		directory = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(directory, "outpost", "outposts.json"), nil
}

func configPath(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find configuration directory: %w", err)
	}
	return filepath.Join(dir, "outpost", name), nil
}

func LoadClient() (Client, error) {
	path, err := ClientPath()
	if err != nil {
		return Client{}, err
	}
	return LoadClientFile(path)
}

func LoadClientFile(path string) (Client, error) {
	var cfg Client
	if err := load(path, &cfg); err != nil {
		return Client{}, err
	}
	selected := os.Getenv("OUTPOST_HOST")
	if len(cfg.Hosts) == 0 {
		if selected != "" {
			return Client{}, fmt.Errorf("%s: host %q requested but no hosts are configured", path, selected)
		}
		if cfg.DaemonURL == "" {
			return Client{}, fmt.Errorf("%s: daemon_url is required", path)
		}
		return cfg, nil
	}
	if selected == "" {
		selected = cfg.DefaultHost
	}
	if selected == "" {
		return Client{}, fmt.Errorf("%s: default_host is required", path)
	}
	host, ok := cfg.Hosts[selected]
	if !ok {
		return Client{}, fmt.Errorf("%s: host %q is not configured", path, selected)
	}
	if host.DaemonURL == "" {
		return Client{}, fmt.Errorf("%s: hosts.%s.daemon_url is required", path, selected)
	}
	cfg.DaemonURL, cfg.SSHHost, cfg.Host = host.DaemonURL, host.SSHHost, selected
	return cfg, nil
}

func LoadDaemon() (Daemon, error) {
	path, err := DaemonPath()
	if err != nil {
		return Daemon{}, err
	}
	return LoadDaemonFile(path)
}

func LoadDaemonFile(path string) (Daemon, error) {
	var cfg Daemon
	if err := load(path, &cfg); err != nil {
		return Daemon{}, err
	}
	if cfg.ListenAddr == "" {
		return Daemon{}, fmt.Errorf("%s: listen_addr is required", path)
	}
	if cfg.FirecrackerVersion == "" {
		cfg.FirecrackerVersion = "v1.10.1"
	}
	return cfg, nil
}

func load(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open configuration %s: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode configuration %s: %w", path, err)
	}
	return nil
}
