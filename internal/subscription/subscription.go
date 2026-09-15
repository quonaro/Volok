// Package subscription renders the shared plain-text VLESS subscription.
package subscription

import (
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"

	"volok/internal/store"
)

// Options controls how VLESS links are rendered for a subscription.
type Options struct {
	// NoVision removes the flow=xtls-rprx-vision parameter.
	NoVision bool
	// XHTTP switches the transport from tcp to xhttp.
	XHTTP bool
}

// Build renders one direct VLESS link per line for enabled nodes.
func Build(cfg *store.Config) string {
	return BuildWith(cfg, Options{})
}

// BuildWith renders links with the given transport options.
func BuildWith(cfg *store.Config, opts Options) string {
	var b strings.Builder
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		link := n.URL
		if opts.NoVision || opts.XHTTP {
			link = transformLink(link, opts)
		}
		b.WriteString(link)
		b.WriteString("\n")
	}
	return b.String()
}

// transformLink modifies a VLESS URL based on the given options.
func transformLink(raw string, opts Options) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()

	if opts.NoVision {
		q.Del("flow")
	}

	if opts.XHTTP {
		q.Set("type", "xhttp")
		q.Set("path", "/")
		q.Del("headerType")
		// XHTTP is incompatible with Vision flow.
		q.Del("flow")
		// XHTTP inbound listens on PORT+1 on the VPS.
		if u.Port() != "" {
			if port, err := strconv.Atoi(u.Port()); err == nil {
				u.Host = u.Hostname() + ":" + strconv.Itoa(port+1)
			}
		}
	}

	u.RawQuery = q.Encode()
	return u.String()
}

// EncodeBase64 encodes a subscription body for clients that require it.
func EncodeBase64(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}
