// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package tui has small Lip Gloss-styled non-curses helpers (banners, y/n
// prompts, install reporters). For the heavyweight wizard see internal/picker.
package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/installer"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/charmbracelet/lipgloss"
)

// Theme bundles the styles used outside the picker.
type Theme struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted    lipgloss.Style
	OK       lipgloss.Style
	Skip     lipgloss.Style
	Warn     lipgloss.Style
	Err      lipgloss.Style
	Section  lipgloss.Style
	Banner   lipgloss.Style
	Dim      lipgloss.Style
}

// DefaultTheme returns the standard non-picker visual identity.
func DefaultTheme() Theme {
	primary := lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#B294FF"}
	accent := lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#7CE38B"}
	muted := lipgloss.AdaptiveColor{Light: "#7E7E87", Dark: "#9595A2"}
	warn := lipgloss.AdaptiveColor{Light: "#D5752E", Dark: "#FFB454"}
	red := lipgloss.AdaptiveColor{Light: "#D02323", Dark: "#FF6B6B"}
	soft := lipgloss.AdaptiveColor{Light: "#E5E5EE", Dark: "#3A3A45"}
	return Theme{
		Title:    lipgloss.NewStyle().Foreground(primary).Bold(true),
		Subtitle: lipgloss.NewStyle().Foreground(muted),
		Muted:    lipgloss.NewStyle().Foreground(muted),
		OK:       lipgloss.NewStyle().Foreground(accent).Bold(true),
		Skip:     lipgloss.NewStyle().Foreground(muted).Italic(true),
		Warn:     lipgloss.NewStyle().Foreground(warn).Bold(true),
		Err:      lipgloss.NewStyle().Foreground(red).Bold(true),
		Section: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(soft).
			Foreground(primary).
			Bold(true).
			Padding(0, 1),
		Banner: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Padding(1, 3),
		Dim: lipgloss.NewStyle().Foreground(soft),
	}
}

// Banner prints a styled banner block to ``w``.
func Banner(w io.Writer, theme Theme, lines ...string) {
	body := strings.Join(lines, "\n")
	fmt.Fprintln(w, theme.Banner.Render(body))
	fmt.Fprintln(w)
}

// Section prints a section heading like ``▸ apt`` underlined.
func Section(w io.Writer, theme Theme, text string) {
	fmt.Fprintln(w, theme.Section.Render("▸ "+text))
}

// AskYesNo prints a y/n prompt with a default and reads from stdin.
// Non-TTY stdin returns the default unchanged.
func AskYesNo(theme Theme, prompt string, dflt bool) bool {
	if !isTTY(os.Stdin) {
		return dflt
	}
	suffix := "[Y/n]"
	if !dflt {
		suffix = "[y/N]"
	}
	fmt.Print("  " + prompt + " " + theme.Muted.Render(suffix) + ": ")
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return dflt
	}
	ans := strings.ToLower(strings.TrimSpace(line))
	switch ans {
	case "y", "yes":
		return true
	case "n", "no":
		return false
	}
	return dflt
}

// Confirm is the three-way result of AskConfirm: continue, abort, or go back.
type Confirm int

const (
	ConfirmYes  Confirm = iota // proceed
	ConfirmNo                  // abort entirely
	ConfirmBack                // return to the previous step (e.g. the picker)
)

// AskConfirm asks a y/n/back prompt with a default. ``b`` always means "go
// back". Non-TTY stdin returns the default — back is never auto-selected.
func AskConfirm(theme Theme, prompt string, dflt Confirm) Confirm {
	if !isTTY(os.Stdin) {
		if dflt == ConfirmBack {
			return ConfirmYes
		}
		return dflt
	}
	suffix := "[Y/n/b]"
	switch dflt {
	case ConfirmNo:
		suffix = "[y/N/b]"
	case ConfirmBack:
		suffix = "[y/n/B]"
	}
	for {
		fmt.Print("  " + prompt + " " + theme.Muted.Render(suffix+" (b = back to picker)") + ": ")
		r := bufio.NewReader(os.Stdin)
		line, err := r.ReadString('\n')
		if err != nil {
			return dflt
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		switch ans {
		case "y", "yes":
			return ConfirmYes
		case "n", "no":
			return ConfirmNo
		case "b", "back":
			return ConfirmBack
		case "":
			return dflt
		}
		fmt.Println("  " + theme.Subtitle.Render("please answer y, n, or b"))
	}
}

// StreamReporter is a Reporter that pretty-prints to a Writer.
type StreamReporter struct {
	W     io.Writer
	Theme Theme
	stats Counts
}

// Counts tallies install outcomes for the final summary.
type Counts struct {
	Installed int
	Skipped   int
	Failed    int
}

// Stats returns the running counts.
func (s *StreamReporter) Stats() Counts { return s.stats }

func (s *StreamReporter) Section(title string) {
	fmt.Fprintln(s.W)
	Section(s.W, s.Theme, title)
}

func (s *StreamReporter) Result(r installer.ItemResult) {
	id := identityFor(r.Package)
	switch r.Outcome {
	case installer.OutcomeInstalled:
		fmt.Fprintln(s.W, "  "+s.Theme.OK.Render("✓")+" "+id)
		s.stats.Installed++
	case installer.OutcomeAlreadyInstalled:
		fmt.Fprintln(s.W, "  "+s.Theme.Skip.Render("·")+" "+s.Theme.Skip.Render(id+" (already installed)"))
		s.stats.Skipped++
	case installer.OutcomeFailed:
		msg := id
		if r.Err != nil {
			msg = id + ": " + r.Err.Error()
		}
		fmt.Fprintln(s.W, "  "+s.Theme.Err.Render("✗")+" "+msg)
		s.stats.Failed++
	case installer.OutcomeSkipped:
		fmt.Fprintln(s.W, "  "+s.Theme.Skip.Render("·")+" "+s.Theme.Skip.Render(id))
		s.stats.Skipped++
	}
}

func (s *StreamReporter) Note(format string, a ...any) {
	fmt.Fprintln(s.W, "  "+s.Theme.Muted.Render(fmt.Sprintf(format, a...)))
}

func (s *StreamReporter) Warn(format string, a ...any) {
	fmt.Fprintln(s.W, "  "+s.Theme.Warn.Render("!")+" "+s.Theme.Warn.Render(fmt.Sprintf(format, a...)))
}

func identityFor(p snapshot.Package) string {
	switch p.Source {
	case snapshot.SourceFlatpak:
		if p.AppID != "" {
			return p.AppID
		}
	case snapshot.SourceVSCode:
		if p.ExtensionID != "" {
			return p.ExtensionID
		}
	}
	return p.Name
}

// isTTY reports whether ``f`` is a terminal.
func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// IsStdinTTY is exported for callers that need it.
func IsStdinTTY() bool { return isTTY(os.Stdin) }
