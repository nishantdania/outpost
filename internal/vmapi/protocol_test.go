package vmapi

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateRejectsMalformedAndMismatchedSSHPublicKeys(t *testing.T) {
	for _, key := range []string{
		"ssh-ed25519 !!!",
		"ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		spec := VMSpec{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8, SSHPublicKey: key}
		if err := ValidateCreate(CreateRequest{Version: Version, Spec: spec}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("key %q error = %v", key, err)
		}
	}
}

func TestValidateRequests(t *testing.T) {
	a := VMSpec{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}
	if err := ValidateCreate(CreateRequest{Version: Version, Spec: a}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCreate(CreateRequest{Version: Version + 1, Spec: a}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("version error = %v", err)
	}
	if err := ValidateID(IDRequest{Version: Version, ID: "../../etc/passwd"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ID error = %v", err)
	}
}
