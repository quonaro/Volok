package utils

import (
	"crypto/sha256"
	"encoding/hex"
)

// GeneratePasswordFromTokenNode generates a deterministic password from
// tokenID and nodeID, used for mixed (SOCKS5/HTTP) inbound authentication.
func GeneratePasswordFromTokenNode(tokenID, nodeID string) string {
	if tokenID == "" || nodeID == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(tokenID))
	h.Write([]byte(nodeID))
	return hex.EncodeToString(h.Sum(nil))
}
