package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nangman-infra/touch-connect/tc-server/internal/application"
)

type Config struct {
	BindAddr   string
	Storage    string
	SQLitePath string
	Settings   application.Settings
}

func FromEnv() (Config, error) {
	settings := application.DefaultSettings()
	cfg := Config{
		BindAddr: getenv("TC_SERVER_BIND_ADDR", "127.0.0.1:8080"),
		Storage:  getenv("TC_SERVER_STORAGE", "memory"),
		Settings: settings,
	}
	cfg.SQLitePath = os.Getenv("TC_SERVER_SQLITE_PATH")
	cfg.Settings.Version = getenv("TC_SERVER_VERSION", settings.Version)
	cfg.Settings.MinimumWorkerVersion = getenv("TC_SERVER_MIN_WORKER_VERSION", settings.MinimumWorkerVersion)
	if value := os.Getenv("TC_SERVER_ENDPOINT_HEARTBEAT_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Settings.EndpointHeartbeatTimeout = parsed
	}
	if value := os.Getenv("TC_SERVER_ATTEMPT_LEASE_DURATION"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Settings.AttemptLeaseDuration = parsed
	}
	if value := os.Getenv("TC_SERVER_MAX_REDELIVERY"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Settings.MaxRedelivery = parsed
	}
	return cfg.Validated()
}

func (c Config) Validated() (Config, error) {
	if c.BindAddr == "" {
		return Config{}, errors.New("TC_SERVER_BIND_ADDR must not be empty")
	}
	switch c.Storage {
	case "memory":
	case "sqlite":
		if c.SQLitePath == "" || !filepath.IsAbs(c.SQLitePath) {
			return Config{}, errors.New("TC_SERVER_SQLITE_PATH must be an absolute path when TC_SERVER_STORAGE=sqlite")
		}
	default:
		return Config{}, errors.New("TC_SERVER_STORAGE must be memory or sqlite")
	}
	settings, err := c.Settings.Validated()
	if err != nil {
		return Config{}, err
	}
	c.Settings = settings
	return c, nil
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
