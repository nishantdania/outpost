package cmd

import (
	"io"
	"strings"
	"testing"
)

func TestCommandSurface(t *testing.T) {
	for _, args := range [][]string{
		{"run"}, {"create"}, {"rm"}, {"delete"}, {"remove"},
		{"start"}, {"stop"}, {"list"}, {"inspect"}, {"snapshot"},
		{"image", "import"}, {"image", "rm"}, {"image", "remove"},
	} {
		root := newRootCmd()
		command, _, err := root.Find(args)
		if err != nil || command == root {
			t.Fatalf("missing command %v: %v", args, err)
		}
	}
}

func TestInvalidCommandArgumentsDoNotReachLauncher(t *testing.T) {
	for _, args := range [][]string{
		{"run"}, {"rm"}, {"snapshot", "dev"},
		{"image", "import", "disk.ext4"}, {"image", "list", "extra"}, {"image", "gc", "extra"},
	} {
		root := newRootCmd()
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.SetArgs(args)
		err := root.Execute()
		if err == nil || strings.Contains(err.Error(), "dial unix") {
			t.Fatalf("expected local validation for %v, got %v", args, err)
		}
	}
}
