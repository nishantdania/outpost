package outpost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Resources struct {
	VCPUs     int `json:"vcpus"`
	MemoryMiB int `json:"memory_mib"`
	DiskGiB   int `json:"disk_gib"`
}

type Record struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	PID    int    `json:"pid,omitempty"`
	Socket string `json:"socket,omitempty"`
	IP     string `json:"ip,omitempty"`
	Tap    string `json:"tap,omitempty"`
	MAC    string `json:"mac,omitempty"`
	Resources
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	path   string
	assets string
	mu     sync.Mutex
}

func New(path string) *Service                    { return &Service{path: path} }
func NewWithRuntime(path, assets string) *Service { return &Service{path: path, assets: assets} }

func (service *Service) load() ([]Record, error) {
	file, err := os.Open(service.path)
	if os.IsNotExist(err) {
		return []Record{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var records []Record
	if err := json.NewDecoder(file).Decode(&records); err != nil {
		return nil, err
	}
	for index := range records {
		if records[index].VCPUs == 0 {
			records[index].VCPUs = 1
		}
		if records[index].MemoryMiB == 0 {
			records[index].MemoryMiB = 256
		}
		if records[index].DiskGiB == 0 {
			records[index].DiskGiB = 4
		}
	}
	return records, nil
}

func defaultResources(resources Resources) Resources {
	if resources.VCPUs == 0 {
		resources.VCPUs = 2
	}
	if resources.MemoryMiB == 0 {
		resources.MemoryMiB = 4096
	}
	if resources.DiskGiB == 0 {
		resources.DiskGiB = 8
	}
	return resources
}

func (service *Service) save(records []Record) error {
	if err := os.MkdirAll(filepath.Dir(service.path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(service.path), ".outposts-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(records); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, service.path)
}
