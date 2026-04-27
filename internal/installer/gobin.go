// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"path"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// GoBinInstaller runs ``go install <path>@<version>`` per item. If the
// snapshot didn't capture a version (older binary or built without module
// info), falls back to ``@latest``.
type GoBinInstaller struct{}

func (GoBinInstaller) Source() string  { return snapshot.SourceGo }
func (GoBinInstaller) Available() bool { return have("go") }

func (g GoBinInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("go")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	installed := scanner.GoBinBasenames()
	for _, item := range items {
		if _, ok := installed[path.Base(item.Name)]; ok {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		ver := strings.TrimSpace(item.Version)
		if ver == "" {
			ver = "latest"
		}
		err := runCtx(ctx, opts, "go", "install", item.Name+"@"+ver)
		out := OutcomeInstalled
		if err != nil {
			out = OutcomeFailed
		}
		r := ItemResult{Package: item, Outcome: out, Err: err}
		rep.Result(r)
		results = append(results, r)
	}
	return results
}
