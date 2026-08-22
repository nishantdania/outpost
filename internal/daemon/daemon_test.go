package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nishantdania/outpost/internal/outpost"
)

func TestCreateOutpost(t *testing.T) {
	handler := New(func(context.Context, string, outpost.Resources) (outpost.Record, error) {
		return outpost.Record{ID: "id", Name: "name", Status: "created"}, nil
	}, func(context.Context) ([]outpost.Record, error) { return nil, nil }, nil, nil, nil, "v", nil, nil, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/outposts", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var record outpost.Record
	if err := json.NewDecoder(response.Body).Decode(&record); err != nil {
		t.Fatal(err)
	}
	if record.Name != "name" {
		t.Fatalf("record = %#v", record)
	}
}
