package cli

import "testing"

func TestCopyArgs(t *testing.T) {
	source, target, mode, err := copyArgs([]string{"--mode", "640", "file", "dev:/file"})
	if err != nil {
		t.Fatal(err)
	}
	if source != "file" || target != "dev:/file" || mode != 0o640 {
		t.Fatalf("got %q %q %o", source, target, mode)
	}
}

func TestCopyArgsDefaultsToPrivate(t *testing.T) {
	_, _, mode, err := copyArgs([]string{"-", "dev:/file"})
	if err != nil {
		t.Fatal(err)
	}
	if mode != 0o600 {
		t.Fatalf("mode = %o", mode)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a'b"); got != `'a'"'"'b'` {
		t.Fatalf("quote = %q", got)
	}
}
