package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return Open(filepath.Join(dir, "volok.json"))
}

func TestInit(t *testing.T) {
	s := tempStore(t)
	cfg, err := s.Init("https://vpn.example.com")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if len(cfg.Token) != 64 || !hex64.MatchString(cfg.Token) {
		t.Fatalf("bad token %q", cfg.Token)
	}
	if len(cfg.Users) != 0 || len(cfg.Nodes) != 0 {
		t.Fatalf("expected empty lists, got %d/%d", len(cfg.Users), len(cfg.Nodes))
	}
	if cfg.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version %d", cfg.SchemaVersion)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode %o", info.Mode().Perm())
	}

	// Refuse overwrite.
	if _, err := s.Init("https://other.example.com"); err == nil {
		t.Fatal("second init must fail")
	}
}

func TestTokenIsFirstJSONField(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") {
		t.Fatal("not an object")
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "{"))
	if !strings.HasPrefix(body, `"token"`) {
		t.Fatalf("token is not the first field: %s", body[:40])
	}
}

func TestUpdateNodeAndPersist(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	cfg, err := s.Update(func(c *Config) error {
		c.Nodes = append(c.Nodes, Node{ID: "abc", Name: "Finland", URL: "vless://uuid@203.0.113.10:44333?security=reality", Enabled: true})
		return nil
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(cfg.Nodes) != 1 {
		t.Fatalf("nodes %d", len(cfg.Nodes))
	}

	got, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 1 || got.Nodes[0].ID != "abc" {
		t.Fatalf("persist failed: %+v", got.Nodes)
	}
}

func TestRejectDuplicateEndpoints(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	_, err := s.Update(func(c *Config) error {
		c.Nodes = append(c.Nodes,
			Node{ID: "a", Name: "A", URL: "vless://uuid@203.0.113.10:44333?security=reality", Enabled: true},
			Node{ID: "b", Name: "B", URL: "vless://other@203.0.113.10:44333?security=reality", Enabled: true},
		)
		return nil
	})
	if err == nil {
		t.Fatal("duplicate endpoint must fail")
	}
}

func TestUsersUniquenessAndAdminCollision(t *testing.T) {
	s := tempStore(t)
	cfg, err := s.Init("https://vpn.example.com")
	if err != nil {
		t.Fatal(err)
	}
	admin := cfg.Token

	if _, err := s.Update(func(c *Config) error {
		c.Users = append(c.Users, admin)
		return nil
	}); err == nil {
		t.Fatal("admin collision must fail")
	}
	if _, err := s.Update(func(c *Config) error {
		c.Users = append(c.Users, strings.Repeat("a", 64), strings.Repeat("a", 64))
		return nil
	}); err == nil {
		t.Fatal("duplicate users must fail")
	}
	if _, err := s.Update(func(c *Config) error {
		c.Users = []string{strings.Repeat("b", 64)}
		return nil
	}); err != nil {
		t.Fatalf("valid user add failed: %v", err)
	}
}

func TestCorruptFileRejected(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.path, []byte(`{"token": "broken`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err == nil {
		t.Fatal("corrupt file must fail")
	}
	if _, err := s.Update(func(c *Config) error { return nil }); err == nil {
		t.Fatal("corrupt file must fail before update")
	}
}

func TestUnknownAndDuplicateKeysRejected(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	bad := `{"token":"` + strings.Repeat("a", 64) + `","bogus":1,` +
		`"schema_version":1,"listen":"127.0.0.1:41220",` +
		`"public_url":"https://x.example","nodes":[]}`
	if err := os.WriteFile(s.path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err == nil {
		t.Fatal("unknown key must fail")
	}

	dup := `{"token":"` + strings.Repeat("a", 64) + `","token":"` +
		strings.Repeat("b", 64) + `","schema_version":1,` +
		`"listen":"127.0.0.1:41220","public_url":"https://x.example","nodes":[]}`
	if err := os.WriteFile(s.path, []byte(dup), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err == nil {
		t.Fatal("duplicate key must fail")
	}
}

func TestUnchangedUpdateDoesNotRewrite(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.path)
	if _, err := s.Update(func(c *Config) error { return nil }); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.path)
	if string(before) != string(after) {
		t.Fatal("unchanged update must not rewrite the file")
	}
}

func TestTrailingJSONRejected(t *testing.T) {
	s := tempStore(t)
	if _, err := s.Init("https://vpn.example.com"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.path)
	if err := os.WriteFile(s.path, append(data, []byte(`{}`)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Read(); err == nil {
		t.Fatal("trailing json must fail")
	}
}
