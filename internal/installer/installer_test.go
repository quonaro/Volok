package installer

import (
	"strings"
	"testing"
)

func TestRenderScriptInjectsValues(t *testing.T) {
	out, err := RenderScript("https://vpn.example.com", "abcd1234")
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "VOLOK_PUBLIC_URL='https://vpn.example.com'") {
		t.Fatalf("public url not injected: %s", s[:200])
	}
	if !strings.Contains(s, "VOLOK_TOKEN='abcd1234'") {
		t.Fatal("token not injected")
	}
	if strings.Contains(s, "__VOLOK_PUBLIC_URL__") || strings.Contains(s, "__VOLOK_TOKEN__") {
		t.Fatal("placeholders left in rendered script")
	}
}

func TestRenderScriptShellEscaping(t *testing.T) {
	// A token cannot contain quotes by construction, but escaping must be
	// robust for the public URL origin too.
	out, err := RenderScript("https://vpn.example.com", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "'https://vpn.example.com'") {
		t.Fatal("origin must be single-quoted")
	}
}

func TestRenderScriptRequiresValues(t *testing.T) {
	if _, err := RenderScript("", "abc"); err == nil {
		t.Fatal("empty public url must fail")
	}
	if _, err := RenderScript("https://x.example", ""); err == nil {
		t.Fatal("empty token must fail")
	}
}

func TestScriptHasNoEarlySideEffects(t *testing.T) {
	out, err := RenderScript("https://vpn.example.com", strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mainIdx := strings.LastIndex(s, "main \"$@\"")
	if mainIdx == -1 {
		t.Fatal("script must call main only at the end")
	}
	firstInstall := strings.Index(s, "install_deps")
	if firstInstall == -1 || firstInstall > mainIdx {
		t.Fatal("install actions must be defined before the final main call")
	}
}
