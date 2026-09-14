// Package installer renders the VPS installer script served by Volok.
package installer

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed node.sh
var nodeScript []byte

const (
	placeholderPublicURL = "__VOLOK_PUBLIC_URL__"
	placeholderToken     = "__VOLOK_TOKEN__"
)

// RenderScript returns the installer script with the Volok origin and admin
// token injected. Values are shell-single-quote escaped before substitution.
func RenderScript(publicURL, token string) ([]byte, error) {
	if publicURL == "" || token == "" {
		return nil, fmt.Errorf("public_url and token are required")
	}
	out := string(nodeScript)
	if !strings.Contains(out, placeholderPublicURL) || !strings.Contains(out, placeholderToken) {
		return nil, fmt.Errorf("installer template is missing placeholders")
	}
	out = strings.ReplaceAll(out, placeholderPublicURL, shellQuote(publicURL))
	out = strings.ReplaceAll(out, placeholderToken, shellQuote(token))
	return []byte(out), nil
}

// shellQuote escapes a value for embedding inside single quotes.
func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
