// Package geo resolves the country of a VLESS endpoint from its IP and
// builds a display name matching the node.sh installer format:
// "<flag> <Country>#<4 digits>".
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// timeout matches the --max-time 10 used by node.sh.
const timeout = 10 * time.Second

// client is the package-level HTTP client with a fixed timeout.
var client = &http.Client{Timeout: timeout}

// Result holds the resolved country code and English name.
type Result struct {
	Code string // ISO 3166-1 alpha-2, e.g. "DE"
	Name string // English country name, e.g. "Germany"
}

// Detect resolves the country of an IP via free geo APIs. It tries
// ipwho.is first, then falls back to ip-api.com.
func Detect(ctx context.Context, ip string) (Result, error) {
	if r, err := detectIPWhoIs(ctx, ip); err == nil {
		return r, nil
	}
	return detectIPAPI(ctx, ip)
}

// detectIPWhoIs queries https://ipwho.is/{ip}.
func detectIPWhoIs(ctx context.Context, ip string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipwho.is/"+ip, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("ipwho.is: status %d", resp.StatusCode)
	}
	var body struct {
		Success     bool   `json:"success"`
		CountryCode string `json:"country_code"`
		Country     string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, err
	}
	if !body.Success || body.CountryCode == "" {
		return Result{}, fmt.Errorf("ipwho.is: no result")
	}
	return Result{Code: body.CountryCode, Name: body.Country}, nil
}

// detectIPAPI queries http://ip-api.com/json/{ip}.
func detectIPAPI(ctx context.Context, ip string) (Result, error) {
	u := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode,country", ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("ip-api.com: status %d", resp.StatusCode)
	}
	var body struct {
		Status      string `json:"status"`
		CountryCode string `json:"countryCode"`
		Country     string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, err
	}
	if body.Status != "success" || body.CountryCode == "" {
		return Result{}, fmt.Errorf("ip-api.com: no result")
	}
	return Result{Code: body.CountryCode, Name: body.Country}, nil
}

// FlagFromCode converts a 2-letter ISO country code to a flag emoji
// using regional indicator symbols. Returns "" for invalid codes.
func FlagFromCode(code string) string {
	if len(code) != 2 {
		return ""
	}
	code = strings.ToUpper(code)
	var b strings.Builder
	const base = 0x1F1E6 // 'A' regional indicator
	for i := 0; i < 2; i++ {
		c := code[i]
		if c < 'A' || c > 'Z' {
			return ""
		}
		b.WriteRune(rune(base + int(c-'A')))
	}
	return b.String()
}

// Suffix extracts the trailing #NNNN number from a node name.
func Suffix(name string) (int, bool) {
	i := strings.LastIndex(name, "#")
	if i < 0 || i == len(name)-1 {
		return 0, false
	}
	v, err := strconv.Atoi(name[i+1:])
	if err != nil {
		return 0, false
	}
	return v, true
}

// SuffixSet collects the #NNNN suffixes present in names.
func SuffixSet(names []string) map[int]bool {
	used := make(map[int]bool, len(names))
	for _, n := range names {
		if v, ok := Suffix(n); ok {
			used[v] = true
		}
	}
	return used
}

// UniqueSuffix returns a random 4-digit suffix absent from used. When every
// suffix is taken (practically impossible: 9000 options) it falls back to a
// plain random one.
func UniqueSuffix(used map[int]bool) int {
	for i := 0; i < 100; i++ {
		s := rand.Intn(9000) + 1000
		if !used[s] {
			return s
		}
	}
	for s := 1000; s <= 9999; s++ {
		if !used[s] {
			return s
		}
	}
	return rand.Intn(9000) + 1000
}

// EnsureUniqueSuffix rewrites the numeric #suffix of name when it collides
// with an entry in used. Names without a numeric suffix pass through.
func EnsureUniqueSuffix(name string, used map[int]bool) string {
	i := strings.LastIndex(name, "#")
	if i < 0 {
		return name
	}
	v, err := strconv.Atoi(name[i+1:])
	if err != nil || !used[v] {
		return name
	}
	return name[:i+1] + strconv.Itoa(UniqueSuffix(used))
}

// IsCanonical reports whether name is already in the "<flag> <Country>#<NNNN>"
// format: it must contain a regional indicator symbol and end with a 4-digit
// numeric suffix.
func IsCanonical(name string) bool {
	if _, ok := Suffix(name); !ok {
		return false
	}
	for _, r := range name {
		if r >= 0x1F1E6 && r <= 0x1F1FF {
			return true
		}
	}
	return false
}

// NodeName builds the display name: "<flag> <Country>#<suffix>".
// Falls back to "VPS#<suffix>" when the country is unknown.
func NodeName(r Result, suffix int) string {
	if r.Code == "" || r.Name == "" {
		return fmt.Sprintf("VPS#%d", suffix)
	}
	flag := FlagFromCode(r.Code)
	if flag == "" {
		return fmt.Sprintf("%s#%d", r.Name, suffix)
	}
	return fmt.Sprintf("%s %s#%d", flag, r.Name, suffix)
}
