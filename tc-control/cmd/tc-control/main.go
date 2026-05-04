package main

import (
	"log"
	"net/http"

	tccontrol "github.com/nangman-infra/touch-connect/tc-control"
	"github.com/nangman-infra/touch-connect/tc-control/internal/config"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	control, err := tccontrol.New(cfg.ServerURL, &http.Client{Timeout: cfg.Timeout}, cfg.Version)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tc-control listening on %s server=%s version=%s", cfg.BindAddr, cfg.ServerURL, cfg.Version)
	if err := http.ListenAndServe(cfg.BindAddr, control.Handler()); err != nil {
		log.Fatal(err)
	}
}
