package main

import "testing"

func TestBindPortParsesListenAddress(t *testing.T) {
	for _, item := range []struct {
		addr string
		want int
	}{
		{"127.0.0.1:8080", 8080},
		{"0.0.0.0:9090", 9090},
		{":7070", 7070},
	} {
		got, err := bindPort(item.addr)
		if err != nil || got != item.want {
			t.Fatalf("bindPort(%q)=%d err=%v want=%d", item.addr, got, err, item.want)
		}
	}
}

func TestBindPortRejectsInvalidAddress(t *testing.T) {
	for _, addr := range []string{"127.0.0.1", "127.0.0.1:bad", "127.0.0.1:0"} {
		if _, err := bindPort(addr); err == nil {
			t.Fatalf("expected bindPort(%q) to fail", addr)
		}
	}
}
