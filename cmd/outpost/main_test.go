package main

import (
	"errors"
	"os/exec"
	"testing"
)

func TestExitCode(t *testing.T) {
	if exitCode(nil) != 0 || exitCode(errors.New("failure")) != 1 {
		t.Fatal("unexpected ordinary exit code")
	}
	err := exec.Command("sh", "-c", "exit 37").Run()
	if exitCode(err) != 37 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	err = exec.Command("sh", "-c", "kill -TERM $$").Run()
	if exitCode(err) < 0 {
		t.Fatalf("negative exit code = %d", exitCode(err))
	}
}
