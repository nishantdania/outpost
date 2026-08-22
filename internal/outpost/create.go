package outpost

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

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
	if service.assets != "" {
		if err := service.start(&record); err != nil {
			return Record{}, err
		}
		record.Status = "running"
	}
	if err := service.save(append(records, record)); err != nil {
		return Record{}, err
	}
	return record, nil
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
