// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// CargoInstaller runs ``cargo install <crate>``. Slow (compiles from source);
// we don't pin versions here — cargo grabs the latest.
type CargoInstaller struct{}

func (CargoInstaller) Source() string  { return snapshot.SourceCargo }
func (CargoInstaller) Available() bool { return have("cargo") }

func (c CargoInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("cargo")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	for _, item := range items {
		crate := item.Crate
		if crate == "" {
			crate = item.Name
		}
		if cargoInstalled(crate) {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "cargo", "install", crate)
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

func cargoInstalled(crate string) bool {
	out, err := exec.Command("cargo", "install", "--list").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		// "<crate> v..." at column 0
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		head := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(head) > 0 && head[0] == crate {
			return true
		}
	}
	return false
}
