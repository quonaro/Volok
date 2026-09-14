package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func randRead(b []byte) (int, error) {
	return rand.Read(b)
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// NewToken generates a 32-byte random secret as a 64-char hex string.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// parseListen validates an ip:port listen address.
func parseListen(addr string) (net.Addr, error) {
	if addr == "" {
		return nil, fmt.Errorf("listen address is empty")
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address %q: %w", addr, err)
	}
	if host == "" {
		return nil, fmt.Errorf("listen host is empty")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid listen port %q", portStr)
	}
	return &net.TCPAddr{}, nil
}

// CleanPublicURL normalizes a public origin. Production requires https;
// plain http is accepted only for loopback addresses so the installer can be
// exercised locally in an isolated test environment.
func CleanPublicURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("public_url must be an absolute URL")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopback(u.Hostname())) {
		return "", fmt.Errorf("public_url must use https (http allowed only on loopback for testing)")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("public_url must be a plain origin without path, query or userinfo")
	}
	return u.Scheme + "://" + u.Host, nil
}

func isLoopback(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}
