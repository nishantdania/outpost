package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nishantdania/outpost/internal/api"
	"github.com/nishantdania/outpost/internal/outpost"
	"github.com/nishantdania/outpost/internal/service"
	"github.com/nishantdania/outpost/internal/testutil"
)

func TestCreateOutpostAndListOutposts(t *testing.T) {
	store := newTestStore(t)
	handler := testHandler(t, store)

	createRec := createOutpost(t, handler, "demo")
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created api.Outpost
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Id == "" || created.Name != "demo" || created.ImageId != "default" || created.Vcpus != outpost.DefaultVCPUs || created.MemoryMib != outpost.DefaultMemoryMiB || created.DiskGib != outpost.DefaultDiskGiB || created.DesiredState != api.OutpostDesiredState(outpost.DesiredRunning) || created.Status != api.OutpostStatus(outpost.StatusRunning) || created.GuestIp != "172.30.0.2" || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created Outpost = %v, want running Outpost with resources and timestamps", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/outposts", nil)
	listRec := httptest.NewRecorder()
	handler.ListOutposts(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var outposts []api.Outpost
	if err := json.Unmarshal(listRec.Body.Bytes(), &outposts); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(outposts) != 1 || outposts[0] != created {
		t.Fatalf("listed Outposts = %v, want %v", outposts, created)
	}
}

func TestCreateOutpostRejectsDuplicateName(t *testing.T) {
	handler := testHandler(t, newTestStore(t))

	if rec := createOutpost(t, handler, "demo"); rec.Code != http.StatusCreated {
		t.Fatalf("first create status = %d, want %d", rec.Code, http.StatusCreated)
	}

	second := createOutpost(t, handler, "Demo")
	if second.Code != http.StatusConflict {
		t.Fatalf("second create status = %d, want %d", second.Code, http.StatusConflict)
	}

	var response api.Error
	if err := json.Unmarshal(second.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if response.Error != outpost.ErrNameTaken.Error() {
		t.Fatalf("error = %q, want %q", response.Error, outpost.ErrNameTaken)
	}
}

func TestCreateOutpostReturnsDistinctInvalidKeyBadRequest(t *testing.T) {
	handler := testHandler(t, newTestStore(t))
	for _, key := range []string{
		"ssh-ed25519 !!!",
		"ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		body, err := json.Marshal(map[string]any{"name": "demo", "image_id": "default", "vcpus": 2, "memory_mib": 4096, "disk_gib": 8, "ssh_public_key": key})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/outposts", strings.NewReader(string(body)))
		rec := httptest.NewRecorder()
		handler.CreateOutpost(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("key %q status = %d", key, rec.Code)
		}
		var response api.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error != outpost.ErrInvalidSSHPublicKey.Error() {
			t.Fatalf("error = %q", response.Error)
		}
	}
}

func TestCreateOutpostRejectsOutOfBoundsResources(t *testing.T) {
	handler := testHandler(t, newTestStore(t))
	req := httptest.NewRequest(http.MethodPost, "/v1/outposts", strings.NewReader(`{"name":"demo","image_id":"default","vcpus":33,"memory_mib":4096,"disk_gib":8}`))
	rec := httptest.NewRecorder()
	handler.CreateOutpost(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func createOutpost(t *testing.T, handler handler, name string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/v1/outposts", strings.NewReader(`{"name":"`+name+`","image_id":"default","vcpus":2,"memory_mib":4096,"disk_gib":8}`))
	rec := httptest.NewRecorder()
	handler.CreateOutpost(rec, req)

	return rec
}

func testHandler(t *testing.T, store *outpost.Store) handler {
	t.Helper()
	return handler{service: service.New(store, &testutil.FakeManager{StartFunc: func(context.Context, string) (string, error) { return "172.30.0.2", nil }})}
}

func newTestStore(t *testing.T) *outpost.Store {
	t.Helper()

	store, err := outpost.Open(context.Background(), filepath.Join(t.TempDir(), "outpost.db"))
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
