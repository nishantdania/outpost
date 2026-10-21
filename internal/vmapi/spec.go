package vmapi

import (
	"regexp"

	"github.com/google/uuid"
	"github.com/nishantdania/outpost/internal/outpost"
)

type VMSpec struct {
	ID        string
	ImageID   string
	VCPUs     int
	MemoryMiB int
	DiskGiB   int
}

type RuntimeState struct {
	Spec    VMSpec
	Running bool
}

var imageID = regexp.MustCompile(`^(default|sha256:[a-f0-9]{64})$`)

func ValidateSpec(spec VMSpec) error {
	if !ValidID(spec.ID) || !imageID.MatchString(spec.ImageID) ||
		spec.VCPUs < outpost.MinVCPUs || spec.VCPUs > outpost.MaxVCPUs ||
		spec.MemoryMiB < outpost.MinMemoryMiB || spec.MemoryMiB > outpost.MaxMemoryMiB ||
		spec.DiskGiB < outpost.MinDiskGiB || spec.DiskGiB > outpost.MaxDiskGiB {
		return ErrInvalid
	}
	return nil
}

func ValidID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}
