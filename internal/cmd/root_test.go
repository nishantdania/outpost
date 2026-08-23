package cmd

import "testing"

func TestRootCommandHasListCommand(t *testing.T) {
	root := newRootCmd()

	list, args, err := root.Find([]string{"list"})
	if err != nil {
		t.Fatalf("find list command: %v", err)
	}

	if len(args) != 0 {
		t.Fatalf("args = %v, want none", args)
	}

	if list.Name() != "list" {
		t.Fatalf("command name = %q, want %q", list.Name(), "list")
	}
}
