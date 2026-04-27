// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// OllamaInstaller runs ``ollama pull <model>`` per item. Pulls can be large
// (several GB) and slow; we surface ollama's own progress output by streaming
// child stdout/stderr to the user's TTY.
type OllamaInstaller struct{}

func (OllamaInstaller) Source() string  { return snapshot.SourceOllama }
func (OllamaInstaller) Available() bool { return have("ollama") }

func (o OllamaInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("ollama")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	installed := scanner.OllamaInstalledModels()
	for _, item := range items {
		if _, ok := installed[item.Name]; ok {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "ollama", "pull", item.Name)
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
