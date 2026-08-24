package launcher

import (
	"context"
	"sort"
	"sync"

	"github.com/nishantdania/outpost/internal/vmapi"
)

type Runtime interface {
	Create(context.Context, vmapi.VMSpec) error
	Start(context.Context, string) (string, error)
	Stop(context.Context, string) error
	Delete(context.Context, string) error
	Inspect(context.Context, string) (vmapi.RuntimeState, error)
	List(context.Context) ([]vmapi.RuntimeState, error)
}

type MemoryRuntime struct {
	mu  sync.Mutex
	vms map[string]memoryVM
}

type memoryVM struct {
	spec    vmapi.VMSpec
	running bool
}

func NewMemoryRuntime() *MemoryRuntime { return &MemoryRuntime{vms: make(map[string]memoryVM)} }

func (r *MemoryRuntime) Create(ctx context.Context, spec vmapi.VMSpec) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vm, exists := r.vms[spec.ID]
	if exists {
		if vm.spec != spec {
			return vmapi.ErrConflict
		}
		return nil
	}
	r.vms[spec.ID] = memoryVM{spec: spec}
	return nil
}
func (r *MemoryRuntime) Start(ctx context.Context, id string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vm, exists := r.vms[id]
	if !exists {
		return "", vmapi.ErrNotFound
	}
	vm.running = true
	r.vms[id] = vm
	return "172.30.0.2", nil
}
func (r *MemoryRuntime) Stop(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vm, exists := r.vms[id]
	if !exists {
		return vmapi.ErrNotFound
	}
	vm.running = false
	r.vms[id] = vm
	return nil
}
func (r *MemoryRuntime) Inspect(ctx context.Context, id string) (vmapi.RuntimeState, error) {
	if err := ctx.Err(); err != nil {
		return vmapi.RuntimeState{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vm, exists := r.vms[id]
	if !exists {
		return vmapi.RuntimeState{}, vmapi.ErrNotFound
	}
	return vmapi.RuntimeState{Spec: vm.spec, Running: vm.running}, nil
}
func (r *MemoryRuntime) List(ctx context.Context) ([]vmapi.RuntimeState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	vms := make([]vmapi.RuntimeState, 0, len(r.vms))
	for _, vm := range r.vms {
		vms = append(vms, vmapi.RuntimeState{Spec: vm.spec, Running: vm.running})
	}
	sort.Slice(vms, func(i, j int) bool { return vms[i].Spec.ID < vms[j].Spec.ID })
	return vms, nil
}
func (r *MemoryRuntime) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.vms, id)
	return nil
}
