package ark

import "time"

const (
	DefaultImageID   = "default"
	DefaultVCPUs     = 2
	DefaultMemoryMiB = 4096
	DefaultDiskGiB   = 8
	MinVCPUs         = 1
	MaxVCPUs         = 32
	MinMemoryMiB     = 128
	MaxMemoryMiB     = 131072
	MinDiskGiB       = 1
	MaxDiskGiB       = 1024

	DesiredRunning = "running"
	DesiredStopped = "stopped"
	DesiredDeleted = "deleted"

	StatusProvisioning = "provisioning"
	StatusRunning      = "running"
	StatusStopping     = "stopping"
	StatusStopped      = "stopped"
	StatusDeleting     = "deleting"
	StatusFailed       = "failed"
)

type Ark struct {
	ID           string
	Name         string
	ImageID      string
	VCPUs        int
	MemoryMiB    int
	DiskGiB      int
	DesiredState string
	Status       string
	GuestIP      string
	Failure      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateInput struct {
	Name      string
	ImageID   string
	VCPUs     int
	MemoryMiB int
	DiskGiB   int
}
