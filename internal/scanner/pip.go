// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// PipScanner enumerates user-installed pip packages.
//
// We deliberately use ``pip list --user`` (not the system-wide list); on
// modern Python (PEP 668), system-wide ``pip install`` is blocked anyway,
// and ``--user`` matches what we'd actually restore.
type PipScanner struct{}

func (PipScanner) Name() string    { return snapshot.SourcePip }
func (PipScanner) Available() bool { return Have("pip") || Have("pip3") }

func (s PipScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	cmd := "pip"
	if !Have("pip") {
		cmd = "pip3"
	}
	out, err := RunCmd(ctx, cmd, "list", "--user", "--format=json")
	if err != nil {
		return nil, err
	}
	var entries []struct{ Name, Version string }
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return nil, err
	}
	pkgs := make([]snapshot.Package, 0, len(entries))
	for _, e := range entries {
		pkgs = append(pkgs, snapshot.Package{
			ID:      "pip:" + e.Name,
			Name:    e.Name,
			Source:  snapshot.SourcePip,
			Version: e.Version,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
