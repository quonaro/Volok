package store

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"volok/internal/vless"
)

// SchemaVersion is the current structure version of volok.json.
const SchemaVersion = 1

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Config is the single JSON document managed by Volok.
type Config struct {
	Token         string   `json:"token"`
	Users         []string `json:"users"`
	SchemaVersion int      `json:"schema_version"`
	Listen        string   `json:"listen"`
	PublicURL     string   `json:"public_url"`
	Nodes         []Node   `json:"nodes"`
	Proxy         *Proxy   `json:"proxy,omitempty"`
}

// Proxy holds the REALITY identity for the router's sing-box inbound.
// When present, the subscription can return relay links via ?proxy=true.
type Proxy struct {
	UUID       string `json:"uuid"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	ShortID    string `json:"short_id"`
	Port       int    `json:"port"`
	SNI        string `json:"sni"`
}

// Validate checks the proxy identity fields.
func (p *Proxy) Validate() error {
	if p.UUID == "" {
		return fmt.Errorf("uuid is empty")
	}
	if p.PrivateKey == "" || p.PublicKey == "" {
		return fmt.Errorf("keys are empty")
	}
	if p.ShortID == "" {
		return fmt.Errorf("short_id is empty")
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port must be 1..65535")
	}
	if p.SNI == "" {
		return fmt.Errorf("sni is empty")
	}
	return nil
}

// Node is one direct VLESS endpoint stored in the library.
type Node struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// GenerateID creates a random 16-byte hex node identifier.
func GenerateID() (string, error) {
	b := make([]byte, 16)
	if _, err := randRead(b); err != nil {
		return "", fmt.Errorf("generating node id: %w", err)
	}
	return hexEncode(b), nil
}

// Validate checks the whole document after parsing and before writing.
func (c *Config) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema_version %d", c.SchemaVersion)
	}
	if !hex64.MatchString(c.Token) {
		return fmt.Errorf("token must be a 64-char hex string")
	}
	if _, err := parseListen(c.Listen); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	if _, err := CleanPublicURL(c.PublicURL); err != nil {
		return fmt.Errorf("public_url: %w", err)
	}

	seen := map[string]bool{c.Token: true}
	for _, token := range c.Users {
		if !hex64.MatchString(token) {
			return fmt.Errorf("user token must be a 64-char hex string")
		}
		if seen[token] {
			return fmt.Errorf("duplicate token in users")
		}
		seen[token] = true
	}

	ids := map[string]bool{}
	endpoints := map[string]bool{}
	for i := range c.Nodes {
		n := &c.Nodes[i]
		if n.ID == "" || ids[n.ID] {
			return fmt.Errorf("node id must be non-empty and unique")
		}
		ids[n.ID] = true
		if n.Name == "" || len(n.Name) > 128 {
			return fmt.Errorf("node %s: name must be 1..128 chars", n.ID)
		}
		if strings.ContainsAny(n.Name, "\r\n\x00") {
			return fmt.Errorf("node %s: name contains control characters", n.ID)
		}
		p, err := vless.Parse(n.URL)
		if err != nil {
			return fmt.Errorf("node %s: %w", n.ID, err)
		}
		endpoint := fmt.Sprintf("%s:%d", p.Host, p.Port)
		if endpoints[endpoint] {
			return fmt.Errorf("node %s: duplicate endpoint %s", n.ID, endpoint)
		}
		endpoints[endpoint] = true
	}

	if c.Proxy != nil {
		if err := c.Proxy.Validate(); err != nil {
			return fmt.Errorf("proxy: %w", err)
		}
	}
	return nil
}

// HasUser reports whether token matches one of the reader tokens.
func (c *Config) HasUser(token string) bool {
	for _, u := range c.Users {
		if u == token {
			return true
		}
	}
	return false
}

// Canonical returns the parsed public origin for building absolute URLs.
func (c *Config) Canonical() (*url.URL, error) {
	return url.Parse(c.PublicURL)
}
