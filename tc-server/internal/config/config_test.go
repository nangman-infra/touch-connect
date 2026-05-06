package config

import (
	"testing"
	"time"
)

func TestFromEnvSQLite(t *testing.T) {
	t.Setenv("TC_SERVER_BIND_ADDR", "0.0.0.0:9090")
	t.Setenv("TC_SERVER_STORAGE", "sqlite")
	t.Setenv("TC_SERVER_SQLITE_PATH", "/tmp/touch-connect.db")
	t.Setenv("TC_SERVER_VERSION", "test-version")
	t.Setenv("TC_SERVER_MIN_WORKER_VERSION", "0.2.0")
	t.Setenv("TC_SERVER_ENDPOINT_HEARTBEAT_TIMEOUT", "10s")
	t.Setenv("TC_SERVER_ATTEMPT_LEASE_DURATION", "7s")
	t.Setenv("TC_SERVER_MAX_REDELIVERY", "9")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}
	if cfg.BindAddr != "0.0.0.0:9090" || cfg.Storage != "sqlite" || cfg.SQLitePath != "/tmp/touch-connect.db" {
		t.Fatalf("unexpected storage config: %+v", cfg)
	}
	if cfg.Settings.Version != "test-version" || cfg.Settings.MinimumWorkerVersion != "0.2.0" {
		t.Fatalf("unexpected version settings: %+v", cfg.Settings)
	}
	if cfg.Settings.EndpointHeartbeatTimeout != 10*time.Second || cfg.Settings.AttemptLeaseDuration != 7*time.Second || cfg.Settings.MaxRedelivery != 9 {
		t.Fatalf("unexpected timing settings: %+v", cfg.Settings)
	}
}

func TestValidatedRejectsInvalidStorage(t *testing.T) {
	cases := []Config{
		{BindAddr: "127.0.0.1:8080", Storage: "bad"},
		{BindAddr: "127.0.0.1:8080", Storage: "sqlite"},
		{BindAddr: "127.0.0.1:8080", Storage: "sqlite", SQLitePath: "relative.db"},
	}
	for _, cfg := range cases {
		if _, err := cfg.Validated(); err == nil {
			t.Fatalf("expected invalid config to fail: %+v", cfg)
		}
	}
}

func TestFromEnvRejectsInvalidDurationsAndNumbers(t *testing.T) {
	t.Setenv("TC_SERVER_ENDPOINT_HEARTBEAT_TIMEOUT", "bad")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected invalid heartbeat timeout to fail")
	}

	t.Setenv("TC_SERVER_ENDPOINT_HEARTBEAT_TIMEOUT", "")
	t.Setenv("TC_SERVER_ATTEMPT_LEASE_DURATION", "bad")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected invalid lease duration to fail")
	}

	t.Setenv("TC_SERVER_ATTEMPT_LEASE_DURATION", "")
	t.Setenv("TC_SERVER_MAX_REDELIVERY", "bad")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected invalid redelivery count to fail")
	}
}
