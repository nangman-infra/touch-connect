package localdiscovery

import (
	"net"
	"reflect"
	"testing"

	"github.com/grandcat/zeroconf"
)

func TestServiceFromEntryPrefersAdvertisedURL(t *testing.T) {
	entry := &zeroconf.ServiceEntry{
		ServiceRecord: zeroconf.ServiceRecord{Instance: "touch-connect"},
		Port:          8080,
		Text:          []string{"component=tc-server", "url=http://192.168.10.34:8080/"},
		AddrIPv4:      []net.IP{net.ParseIP("192.168.10.99")},
	}
	service := ServiceFromEntry(entry)
	if !reflect.DeepEqual(service.URLs, []string{"http://192.168.10.34:8080"}) {
		t.Fatalf("expected advertised URL to win, got %+v", service)
	}
}

func TestServiceFromEntryBuildsURLFromAddresses(t *testing.T) {
	entry := &zeroconf.ServiceEntry{
		ServiceRecord: zeroconf.ServiceRecord{Instance: "touch-connect"},
		Port:          8080,
		Text:          []string{"component=tc-server"},
		AddrIPv4:      []net.IP{net.ParseIP("192.168.10.44")},
	}
	service := ServiceFromEntry(entry)
	if !reflect.DeepEqual(service.URLs, []string{"http://192.168.10.44:8080"}) {
		t.Fatalf("expected IPv4 URL, got %+v", service)
	}
}

func TestServerURLsFromServicesDedupesAndFilters(t *testing.T) {
	got := ServerURLsFromServices([]Service{
		{URLs: []string{"http://192.168.10.34:8080/", "ftp://192.168.10.34:8080"}},
		{URLs: []string{"http://192.168.10.34:8080", "http://192.168.10.44:8080"}},
	})
	want := []string{"http://192.168.10.34:8080", "http://192.168.10.44:8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected URLs got=%+v want=%+v", got, want)
	}
}
