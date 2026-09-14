// Package store implements the single-file JSON storage of Volok.
//
// CLI and HTTP share the same Store so that mutations are serialized across
// processes with an advisory file lock and written atomically.
package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// MaxFileSize bounds the JSON document.
	MaxFileSize = 4 << 20
	// MaxNodes bounds the node library.
	MaxNodes = 1000
	// MaxUsers bounds the reader token list.
	MaxUsers = 1000
	// LockTimeout bounds waiting for the inter-process lock.
	LockTimeout = 10 * time.Second
)

// Store reads and writes the Volok JSON document under a file lock.
type Store struct {
	path string
	lock string
}

// Open returns a Store for the given JSON path.
func Open(path string) *Store {
	return &Store{
		path: path,
		lock: path + ".lock",
	}
}

// Exists reports whether the JSON document already exists.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Init creates the document with a fresh admin token and empty lists.
// It refuses to overwrite an existing file.
func (s *Store) Init(publicURL string) (*Config, error) {
	if s.Exists() {
		return nil, fmt.Errorf("refusing to overwrite existing %s", s.path)
	}
	token, err := NewToken()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Token:         token,
		Users:         []string{},
		SchemaVersion: SchemaVersion,
		Listen:        "127.0.0.1:41230",
		PublicURL:     publicURL,
		Nodes:         []Node{},
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := s.writeLocked(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Read loads a snapshot under a shared lock.
func (s *Store) Read() (*Config, error) {
	lock, err := acquireFileLock(s.lock, false, LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.release()

	return s.readLocked()
}

// Update applies fn to a fresh snapshot under an exclusive lock and persists
// the result atomically if anything changed.
func (s *Store) Update(fn func(*Config) error) (*Config, error) {
	lock, err := acquireFileLock(s.lock, true, LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.release()

	cfg, err := s.readLocked()
	if err != nil {
		return nil, err
	}
	before := mustJSON(cfg)
	if err := fn(cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	after := mustJSON(cfg)
	if string(before) == string(after) {
		return cfg, nil
	}
	if err := s.writeLocked(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// readLocked parses the current document. Caller must hold the lock.
func (s *Store) readLocked() (*Config, error) {
	f, err := os.Open(s.path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", s.path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stating %s: %w", s.path, err)
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("%s exceeds %d bytes", s.path, MaxFileSize)
	}

	data := make([]byte, info.Size())
	if _, err := f.Read(data); err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.path, err)
	}
	cfg, err := decodeStrict(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", s.path, err)
	}
	if len(cfg.Nodes) > MaxNodes {
		return nil, fmt.Errorf("%s: too many nodes", s.path)
	}
	if len(cfg.Users) > MaxUsers {
		return nil, fmt.Errorf("%s: too many users", s.path)
	}
	return cfg, nil
}

// writeLocked atomically replaces the document. Caller must hold the lock.
func (s *Store) writeLocked(cfg *Config) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cfg); err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	data := buf.Bytes()

	tmp, err := os.CreateTemp(dir, ".volok-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	syncDir(dir)
	return nil
}

// syncDir fsyncs the directory after rename where possible.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

func mustJSON(cfg *Config) []byte {
	data, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return data
}
