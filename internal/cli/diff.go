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
)

// DiffCmd compares two snapshots and prints what was added / removed /
// version-changed.
type DiffCmd struct {
	A string `arg:"" help:"First snapshot (the 'before')." type:"existingfile"`
	B string `arg:"" help:"Second snapshot (the 'after')." type:"existingfile"`
}

// Run executes the diff command.
func (c *DiffCmd) Run(ctx context.Context) error {
	a, err := snapshot.Load(c.A)
	if err != nil {
		return fmt.Errorf("load %s: %w", c.A, err)
	}
	b, err := snapshot.Load(c.B)
	if err != nil {
		return fmt.Errorf("load %s: %w", c.B, err)
	}

	d := computeDiff(a, b)
	theme := tui.DefaultTheme()
	printDiff(d, a, b, theme)
	return nil
}

type diffEntry struct {
	A snapshot.Package
	B snapshot.Package
}

type diffResult struct {
	Added   []snapshot.Package
	Removed []snapshot.Package
	Changed []diffEntry // version differs
}

func computeDiff(a, b *snapshot.Snapshot) diffResult {
	keyOf := func(p snapshot.Package) [2]string { return p.NaturalKey() }

	aByKey := map[[2]string]snapshot.Package{}
	for _, p := range a.Packages {
		aByKey[keyOf(p)] = p
	}
	bByKey := map[[2]string]snapshot.Package{}
	for _, p := range b.Packages {
		bByKey[keyOf(p)] = p
	}

	res := diffResult{}
	for k, pb := range bByKey {
		pa, exists := aByKey[k]
		switch {
		case !exists:
			res.Added = append(res.Added, pb)
		case versionOf(pa) != versionOf(pb):
			res.Changed = append(res.Changed, diffEntry{A: pa, B: pb})
		}
	}
	for k, pa := range aByKey {
		if _, ok := bByKey[k]; !ok {
			res.Removed = append(res.Removed, pa)
		}
	}

	bySrcThenName := func(slice []snapshot.Package) {
		sort.Slice(slice, func(i, j int) bool {
			if slice[i].Source != slice[j].Source {
				return slice[i].Source < slice[j].Source
			}
			return slice[i].Name < slice[j].Name
		})
	}
	bySrcThenName(res.Added)
	bySrcThenName(res.Removed)
	sort.Slice(res.Changed, func(i, j int) bool {
		if res.Changed[i].B.Source != res.Changed[j].B.Source {
			return res.Changed[i].B.Source < res.Changed[j].B.Source
		}
		return res.Changed[i].B.Name < res.Changed[j].B.Name
	})
	return res
}

func versionOf(p snapshot.Package) string {
	if p.Version != "" {
		return p.Version
	}
	return p.Branch
}

func printDiff(d diffResult, a, b *snapshot.Snapshot, theme tui.Theme) {
	fmt.Println()
	fmt.Println(theme.Title.Render("Reliure diff"))
	fmt.Println("  " + theme.Subtitle.Render(
		fmt.Sprintf("a: %s · %d package(s)", a.Meta.DateShort(), len(a.Packages))))
	fmt.Println("  " + theme.Subtitle.Render(
		fmt.Sprintf("b: %s · %d package(s)", b.Meta.DateShort(), len(b.Packages))))
	fmt.Println()
	fmt.Println("  " +
		theme.OK.Render(fmt.Sprintf("+%d added", len(d.Added))) + "  · " +
		theme.Err.Render(fmt.Sprintf("-%d removed", len(d.Removed))) + "  · " +
		theme.Warn.Render(fmt.Sprintf("~%d changed", len(d.Changed))))
	fmt.Println()

	if len(d.Added) > 0 {
		fmt.Println(theme.OK.Render(strings.Repeat("─", 4) + " added " + strings.Repeat("─", 70)))
		for _, p := range d.Added {
			fmt.Printf("  %s  %s  %s\n",
				theme.OK.Render("+"),
				theme.Subtitle.Render(p.Source),
				p.Name+versionSuffix(p))
		}
		fmt.Println()
	}
	if len(d.Removed) > 0 {
		fmt.Println(theme.Err.Render(strings.Repeat("─", 4) + " removed " + strings.Repeat("─", 68)))
		for _, p := range d.Removed {
			fmt.Printf("  %s  %s  %s\n",
				theme.Err.Render("-"),
				theme.Subtitle.Render(p.Source),
				p.Name+versionSuffix(p))
		}
		fmt.Println()
	}
	if len(d.Changed) > 0 {
		fmt.Println(theme.Warn.Render(strings.Repeat("─", 4) + " changed " + strings.Repeat("─", 68)))
		for _, e := range d.Changed {
			fmt.Printf("  %s  %s  %s  %s → %s\n",
				theme.Warn.Render("~"),
				theme.Subtitle.Render(e.B.Source),
				e.B.Name,
				theme.Muted.Render(versionOf(e.A)),
				theme.Muted.Render(versionOf(e.B)))
		}
		fmt.Println()
	}
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 {
		fmt.Println("  " + theme.Subtitle.Render("(snapshots are identical)"))
	}
}

func versionSuffix(p snapshot.Package) string {
	v := versionOf(p)
	if v == "" {
		return ""
	}
	return "  " + v
}
