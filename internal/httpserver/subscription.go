package httpserver

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"volok/internal/proxy"
	"volok/internal/store"
	"volok/internal/subscription"
	"volok/internal/vless"
)

const strTrue = "true"

// handleSubscription serves the shared VLESS subscription to reader tokens.
// Query params:
//   - proxy=true    relay links through the router instead of direct VPS links
//   - all=true      every enabled node as both a direct and a relay link
//   - format=base64 base64-encode the body for clients that require it
func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	token, ok := pickToken(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid authorization")
		return
	}

	cfg, err := s.store.Read()
	if err != nil {
		slog.Error("reading store", "error", err)
		writeError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}
	if !cfg.HasUser(token) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body, title string
	switch {
	case r.URL.Query().Get("all") == strTrue:
		body = allBody(cfg)
		title = "Volok · All"
	case r.URL.Query().Get("proxy") == strTrue && cfg.Proxy != nil:
		body = proxyBody(cfg)
		title = "Volok · Relay"
	default:
		body = subscription.Build(cfg)
		title = "Volok"
	}
	if r.URL.Query().Get("format") == "base64" {
		body = subscription.EncodeBase64(body)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	setSubscriptionHeaders(w, cfg, title)
	noStoreHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// proxyBody renders VLESS links pointing at the router's proxy inbound
// instead of the direct VPS endpoints.
func proxyBody(cfg *store.Config) string {
	var b strings.Builder
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		link, err := proxy.ProxyLink(cfg, n)
		if err != nil {
			slog.Error("building proxy link", "node", n.ID, "error", err)
			continue
		}
		b.WriteString(link)
		b.WriteString("\n")
	}
	return b.String()
}

// allBody renders every enabled node twice: once as a direct link and once
// as a relay link through the router proxy (when configured). Relay links
// get a " - PROXY" name suffix so clients can tell them apart.
func allBody(cfg *store.Config) string {
	var b strings.Builder
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		b.WriteString(vless.WithName(n.URL, n.Name))
		b.WriteString("\n")
		if cfg.Proxy == nil {
			continue
		}
		link, err := proxy.ProxyLink(cfg, n)
		if err != nil {
			slog.Error("building proxy link", "node", n.ID, "error", err)
			continue
		}
		b.WriteString(suffixLink(link, " - PROXY"))
		b.WriteString("\n")
	}
	return b.String()
}

// suffixLink appends a suffix to the URL fragment name.
func suffixLink(raw, suffix string) string {
	base, frag, ok := strings.Cut(raw, "#")
	if !ok {
		return raw
	}
	name, err := url.PathUnescape(frag)
	if err != nil {
		return raw
	}
	return base + "#" + url.PathEscape(name+suffix)
}

// setSubscriptionHeaders adds metadata headers that mobile clients
// (v2rayNG, Happ, Throne) use to render the group title, auto-update
// interval and announcement. The title varies per subscription variant so
// adding several subscription URLs yields distinct client-side groups.
func setSubscriptionHeaders(w http.ResponseWriter, cfg *store.Config, title string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(title))
	w.Header().Set("Profile-Title", "base64:"+encoded)
	w.Header().Set("Profile-Update-Interval", strconv.Itoa(12))
	w.Header().Set("Profile-Web-Page-URL", cfg.PublicURL)
	announce := base64.StdEncoding.EncodeToString([]byte("Volok — personal VLESS node library"))
	w.Header().Set("Announce", "base64:"+announce)
}

func writeError(w http.ResponseWriter, status int, message string) {
	noStoreHeaders(w)
	http.Error(w, message, status)
}
