package main

import (
	"log"
	"net/http"

	"github.com/nishantdania/outpost/internal/config"
	"github.com/nishantdania/outpost/internal/daemon"
	"github.com/nishantdania/outpost/internal/outpost"
)

func main() {
	cfg, err := config.LoadDaemon()
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: daemon.New(outpost.Create),
	}

	log.Printf("outpostd listening on %s", cfg.ListenAddr)
	log.Fatal(server.ListenAndServe())
}
