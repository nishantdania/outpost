package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/nishantdania/ark/internal/cmd"
)

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if code := exitError.ExitCode(); code > 0 {
			return code
		}
		return 1
	}
	return 1
}
func run() int {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitCode(err)
	}
	return 0
}
func main() { os.Exit(run()) }
