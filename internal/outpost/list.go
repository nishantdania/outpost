package outpost

import "context"

func (service *Service) List(context.Context) ([]Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	records, err := service.load()
	if err != nil {
		return nil, err
	}
	changed := false
	for index := range records {
		if resources, ok := service.runtimeResources(records[index]); ok && records[index].Resources != resources {
			records[index].Resources = resources
			changed = true
		}
		if records[index].Status == "running" && !alive(records[index].PID) {
			records[index].Status = "stopped"
			records[index].PID = 0
			records[index].Socket = ""
			changed = true
		}
	}
	if changed {
		if err := service.save(records); err != nil {
			return nil, err
		}
	}
	return records, nil
}
