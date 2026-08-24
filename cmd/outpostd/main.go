package main

import (
	"log"
	"os"

	"github.com/nishantdania/outpost/internal/daemon"
)

func main() {
	if err := daemon.Execute(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
