package launcherclient

import (
	"testing"
	"time"
)

func TestLifecycleRequestTimeout(t *testing.T) {
	if lifecycleRequestTimeout != 5*time.Minute {
		t.Fatalf("lifecycleRequestTimeout = %s, want %s", lifecycleRequestTimeout, 5*time.Minute)
	}
}
