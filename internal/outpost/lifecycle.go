package outpost

import (
	"context"
	"fmt"
)

func (service *Service) Start(_ context.Context, id string) (Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	records, err := service.load()
	if err != nil {
		return Record{}, err
	}
	for index := range records {
		if records[index].ID != id {
			continue
		}
		if records[index].Status == "running" && alive(records[index].PID) {
			return records[index], nil
		}
		if err := service.start(&records[index]); err != nil {
			return Record{}, err
		}
		records[index].Status = "running"
		if err := service.save(records); err != nil {
			service.stop(records[index])
			return Record{}, err
		}
		return records[index], nil
	}
	return Record{}, fmt.Errorf("outpost not found")
}

func (service *Service) Stop(_ context.Context, id string) (Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	records, err := service.load()
	if err != nil {
		return Record{}, err
	}
	for index := range records {
		if records[index].ID != id {
			continue
		}
		service.stopProcess(records[index])
		records[index].Status = "stopped"
		records[index].PID = 0
		records[index].Socket = ""
		if err := service.save(records); err != nil {
			return Record{}, err
		}
		return records[index], nil
	}
	return Record{}, fmt.Errorf("outpost not found")
}
