package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/system"
	"github.com/benjaminbellamy/reliure/internal/tui"
)

// scanOpts controls a snapshot scan.
type scanOpts struct {
	Sources          []string // nil = all
	IncludeInference bool
	Exclude          []string // glob patterns against Package.ID
}

// runScan builds a Snapshot by running the selected scanners.
func runScan(ctx context.Context, theme tui.Theme, opts scanOpts) (*snapshot.Snapshot, error) {
	registry := scanner.DefaultRegistry()
	if opts.Sources != nil {
		valid := map[string]struct{}{}
		for _, n := range registry.AllNames(true) {
			valid[n] = struct{}{}
		}
		var unknown []string
		for _, s := range opts.Sources {
			if _, ok := valid[s]; !ok {
				unknown = append(unknown, s)
			}
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("unknown source(s): %s", strings.Join(unknown, ", "))
		}
	}
	scs := registry.Selected(opts.Sources, opts.IncludeInference)
	if len(scs) == 0 {
		return nil, fmt.Errorf("no scanners selected")
	}

	meta := snapshot.NewMeta(system.Hostname(), system.PrettyName(), system.Codename())
	snap := snapshot.New(meta)

	runOne := func(s scanner.Scanner) {
		spin := tui.NewSpinner("scanning " + s.Name() + " …")
		spin.Start()
		res := scanner.Run(ctx, s)
		spin.Stop()
		// Print a one-line summary in place of the (now-cleared) spinner.
		switch {
		case res.SkippedReason != "":
			fmt.Println("  " + theme.Skip.Render("·") + " " + theme.Skip.Render(s.Name()+" — "+res.SkippedReason))
		case len(res.Errors) > 0:
			for _, e := range res.Errors {
				fmt.Println("  " + theme.Err.Render("✗") + " " + theme.Err.Render(s.Name()+" — "+e))
			}
		default:
			snap.Packages = append(snap.Packages, res.Packages...)
			fmt.Println("  " + theme.OK.Render("✓") + " " + theme.Title.Render(s.Name()) + "  " +
				theme.Muted.Render(fmt.Sprintf("%d package(s)", len(res.Packages))))
		}
	}

	// Wifi/VPN need root to read /etc/NetworkManager/system-connections.
	// Run them first so we can immediately drop back to $SUDO_USER for
	// the rest — otherwise ``code`` self-aborts as uid 0, cargo/pipx/npm
	// fall off sudo's secure_path, and the history scanner reads /root's
	// (empty) bash_history. The drop is a no-op when reliure isn't running
	// under sudo.
	rootScanners, userScanners := partitionRootScanners(scs)
	for _, s := range rootScanners {
		runOne(s)
	}
	if dropped, err := dropPrivilegesIfSudo(); err != nil {
		fmt.Println("  " + theme.Err.Render("⚠ could not drop privileges: "+err.Error()))
		fmt.Println("  " + theme.Subtitle.Render("  remaining scanners may fail under root"))
	} else if dropped {
		fmt.Println("  " + theme.Subtitle.Render("· dropped privileges to "+os.Getenv("USER")))
	}
	for _, s := range userScanners {
		runOne(s)
	}

	snap.Dedupe()

	if len(opts.Exclude) > 0 {
		before := len(snap.Packages)
		snap.Packages = applyExcludes(snap.Packages, opts.Exclude)
		dropped := before - len(snap.Packages)
		fmt.Println("  " + theme.Muted.Render(fmt.Sprintf("excluded %d entry(ies) matching %v", dropped, opts.Exclude)))
	}

	return snap, nil
}

// applyExcludes removes packages whose ID matches any of the given globs.
func applyExcludes(pkgs []snapshot.Package, patterns []string) []snapshot.Package {
	out := pkgs[:0]
	for _, p := range pkgs {
		drop := false
		for _, pat := range patterns {
			if ok, _ := filepath.Match(pat, p.ID); ok {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, p)
		}
	}
	return out
}
