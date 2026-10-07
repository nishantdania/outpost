package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nishantdania/outpost/internal/api"
)

func TestSnapshotClientTimeoutAndAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("snapshot lost authentication")
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	c, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	lifecycle := c.api.ClientInterface.(*api.Client).Client.(*http.Client)
	snapshot := c.snapshotAPI.ClientInterface.(*api.Client).Client.(*http.Client)
	if lifecycle.Timeout != lifecycleRequestTimeout || snapshot.Timeout != 0 {
		t.Fatalf("timeouts: lifecycle=%s snapshot=%s", lifecycle.Timeout, snapshot.Timeout)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.SnapshotOutpost(ctx, "vm", "tag"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("snapshot cancellation = %v", err)
	}
}
