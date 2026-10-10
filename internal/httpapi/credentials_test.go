package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/client"
	"github.com/nishantdania/outpost/internal/credentials"
	"github.com/nishantdania/outpost/internal/image"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
	"github.com/nishantdania/outpost/internal/testutil"
)

type credentialSnapshotManager struct {
	testutil.FakeManager
	created []outpost.Outpost
	disk    string
}

func (m *credentialSnapshotManager) Create(ctx context.Context, a outpost.Outpost) error {
	m.created = append(m.created, a)
	return m.FakeManager.Create(ctx, a)
}
func (m *credentialSnapshotManager) OpenSnapshot(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.disk)), nil
}

func TestSnapshotCredentialsInheritedPinnedAndSecretFree(t *testing.T) {
	db := newTestStore(t)
	images, err := image.New(filepath.Join(t.TempDir(), "images"), db, nil)
	if err != nil {
		t.Fatal(err)
	}
	keys, state := t.TempDir(), t.TempDir()
	os.Chmod(keys, 0700)
	os.Chmod(state, 0700)
	secret := "synthetic-key-never-in-snapshot"
	if err := os.WriteFile(filepath.Join(keys, "development-key"), []byte(secret), 0600); err != nil {
		t.Fatal(err)
	}
	host, err := credentials.NewHost(keys, state)
	if err != nil {
		t.Fatal(err)
	}
	manager := &credentialSnapshotManager{disk: "synthetic prepared disk"}
	app := service.New(db, manager).WithImages(images).WithCredentials(host)
	input := outpost.CreateInput{Name: "template", ImageID: "default", VCPUs: 2, MemoryMiB: 1024, DiskGiB: 8}
	if _, err := app.Create(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Stop(t.Context(), "template"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(newRouter(app, "token"))
	defer server.Close()
	c, err := client.New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	profile := credentials.Profile{Credentials: []credentials.Binding{{Env: "PROVIDER_KEY", Secret: "development-key", Host: "api.example.com", Header: "X-Api-Key", Encoding: "raw"}}}
	saved, err := c.SnapshotOutpostWithCredentials(t.Context(), "template", "prepared", &profile)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CredentialProfile == nil || saved.CredentialProfile.JSON() != profile.JSON() {
		t.Fatal("image metadata lost profile")
	}
	for _, name := range []string{"fork-one", "fork-two"} {
		input.Name = name
		input.ImageID = "prepared"
		vm, err := app.Create(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := db.Get(t.Context(), name)
		if err != nil || loaded.CredentialConfig != profile.JSON() {
			t.Fatal("profile not pinned")
		}
		material := manager.created[len(manager.created)-1]
		if material.CredentialEnv != profile.Environment() || material.EgressCA != host.PublicCA {
			t.Fatal("public guest material missing")
		}
		if strings.Contains(material.CredentialEnv, secret) || strings.Contains(material.EgressCA, secret) || strings.Contains(vm.CredentialConfig, secret) {
			t.Fatal("host key entered guest spec")
		}
	}
	if manager.created[1].CredentialEnv != manager.created[2].CredentialEnv {
		t.Fatal("snapshot references were not preserved across forks")
	}
	// A resnapshot inherits without resending the profile. Use a different disk
	// payload to exercise an independent artifact and tag lifecycle.
	if _, err := app.Stop(t.Context(), "fork-one"); err != nil {
		t.Fatal(err)
	}
	manager.disk = "changed fork disk"
	inherited, err := c.SnapshotOutpost(t.Context(), "fork-one", "resnapshot")
	if err != nil || inherited.CredentialProfile == nil || inherited.CredentialProfile.JSON() != profile.JSON() {
		t.Fatalf("resnapshot: %v", err)
	}
	if _, err := app.Delete(t.Context(), "template"); err != nil {
		t.Fatal(err)
	}
	// Tag changes do not change profiles pinned on existing VMs.
	if err := db.PutImage(t.Context(), saved.Digest, saved.Size, "alias"); err != nil {
		t.Fatal(err)
	}
	before, _ := db.Get(t.Context(), "fork-two")
	if _, err := app.Stop(t.Context(), "fork-two"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Start(t.Context(), "fork-two"); err != nil {
		t.Fatal(err)
	}
	after, _ := db.Get(t.Context(), "fork-two")
	if before.CredentialConfig != after.CredentialConfig {
		t.Fatal("restart changed profile")
	}
	// Rotation changes only the host file and does not rewrite guest references.
	os.WriteFile(filepath.Join(keys, "development-key"), []byte("rotated-synthetic-key"), 0600)
	if after.CredentialConfig != profile.JSON() {
		t.Fatal("rotation changed reference")
	}
	// A server without an egress host must reject managed creation before creating a record.
	disabled := service.New(db, manager)
	input.Name = "disabled"
	if _, err := disabled.Create(t.Context(), input); err != service.ErrCredentialsUnavailable {
		t.Fatalf("disabled create: %v", err)
	}
	if _, err := db.Get(t.Context(), "disabled"); err != outpost.ErrNotFound {
		t.Fatal("disabled create persisted VM")
	}
	// Snapshot/profile payloads remain authenticated and bounded.
	for _, test := range []struct {
		body   string
		auth   bool
		status int
	}{
		{body: profile.JSON(), auth: false, status: 401},
		{body: `{"credentials":[],"secret_value":"not permitted"}`, auth: true, status: 400},
		{body: strings.Repeat("x", 65537), auth: true, status: 400},
	} {
		r := httptest.NewRequest(http.MethodPost, "/v1/outposts/fork-one/snapshot?tag=invalid", strings.NewReader(test.body))
		r.Header.Set("Content-Type", "application/json")
		if test.auth {
			r.Header.Set("Authorization", "Bearer token")
		}
		w := httptest.NewRecorder()
		newRouter(app, "token").ServeHTTP(w, r)
		if w.Code != test.status {
			t.Fatalf("status %d", w.Code)
		}
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("response leaked key")
		}
	}
}
