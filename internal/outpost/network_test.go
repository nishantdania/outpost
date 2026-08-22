package outpost

import (
	"fmt"
	"testing"
)

func TestNetworkIndexUsesFirstAvailableTap(t *testing.T) {
	index, err := networkIndex([]Record{{Tap: "outpost-tap0"}, {Tap: "outpost-tap2"}})
	if err != nil {
		t.Fatal(err)
	}
	if index != 1 {
		t.Fatalf("index = %d", index)
	}
}

func TestNetworkIndexCapacity(t *testing.T) {
	records := make([]Record, 16)
	for index := range records {
		records[index].Tap = fmt.Sprintf("outpost-tap%d", index)
	}
	if _, err := networkIndex(records); err == nil {
		t.Fatal("expected capacity error")
	}
}
