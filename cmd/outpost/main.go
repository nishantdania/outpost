package main

import (
	"context"
	"os"

	"github.com/nishantdania/outpost/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], version, os.Stdout, os.Stderr))
}
