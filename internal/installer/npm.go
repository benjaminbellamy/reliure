// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"encoding/json"
	"os/exec"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// NpmInstaller runs ``npm install -g <pkg>``. May need elevated privileges
// depending on the user's npm prefix; we let npm decide and surface errors.
type NpmInstaller struct{}

func (NpmInstaller) Source() string  { return snapshot.SourceNpm }
func (NpmInstaller) Available() bool { return have("npm") }

func (n NpmInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("npm")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	installed := npmGlobals()
	for _, item := range items {
		if _, ok := installed[item.Name]; ok {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "npm", "install", "-g", item.Name)
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

func npmGlobals() map[string]struct{} {
	out := map[string]struct{}{}
	body, err := exec.Command("npm", "list", "-g", "--depth=0", "--json").Output()
	if err != nil {
		return out
	}
	var doc struct {
		Dependencies map[string]any `json:"dependencies"`
	}
	if json.Unmarshal(body, &doc) != nil {
		return out
	}
	for name := range doc.Dependencies {
		out[name] = struct{}{}
	}
	return out
}
