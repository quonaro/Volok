package geo

import (
	"context"
	"strings"
	"testing"
)

const germany = "Germany"

func TestFlagFromCode(t *testing.T) {
	cases := []struct {
		code string
		want string
	}{
		{"DE", "\U0001F1E9\U0001F1EA"}, // Germany
		{"FI", "\U0001F1EB\U0001F1EE"}, // Finland
		{"US", "\U0001F1FA\U0001F1F8"}, // United States
		{"de", "\U0001F1E9\U0001F1EA"}, // lowercase normalized
		{"", ""},
		{"D", ""},
		{"DEU", ""},
		{"D1", ""},
	}
	for _, tc := range cases {
		got := FlagFromCode(tc.code)
		if got != tc.want {
			t.Errorf("FlagFromCode(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestNodeName(t *testing.T) {
	name := NodeName(Result{Code: "DE", Name: germany}, 1234)
	if name != "\U0001F1E9\U0001F1EA Germany#1234" {
		t.Fatalf("unexpected name: %q", name)
	}

	// Empty result falls back to VPS#<suffix>.
	fallback := NodeName(Result{}, 5678)
	if fallback != "VPS#5678" {
		t.Fatalf("expected VPS fallback, got %q", fallback)
	}

	// Invalid code but valid name: name only, no flag.
	noFlag := NodeName(Result{Code: "12", Name: "Unknown"}, 9999)
	if noFlag != "Unknown#9999" {
		t.Fatalf("expected name without flag, got %q", noFlag)
	}
}

func TestSuffix(t *testing.T) {
	if v, ok := Suffix("\U0001F1E9\U0001F1EA Germany#1234"); !ok || v != 1234 {
		t.Fatalf("got %d %v", v, ok)
	}
	for _, name := range []string{"Germany#", germany, "Germany#abc", ""} {
		if _, ok := Suffix(name); ok {
			t.Fatalf("expected no suffix for %q", name)
		}
	}
}

func TestSuffixSet(t *testing.T) {
	used := SuffixSet([]string{"A#1111", "B#2222", "C", "D#xyz"})
	if !used[1111] || !used[2222] || len(used) != 2 {
		t.Fatalf("unexpected set: %v", used)
	}
}

func TestUniqueSuffix(t *testing.T) {
	used := map[int]bool{1234: true}
	for i := 0; i < 50; i++ {
		s := UniqueSuffix(used)
		if s == 1234 {
			t.Fatal("returned a used suffix")
		}
		if s < 1000 || s > 9999 {
			t.Fatalf("suffix %d out of range", s)
		}
	}
	// Nil map works too.
	if s := UniqueSuffix(nil); s < 1000 || s > 9999 {
		t.Fatalf("suffix %d out of range", s)
	}
}

func TestEnsureUniqueSuffix(t *testing.T) {
	used := map[int]bool{1234: true}

	got := EnsureUniqueSuffix("Germany#1234", used)
	if !strings.HasPrefix(got, "Germany#") || got == "Germany#1234" {
		t.Fatalf("collision not resolved: %q", got)
	}

	// Unique suffix passes through.
	if got := EnsureUniqueSuffix("Germany#5555", used); got != "Germany#5555" {
		t.Fatalf("changed unique name: %q", got)
	}

	// No numeric suffix passes through.
	if got := EnsureUniqueSuffix(germany, used); got != germany {
		t.Fatalf("changed suffixless name: %q", got)
	}
	if got := EnsureUniqueSuffix("Germany#x", used); got != "Germany#x" {
		t.Fatalf("changed non-numeric suffix: %q", got)
	}
}

func TestIsCanonical(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"\U0001F1E9\U0001F1EA Germany#1234", true},
		{"\U0001F1EB\U0001F1EE Finland#4329", true},
		{"Finland", false},
		{"Germany#1234", false},                 // no flag
		{"\U0001F1E9\U0001F1EA Germany", false}, // no numeric suffix
		{"\U0001F1E9\U0001F1EA Germany#abc", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsCanonical(tc.name); got != tc.want {
			t.Errorf("IsCanonical(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDetectIPWhoIs(t *testing.T) {
	// Detect hardcodes the public API URL; skip when offline.
	r, err := detectIPWhoIs(context.Background(), "203.0.113.10")
	if err != nil {
		t.Skipf("ipwho.is unreachable: %v", err)
	}
	if r.Code != "DE" || r.Name != germany {
		t.Fatalf("got %+v", r)
	}
}

func TestDetectIPAPI(t *testing.T) {
	r, err := detectIPAPI(context.Background(), "203.0.113.10")
	if err != nil {
		t.Skipf("ip-api.com unreachable: %v", err)
	}
	if r.Code == "" || r.Name == "" {
		t.Fatalf("got empty result: %+v", r)
	}
}
