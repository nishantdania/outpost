package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nishantdania/ark/internal/ark"
)

func main() {
	outputPath := flag.String("output", "", "schema output path")
	flag.Parse()

	store, err := ark.Open(context.Background(), ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	schema, err := store.Schema(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	if *outputPath == "" {
		fmt.Print(schema)
		return
	}

	if err := os.WriteFile(*outputPath, []byte(schema), 0o644); err != nil {
		log.Fatal(err)
	}
}
