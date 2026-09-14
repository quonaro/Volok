// Package vless parses and validates VLESS share URLs.
//
// URLs are preserved as-is as much as possible: unknown query parameters are
// not dropped, because Volok only stores and serves the exact link.
package vless

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ErrInvalid is returned when a VLESS URL fails validation.
var ErrInvalid = errors.New("invalid vless url")

// MaxURLLength bounds stored share links.
const MaxURLLength = 4096

// Parsed holds the parts of a VLESS URL that Volok itself interprets.
// Unknown query parameters remain inside Raw.
type Parsed struct {
	Host string
	Port int
	Raw  string
}

// Parse validates a VLESS share URL and extracts host and port.
func Parse(raw string) (Parsed, error) {
	if strings.ContainsAny(raw, "\r\n\x00") {
		return Parsed{}, fmt.Errorf("%w: control characters", ErrInvalid)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Parsed{}, fmt.Errorf("%w: empty url", ErrInvalid)
	}
	if len(raw) > MaxURLLength {
		return Parsed{}, fmt.Errorf("%w: url too long", ErrInvalid)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Parsed{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if u.Scheme != "vless" {
		return Parsed{}, fmt.Errorf("%w: scheme %q", ErrInvalid, u.Scheme)
	}
	if u.User == nil || u.User.Username() == "" {
		return Parsed{}, fmt.Errorf("%w: missing client id", ErrInvalid)
	}
	host := u.Hostname()
	if host == "" {
		return Parsed{}, fmt.Errorf("%w: missing host", ErrInvalid)
	}
	if ip := net.ParseIP(host); ip == nil && !validHostname(host) {
		return Parsed{}, fmt.Errorf("%w: invalid host %q", ErrInvalid, host)
	}

	port := 443
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return Parsed{}, fmt.Errorf("%w: invalid port %q", ErrInvalid, p)
		}
		port = n
	}

	return Parsed{Host: host, Port: port, Raw: raw}, nil
}

// WithName replaces the fragment (display name) of a VLESS URL, keeping
// everything else byte-for-byte intact. Returns the original URL if the
// replacement is empty.
func WithName(raw, name string) string {
	if name == "" {
		return raw
	}
	base, _, _ := strings.Cut(raw, "#")
	return base + "#" + url.PathEscape(name)
}

func validHostname(h string) bool {
	if len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(h, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if !isLabelChar(c) {
				return false
			}
			if (i == 0 || i == len(label)-1) && c == '-' {
				return false
			}
		}
	}
	return true
}

func isLabelChar(c byte) bool {
	return c == '-' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
