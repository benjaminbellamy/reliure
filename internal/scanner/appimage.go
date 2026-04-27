// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// AppImageScanner finds *.AppImage files in the conventional install
// locations: ~/Applications/, /opt/, ~/.local/bin/. ``~/Downloads/`` is
// deliberately excluded — too noisy with one-off files the user never
// actually used.
//
// AppImages can't be reinstalled by a package manager: the user has to
// re-download them from the publisher's site. Entries are direct (the file
// exists, no inference) but carry a manual-action ``RestoreHint``.
type AppImageScanner struct{}

func (AppImageScanner) Name() string    { return snapshot.SourceAppImage }
func (AppImageScanner) Available() bool { return true }

func appImageDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"/opt"}
	}
	return []string{
		filepath.Join(home, "Applications"),
		filepath.Join(home, ".local", "bin"),
		"/opt",
	}
}

func (AppImageScanner) Scan(_ context.Context) ([]snapshot.Package, error) {
	pkgs := []snapshot.Package{}
	seen := map[string]struct{}{}
	for _, dir := range appImageDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(strings.ToLower(name), ".appimage") {
				continue
			}
			full := filepath.Join(dir, name)
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			label := strings.TrimSuffix(name, filepath.Ext(name))
			pkgs = append(pkgs, snapshot.Package{
				ID:          "appimage:" + label,
				Name:        label,
				Source:      snapshot.SourceAppImage,
				Evidence:    full,
				DetectedVia: "filesystem",
				RestoreHint: "re-download " + name + " from the publisher's site and chmod +x",
			})
		}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// isAppImage reports whether a filename is an AppImage — used by the manual
// scanner to skip files the AppImage scanner already covers.
func isAppImage(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".appimage")
}
