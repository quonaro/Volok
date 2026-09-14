package vless

import (
	"strings"
	"testing"
)

const exampleHost = "example.com"

func TestParseValid(t *testing.T) {
	cases := []struct {
		name string
		url  string
		host string
		port int
	}{
		{"ipv4", "vless://uuid@203.0.113.10:44333?security=reality&type=tcp", "203.0.113.10", 44333},
		{"dns", "vless://uuid@" + exampleHost + ":443?security=reality", exampleHost, 443},
		{"default port", "vless://uuid@" + exampleHost + "?security=reality", exampleHost, 443},
		{"ipv6", "vless://uuid@[2001:db8::1]:44333?security=reality", "2001:db8::1", 44333},
		{"fragment", "vless://uuid@" + exampleHost + ":443?security=reality#My Node", exampleHost, 443},
		{"unknown params", "vless://uuid@" + exampleHost + ":443?security=reality&pqv=abc&foo=bar", exampleHost, 443},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse(tc.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Host != tc.host || p.Port != tc.port {
				t.Fatalf("got %s:%d, want %s:%d", p.Host, p.Port, tc.host, tc.port)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{
		"",
		"ss://uuid@example.com",
		"vless://@example.com:443",
		"vless://uuid@:443",
		"vless://uuid@exam ple.com:443",
		"vless://uuid@example.com:0",
		"vless://uuid@example.com:70000",
		"vless://uuid@example.com:443\r\n",
		strings.Repeat("a", MaxURLLength+1) + "://x",
	}
	for _, tc := range cases {
		if _, err := Parse(tc); err == nil {
			t.Fatalf("expected error for %q", tc)
		}
	}
}

func TestWithName(t *testing.T) {
	base := "vless://uuid@example.com:443?security=reality"
	got := WithName(base, "Finland Node")
	want := base + "#Finland%20Node"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if WithName(base, "") != base {
		t.Fatal("empty name must keep original url")
	}
}
