package cmd

import "testing"

func TestRootCommandUsesServerEnvironment(t *testing.T) {
	t.Setenv("OUTPOST_SERVER", "https://handoff.example")
	root := newRootCmd()
	if got, _ := root.PersistentFlags().GetString("server"); got != "https://handoff.example" {
		t.Fatalf("server = %q", got)
	}
	if err := root.PersistentFlags().Set("server", "https://override.example"); err != nil {
		t.Fatal(err)
	}
	if got, _ := root.PersistentFlags().GetString("server"); got != "https://override.example" {
		t.Fatalf("server override = %q", got)
	}
}

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
