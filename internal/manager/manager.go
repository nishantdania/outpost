// Package manager owns the application's VM and disk-image operations.
// Transports call this directly; SQLite, disk I/O, and host execution remain
// separate implementations rather than additional workflow/facade layers.
package manager

import (
	"context"
	"errors"
	"sync"

	"github.com/nishantdania/outpost/internal/artifact"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/vmapi"
)

type Manager struct {
	store     *outpost.Store
	manager   vmapi.Manager
	artifacts *artifact.Store
	imagesMu  sync.Mutex // publication/registration/pinning must not race removal/GC
}

// New reconciles abandoned image publications before requests are accepted.
// The caller must hold the launcher's exclusive filesystem locks.
func New(store *outpost.Store, runtime vmapi.Manager, disks *artifact.Store) (*Manager, error) {
	if store == nil || runtime == nil || disks == nil {
		return nil, errors.New("manager requires a store, runtime, and image storage")
	}
	images, err := store.Catalog.ListImages(context.Background())
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(images))
	for _, image := range images {
		known[image.Digest] = true
	}
	if err := disks.Reconcile(known); err != nil {
		return nil, err
	}
	return &Manager{store: store, manager: runtime, artifacts: disks}, nil
}
