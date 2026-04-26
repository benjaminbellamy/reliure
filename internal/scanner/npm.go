// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// NpmScanner enumerates globally-installed npm packages.
type NpmScanner struct{}

func (NpmScanner) Name() string    { return snapshot.SourceNpm }
func (NpmScanner) Available() bool { return Have("npm") }

func (NpmScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "npm", "list", "-g", "--depth=0", "--json")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Dependencies map[string]struct {
			Version string `json:"version"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, err
	}
	pkgs := make([]snapshot.Package, 0, len(doc.Dependencies))
	for name, info := range doc.Dependencies {
		pkgs = append(pkgs, snapshot.Package{
			ID:      "npm:" + name,
			Name:    name,
			Source:  snapshot.SourceNpm,
			Version: info.Version,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
