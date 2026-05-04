package config

import (
	"errors"
	"os"
	"time"
)

type Config struct {
	ControlURL       string
	Timeout          time.Duration
	JSON             bool
	Version          string
	ExpectedContract string
}

func Default() Config {
	return Config{
		ControlURL:       getenv("TCCTL_CONTROL_URL", "http://127.0.0.1:8081"),
		Timeout:          5 * time.Second,
		Version:          "0.1.0-dev",
		ExpectedContract: getenv("TCCTL_CONTRACT_VERSION", "2026-05-03"),
	}
}

func FromEnv() (Config, error) {
	cfg := Default()
	if value := os.Getenv("TCCTL_TIMEOUT"); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, err
		}
		cfg.Timeout = parsed
	}
	return cfg.Validated()
}

func (c Config) Validated() (Config, error) {
	if c.ControlURL == "" {
		return Config{}, errors.New("control URL must not be empty")
	}
	if c.Timeout <= 0 {
		return Config{}, errors.New("timeout must be positive")
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
