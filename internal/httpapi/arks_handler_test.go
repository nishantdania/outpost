package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/ark/internal/api"
	"github.com/nishantdania/ark/internal/ark"
	"github.com/nishantdania/ark/internal/service"
	"github.com/nishantdania/ark/internal/vmapi"
)

func TestCreateArkAndListArks(t *testing.T) {
	store := newTestStore(t)
	handler := testHandler(t, store)

	createRec := createArk(t, handler, "demo")
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created api.Ark
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Id == "" || created.Name != "demo" || created.ImageId != "default" || created.Vcpus != ark.DefaultVCPUs || created.MemoryMib != ark.DefaultMemoryMiB || created.DiskGib != ark.DefaultDiskGiB || created.DesiredState != api.ArkDesiredState(ark.DesiredRunning) || created.Status != api.ArkStatus(ark.StatusRunning) || created.GuestIp != "172.30.0.2" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created Ark = %v, want running Ark with resources and timestamps", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/arks", nil)
	listRec := httptest.NewRecorder()
	handler.ListArks(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var arks []api.Ark
	if err := json.Unmarshal(listRec.Body.Bytes(), &arks); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(arks) != 1 || arks[0] != created {
		t.Fatalf("listed Arks = %v, want %v", arks, created)
	}
}

func TestCreateArkRejectsDuplicateName(t *testing.T) {
	handler := testHandler(t, newTestStore(t))

	if rec := createArk(t, handler, "demo"); rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want %d", rec.Code, http.StatusCreated)
	}

	second := createArk(t, handler, "Demo")
	if second.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, want %d", second.Code, http.StatusConflict)
	}

	var response api.Error
	if err := json.Unmarshal(second.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error != ark.ErrNameTaken.Error() {
		t.Fatalf("error = %q, want %q", response.Error, ark.ErrNameTaken)
	}
}

func TestCreateArkReturnsDistinctInvalidKeyBadRequest(t *testing.T) {
	handler := testHandler(t, newTestStore(t))
	for _, key := range []string{
		"ssh-ed25519 !!!",
		"ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		body, err := json.Marshal(map[string]any{"name": "demo", "image_id": "default", "vcpus": 2, "memory_mib": 4096, "disk_gib": 8, "ssh_public_key": key})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/arks", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		handler.CreateArk(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("key %q status = %d", key, rec.Code)
		}
		var response api.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error != ark.ErrInvalidSSHPublicKey.Error() {
			t.Fatalf("error = %q", response.Error)
		}
	}
}

func TestCreateArkRejectsOutOfBoundsResources(t *testing.T) {
	handler := testHandler(t, newTestStore(t))
	req := httptest.NewRequest(http.MethodPost, "/v1/arks", strings.NewReader(`{"name":"demo","image_id":"default","vcpus":33,"memory_mib":4096,"disk_gib":8}`))
	rec := httptest.NewRecorder()
	handler.CreateArk(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func createArk(t *testing.T, handler handler, name string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/arks", strings.NewReader(`{"name":"`+name+`","image_id":"default","vcpus":2,"memory_mib":4096,"disk_gib":8}`))
	rec := httptest.NewRecorder()
	handler.CreateArk(rec, req)

	return rec
}

func testHandler(t *testing.T, store *ark.Store) handler {
	t.Helper()
	return handler{service: service.New(store, &vmapi.FakeManager{StartFunc: func(context.Context, string) (string, error) { return "172.30.0.2", nil }})}
}

func newTestStore(t *testing.T) *ark.Store {
	t.Helper()

	store, err := ark.Open(context.Background(), filepath.Join(t.TempDir(), "ark.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	return store
}
