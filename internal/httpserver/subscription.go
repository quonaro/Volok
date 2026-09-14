package httpserver

import (
	"log/slog"
	"net/http"

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

	body := subscription.Build(cfg)
	if r.URL.Query().Get("format") == "base64" {
		body = subscription.EncodeBase64(body)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	noStoreHeaders(w)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}

func writeError(w http.ResponseWriter, status int, message string) {
	noStoreHeaders(w)
	http.Error(w, message, status)
}
