package daemon

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type fakeServer struct {
	serveErr     error
	shutdownErr  error
	started      chan struct{}
	shutdownDone chan struct{}
	shutdownCall chan struct{}
}

func newFakeServer(serveErr, shutdownErr error) *fakeServer {
	return &fakeServer{
		serveErr:     serveErr,
		shutdownErr:  shutdownErr,
		started:      make(chan struct{}),
		shutdownDone: make(chan struct{}),
		shutdownCall: make(chan struct{}),
	}
}

func (s *fakeServer) ListenAndServe() error {
	close(s.started)
	<-s.shutdownDone
	return s.serveErr
}

func (s *fakeServer) Shutdown(context.Context) error {
	close(s.shutdownCall)
	close(s.shutdownDone)
	return s.shutdownErr
}

func TestBoundedError(t *testing.T) {
	value := boundedError(errors.New(string(make([]byte, 300)) + "final useful error"))
	if len(value) != 256 || value[len(value)-18:] != "final useful error" {
		t.Fatalf("diagnostic = %q", value)
	}
}

func TestRunRejectsEmptyTokenBeforeOpeningDatabase(t *testing.T) {
	err := run(context.Background(), Config{DatabasePath: "/path/that-must-not-be-opened/outpost.db"})
	if !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("run() error = %v, want %v", err, ErrTokenRequired)
	}
}

func TestRunServerReturnsServerError(t *testing.T) {
	want := errors.New("listen failed")
	server := newFakeServer(want, nil)
	close(server.shutdownDone)

	if err := runServer(context.Background(), server); !errors.Is(err, want) {
		t.Fatalf("runServer() error = %v, want %v", err, want)
	}
}

func TestRunServerShutsDownWhenContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeServer(http.ErrServerClosed, nil)
	result := make(chan error, 1)

	go func() {
		result <- runServer(ctx, server)
	}()

	<-server.started
	cancel()

	if err := <-result; err != nil {
		t.Fatalf("runServer() error = %v, want nil", err)
	}

	select {
	case <-server.shutdownCall:
	default:
		t.Fatal("Shutdown() was not called")
	}
}

func TestRunServerReturnsShutdownError(t *testing.T) {
	want := errors.New("shutdown failed")
	ctx, cancel := context.WithCancel(context.Background())
	server := newFakeServer(nil, want)
	result := make(chan error, 1)

	go func() {
		result <- runServer(ctx, server)
	}()

	<-server.started
	cancel()

	if err := <-result; !errors.Is(err, want) {
		t.Fatalf("runServer() error = %v, want %v", err, want)
	}
}
