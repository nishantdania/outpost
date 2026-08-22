package outpost

import "context"

func (service *Service) List(context.Context) ([]Record, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.load()
}
