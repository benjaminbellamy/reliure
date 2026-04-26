// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"os/exec"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// PipInstaller installs into the user's pip site-packages (no sudo).
type PipInstaller struct{}

func (PipInstaller) Source() string  { return snapshot.SourcePip }
func (PipInstaller) Available() bool { return have("pip") || have("pip3") }

func (p PipInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("pip")
	rep.SectionTotal(len(items))
	cmd := "pip"
	if !have("pip") {
		cmd = "pip3"
	}
	results := make([]ItemResult, 0, len(items))
	for _, item := range items {
		if pipInstalled(cmd, item.Name) {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, cmd, "install", "--user", item.Name)
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

func pipInstalled(cmd, name string) bool {
	return exec.Command(cmd, "show", name).Run() == nil
}
