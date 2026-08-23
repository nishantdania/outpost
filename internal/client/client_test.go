package client

import (
	"testing"
	"time"
)

func TestLifecycleRequestTimeout(t *testing.T) {
	if lifecycleRequestTimeout < 2*time.Minute {
		t.Fatalf("lifecycleRequestTimeout = %s, want at least two minutes", lifecycleRequestTimeout)
	}
}
