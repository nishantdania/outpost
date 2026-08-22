package outpost

import (
	"context"
	"testing"
)

func TestCreate(t *testing.T) {
	result, err := Create(context.Background())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Message != "Hello, World!" {
		t.Errorf("Create() message = %q, want %q", result.Message, "Hello, World!")
	}
}
