package application

import (
	"errors"
	"time"
)

const ContractVersion = "2026-05-03"

type Settings struct {
	Version                  string
	MinimumWorkerVersion     string
	EndpointHeartbeatTimeout time.Duration
	AttemptLeaseDuration     time.Duration
	MaxRedelivery            int
	Now                      func() time.Time
}

func DefaultSettings() Settings {
	return Settings{
		Version:                  "0.1.0-dev",
		MinimumWorkerVersion:     "0.1.0-dev",
		EndpointHeartbeatTimeout: 30 * time.Second,
		AttemptLeaseDuration:     2 * time.Minute,
		MaxRedelivery:            3,
		Now:                      time.Now,
	}
}

func (s Settings) Validated() (Settings, error) {
	if s.Version == "" || s.MinimumWorkerVersion == "" {
		return Settings{}, errors.New("server version settings are required")
	}
	if s.EndpointHeartbeatTimeout <= 0 {
		return Settings{}, errors.New("endpoint heartbeat timeout must be positive")
	}
	if s.AttemptLeaseDuration <= 0 {
		return Settings{}, errors.New("attempt lease duration must be positive")
	}
	if s.MaxRedelivery < 0 {
		return Settings{}, errors.New("max redelivery must not be negative")
	}
	if s.Now == nil {
		s.Now = time.Now
	}
	return s, nil
}
