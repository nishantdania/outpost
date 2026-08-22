package outpost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Record struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	path string
	mu   sync.Mutex
}

func New(path string) *Service {
	return &Service{path: path}
}

func (service *Service) Create(_ context.Context, name string) (Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()

	records, err := service.load()
	if err != nil {
		return Record{}, err
	}
	id, err := uuid()
	if err != nil {
		return Record{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = id
	}
	record := Record{ID: id, Name: name, Status: "created", CreatedAt: time.Now().UTC()}
	records = append(records, record)
	if err := service.save(records); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (service *Service) List(context.Context) ([]Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.load()
}

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
	return records, nil
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

func uuid() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	bytes[6] = bytes[6]&0x0f | 0x40
	bytes[8] = bytes[8]&0x3f | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(bytes[0:4]), hex.EncodeToString(bytes[4:6]), hex.EncodeToString(bytes[6:8]), hex.EncodeToString(bytes[8:10]), hex.EncodeToString(bytes[10:16])), nil
}
