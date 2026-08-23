package vmapi

import (
	"regexp"

	"github.com/google/uuid"
	"github.com/nishantdania/ark/internal/ark"
)

const Version = 1

var imageID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type VMSpec struct {
	ID           string `json:"id"`
	ImageID      string `json:"image_id"`
	VCPUs        int    `json:"vcpus"`
	MemoryMiB    int    `json:"memory_mib"`
	DiskGiB      int    `json:"disk_gib"`
	SSHPublicKey string `json:"ssh_public_key"`
}

type CreateRequest struct {
	Version int    `json:"version"`
	Spec    VMSpec `json:"spec"`
}

type VersionRequest struct {
	Version int `json:"version"`
}

type IDRequest struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
}

type StartResponse struct {
	GuestIP string `json:"guest_ip"`
}

type RuntimeState struct {
	Spec    VMSpec `json:"spec"`
	Running bool   `json:"running"`
}

type InspectResponse struct {
	State RuntimeState `json:"state"`
}

type ListResponse struct {
	VMs []RuntimeState `json:"vms"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func ValidateCreate(request CreateRequest) error {
	if request.Version != Version || !validSpec(request.Spec) {
		return ErrInvalid
	}
	return nil
}

func ValidateVersion(request VersionRequest) error {
	if request.Version != Version {
		return ErrInvalid
	}
	return nil
}

func ValidateID(request IDRequest) error {
	if request.Version != Version || !validID(request.ID) {
		return ErrInvalid
	}
	return nil
}

func validSpec(spec VMSpec) bool {
	return validID(spec.ID) && imageID.MatchString(spec.ImageID) && ark.ValidateSSHPublicKey(spec.SSHPublicKey) == nil &&
		spec.VCPUs >= ark.MinVCPUs && spec.VCPUs <= ark.MaxVCPUs &&
		spec.MemoryMiB >= ark.MinMemoryMiB && spec.MemoryMiB <= ark.MaxMemoryMiB &&
		spec.DiskGiB >= ark.MinDiskGiB && spec.DiskGiB <= ark.MaxDiskGiB
}
func validID(id string) bool {
	parsed, err := uuid.Parse(id)
	return err == nil && parsed.String() == id
}
