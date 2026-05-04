package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	tcserver "github.com/nangman-infra/touch-connect/tc-server"
	"github.com/nangman-infra/touch-connect/tc-server/internal/config"
)

func main() {
	cfg, err := config.FromEnv()
	if err != nil {
		log.Fatal(err)
	}
	flags := flag.NewFlagSet("tc-server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.BindAddr, "bind", cfg.BindAddr, "listen address, also configurable with TC_SERVER_BIND_ADDR")
	flags.StringVar(&cfg.Storage, "storage", cfg.Storage, "storage backend: memory or sqlite")
	flags.StringVar(&cfg.SQLitePath, "sqlite-path", cfg.SQLitePath, "absolute SQLite path when storage=sqlite")
	flags.StringVar(&cfg.Settings.Version, "version", cfg.Settings.Version, "server version")
	flags.StringVar(&cfg.Settings.MinimumWorkerVersion, "minimum-worker-version", cfg.Settings.MinimumWorkerVersion, "minimum compatible worker version")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: tc-server [flags]")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "flags:")
		flags.PrintDefaults()
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "environment:")
		fmt.Fprintln(flags.Output(), "  TC_SERVER_BIND_ADDR, TC_SERVER_STORAGE, TC_SERVER_SQLITE_PATH")
		fmt.Fprintln(flags.Output(), "  TC_SERVER_VERSION, TC_SERVER_MIN_WORKER_VERSION")
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			return
		}
		log.Fatal(err)
	}
	cfg, err = cfg.Validated()
	if err != nil {
		log.Fatal(err)
	}
	server, err := newServer(cfg)
	if err != nil {
		log.Fatal(err)
	}
	reconcileCtx, stopReconcile := context.WithCancel(context.Background())
	defer stopReconcile()
	server.StartBackgroundReconcile(reconcileCtx, 0)
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
