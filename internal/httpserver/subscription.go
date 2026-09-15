package httpserver

import (
	"encoding/base64"
	"log/slog"
	"net/http"
	"strconv"

	"volok/internal/proxy"
	"volok/internal/store"
	"volok/internal/subscription"
)

// handleSubscription serves the shared VLESS subscription to reader tokens.
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

	opts := subOptions(r)
	body := buildBody(cfg, r, opts)
	if r.URL.Query().Get("format") == "base64" {
		body = subscription.EncodeBase64(body)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	setSubscriptionHeaders(w, cfg)
	noStoreHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

// buildBody renders the subscription body, switching to proxy relay links
// when ?proxy=true is requested and a proxy identity is configured.
func buildBody(cfg *store.Config, r *http.Request, opts subscription.Options) string {
	if r.URL.Query().Get("proxy") == "true" && cfg.Proxy != nil {
		return proxyBody(cfg, opts)
	}
	return subscription.BuildWith(cfg, opts)
}

// proxyBody renders VLESS links pointing at the router's proxy inbound
// instead of the direct VPS endpoints.
func proxyBody(cfg *store.Config, opts subscription.Options) string {
	var b []byte
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		link, err := proxy.ProxyLink(cfg, n, struct{ NoVision, XHTTP bool }{
			NoVision: opts.NoVision,
			XHTTP:    opts.XHTTP,
		})
		if err != nil {
			slog.Error("building proxy link", "node", n.ID, "error", err)
			continue
		}
		b = append(b, []byte(link)...)
		b = append(b, '\n')
	}
	return string(b)
}

// setSubscriptionHeaders adds metadata headers that mobile clients
// (v2rayNG, Happ, Throne) use to render the group title, auto-update
// interval and announcement.
func setSubscriptionHeaders(w http.ResponseWriter, cfg *store.Config) {
	title := base64.StdEncoding.EncodeToString([]byte("Volok"))
	w.Header().Set("Profile-Title", "base64:"+title)
	w.Header().Set("Profile-Update-Interval", strconv.Itoa(12))
	w.Header().Set("Profile-Web-Page-URL", cfg.PublicURL)
	announce := base64.StdEncoding.EncodeToString([]byte("Volok — personal VLESS node library"))
	w.Header().Set("Announce", "base64:"+announce)
}

func writeError(w http.ResponseWriter, status int, message string) {
	noStoreHeaders(w)
	http.Error(w, message, status)
}

// subOptions parses subscription transport options from query params.
//   - vision=false  removes flow=xtls-rprx-vision
//   - xhttp=true    switches transport to xhttp
func subOptions(r *http.Request) subscription.Options {
	q := r.URL.Query()
	return subscription.Options{
		NoVision: q.Get("vision") == "false",
		XHTTP:    q.Get("xhttp") == "true",
	}
}
