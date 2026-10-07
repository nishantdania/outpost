package launcherclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type snapshotTransport func(*http.Request) (*http.Response, error)

func (f snapshotTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSnapshotIgnoresLifecycleTimeoutButHonorsContext(t *testing.T) {
	c := New("unused")
	c.http.Timeout = time.Millisecond
	c.http.Transport = snapshotTransport(func(r *http.Request) (*http.Response, error) {
		select {
		case <-time.After(20 * time.Millisecond):
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/octet-stream"}}, ContentLength: 4, Body: io.NopCloser(strings.NewReader("disk"))}, nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	disk, err := c.OpenSnapshot(t.Context(), "vm")
	if err != nil {
		t.Fatal(err)
	}
	disk.Close()
	if c.http.Timeout != time.Millisecond {
		t.Fatal("shared lifecycle timeout changed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := c.OpenSnapshot(ctx, "vm"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation = %v", err)
	}
}
