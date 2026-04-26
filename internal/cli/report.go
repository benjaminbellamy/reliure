// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benjaminbellamy/reliure/internal/report"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/tui"
)

// ReportCmd renders a snapshot as a styled standalone HTML document. The
// result is print-friendly: open in a browser and use Cmd/Ctrl-P → "Save as
// PDF" for a beautiful printable artefact.
type ReportCmd struct {
	Snapshot string `arg:"" help:"Path to the snapshot YAML." type:"existingfile"`
	Output   string `short:"o" help:"Output HTML path (default: ./reliure-report-YYYYMMDD.html in the current directory)." placeholder:"PATH"`
}

// Run executes the report command.
func (c *ReportCmd) Run(ctx context.Context) error {
	snap, err := snapshot.Load(c.Snapshot)
	if err != nil {
		return err
	}

	// Default output: current working directory. The snapshot itself lives
	// under ~/.config/, which most browsers won't open via file:// without
	// poking holes in their sandbox — putting the report next to the user
	// makes "double-click to open" Just Work.
	out := c.Output
	if out == "" {
		out = filepath.Join(".", "reliure-report-"+snap.Meta.DateCompact()+".html")
	}

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := report.RenderHTML(snap, Version, Tagline, Copyright, f); err != nil {
		return err
	}

	abs, _ := filepath.Abs(out)
	theme := tui.DefaultTheme()
	fmt.Fprintln(os.Stdout, "  "+theme.OK.Render("✓")+" "+theme.Title.Render(abs))
	fmt.Fprintln(os.Stdout, "  "+theme.Subtitle.Render(
		"open in a browser; Cmd/Ctrl-P → \"Save as PDF\" for a printable copy."))
	return nil
}
