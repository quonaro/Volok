package httpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"volok/internal/store"
)

func setupServer(t *testing.T) (*httptest.Server, *store.Store, string, string) {
	t.Helper()
	s := store.Open(filepath.Join(t.TempDir(), "volok.json"))
	cfg, err := s.Init("https://vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(func(c *store.Config) error {
		c.Nodes = append(c.Nodes, store.Node{
			ID: "n1", Name: "Finland",
			URL:     "vless://uuid@203.0.113.10:44333?security=reality&type=tcp",
			Enabled: true,
		})
		c.Nodes = append(c.Nodes, store.Node{
			ID: "n2", Name: "Off",
			URL:     "vless://uuid2@198.51.100.20:44444?security=reality&type=tcp",
			Enabled: false,
		})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	user, err := store.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(func(c *store.Config) error {
		c.Users = append(c.Users, user)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(New(s).handler())
	t.Cleanup(ts.Close)
	return ts, s, cfg.Token, user
}

func (s *Server) handler() http.Handler {
	return s.http.Handler
}

// request performs an HTTP call and returns status, headers and body.
func request(t *testing.T, method, url string, body io.Reader, auth string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, data
}

func TestHealthzOpen(t *testing.T) {
	ts, s, _, _ := setupServer(t)
	status, _, body := request(t, "GET", ts.URL+"/healthz", nil, "")
	if status != 200 || string(body) != "ok\n" {
		t.Fatalf("healthz: status %d body %q", status, body)
	}
	_ = s
}

func TestSubscriptionAccess(t *testing.T) {
	ts, _, admin, user := setupServer(t)

	// User token via query works and only includes enabled nodes.
	status, headers, body := request(t, "GET", ts.URL+"/sub?token="+user, nil, "")
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	if !strings.Contains(string(body), "203.0.113.10") {
		t.Fatal("enabled node missing")
	}
	if strings.Contains(string(body), "198.51.100.20") {
		t.Fatal("disabled node must be excluded")
	}
	if headers.Get("Cache-Control") != "no-store" {
		t.Fatal("missing no-store header")
	}

	// Admin token must NOT read the subscription.
	status, _, _ = request(t, "GET", ts.URL+"/sub?token="+admin, nil, "")
	if status != 401 {
		t.Fatalf("admin reading sub: status %d, want 401", status)
	}

	// Missing token.
	status, _, _ = request(t, "GET", ts.URL+"/sub", nil, "")
	if status != 400 {
		t.Fatalf("no token: status %d, want 400", status)
	}

	// Bearer works too.
	status, _, body = request(t, "GET", ts.URL+"/sub", nil, user)
	if status != 200 || !strings.Contains(string(body), "203.0.113.10") {
		t.Fatalf("bearer sub failed: %d", status)
	}
}

func TestSubscriptionBase64(t *testing.T) {
	ts, _, _, user := setupServer(t)
	status, _, body := request(t, "GET", ts.URL+"/sub?token="+user+"&format=base64", nil, "")
	if status != 200 {
		t.Fatalf("status %d", status)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(body)))
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}
	if !strings.Contains(string(decoded), "vless") {
		t.Fatal("base64 subscription must decode to vless links")
	}
}

func TestRegisterAdminOnly(t *testing.T) {
	ts, _, admin, user := setupServer(t)

	status, _, _ := request(t, "GET", ts.URL+"/register?token="+user, nil, "")
	if status != 401 {
		t.Fatalf("user register: status %d, want 401", status)
	}

	status, _, body := request(t, "GET", ts.URL+"/register?token="+admin, nil, "")
	if status != 200 {
		t.Fatalf("admin register: status %d", status)
	}
	if !strings.Contains(string(body), "#!/usr/bin/env bash") {
		t.Fatal("installer script expected")
	}
	// The admin token is injected for the registration callback; the script
	// only receives it via env and Volok never persists it anywhere.
	if !strings.Contains(string(body), "VOLOK_TOKEN=") {
		t.Fatal("installer must receive the admin token via VOLOK_TOKEN env")
	}
}

func TestRegisterNodeFlow(t *testing.T) {
	ts, s, admin, user := setupServer(t)

	link := "vless://abc@203.0.113.77:55555?security=reality&type=tcp"
	payload := `{"name":"New","url":"` + link + `"}`

	// User cannot register.
	status, _, _ := request(t, "PUT", ts.URL+"/nodes/new1", bytes.NewBufferString(payload), user)
	if status != 401 {
		t.Fatalf("user PUT: status %d", status)
	}

	// Admin creates.
	status, _, body := request(t, "PUT", ts.URL+"/nodes/new1", bytes.NewBufferString(payload), admin)
	var out map[string]string
	_ = json.Unmarshal(body, &out)
	if status != 201 || out["result"] != "created" {
		t.Fatalf("create: status %d, out %v", status, out)
	}

	// Repeat is unchanged.
	status, _, body = request(t, "PUT", ts.URL+"/nodes/new1", bytes.NewBufferString(payload), admin)
	_ = json.Unmarshal(body, &out)
	if status != 200 || out["result"] != "unchanged" {
		t.Fatalf("repeat: status %d, out %v", status, out)
	}

	// Only one node stored.
	cfg, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(cfg.Nodes))
	}

	// Duplicate endpoint conflict.
	dup := `{"name":"Clone","url":"vless://def@203.0.113.77:55555?security=reality&type=tcp"}`
	status, _, _ = request(t, "PUT", ts.URL+"/nodes/other", bytes.NewBufferString(dup), admin)
	if status != 409 {
		t.Fatalf("duplicate endpoint: status %d, want 409", status)
	}

	// Malformed body.
	status, _, _ = request(t, "PUT", ts.URL+"/nodes/bad", bytes.NewBufferString(`{"name":"X","url":"not-a-url"}`), admin)
	if status != 400 {
		t.Fatalf("bad url: status %d, want 400", status)
	}
}

func TestUnknownFieldsRejectedInRegistration(t *testing.T) {
	ts, _, admin, _ := setupServer(t)
	payload := `{"name":"X","url":"vless://abc@203.0.113.9:443?security=reality","token":"steal"}`
	status, _, _ := request(t, "PUT", ts.URL+"/nodes/x", bytes.NewBufferString(payload), admin)
	if status != 400 {
		t.Fatalf("unknown field: status %d, want 400", status)
	}
}

func TestNotFound(t *testing.T) {
	ts, s, _, _ := setupServer(t)
	status, _, _ := request(t, "GET", ts.URL+"/", nil, "")
	if status != 404 {
		t.Fatalf("status %d, want 404", status)
	}
	_ = s
}
