package tcworker

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nangman-infra/touch-connect/internal/localdiscovery"
)

const DefaultWorkerServerURL = "http://127.0.0.1:8080"

type ServerCandidate struct {
	URL       string
	Source    string
	Status    string
	Component string
	Version   string
	Latency   time.Duration
	Error     string
	index     int
}

type ServerDiscoveryOptions struct {
	CandidateURLs []string
	HTTPClient    *http.Client
	Timeout       time.Duration
	MDNSTimeout   time.Duration
	MaxLANHosts   int
	DisableMDNS   bool
	MDNSLookup    func(context.Context, time.Duration) ([]string, error)
}

func DiscoverWorkerServerURL(ctx context.Context, options ServerDiscoveryOptions) (string, []ServerCandidate) {
	candidates := DiscoverServerCandidates(ctx, options)
	for _, candidate := range candidates {
		if candidate.isReady() {
			return candidate.URL, candidates
		}
	}
	return "", candidates
}

func DiscoverServerCandidates(ctx context.Context, options ServerDiscoveryOptions) []ServerCandidate {
	options = normalizedServerDiscoveryOptions(options)
	urls := discoveryCandidateURLs(ctx, options)
	if len(urls) == 0 {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: serverProbeTimeout(options.Timeout)}
	}
	results := make([]ServerCandidate, 0, len(urls))
	resultCh := make(chan ServerCandidate, len(urls))
	guard := make(chan struct{}, 32)
	var wg sync.WaitGroup
	for index, item := range urls {
		item := item
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			select {
			case guard <- struct{}{}:
				defer func() { <-guard }()
			case <-probeCtx.Done():
				resultCh <- ServerCandidate{URL: item.url, Source: item.source, Error: probeCtx.Err().Error(), index: index}
				return
			}
			result := probeServerCandidate(probeCtx, client, item.url, item.source)
			result.index = index
			resultCh <- result
		}(index)
	}
	wg.Wait()
	close(resultCh)
	for result := range resultCh {
		results = append(results, result)
	}
	sort.SliceStable(results, func(i int, j int) bool {
		return results[i].index < results[j].index
	})
	return results
}

func normalizedServerDiscoveryOptions(options ServerDiscoveryOptions) ServerDiscoveryOptions {
	if options.Timeout == 0 {
		options.Timeout = 900 * time.Millisecond
	}
	if options.MaxLANHosts == 0 {
		options.MaxLANHosts = 254
	}
	if options.MDNSTimeout == 0 {
		options.MDNSTimeout = 600 * time.Millisecond
	}
	return options
}

type discoveryURL struct {
	url    string
	source string
}

func discoveryCandidateURLs(ctx context.Context, options ServerDiscoveryOptions) []discoveryURL {
	var candidates []discoveryURL
	if len(options.CandidateURLs) > 0 {
		for _, value := range options.CandidateURLs {
			if normalized := normalizeServerCandidateURL(value); normalized != "" {
				candidates = append(candidates, discoveryURL{url: normalized, source: "configured"})
			}
		}
		return dedupeDiscoveryURLs(candidates)
	}
	if !options.DisableMDNS {
		for _, value := range discoverMDNSServerURLs(ctx, options) {
			candidates = append(candidates, discoveryURL{url: value, source: "mdns"})
		}
	}
	candidates = append(candidates,
		discoveryURL{url: DefaultWorkerServerURL, source: "loopback"},
		discoveryURL{url: "http://localhost:8080", source: "loopback"},
	)
	candidates = append(candidates, localLANDiscoveryURLs(options.MaxLANHosts)...)
	return dedupeDiscoveryURLs(candidates)
}

func discoverMDNSServerURLs(ctx context.Context, options ServerDiscoveryOptions) []string {
	lookup := options.MDNSLookup
	if lookup == nil {
		lookup = defaultMDNSLookup
	}
	urls, err := lookup(ctx, options.MDNSTimeout)
	if err != nil {
		return nil
	}
	return urls
}

func defaultMDNSLookup(ctx context.Context, timeout time.Duration) ([]string, error) {
	services, err := localdiscovery.Browse(ctx, timeout)
	if err != nil {
		return nil, err
	}
	return localdiscovery.ServerURLsFromServices(services), nil
}

func localLANDiscoveryURLs(limit int) []discoveryURL {
	if limit <= 0 {
		return nil
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []discoveryURL
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipv4FromAddr(addr)
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			network := ip.Mask(net.CIDRMask(24, 32)).To4()
			if network == nil {
				continue
			}
			for host := byte(1); host < 255; host++ {
				candidate := net.IPv4(network[0], network[1], network[2], host)
				out = append(out, discoveryURL{url: "http://" + candidate.String() + ":8080", source: "lan"})
				if len(out) >= limit {
					return out
				}
			}
		}
	}
	return out
}

func ipv4FromAddr(addr net.Addr) net.IP {
	switch item := addr.(type) {
	case *net.IPNet:
		return item.IP.To4()
	case *net.IPAddr:
		return item.IP.To4()
	default:
		return nil
	}
}

func dedupeDiscoveryURLs(candidates []discoveryURL) []discoveryURL {
	seen := make(map[string]bool, len(candidates))
	out := make([]discoveryURL, 0, len(candidates))
	for _, candidate := range candidates {
		normalized := normalizeServerCandidateURL(candidate.url)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		candidate.url = normalized
		out = append(out, candidate)
	}
	return out
}

func normalizeServerCandidateURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func probeServerCandidate(ctx context.Context, client *http.Client, baseURL string, source string) ServerCandidate {
	started := time.Now()
	candidate := ServerCandidate{URL: baseURL, Source: source}
	healthURL := strings.TrimRight(baseURL, "/") + "/healthz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	resp, err := client.Do(req)
	candidate.Latency = time.Since(started)
	if err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		candidate.Error = resp.Status
		return candidate
	}
	var health struct {
		Status    string `json:"status"`
		Component string `json:"component"`
		Version   string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		candidate.Error = err.Error()
		return candidate
	}
	candidate.Status = health.Status
	candidate.Component = health.Component
	candidate.Version = health.Version
	if !candidate.isReady() {
		candidate.Error = errors.New("not a tc-server health endpoint").Error()
	}
	return candidate
}

func serverProbeTimeout(discoveryTimeout time.Duration) time.Duration {
	if discoveryTimeout <= 250*time.Millisecond {
		return discoveryTimeout
	}
	if discoveryTimeout < time.Second {
		return discoveryTimeout / 3
	}
	return 300 * time.Millisecond
}

func (c ServerCandidate) isReady() bool {
	return strings.EqualFold(c.Component, "tc-server") && strings.EqualFold(c.Status, "ok")
}
