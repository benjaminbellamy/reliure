// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// GNOMEExtInstaller doesn't actually install — there's no scriptable install
// path for arbitrary extensions.gnome.org extensions. Instead, it prints the
// uuid + URL for each picked extension as a manual to-do list.
type GNOMEExtInstaller struct{}

func (GNOMEExtInstaller) Source() string  { return snapshot.SourceGnomeExt }
func (GNOMEExtInstaller) Available() bool { return true }

func (g GNOMEExtInstaller) Install(_ context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("gnome extensions")
	rep.SectionTotal(len(items))
	rep.Note("install these via the Extensions app (or extensions.gnome.org):")
	results := make([]ItemResult, 0, len(items))
	for _, p := range items {
		url := p.URL
		if url == "" {
			url = "https://extensions.gnome.org/extension/?uuid=" + p.AppID
		}
		rep.Note("  • %s — %s", p.Name, url)
		results = append(results, ItemResult{Package: p, Outcome: OutcomeSkipped})
	}
	return results
}
