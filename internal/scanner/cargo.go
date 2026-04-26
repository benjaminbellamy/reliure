// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// CargoScanner enumerates ``cargo install``-ed crates.
//
// Output of ``cargo install --list``:
//
//	bat v0.24.0:
//	    bat
//	rg v14.0.0:
//	    rg
//
// Lines that don't start with whitespace are "<crate> v<version>:" entries;
// indented lines are the binary names that crate provides (we ignore them —
// the crate name + version is the canonical install identity).
type CargoScanner struct{}

func (CargoScanner) Name() string    { return snapshot.SourceCargo }
func (CargoScanner) Available() bool { return Have("cargo") }

func (CargoScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "cargo", "install", "--list")
	if err != nil {
		return nil, err
	}
	pkgs := []snapshot.Package{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		// "name v1.2.3:" — strip the trailing colon and split on " v".
		head := strings.TrimSuffix(strings.TrimSpace(line), ":")
		idx := strings.LastIndex(head, " v")
		if idx < 0 {
			continue
		}
		name, ver := head[:idx], head[idx+2:]
		pkgs = append(pkgs, snapshot.Package{
			ID:      "cargo:" + name,
			Name:    name,
			Source:  snapshot.SourceCargo,
			Version: ver,
			Crate:   name,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
