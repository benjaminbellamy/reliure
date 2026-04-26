// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// PipxScanner enumerates pipx-installed CLI tools.
type PipxScanner struct{}

func (PipxScanner) Name() string    { return snapshot.SourcePipx }
func (PipxScanner) Available() bool { return Have("pipx") }

func (PipxScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "pipx", "list", "--json")
	if err != nil {
		return nil, err
	}
	// Top-level shape: {"venvs": {"<name>": {"metadata": {"main_package": {"package": "...", "package_version": "..."}}}}}
	var doc struct {
		Venvs map[string]struct {
			Metadata struct {
				MainPackage struct {
					Package        string `json:"package"`
					PackageVersion string `json:"package_version"`
				} `json:"main_package"`
			} `json:"metadata"`
		} `json:"venvs"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		return nil, err
	}
	pkgs := make([]snapshot.Package, 0, len(doc.Venvs))
	for _, v := range doc.Venvs {
		mp := v.Metadata.MainPackage
		if mp.Package == "" {
			continue
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:      "pipx:" + mp.Package,
			Name:    mp.Package,
			Source:  snapshot.SourcePipx,
			Version: mp.PackageVersion,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
