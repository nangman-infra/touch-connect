package localdiscovery

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	ServiceType = "_touch-connect._tcp"
	Domain      = "local."
)

type AdvertiseOptions struct {
	Enabled      bool
	InstanceName string
	Port         int
	Version      string
	URL          string
}

type Service struct {
	Instance string
	HostName string
	Port     int
	URLs     []string
	Text     []string
}

func Advertise(ctx context.Context, options AdvertiseOptions) (func(), error) {
	if !options.Enabled {
		return func() {}, nil
	}
	options = normalizedAdvertiseOptions(options)
	if options.Port <= 0 {
		return nil, errors.New("touch-connect discovery advertise port must be positive")
	}
	text := []string{"component=tc-server"}
	if strings.TrimSpace(options.Version) != "" {
		text = append(text, "version="+strings.TrimSpace(options.Version))
	}
	if normalizedURL := normalizeHTTPURL(options.URL); normalizedURL != "" {
		text = append(text, "url="+normalizedURL)
	}
	server, err := zeroconf.Register(options.InstanceName, ServiceType, Domain, options.Port, text, nil)
	if err != nil {
		return nil, err
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			server.Shutdown()
		})
	}
	go func() {
		<-ctx.Done()
		stop()
	}()
	return stop, nil
}

func Browse(ctx context.Context, timeout time.Duration) ([]Service, error) {
	if timeout <= 0 {
		timeout = time.Second
	}
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}
	browseCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	entries := make(chan *zeroconf.ServiceEntry)
	errCh := make(chan error, 1)
	go func() {
		errCh <- resolver.Browse(browseCtx, ServiceType, Domain, entries)
	}()
	var services []Service
	for {
		select {
		case entry := <-entries:
			if entry != nil {
				services = append(services, ServiceFromEntry(entry))
			}
		case err := <-errCh:
			sortServices(services)
			return services, err
		case <-browseCtx.Done():
			sortServices(services)
			select {
			case err := <-errCh:
				return services, err
			default:
				return services, nil
			}
		}
	}
}

func ServiceFromEntry(entry *zeroconf.ServiceEntry) Service {
	if entry == nil {
		return Service{}
	}
	service := Service{
		Instance: entry.Instance,
		HostName: entry.HostName,
		Port:     entry.Port,
		Text:     append([]string(nil), entry.Text...),
	}
	if textURL := textValue(entry.Text, "url"); textURL != "" {
		if normalized := normalizeHTTPURL(textURL); normalized != "" {
			service.URLs = append(service.URLs, normalized)
			return service
		}
	}
	for _, ip := range entry.AddrIPv4 {
		if ip == nil {
			continue
		}
		service.URLs = append(service.URLs, "http://"+net.JoinHostPort(ip.String(), strconv.Itoa(entry.Port)))
	}
	for _, ip := range entry.AddrIPv6 {
		if ip == nil || ip.IsLinkLocalUnicast() {
			continue
		}
		service.URLs = append(service.URLs, "http://"+net.JoinHostPort(ip.String(), strconv.Itoa(entry.Port)))
	}
	if len(service.URLs) == 0 && strings.TrimSpace(entry.HostName) != "" {
		host := strings.TrimSuffix(entry.HostName, ".")
		service.URLs = append(service.URLs, "http://"+net.JoinHostPort(host, strconv.Itoa(entry.Port)))
	}
	return service
}

func ServerURLsFromServices(services []Service) []string {
	var urls []string
	seen := map[string]bool{}
	for _, service := range services {
		for _, candidate := range service.URLs {
			normalized := normalizeHTTPURL(candidate)
			if normalized == "" || seen[normalized] {
				continue
			}
			seen[normalized] = true
			urls = append(urls, normalized)
		}
	}
	return urls
}

func normalizedAdvertiseOptions(options AdvertiseOptions) AdvertiseOptions {
	if strings.TrimSpace(options.InstanceName) == "" {
		options.InstanceName = defaultInstanceName()
	}
	return options
}

func defaultInstanceName() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "touch-connect"
	}
	return "touch-connect-" + safeInstancePart(host)
}

func textValue(text []string, key string) string {
	prefix := key + "="
	for _, item := range text {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(item, prefix))
		}
	}
	return ""
}

func normalizeHTTPURL(value string) string {
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

func sortServices(services []Service) {
	sort.SliceStable(services, func(i int, j int) bool {
		return services[i].Instance < services[j].Instance
	})
}

func safeInstancePart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, item := range value {
		if item >= 'a' && item <= 'z' || item >= '0' && item <= '9' {
			builder.WriteRune(item)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "local"
	}
	return result
}
