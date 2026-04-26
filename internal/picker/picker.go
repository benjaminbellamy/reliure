// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package picker is the multi-page checkbox wizard used at both backup and
// restore time. Built on Bubble Tea + Bubbles + Lip Gloss.
package picker

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Item is a single checkbox row.
type Item struct {
	ID      string
	Label   string  // visible text (without checkbox glyph)
	Checked bool    // initial state
	Badges  []Badge // visual decorators (e.g. [installed], [essential], version)
}

// Badge is a small inline tag rendered after the item label.
type Badge struct {
	Kind BadgeKind
	Text string
}

// BadgeKind picks the visual style for a Badge.
type BadgeKind int

const (
	BadgeNeutral BadgeKind = iota
	BadgeInstalled
	BadgeEssential
	BadgeVersion
	BadgeOS // "[os]" — package belongs to the OS install (not user-added)
)

// Page is one source's worth of items.
type Page struct {
	ID    string
	Title string
	Items []Item
}

// Result is what Run returns.
type Result struct {
	Aborted    bool
	Selections map[string][]string // page ID → selected item IDs
}

// ErrAborted is returned when the user pressed Quit / Esc.
var ErrAborted = errors.New("picker aborted")

// Run launches the picker. ``ctx`` is cosmetic (string shown in the title
// bar, e.g. "Reliure backup", "Reliure restore"). Blocks until the user
// confirms or quits.
func Run(title string, pages []Page) (Result, error) {
	if len(pages) == 0 {
		return Result{Aborted: false, Selections: map[string][]string{}}, nil
	}
	m := newModel(title, pages)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return Result{}, fmt.Errorf("picker: %w", err)
	}
	mm := final.(*model)
	if mm.aborted {
		return Result{Aborted: true}, ErrAborted
	}
	return mm.result(), nil
}
