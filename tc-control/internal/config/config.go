package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	BindAddr  string
	ServerURL string
	Version   string
	Timeout   time.Duration
}

func FromEnv() (Config, error) {
	cfg := Config{
		BindAddr:  getenv("TC_CONTROL_BIND_ADDR", "127.0.0.1:8081"),
		ServerURL: os.Getenv("TC_CONTROL_SERVER_URL"),
		Version:   getenv("TC_CONTROL_VERSION", "0.1.0-dev"),
		Timeout:   5 * time.Second,
	}
	if value := os.Getenv("TC_CONTROL_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Timeout = parsed
	}
	return cfg.Validated()
}

func (c Config) Validated() (Config, error) {
	if c.BindAddr == "" {
		return Config{}, errors.New("TC_CONTROL_BIND_ADDR must not be empty")
	}
	if c.ServerURL == "" {
		return Config{}, errors.New("TC_CONTROL_SERVER_URL is required")
	}
	if c.Timeout <= 0 {
		return Config{}, errors.New("TC_CONTROL_TIMEOUT must be positive")
	}
	return c, nil
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
