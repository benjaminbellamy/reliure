// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// PipxInstaller installs CLI tools via pipx (each in its own venv).
type PipxInstaller struct{}

func (PipxInstaller) Source() string  { return snapshot.SourcePipx }
func (PipxInstaller) Available() bool { return have("pipx") }

func (p PipxInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("pipx")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	for _, item := range items {
		if pipxInstalled(item.Name) {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "pipx", "install", item.Name)
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

func pipxInstalled(name string) bool {
	out, err := exec.Command("pipx", "list", "--short").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			return true
		}
	}
	return false
}
