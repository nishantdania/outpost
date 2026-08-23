package main

import (
	"log"

	"github.com/nishantdania/ark/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
