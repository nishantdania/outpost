package outpost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func (service *Service) Create(_ context.Context, name string, resources Resources) (Record, error) {
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
	resources = defaultResources(resources)
	if resources.VCPUs < 1 || resources.VCPUs > 32 {
		return Record{}, fmt.Errorf("cpus must be between 1 and 32")
	}
	if resources.MemoryMiB < 128 || resources.MemoryMiB > 131072 {
		return Record{}, fmt.Errorf("memory must be between 128 MiB and 128 GiB")
	}
	if resources.DiskGiB < 1 || resources.DiskGiB > 1024 {
		return Record{}, fmt.Errorf("disk must be between 1 and 1024 GiB")
	}
	if name == "" {
		name = id
	}
	for _, record := range records {
		if record.Name == name {
			return Record{}, fmt.Errorf("outpost name already exists")
		}
	}
	record := Record{ID: id, Name: name, Status: "created", Resources: resources, CreatedAt: time.Now().UTC()}
	if service.assets != "" {
		index, err := networkIndex(records)
		if err != nil {
			return Record{}, err
		}
		record.IP = fmt.Sprintf("172.30.0.%d", index+2)
		record.Tap = fmt.Sprintf("outpost-tap%d", index)
		record.MAC = fmt.Sprintf("06:00:ac:1e:00:%02x", index+2)
		if err := service.start(&record); err != nil {
			service.stop(record)
			return Record{}, err
		}
		record.Status = "running"
	}
	if err := service.save(append(records, record)); err != nil {
		return Record{}, err
	}
	return record, nil
}

func networkIndex(records []Record) (int, error) {
	for index := 0; index < 16; index++ {
		tap := fmt.Sprintf("outpost-tap%d", index)
		used := false
		for _, record := range records {
			if record.Tap == tap {
				used = true
				break
			}
		}
		if !used {
			return index, nil
		}
	}
	return 0, fmt.Errorf("network capacity reached")
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
