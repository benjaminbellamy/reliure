// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// udevTargetDir is the canonical restore destination. Mirrors the scanner's
// udevRulesDir in production but kept as a separate package var so tests
// here don't have to reach into the scanner package.
var udevTargetDir = "/etc/udev/rules.d"

// UdevInstaller writes user-installed udev rules back under
// /etc/udev/rules.d/, then reloads udev once per batch so the kernel picks
// up the new device-permission rules without a reboot. Idempotent: a rule
// already present at the target with identical bytes is reported as
// ``already installed`` and the reload only fires if at least one file was
// actually written.
type UdevInstaller struct{}

func (UdevInstaller) Source() string  { return snapshot.SourceUdev }
func (UdevInstaller) Available() bool { return have("udevadm") }

func (u UdevInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("udev")
	rep.SectionTotal(len(items))

	results := make([]ItemResult, 0, len(items))
	wroteAny := false

	for _, item := range items {
		filename := filepath.Base(item.Evidence)
		if filename == "" || filename == "." || filename == "/" {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing or invalid evidence path %q", item.Evidence)}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		target := filepath.Join(udevTargetDir, filename)

		decoded, err := base64.StdEncoding.DecodeString(item.Payload)
		if err != nil || len(decoded) == 0 {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing or invalid payload")}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		// Idempotency: if the same bytes are already in place, skip silently
		// — same shape as the bluetooth installer's "target exists → already
		// installed" check, but we compare contents because users do edit
		// rules in place.
		if existing, rerr := os.ReadFile(target); rerr == nil && bytes.Equal(existing, decoded) {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		tmp, terr := os.CreateTemp("", "reliure-udev-*.rules")
		if terr != nil {
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: terr}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		_, werr := tmp.Write(decoded)
		_ = tmp.Close()
		if werr != nil {
			_ = os.Remove(tmp.Name())
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: werr}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		ierr := runCtx(ctx, opts, "sudo",
			"install", "-m", "644", "-o", "root", "-g", "root",
			tmp.Name(), target)
		_ = os.Remove(tmp.Name())
		out := OutcomeInstalled
		if ierr != nil {
			out = OutcomeFailed
		} else {
			wroteAny = true
		}
		r := ItemResult{Package: item, Outcome: out, Err: ierr}
		rep.Result(r)
		results = append(results, r)
	}

	// Reload udev once per batch — far cheaper than per-rule and udev's
	// own docs recommend reload-rules + trigger together for the new rules
	// to take effect against already-attached devices.
	if wroteAny {
		if err := runCtx(ctx, opts, "sudo", "udevadm", "control", "--reload-rules"); err != nil && !opts.DryRun {
			rep.Warn("udevadm control --reload-rules failed: %v", err)
		}
		if err := runCtx(ctx, opts, "sudo", "udevadm", "trigger"); err != nil && !opts.DryRun {
			rep.Warn("udevadm trigger failed: %v", err)
		}
	}
	return results
}
