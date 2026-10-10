package vmapi

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/nishantdania/outpost/internal/credentials"
)

func TestEgressGuestMaterialRejectsSecretsAndShellInjection(t *testing.T) {
	directory, state := t.TempDir(), t.TempDir()
	os.Chmod(directory, 0700)
	os.Chmod(state, 0700)
	host, err := credentials.NewHost(directory, state)
	if err != nil {
		t.Fatal(err)
	}
	profile := credentials.Profile{Credentials: []credentials.Binding{{Env: "API_KEY", Secret: "key", Host: "api.example.com", Header: "Authorization", Encoding: "bearer"}}}
	spec := VMSpec{ID: uuid.NewString(), ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8, EgressCA: host.PublicCA, CredentialEnv: profile.Environment()}
	if err := ValidateCreate(CreateRequest{Version: Version, Spec: spec}); err != nil {
		t.Fatal(err)
	}
	for _, env := range []string{"API_KEY=real-secret\n", "API_KEY=$(touch /host)\n", "API_KEY=value; command\n", profile.Environment() + profile.Environment()} {
		bad := spec
		bad.CredentialEnv = env
		if err := ValidateCreate(CreateRequest{Version: Version, Spec: bad}); err == nil {
			t.Fatal("unsafe guest material accepted")
		}
	}
	for _, ca := range []string{"", host.PublicCA + "-----BEGIN PRIVATE KEY-----\nnot-allowed\n-----END PRIVATE KEY-----\n"} {
		bad := spec
		bad.EgressCA = ca
		if err := ValidateCreate(CreateRequest{Version: Version, Spec: bad}); err == nil {
			t.Fatal("invalid CA material accepted")
		}
	}
}
