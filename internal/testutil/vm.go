// Package testutil provides VM implementations for tests.
package testutil

import (
	"context"
	"sort"
	"sync"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type FakeManager struct {
	CreateFunc func(context.Context, outpost.Outpost) error
	StartFunc  func(context.Context, string) (string, error)
	StopFunc   func(context.Context, string) error
	DeleteFunc func(context.Context, string) error
	Calls      []string
}

func (m *FakeManager) Create(ctx context.Context, a outpost.Outpost) error {
	m.Calls = append(m.Calls, "create:"+a.ID)
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, a)
	}
	return nil
}

func (m *FakeManager) Start(ctx context.Context, id string) (string, error) {
	m.Calls = append(m.Calls, "start:"+id)
	if m.StartFunc != nil {
		return m.StartFunc(ctx, id)
	}
	return "", nil
}

func (m *FakeManager) Stop(ctx context.Context, id string) error {
	m.Calls = append(m.Calls, "stop:"+id)
	if m.StopFunc != nil {
		return m.StopFunc(ctx, id)
	}
	return nil
}

func (m *FakeManager) Delete(ctx context.Context, id string) error {
	m.Calls = append(m.Calls, "delete:"+id)
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

type MemoryRuntime struct {
	mu  sync.Mutex
	vms map[string]memoryVM
}

type memoryVM struct {
	spec    vmapi.VMSpec
	running bool
}

func NewMemoryRuntime() *MemoryRuntime {
	return &MemoryRuntime{vms: make(map[string]memoryVM)}
}

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
