package outpost

import "context"

func (service *Service) Delete(_ context.Context, identifier string) (bool, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	records, err := service.load()
	if err != nil {
		return false, err
	}
	for index, record := range records {
		if record.ID == identifier || record.Name == identifier {
			service.stop(record)
			return true, service.save(append(records[:index], records[index+1:]...))
		}
	}
	return false, nil
}
