package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// decodeStrict parses volok.json into Config, rejecting unknown fields,
// duplicate keys and any trailing JSON after the first object.
func decodeStrict(data []byte) (*Config, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parsing json: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("parsing json: expected top-level object")
	}

	var cfg Config
	seen := map[string]bool{}
	for dec.More() {
		key, raw, err := nextField(dec, seen)
		if err != nil {
			return nil, err
		}
		if err := applyField(&cfg, key, raw); err != nil {
			return nil, err
		}
	}

	// The object must end cleanly and nothing may follow it.
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("parsing json: unterminated object: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parsing json: trailing data after object")
		}
		return nil, fmt.Errorf("parsing json: trailing data: %w", err)
	}

	for _, name := range []string{"token", "schema_version", "listen", "public_url", "nodes"} {
		if !seen[name] {
			return nil, fmt.Errorf("parsing json: missing field %q", name)
		}
	}
	return &cfg, nil
}

// nextField reads one object key and its raw value, rejecting duplicates.
func nextField(dec *json.Decoder, seen map[string]bool) (string, json.RawMessage, error) {
	keyTok, err := dec.Token()
	if err != nil {
		return "", nil, fmt.Errorf("parsing json: %w", err)
	}
	key, ok := keyTok.(string)
	if !ok {
		return "", nil, fmt.Errorf("parsing json: non-string key")
	}
	if seen[key] {
		return "", nil, fmt.Errorf("parsing json: duplicate key %q", key)
	}
	seen[key] = true

	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return "", nil, fmt.Errorf("parsing json field %q: %w", key, err)
	}
	return key, raw, nil
}

// applyField maps a JSON field onto cfg.
func applyField(cfg *Config, key string, raw json.RawMessage) error {
	switch key {
	case "token":
		return json.Unmarshal(raw, &cfg.Token)
	case "users":
		return json.Unmarshal(raw, &cfg.Users)
	case "schema_version":
		num, err := decNumber(raw)
		if err != nil {
			return fmt.Errorf("schema_version: %w", err)
		}
		cfg.SchemaVersion = num
		return nil
	case "listen":
		return json.Unmarshal(raw, &cfg.Listen)
	case "public_url":
		return json.Unmarshal(raw, &cfg.PublicURL)
	case "nodes":
		return json.Unmarshal(raw, &cfg.Nodes)
	default:
		return fmt.Errorf("parsing json: unknown field %q", key)
	}
}

func decNumber(raw json.RawMessage) (int, error) {
	var num json.Number
	if err := json.Unmarshal(raw, &num); err != nil {
		return 0, fmt.Errorf("expected integer: %w", err)
	}
	i, err := num.Int64()
	if err != nil {
		return 0, fmt.Errorf("expected integer: %w", err)
	}
	return int(i), nil
}
