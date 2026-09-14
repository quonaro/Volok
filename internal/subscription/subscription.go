// Package subscription renders the shared plain-text VLESS subscription.
package subscription

import (
	"encoding/base64"
	"strings"

	"volok/internal/store"
)

// Build renders one direct VLESS link per line for enabled nodes.
func Build(cfg *store.Config) string {
	var b strings.Builder
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		b.WriteString(n.URL)
		b.WriteString("\n")
	}
	return b.String()
}

// EncodeBase64 encodes a subscription body for clients that require it.
func EncodeBase64(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}
