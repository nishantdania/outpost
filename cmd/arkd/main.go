package main

import (
	"log"

	"github.com/nishantdania/ark/internal/daemon"
)

func main() {
	if err := daemon.Run(); err != nil {
		log.Fatal(err)
	}
}
