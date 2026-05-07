package tcworker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDiscoverWorkerServerURLFindsHealthyTCServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("unexpected probe path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","component":"tc-server","version":"test"}`))
	}))
	defer server.Close()

	url, candidates := DiscoverWorkerServerURL(context.Background(), ServerDiscoveryOptions{
		CandidateURLs: []string{server.URL},
		Timeout:       time.Second,
	})
	if url != server.URL {
		t.Fatalf("expected discovered server %s, got %s candidates=%+v", server.URL, url, candidates)
	}
	if len(candidates) != 1 || !candidates[0].isReady() || candidates[0].Version != "test" {
		t.Fatalf("unexpected discovery candidates: %+v", candidates)
	}
}

func TestDiscoverWorkerServerURLRejectsNonTCServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","component":"not-touch-connect"}`))
	}))
	defer server.Close()

	url, candidates := DiscoverWorkerServerURL(context.Background(), ServerDiscoveryOptions{
		CandidateURLs: []string{server.URL},
		Timeout:       time.Second,
	})
	if url != "" {
		t.Fatalf("expected non-tc-server probe to be rejected, got %s", url)
	}
	if len(candidates) != 1 || !strings.Contains(candidates[0].Error, "not a tc-server") {
		t.Fatalf("expected rejection error, got %+v", candidates)
	}
}

func TestDiscoveryCandidateURLNormalization(t *testing.T) {
	candidates := discoveryCandidateURLs(context.Background(), ServerDiscoveryOptions{
		CandidateURLs: []string{
			" http://127.0.0.1:8080/ ",
			"http://127.0.0.1:8080",
			"ftp://127.0.0.1:8080",
			"not-a-url",
		},
	})
	if len(candidates) != 1 || candidates[0].url != DefaultWorkerServerURL {
		t.Fatalf("unexpected normalized candidates: %+v", candidates)
	}
}

func TestDiscoveryCandidateURLOrderPrefersMDNS(t *testing.T) {
	candidates := discoveryCandidateURLs(context.Background(), normalizedServerDiscoveryOptions(ServerDiscoveryOptions{
		MaxLANHosts: 1,
		MDNSLookup: func(context.Context, time.Duration) ([]string, error) {
			return []string{"http://192.168.10.34:8080"}, nil
		},
	}))
	if len(candidates) < 3 {
		t.Fatalf("expected mdns, loopback, and lan candidates, got %+v", candidates)
	}
	if candidates[0].url != "http://192.168.10.34:8080" || candidates[0].source != "mdns" {
		t.Fatalf("expected mdns candidate first, got %+v", candidates)
	}
}
