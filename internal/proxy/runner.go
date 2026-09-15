// Package proxy builds sing-box configurations for the router relay mode.
// The router runs sing-box with a VLESS+REALITY inbound and one outbound
// per enabled node, so mobile clients connect to the router and traffic
// is relayed to the VPS nodes.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"

	singbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"

	"volok/internal/store"
	"volok/internal/vless"
)

const (
	defaultFlow        = "xtls-rprx-vision"
	defaultFingerprint = "chrome"
)

// Runner holds an in-process sing-box instance for the proxy relay.
type Runner struct {
	box *singbox.Box
}

// Start launches a sing-box proxy inside the current process.
// It blocks until ctx is canceled, then shuts down the box.
func (r *Runner) Start(ctx context.Context, cfg *store.Config) error {
	if cfg.Proxy == nil {
		return fmt.Errorf("proxy identity not set")
	}

	jsonConfig, err := BuildSingBoxConfig(cfg)
	if err != nil {
		return fmt.Errorf("building sing-box config: %w", err)
	}

	registryCtx := include.Context(ctx)
	var opts option.Options
	if err := singjson.UnmarshalContextDisallowUnknownFields(registryCtx, []byte(jsonConfig), &opts); err != nil {
		return fmt.Errorf("parsing sing-box config: %w", err)
	}

	instance, err := singbox.New(singbox.Options{
		Options: opts,
		Context: registryCtx,
	})
	if err != nil {
		return fmt.Errorf("creating sing-box instance: %w", err)
	}
	if err := instance.Start(); err != nil {
		return fmt.Errorf("starting sing-box: %w", err)
	}
	r.box = instance

	slog.Info("sing-box proxy started", "port", cfg.Proxy.Port, "sni", cfg.Proxy.SNI)

	<-ctx.Done()
	slog.Info("stopping sing-box proxy")
	return r.box.Close()
}

// singBoxConfig is the minimal sing-box JSON structure Volok generates.
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
}

type sbRealityIn struct {
	Enabled    bool        `json:"enabled"`
	Handshake  sbHandshake `json:"handshake"`
	PrivateKey string      `json:"private_key"`
	ShortID    []string    `json:"short_id"`
}

type sbHandshake struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
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
// outbound. The first outbound is the default route.
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
					Server:     p.SNI,
					ServerPort: 443,
				},
				PrivateKey: p.PrivateKey,
				ShortID:    []string{p.ShortID},
			},
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

	name := n.Name
	if u, err := url.Parse(n.URL); err == nil && u.Fragment != "" {
		name = u.Fragment
	}

	return fmt.Sprintf("vless://%s@%s:%s?%s#%s",
		p.UUID, host, strconv.Itoa(port), params.Encode(), url.PathEscape(name)), nil
}
