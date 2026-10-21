package launcher

import (
	"context"
	"errors"
	"fmt"

	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

// runtimeView hydrates an in-memory execution record from the canonical database.
func runtimeView(r outpost.Runtime) runtimeRecord {
	a := r.VM
	return runtimeRecord{
		Spec: vmapi.VMSpec{ID: a.ID, ImageID: a.ImageID, VCPUs: a.VCPUs, MemoryMiB: a.MemoryMiB, DiskGiB: a.DiskGiB},
		Tap:  r.Tap, GuestIP: a.GuestIP, Gateway: r.Gateway,
		PID: r.PID, StartTime: r.StartTime, UFWRoute: r.UFWRoute,
	}
}

func (r *FirecrackerRuntime) load(id string) (runtimeRecord, error) {
	if !validID(id) {
		return runtimeRecord{}, vmapi.ErrInvalid
	}
	row, err := r.config.Store.Runtime(context.Background(), id)
	if errors.Is(err, outpost.ErrNotFound) {
		return runtimeRecord{}, vmapi.ErrNotFound
	}
	if err != nil {
		return runtimeRecord{}, err
	}
	m := runtimeView(row)
	if !runtimeValid(m) {
		return runtimeRecord{}, fmt.Errorf("invalid runtime record for %s: %w", id, vmapi.ErrInvalid)
	}
	return m, nil
}

func (r *FirecrackerRuntime) loadAll() ([]runtimeRecord, error) {
	rows, err := r.config.Store.Runtimes(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]runtimeRecord, 0, len(rows))
	for _, row := range rows {
		m := runtimeView(row)
		if !runtimeValid(m) {
			return nil, fmt.Errorf("invalid runtime record for %s: %w", m.Spec.ID, vmapi.ErrInvalid)
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *FirecrackerRuntime) save(id string, m runtimeRecord) error {
	if m.Spec.ID != id || !runtimeValid(m) {
		return fmt.Errorf("invalid runtime record: %w", vmapi.ErrInvalid)
	}
	err := r.config.Store.SaveRuntime(context.Background(), outpost.Runtime{
		VM:  outpost.Outpost{ID: id, ImageID: m.Spec.ImageID, VCPUs: m.Spec.VCPUs, MemoryMiB: m.Spec.MemoryMiB, DiskGiB: m.Spec.DiskGiB, GuestIP: m.GuestIP},
		Tap: m.Tap, Gateway: m.Gateway, PID: m.PID, StartTime: m.StartTime, UFWRoute: m.UFWRoute,
	})
	if errors.Is(err, outpost.ErrStateMismatch) {
		return vmapi.ErrConflict
	}
	return err
}
