package store

import (
	"crypto/ecdh"
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

// NewUUID generates a random UUID v4 string.
func NewUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// NewX25519Keys generates an X25519 key pair for REALITY.
// Returns (privateKeyHex, publicKeyHex, error).
func NewX25519Keys() (string, string, error) {
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generating x25519 key: %w", err)
	}
	return hex.EncodeToString(priv.Bytes()), hex.EncodeToString(priv.PublicKey().Bytes()), nil
}

// NewShortID generates a random 8-byte hex short ID for REALITY.
func NewShortID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating short id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// NewProxy generates a complete REALITY identity for the router proxy.
func NewProxy(port int, sni string) (*Proxy, error) {
	uuid, err := NewUUID()
	if err != nil {
		return nil, err
	}
	priv, pub, err := NewX25519Keys()
	if err != nil {
		return nil, err
	}
	sid, err := NewShortID()
	if err != nil {
		return nil, err
	}
	return &Proxy{
		UUID:       uuid,
		PrivateKey: priv,
		PublicKey:  pub,
		ShortID:    sid,
		Port:       port,
		SNI:        sni,
	}, nil
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
