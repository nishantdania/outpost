package outpost

import (
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

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

type Outpost struct {
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
	SSHPublicKey string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateInput struct {
	Name         string
	ImageID      string
	VCPUs        int
	MemoryMiB    int
	DiskGiB      int
	SSHPublicKey string
}

func ValidateSSHPublicKey(value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return ErrInvalidSSHPublicKey
	}
	key, comment, options, rest, err := ssh.ParseAuthorizedKey([]byte(value))
	if err != nil || len(options) != 0 || len(strings.TrimSpace(string(rest))) != 0 {
		return ErrInvalidSSHPublicKey
	}
	fields := strings.Fields(value)
	if len(fields) < 2 || fields[0] != key.Type() || strings.ContainsAny(comment, "\r\n") {
		return ErrInvalidSSHPublicKey
	}
	switch key.Type() {
	case ssh.KeyAlgoED25519, ssh.KeyAlgoRSA, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521:
		return nil
	default:
		return ErrInvalidSSHPublicKey
	}
}
