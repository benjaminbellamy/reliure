package snapshot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

// Load reads a snapshot YAML from disk.
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Unmarshal(data)
}

// Unmarshal parses snapshot YAML from bytes.
func Unmarshal(data []byte) (*Snapshot, error) {
	var s Snapshot
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse snapshot yaml: %w", err)
	}
	if s.ReliureVersion == "" {
		s.ReliureVersion = ReliureVersion
	}
	return &s, nil
}

// Dump writes a snapshot to disk, creating parent directories as needed.
// The write is atomic: a temp file is fsync'd then renamed in place.
func Dump(s *Snapshot, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	body, err := Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reliure-snapshot-*.yaml")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}

// Marshal serialises a snapshot to YAML bytes.
func Marshal(s *Snapshot) ([]byte, error) {
	if s.ReliureVersion == "" {
		s.ReliureVersion = ReliureVersion
	}
	body, err := yaml.MarshalWithOptions(s, yaml.Indent(2), yaml.IndentSequence(true))
	if err != nil {
		return nil, fmt.Errorf("marshal snapshot: %w", err)
	}
	return body, nil
}
