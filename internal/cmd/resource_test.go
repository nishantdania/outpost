package cmd

import (
	"strconv"
	"testing"
)

func TestCreateSizeParsing(t *testing.T) {
	for _, test := range []struct {
		value string
		parse func(string) (int, error)
		want  int
	}{{"4G", parseMemoryMiB, 4096}, {"4GB", parseMemoryMiB, 4096}, {"4096M", parseMemoryMiB, 4096}, {"8G", parseDiskGiB, 8}, {"8GB", parseDiskGiB, 8}} {
		got, err := test.parse(test.value)
		if err != nil || got != test.want {
			t.Fatalf("parse %q = %d, %v; want %d, nil", test.value, got, err, test.want)
		}
	}
	if _, err := parseMemoryMiB(strconv.Itoa(int(^uint(0)>>1)/1024+1) + "G"); err == nil {
		t.Fatal("parseMemoryMiB() error = nil for overflow")
	}
}
