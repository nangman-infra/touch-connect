package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/nangman-infra/touch-connect/internal/localdiscovery"
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
	flags.BoolVar(&cfg.DiscoveryEnabled, "discovery", cfg.DiscoveryEnabled, "advertise tc-server on the local network with mDNS/Bonjour")
	flags.StringVar(&cfg.DiscoveryName, "discovery-name", cfg.DiscoveryName, "mDNS/Bonjour instance name")
	flags.StringVar(&cfg.AdvertiseURL, "advertise-url", cfg.AdvertiseURL, "reachable server URL published in mDNS TXT records")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "usage: tc-server [flags]")
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "flags:")
		flags.PrintDefaults()
		fmt.Fprintln(flags.Output(), "")
		fmt.Fprintln(flags.Output(), "environment:")
		fmt.Fprintln(flags.Output(), "  TC_SERVER_BIND_ADDR, TC_SERVER_STORAGE, TC_SERVER_SQLITE_PATH")
		fmt.Fprintln(flags.Output(), "  TC_SERVER_VERSION, TC_SERVER_MIN_WORKER_VERSION")
		fmt.Fprintln(flags.Output(), "  TC_SERVER_DISCOVERY, TC_SERVER_DISCOVERY_NAME, TC_SERVER_ADVERTISE_URL")
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
	stopDiscovery := startLocalDiscovery(reconcileCtx, cfg)
	defer stopDiscovery()
	log.Printf("tc-server listening on %s version=%s storage=%s", cfg.BindAddr, cfg.Settings.Version, cfg.Storage)
	if err := http.ListenAndServe(cfg.BindAddr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func startLocalDiscovery(ctx context.Context, cfg config.Config) func() {
	if !cfg.DiscoveryEnabled {
		return func() {}
	}
	port, err := bindPort(cfg.BindAddr)
	if err != nil {
		log.Printf("tc-server mDNS discovery disabled: %v", err)
		return func() {}
	}
	stop, err := localdiscovery.Advertise(ctx, localdiscovery.AdvertiseOptions{
		Enabled:      true,
		InstanceName: cfg.DiscoveryName,
		Port:         port,
		Version:      cfg.Settings.Version,
		URL:          cfg.AdvertiseURL,
	})
	if err != nil {
		log.Printf("tc-server mDNS discovery disabled: %v", err)
		return func() {}
	}
	log.Printf("tc-server mDNS discovery advertising service=%s.%s name=%s port=%d url=%s", localdiscovery.ServiceType, localdiscovery.Domain, cfg.DiscoveryName, port, cfg.AdvertiseURL)
	return stop
}

func bindPort(bindAddr string) (int, error) {
	host, portText, err := net.SplitHostPort(bindAddr)
	if err != nil {
		if strings.Count(bindAddr, ":") == 1 {
			parts := strings.Split(bindAddr, ":")
			host = parts[0]
			portText = parts[1]
		} else {
			return 0, fmt.Errorf("parse bind address %q: %w", bindAddr, err)
		}
	}
	_ = host
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 {
		return 0, fmt.Errorf("bind address %q has invalid port", bindAddr)
	}
	return port, nil
}

func newServer(cfg config.Config) (*tcserver.Server, error) {
	if cfg.Storage == "sqlite" {
		return tcserver.NewSQLiteServer(cfg.SQLitePath, cfg.Settings)
	}
	return tcserver.NewInMemoryServerWithSettings(cfg.Settings)
}
