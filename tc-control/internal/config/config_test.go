package config

import (
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	t.Setenv("TC_CONTROL_BIND_ADDR", "0.0.0.0:9091")
	t.Setenv("TC_CONTROL_SERVER_URL", "http://tc-server:8080")
	t.Setenv("TC_CONTROL_VERSION", "test-version")
	t.Setenv("TC_CONTROL_TIMEOUT", "3s")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv returned error: %v", err)
	}
	if cfg.BindAddr != "0.0.0.0:9091" || cfg.ServerURL != "http://tc-server:8080" || cfg.Version != "test-version" || cfg.Timeout != 3*time.Second {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidatedRejectsInvalidValues(t *testing.T) {
	cases := []Config{
		{ServerURL: "http://tc-server:8080", Timeout: time.Second},
		{BindAddr: "127.0.0.1:8081", Timeout: time.Second},
		{BindAddr: "127.0.0.1:8081", ServerURL: "http://tc-server:8080"},
	}
	for _, cfg := range cases {
		if _, err := cfg.Validated(); err == nil {
			t.Fatalf("expected invalid config to fail: %+v", cfg)
		}
	}
}

func TestFromEnvRejectsInvalidTimeout(t *testing.T) {
	t.Setenv("TC_CONTROL_SERVER_URL", "http://tc-server:8080")
	t.Setenv("TC_CONTROL_TIMEOUT", "not-a-duration")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected invalid timeout to fail")
	}
}
