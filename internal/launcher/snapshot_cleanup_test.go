package launcher

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http/httptest"
	"strings"
	"testing"
)

type cleanupErrorDisk struct{ io.Reader }

func (cleanupErrorDisk) Close() error { return errors.New("private export removal failed") }

type cleanupErrorRuntime struct{ Runtime }

func (cleanupErrorRuntime) OpenSnapshot(context.Context, string) (io.ReadCloser, error) {
	return cleanupErrorDisk{strings.NewReader("disk")}, nil
}

func TestSnapshotServerReportsCleanupFailure(t *testing.T) {
	var output bytes.Buffer
	old := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(old)
	s := &Server{runtime: cleanupErrorRuntime{}}
	request := httptest.NewRequest("POST", "/v1/snapshot", strings.NewReader(`{"version":1,"id":"`+testSpec().ID+`"}`))
	response := httptest.NewRecorder()
	s.snapshot(response, request)
	if response.Body.String() != "disk" {
		t.Fatalf("response = %s", response.Body.String())
	}
	if !strings.Contains(output.String(), "private export removal failed") {
		t.Fatalf("cleanup failure not surfaced: %s", output.String())
	}
}
