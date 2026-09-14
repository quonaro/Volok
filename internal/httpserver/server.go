// Package httpserver serves the Volok subscription, installer and
// registration endpoints over HTTP.
package httpserver

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"time"

	"volok/internal/store"
)

const maxBodyBytes = 16 << 10

// Server exposes the Volok JSON store over HTTP.
type Server struct {
	store *store.Store
	http  *http.Server
}

// New builds a Server for the given store.
func New(s *store.Store) *Server {
	srv := &Server{store: s}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.handleHealth)
	mux.HandleFunc("GET /sub", srv.handleSubscription)
	mux.HandleFunc("GET /register", srv.handleRegister)
	mux.HandleFunc("PUT /nodes/{id}", srv.handleRegisterNode)
	mux.HandleFunc("/", srv.handleNotFound)

	srv.http = &http.Server{
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return srv
}

// Serve accepts connections on ln until shutdown or error.
func (s *Server) Serve(ln net.Listener) error {
	err := s.http.Serve(ln)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	http.NotFound(w, nil)
}

// bearerToken extracts a token from the Authorization header.
func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// queryToken extracts a token from the query string.
func queryToken(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// pickToken returns the single token provided via header or query.
// Conflicting or repeated values are rejected.
func pickToken(r *http.Request) (string, bool) {
	fromHeader := bearerToken(r)
	fromQuery := queryToken(r)
	qs := r.URL.Query()
	hasQuery := qs.Has("token")

	if fromHeader != "" && hasQuery {
		if fromHeader != fromQuery {
			return "", false
		}
		return fromHeader, true
	}
	if fromHeader != "" {
		return fromHeader, true
	}
	if hasQuery {
		if len(qs["token"]) != 1 {
			return "", false
		}
		return fromQuery, true
	}
	return "", false
}

// isAdmin reports whether token matches the admin secret.
func isAdmin(cfg *store.Config, token string) bool {
	return subtle.ConstantTimeCompare([]byte(cfg.Token), []byte(token)) == 1
}

func noStoreHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
