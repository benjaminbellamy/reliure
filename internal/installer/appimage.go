// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// AppImageInstaller doesn't actually install — there's no scriptable install
// path for AppImages. Like GNOMEExtInstaller, it prints the file name + last
// known location as a manual to-do list for the user.
type AppImageInstaller struct{}

func (AppImageInstaller) Source() string  { return snapshot.SourceAppImage }
func (AppImageInstaller) Available() bool { return true }

func (a AppImageInstaller) Install(_ context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("appimages")
	rep.SectionTotal(len(items))
	rep.Note("re-download these from each publisher's site and chmod +x:")
	results := make([]ItemResult, 0, len(items))
	for _, p := range items {
		evidence := p.Evidence
		if evidence == "" {
			evidence = p.Name
		}
		rep.Note("  • %s — was at %s", p.Name, evidence)
		results = append(results, ItemResult{Package: p, Outcome: OutcomeSkipped})
	}
	return results
}
