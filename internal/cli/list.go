// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/tui"
	"github.com/charmbracelet/lipgloss"
)

// ListCmd prints a styled tabular view of a snapshot.
type ListCmd struct {
	Snapshot   string   `arg:"" help:"Path to the snapshot YAML." type:"existingfile"`
	Sources    []string `name:"source" help:"Restrict to these sources." placeholder:"NAME"`
	Essential  bool     `help:"Show only items flagged essential."`
	Unverified bool     `help:"Show only inferred / unverified items."`
	OS         bool     `name:"os" help:"Show only items flagged as OS-installed."`
	UserOnly   bool     `name:"user-only" help:"Hide OS-installed items (mutually exclusive with --os)."`
}

// Run executes the list command.
func (c *ListCmd) Run(ctx context.Context) error {
	snap, err := snapshot.Load(c.Snapshot)
	if err != nil {
		return err
	}
	pkgs := filterForList(snap.Packages, c)
	if len(pkgs) == 0 {
		fmt.Println("  (no packages match the given filters)")
		return nil
	}

	theme := tui.DefaultTheme()
	printListHeader(snap, theme, len(pkgs))
	printListBody(pkgs, theme)
	return nil
}

func filterForList(items []snapshot.Package, c *ListCmd) []snapshot.Package {
	allowed := map[string]struct{}{}
	for _, s := range c.Sources {
		allowed[s] = struct{}{}
	}
	out := make([]snapshot.Package, 0, len(items))
	for _, p := range items {
		if len(c.Sources) > 0 {
			if _, ok := allowed[p.Source]; !ok {
				continue
			}
		}
		if c.Essential && !p.Essential {
			continue
		}
		if c.Unverified && !p.Unverified {
			continue
		}
		if c.OS && !p.OSPackage {
			continue
		}
		if c.UserOnly && p.OSPackage {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func printListHeader(snap *snapshot.Snapshot, theme tui.Theme, count int) {
	fmt.Println()
	fmt.Println(theme.Title.Render(fmt.Sprintf("Reliure snapshot · %s · %s",
		snap.Meta.Hostname, snap.Meta.DateShort())))
	fmt.Println(theme.Subtitle.Render(fmt.Sprintf("  %s · %d package(s)",
		snap.Meta.OS, count)))
	fmt.Println()
}

func printListBody(pkgs []snapshot.Package, theme tui.Theme) {
	// Compute column widths from the data so things stay aligned.
	srcW, nameW, verW := 8, 8, 8
	for _, p := range pkgs {
		if w := len(p.Source); w > srcW {
			srcW = w
		}
		name := identityForList(p)
		if w := lipgloss.Width(name); w > nameW {
			nameW = w
		}
		ver := versionForList(p)
		if w := lipgloss.Width(ver); w > verW {
			verW = w
		}
	}
	if nameW > 50 {
		nameW = 50
	}
	if verW > 30 {
		verW = 30
	}

	// Header row.
	hdr := lipgloss.NewStyle().Bold(true).Foreground(theme.Title.GetForeground())
	fmt.Printf("  %s  %s  %s  %s\n",
		hdr.Width(srcW).Render("SOURCE"),
		hdr.Width(nameW).Render("NAME"),
		hdr.Width(verW).Render("VERSION"),
		hdr.Render("TAGS"))
	fmt.Println("  " + theme.Dim.Render(strings.Repeat("─", srcW+nameW+verW+24)))

	// Rows.
	srcStyle := theme.Subtitle
	nameStyle := lipgloss.NewStyle()
	verStyle := theme.Muted
	for _, p := range pkgs {
		fmt.Printf("  %s  %s  %s  %s\n",
			srcStyle.Width(srcW).Render(p.Source),
			nameStyle.Width(nameW).Render(truncate(identityForList(p), nameW)),
			verStyle.Width(verW).Render(truncate(versionForList(p), verW)),
			renderTagsForList(p, theme))
	}
	fmt.Println()
}

// identityForList returns the most human-meaningful identifier for a package
// in tabular form (app_id for flatpak, extension_id for vscode, name otherwise).
func identityForList(p snapshot.Package) string {
	switch p.Source {
	case snapshot.SourceFlatpak:
		if p.AppID != "" {
			return p.AppID
		}
	case snapshot.SourceVSCode:
		if p.ExtensionID != "" {
			return p.ExtensionID
		}
	case snapshot.SourceGnomeExt:
		if p.AppID != "" {
			return p.AppID
		}
	}
	return p.Name
}

func versionForList(p snapshot.Package) string {
	if p.Version != "" {
		return p.Version
	}
	return p.Branch
}

func renderTagsForList(p snapshot.Package, theme tui.Theme) string {
	tags := []string{}
	if p.Essential {
		tags = append(tags, theme.Warn.Render("essential"))
	}
	if p.OSPackage {
		tags = append(tags, theme.Dim.Render("os"))
	}
	if p.Unverified {
		tags = append(tags, theme.Warn.Render("unverified"))
	}
	return strings.Join(tags, " · ")
}

func truncate(s string, max int) string {
	if lipgloss.Width(s) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	// Naïve byte-level truncation; package names are ASCII in practice.
	return s[:max-1] + "…"
}

