package outpost

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/credentials"
)

func TestCredentialImageProfileImmutableAndTagPublicationAtomic(t *testing.T) {
	s, err := Open(t.Context(), filepath.Join(t.TempDir(), "outpost.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a, b := "sha256:"+strings.Repeat("a", 64), "sha256:"+strings.Repeat("b", 64)
	profile := credentials.Profile{Credentials: []credentials.Binding{{Env: "TOKEN", Secret: "key", Host: "api.example.com", Header: "Authorization", Encoding: "bearer"}}}
	if err := s.PutSnapshotImage(t.Context(), a, 123, "ready", profile.JSON()); err != nil {
		t.Fatal(err)
	}
	if err := s.PutImage(t.Context(), b, 456, "other"); err != nil {
		t.Fatal(err)
	}
	changed := profile
	changed.AllowedHosts = []string{"cdn.example.com"}
	if err := s.PutSnapshotImage(t.Context(), a, 123, "other", changed.JSON()); !errors.Is(err, ErrCredentialConflict) {
		t.Fatalf("profile conflict: %v", err)
	}
	if digest, err := s.ResolveImage(t.Context(), "other"); err != nil || digest != b {
		t.Fatal("conflict moved existing tag")
	}
	if err := s.PutSnapshotImage(t.Context(), a, 123, "alias", profile.JSON()); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{a, "ready", "alias"} {
		if config, err := s.ImageCredentials(t.Context(), ref); err != nil || config != profile.JSON() {
			t.Fatal("profile changed through alias")
		}
	}
	if err := s.PutSnapshotImage(t.Context(), b, 456, "no-profile", profile.JSON()); !errors.Is(err, ErrCredentialConflict) {
		t.Fatal("retroactively changed a published digest")
	}
	if err := s.PutSnapshotImage(t.Context(), a, 123, "invalid", "not-json"); err == nil {
		t.Fatal("persisted invalid profile")
	}
	input := CreateInput{Name: "fork", ImageID: "ready", VCPUs: 1, MemoryMiB: 128, DiskGiB: 1}
	fork, err := s.CreateWith(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutImage(t.Context(), b, 456, "ready"); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Get(t.Context(), fork.Name)
	if err != nil || loaded.ImageID != a || loaded.CredentialConfig != profile.JSON() {
		t.Fatal("tag movement changed fork authorization")
	}
	if _, err := s.Transition(t.Context(), fork.ID, StatusProvisioning, DesiredRunning, StatusRunning, "172.30.0.2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CredentialGuest(t.Context(), "172.30.0.2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Transition(t.Context(), fork.ID, StatusRunning, DesiredStopped, StatusStopping, "172.30.0.2", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CredentialGuest(t.Context(), "172.30.0.2"); !errors.Is(err, ErrNotFound) {
		t.Fatal("stopping VM retained a grant")
	}
}
