package main

import (
	"log"
	"net/http"

	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	"github.com/nangman-infra/touch-connect/tc-server/internal/config"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	server, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("tc-server listening on %s version=%s storage=%s", cfg.BindAddr, cfg.Settings.Version, cfg.Storage)
	if err := http.ListenAndServe(cfg.BindAddr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func newServer(cfg config.Config) (*tcserver.Server, error) {
	if cfg.Storage == "sqlite" {
		return tcserver.NewSQLiteServer(cfg.SQLitePath, cfg.Settings)
	}
	return tcserver.NewInMemoryServerWithSettings(cfg.Settings)
}
