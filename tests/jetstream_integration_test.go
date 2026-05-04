//go:build integration && jetstream

package tests

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestJetStreamDevNATSIsEnabled(t *testing.T) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	monitorURL := strings.TrimRight(strings.TrimSpace(os.Getenv("NATS_MONITOR_URL")), "/")
	if natsURL == "" || monitorURL == "" {
		t.Skip("set NATS_URL and NATS_MONITOR_URL to run JetStream integration tests")
	}
	if !strings.HasPrefix(natsURL, "nats://") {
		t.Fatalf("NATS_URL must use nats://, got %q", natsURL)
	}

	client := http.Client{Timeout: 2 * time.Second}
	res, err := client.Get(monitorURL + "/jsz")
	if err != nil {
		t.Fatalf("query JetStream monitor endpoint: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected JetStream monitor status 200, got %d", res.StatusCode)
	}

	var jsz map[string]any
	if err := json.NewDecoder(res.Body).Decode(&jsz); err != nil {
		t.Fatalf("decode JetStream monitor response: %v", err)
	}
	if _, ok := jsz["config"]; !ok {
		t.Fatalf("expected JetStream monitor response to include config, got %+v", jsz)
	}
}
