// Package proxy builds sing-box configurations for the router relay mode.
// The router runs sing-box with a VLESS+REALITY inbound and one outbound
// per enabled node, so mobile clients connect to the router and traffic
// is relayed to the VPS nodes.
package proxy

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"volok/internal/store"
	"volok/internal/vless"
)

const (
	defaultFlow        = "xtls-rprx-vision"
	defaultFingerprint = "chrome"
)

// singBoxConfig is the minimal sing-box JSON structure Volok generates.
// Field names match sing-box's expected JSON keys.
type singBoxConfig struct {
	Log       sbLog        `json:"log"`
	Inbounds  []sbInbound  `json:"inbounds"`
	Outbounds []sbOutbound `json:"outbounds"`
	Route     sbRoute      `json:"route"`
}

type sbLog struct {
	Level string `json:"level"`
}

type sbInbound struct {
	Type       string       `json:"type"`
	Tag        string       `json:"tag"`
	Listen     string       `json:"listen"`
	ListenPort int          `json:"listen_port"`
	Users      []sbUser     `json:"users"`
	TLS        sbInboundTLS `json:"tls"`
	Transport  *sbTransport `json:"transport,omitempty"`
}

type sbUser struct {
	Name string `json:"name"`
	UUID string `json:"uuid"`
	Flow string `json:"flow,omitempty"`
}

type sbInboundTLS struct {
	Enabled    bool        `json:"enabled"`
	ServerName string      `json:"server_name"`
	Reality    sbRealityIn `json:"reality"`
	UTLS       sbUTLS      `json:"utls"`
}

type sbRealityIn struct {
	Enabled    bool        `json:"enabled"`
	Handshake  sbHandshake `json:"handshake"`
	PrivateKey string      `json:"private_key"`
	ShortID    []string    `json:"short_id"`
}

type sbHandshake struct {
	Server string `json:"server"`
	Port   int    `json:"port"`
}

type sbUTLS struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint"`
}

type sbTransport struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type sbOutbound struct {
	Type       string         `json:"type"`
	Tag        string         `json:"tag"`
	Server     string         `json:"server,omitempty"`
	ServerPort int            `json:"server_port,omitempty"`
	UUID       string         `json:"uuid,omitempty"`
	Flow       string         `json:"flow,omitempty"`
	TLS        *sbOutboundTLS `json:"tls,omitempty"`
	Transport  *sbTransport   `json:"transport,omitempty"`
}

type sbOutboundTLS struct {
	Enabled    bool         `json:"enabled"`
	ServerName string       `json:"server_name"`
	Reality    sbRealityOut `json:"reality"`
	UTLS       sbUTLS       `json:"utls"`
}

type sbRealityOut struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id"`
}

type sbRoute struct {
	Final string `json:"final,omitempty"`
}

// BuildSingBoxConfig renders a sing-box JSON config for the router proxy.
// The inbound is a VLESS+REALITY listener; each enabled node becomes an
// outbound. A round-robin route rule sends traffic to the first outbound.
func BuildSingBoxConfig(cfg *store.Config) (string, error) {
	if cfg.Proxy == nil {
		return "", fmt.Errorf("proxy identity not set")
	}
	p := cfg.Proxy

	inbound := sbInbound{
		Type:       "vless",
		Tag:        "in",
		Listen:     "0.0.0.0",
		ListenPort: p.Port,
		Users: []sbUser{
			{Name: "volok", UUID: p.UUID, Flow: defaultFlow},
			{Name: "volok-novision", UUID: p.UUID},
		},
		TLS: sbInboundTLS{
			Enabled:    true,
			ServerName: p.SNI,
			Reality: sbRealityIn{
				Enabled: true,
				Handshake: sbHandshake{
					Server: p.SNI,
					Port:   443,
				},
				PrivateKey: p.PrivateKey,
				ShortID:    []string{p.ShortID},
			},
			UTLS: sbUTLS{Enabled: true, Fingerprint: defaultFingerprint},
		},
	}

	outbounds := make([]sbOutbound, 0, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		ob, err := nodeToOutbound(n)
		if err != nil {
			return "", fmt.Errorf("node %s: %w", n.ID, err)
		}
		outbounds = append(outbounds, ob)
	}
	if len(outbounds) == 0 {
		return "", fmt.Errorf("no enabled nodes to relay")
	}

	// First outbound is the default; add a direct outbound for local traffic.
	outbounds = append([]sbOutbound{
		{Type: "direct", Tag: "direct"},
	}, outbounds...)

	config := singBoxConfig{
		Log:       sbLog{Level: "warn"},
		Inbounds:  []sbInbound{inbound},
		Outbounds: outbounds,
		Route:     sbRoute{Final: outbounds[1].Tag},
	}

	b, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding sing-box config: %w", err)
	}
	return string(b) + "\n", nil
}

// nodeToOutbound parses a VLESS URL and builds a sing-box outbound entry.
func nodeToOutbound(n store.Node) (sbOutbound, error) {
	p, err := vless.Parse(n.URL)
	if err != nil {
		return sbOutbound{}, err
	}
	u, err := url.Parse(n.URL)
	if err != nil {
		return sbOutbound{}, err
	}
	q := u.Query()

	ob := sbOutbound{
		Type:       "vless",
		Tag:        n.ID,
		Server:     p.Host,
		ServerPort: p.Port,
		UUID:       u.User.Username(),
		TLS: &sbOutboundTLS{
			Enabled:    true,
			ServerName: q.Get("sni"),
			Reality: sbRealityOut{
				Enabled:   true,
				PublicKey: q.Get("pbk"),
				ShortID:   q.Get("sid"),
			},
			UTLS: sbUTLS{Enabled: true, Fingerprint: q.Get("fp")},
		},
	}

	if flow := q.Get("flow"); flow != "" {
		ob.Flow = flow
	}

	if tp := q.Get("type"); tp == "xhttp" {
		ob.Transport = &sbTransport{Type: "xhttp", Path: q.Get("path")}
		ob.Flow = ""
	}

	return ob, nil
}

// ProxyLink builds a VLESS URL pointing at the router's proxy inbound.
// The display name is taken from the node so the client shows the same
// label as the direct link.
func ProxyLink(cfg *store.Config, n store.Node, opts struct{ NoVision, XHTTP bool }) (string, error) {
	if cfg.Proxy == nil {
		return "", fmt.Errorf("proxy identity not set")
	}
	p := cfg.Proxy

	// Determine the router's public host from public_url.
	origin, err := cfg.Canonical()
	if err != nil {
		return "", err
	}
	host := origin.Hostname()

	port := p.Port
	if opts.XHTTP {
		port++
	}

	params := url.Values{}
	params.Set("encryption", "none")
	params.Set("security", "reality")
	params.Set("fp", defaultFingerprint)
	params.Set("pbk", p.PublicKey)
	params.Set("sid", p.ShortID)
	params.Set("spx", "/")
	if !opts.NoVision && !opts.XHTTP {
		params.Set("flow", defaultFlow)
	}
	if opts.XHTTP {
		params.Set("type", "xhttp")
		params.Set("path", "/")
	} else {
		params.Set("type", "tcp")
		params.Set("headerType", "none")
	}
	params.Set("sni", p.SNI)

	// Preserve the display name from the node's direct link.
	name := n.Name
	if u, err := url.Parse(n.URL); err == nil && u.Fragment != "" {
		name = u.Fragment
	}

	return fmt.Sprintf("vless://%s@%s:%s?%s#%s",
		p.UUID, host, strconv.Itoa(port), params.Encode(), url.PathEscape(name)), nil
}
