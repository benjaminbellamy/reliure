// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// GNOMEExtScanner enumerates installed GNOME Shell extensions.
//
// Detection prefers ``gnome-extensions list`` (part of gnome-shell), then
// falls back to scanning ``~/.local/share/gnome-shell/extensions/`` and
// ``/usr/share/gnome-shell/extensions/`` for ``metadata.json`` files.
//
// User-installed extensions usually come from extensions.gnome.org and have
// no clean apt-style reinstall path; we surface the URL in ``RestoreHint``.
// System-installed extensions are tagged ``OSPackage=true`` since they
// generally come from a ``gnome-shell-extension-*`` apt package.
type GNOMEExtScanner struct{}

func (GNOMEExtScanner) Name() string { return snapshot.SourceGnomeExt }

func (GNOMEExtScanner) Available() bool {
	if Have("gnome-extensions") {
		return true
	}
	for _, p := range gnomeExtDirs() {
		if info, err := os.Stat(p.path); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func (GNOMEExtScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	pkgs := []snapshot.Package{}
	seen := map[string]struct{}{}

	add := func(p snapshot.Package) {
		if _, dup := seen[p.ID]; dup {
			return
		}
		seen[p.ID] = struct{}{}
		pkgs = append(pkgs, p)
	}

	// Filesystem walk first — gives us metadata for every extension on the
	// system regardless of whether gnome-extensions is on PATH.
	for _, d := range gnomeExtDirs() {
		entries, err := os.ReadDir(d.path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			meta, err := readGnomeExtMetadata(filepath.Join(d.path, e.Name(), "metadata.json"))
			if err != nil {
				continue
			}
			add(gnomeExtPackage(meta, d.system))
		}
	}

	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ID < pkgs[j].ID })
	return pkgs, nil
}

type gnomeExtDir struct {
	path   string
	system bool
}

func gnomeExtDirs() []gnomeExtDir {
	home, _ := os.UserHomeDir()
	return []gnomeExtDir{
		{filepath.Join(home, ".local", "share", "gnome-shell", "extensions"), false},
		{"/usr/share/gnome-shell/extensions", true},
	}
}

type gnomeExtMeta struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Version     any    `json:"version"`
}

func readGnomeExtMetadata(path string) (gnomeExtMeta, error) {
	var m gnomeExtMeta
	body, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return m, err
	}
	if m.UUID == "" {
		return m, fmt.Errorf("metadata at %s has no uuid", path)
	}
	return m, nil
}

func gnomeExtPackage(m gnomeExtMeta, system bool) snapshot.Package {
	name := m.Name
	if name == "" {
		name = m.UUID
	}
	url := m.URL
	if url == "" {
		// extensions.gnome.org has a search endpoint by uuid.
		url = "https://extensions.gnome.org/extension/?uuid=" + m.UUID
	}
	hint := fmt.Sprintf("install via the Extensions app, or from %s", url)
	pkg := snapshot.Package{
		ID:          "gnome-ext:" + m.UUID,
		Name:        name,
		Source:      snapshot.SourceGnomeExt,
		AppID:       m.UUID, // reuse for the canonical id
		URL:         url,
		Version:     versionString(m.Version),
		OSPackage:   system,
		RestoreHint: hint,
	}
	return pkg
}

func versionString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", x), "0"), ".")
	default:
		return fmt.Sprintf("%v", v)
	}
}
