package domain

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

const (
	// InboundTypeVLESS is the VLESS REALITY inbound type.
	InboundTypeVLESS = "vless"
	// InboundTypeMixed is the SOCKS5+HTTP mixed proxy inbound type.
	InboundTypeMixed = "mixed"
)

// CountryInfo holds the ISO country code, full name, derived flag emoji,
// and optional lookup/retry state used by the country watcher.
type CountryInfo struct {
	CountryCode  string
	CountryName  string
	Flag         string
	LastLookupAt *time.Time
	NextRetryAt  *time.Time
	Attempts     int
	LastError    string
}

// Node represents a proxy endpoint (exit VLESS server) managed by Outless.
type Node struct {
	ID          string
	URL         string
	GroupIDs    []string
	Country     string
	CountryInfo *CountryInfo
	IsSelf      bool
	ExpiresAt   *time.Time
}

// Token describes subscription access token metadata.
// UUID is the per-token identifier used as a VLESS user id on the hub inbound.
type Token struct {
	ID              string
	Owner           string
	GroupID         string
	GroupIDs        []string
	UUID            string
	AccessURL       string
	IsActive        bool
	QuotaBytes      *int64
	QuotaPeriod     string
	UsedBytes       int64
	LastConnectedAt time.Time
	ExpiresAt       time.Time
	CreatedAt       time.Time
}

// TokenIPRestriction defines an allow or deny rule for a token by IP.
type TokenIPRestriction struct {
	TokenID string
	IP      string
	Mode    string // "allow" or "block"
}

// TokenUsage aggregates per-token traffic for a specific period.
type TokenUsage struct {
	TokenID       string
	PeriodStart   time.Time
	PeriodType    string
	UploadBytes   int64
	DownloadBytes int64
	UpdatedAt     time.Time
}

// Group represents a collection of nodes and tokens for access control.
type Group struct {
	ID            string
	Name          string
	InboundID     string
	TotalNodes    int
	RandomEnabled bool
	RandomLimit   *int
	ShowOrigins   bool
	CreatedAt     time.Time
}

// PublicSource represents an external source of VLESS nodes.
type PublicSource struct {
	ID            string
	URL           string
	GroupID       string
	LastFetchedAt *time.Time
	CreatedAt     time.Time
}

// Inbound represents an entry point managed by Outless.
// Type "vless" uses VLESS REALITY; Type "mixed" uses SOCKS5+HTTP proxy.
type Inbound struct {
	ID           string
	Name         string
	Type         string
	Address      string
	Port         int
	SNI          string
	Handshake    string
	PublicKey    string
	PrivateKey   string
	ShortID      string
	Fingerprint  string
	NameTemplate string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Admin represents an administrative user with access to management endpoints.
type Admin struct {
	ID           string
	Username     string
	PasswordHash string
	TOTPSecret   string
	TOTPEnabled  bool
	CreatedAt    time.Time
}

// GenerateGroupID creates a unique group ID.
func GenerateGroupID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating group id: %w", err)
	}
	return fmt.Sprintf("grp_%d_%x", time.Now().UTC().Unix(), buf), nil
}

// GeneratePublicSourceID creates a unique public source ID.
func GeneratePublicSourceID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generating public source id: %w", err)
	}
	return fmt.Sprintf("pubsrc_%d_%x", time.Now().UTC().Unix(), buf), nil
}

// NormalizeCountryCode uppercases a two-letter ISO 3166-1 alpha-2 code.
func NormalizeCountryCode(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 2 {
		return s
	}
	u := strings.ToUpper(s)
	if u[0] >= 'A' && u[0] <= 'Z' && u[1] >= 'A' && u[1] <= 'Z' {
		return u
	}
	return s
}
