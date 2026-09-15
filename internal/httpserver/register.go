package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"volok/internal/geo"
	"volok/internal/installer"
	"volok/internal/store"
	"volok/internal/vless"
)

// handleRegister serves the VPS installer script to the admin token.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
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
	if !isAdmin(cfg, token) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	script, err := installer.RenderScript(cfg.PublicURL, cfg.Token)
	if err != nil {
		slog.Error("rendering installer", "error", err)
		writeError(w, http.StatusInternalServerError, "installer unavailable")
		return
	}
	noStoreHeaders(w)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(script)
}

type registerNodeRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type registerNodeResponse struct {
	ID     string `json:"id"`
	Result string `json:"result"`
}

// handleRegisterNode idempotently registers a node after installation.
func (s *Server) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	token, ok := pickToken(r)
	if !ok || bearerToken(r) == "" {
		writeError(w, http.StatusBadRequest, "registration requires a Bearer token")
		return
	}
	cfg, err := s.store.Read()
	if err != nil {
		slog.Error("reading store", "error", err)
		writeError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}
	if !isAdmin(cfg, token) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "body too large")
		return
	}
	req, err := decodeRegisterBody(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}
	if _, err := vless.Parse(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, "invalid vless url")
		return
	}

	result := "created"
	_, err = s.store.Update(func(c *store.Config) error {
		n := findNode(c, id)
		if n == nil {
			names := make([]string, 0, len(c.Nodes))
			for _, o := range c.Nodes {
				names = append(names, o.Name)
			}
			name := geo.EnsureUniqueSuffix(req.Name, geo.SuffixSet(names))
			c.Nodes = append(c.Nodes, store.Node{ID: id, Name: name, URL: req.URL, Enabled: true})
			return nil
		}
		if n.URL != req.URL {
			n.URL = req.URL
			result = "updated"
		} else {
			result = "unchanged"
		}
		return nil
	})
	if err != nil {
		var dupErr interface{ Error() string }
		if errors.As(err, &dupErr) && strings.Contains(err.Error(), "duplicate endpoint") {
			writeError(w, http.StatusConflict, "node conflicts with another entry")
			return
		}
		slog.Error("registering node", "id", id, "error", err)
		writeError(w, http.StatusServiceUnavailable, "store unavailable")
		return
	}

	noStoreHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	if result == "created" {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(registerNodeResponse{ID: id, Result: result})
}

func decodeRegisterBody(data []byte) (*registerNodeRequest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var req registerNodeRequest
	if err := dec.Decode(&req); err != nil {
		return nil, fmt.Errorf("invalid body: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid body: trailing data")
	}
	if req.Name == "" || len(req.Name) > 128 {
		return nil, fmt.Errorf("name must be 1..128 chars")
	}
	if strings.ContainsAny(req.Name, "\r\n\x00") {
		return nil, fmt.Errorf("name contains control characters")
	}
	if req.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	return &req, nil
}

func findNode(c *store.Config, id string) *store.Node {
	for i := range c.Nodes {
		if c.Nodes[i].ID == id {
			return &c.Nodes[i]
		}
	}
	return nil
}
