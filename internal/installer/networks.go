// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

const nmConnectionsDir = "/etc/NetworkManager/system-connections"

// WifiInstaller writes wifi .nmconnection files back into NetworkManager's
// system store. Each install runs ``sudo install`` to land the file with
// mode 600 / owner root, then a single ``sudo nmcli connection reload`` at
// the end picks them all up.
type WifiInstaller struct{}

func (WifiInstaller) Source() string  { return snapshot.SourceWifi }
func (WifiInstaller) Available() bool { return have("nmcli") }
func (w WifiInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	return installNMConnections(ctx, "wifi", items, opts, rep)
}

// VPNInstaller mirrors WifiInstaller for VPN/WireGuard connections.
type VPNInstaller struct{}

func (VPNInstaller) Source() string  { return snapshot.SourceVPN }
func (VPNInstaller) Available() bool { return have("nmcli") }
func (v VPNInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	return installNMConnections(ctx, "vpn", items, opts, rep)
}

func installNMConnections(ctx context.Context, kind string, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section(kind)
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))

	wroteAny := false
	for _, item := range items {
		target := filepath.Join(nmConnectionsDir, nmTargetFilename(item))
		if _, err := os.Stat(target); err == nil {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(item.Payload)
		if err != nil || len(decoded) == 0 {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing or invalid payload — snapshot was scanned without sudo")}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		// Stage to a tempfile, then atomically place via ``sudo install``.
		// ``install`` (POSIX) sets mode/owner and replaces the target in one
		// step, avoiding a window where the file exists with looser perms.
		tmp, err := os.CreateTemp("", "reliure-nm-*.nmconnection")
		if err != nil {
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: err}
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
		err = runCtx(ctx, opts, "sudo",
			"install", "-m", "600", "-o", "root", "-g", "root",
			tmp.Name(), target)
		_ = os.Remove(tmp.Name())
		out := OutcomeInstalled
		if err != nil {
			out = OutcomeFailed
		} else {
			wroteAny = true
		}
		r := ItemResult{Package: item, Outcome: out, Err: err}
		rep.Result(r)
		results = append(results, r)
	}

	// One reload covers the whole batch — far cheaper than per-file.
	if wroteAny {
		if err := runCtx(ctx, opts, "sudo", "nmcli", "connection", "reload"); err != nil && !opts.DryRun {
			rep.Warn("nmcli connection reload failed: %v", err)
		}
	}
	return results
}

// nmTargetFilename derives the destination basename. Prefer the recorded
// scan path's basename so the file lands exactly where it was; fall back to
// a sanitised connection id if Evidence is missing.
func nmTargetFilename(p snapshot.Package) string {
	if p.Evidence != "" {
		return filepath.Base(p.Evidence)
	}
	safe := strings.Map(func(r rune) rune {
		if r == '/' || r == '\x00' {
			return '_'
		}
		return r
	}, p.Name)
	return safe + ".nmconnection"
}
