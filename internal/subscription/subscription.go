// Package subscription renders the shared plain-text VLESS subscription.
package subscription

import (
	"encoding/base64"
	"strings"

	"volok/internal/store"
	"volok/internal/vless"
)

// Build renders one direct VLESS link per line for enabled nodes. The stored
// node name is written into the URL fragment so clients display the library
// label rather than whatever fragment the original link carried.
func Build(cfg *store.Config) string {
	var b strings.Builder
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		b.WriteString(vless.WithName(n.URL, n.Name))
		b.WriteString("\n")
	}
	return b.String()
}

// EncodeBase64 encodes a subscription body for clients that require it.
func EncodeBase64(body string) string {
	return base64.StdEncoding.EncodeToString([]byte(body))
}
