// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package installer runs the install commands at restore time. Each source
// has its own implementation; they all share the same dry-run / status
// reporting plumbing here.
package installer

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// Outcome enumerates per-item install results.
type Outcome int

const (
	OutcomeInstalled Outcome = iota
	OutcomeAlreadyInstalled
	OutcomeFailed
	OutcomeSkipped
)

func (o Outcome) String() string {
	switch o {
	case OutcomeInstalled:
		return "installed"
	case OutcomeAlreadyInstalled:
		return "already installed"
	case OutcomeFailed:
		return "failed"
	case OutcomeSkipped:
		return "skipped"
	}
	return "unknown"
}

// ItemResult is the outcome for one Package.
type ItemResult struct {
	Package snapshot.Package
	Outcome Outcome
	Err     error
}

// Reporter is the channel through which installers tell the UI what's
// happening. The CLI implements this with a Lip Gloss-styled streaming view.
type Reporter interface {
	Section(title string)         // "apt", "flatpak", …
	SectionTotal(n int)           // pre-declares item count → enables "[i/n]" prefix on Results
	Result(r ItemResult)          // per-item outcome
	Note(format string, a ...any) // prose narration
	Warn(format string, a ...any)
}

// Options control the installer behaviour. ``DryRun`` prints commands rather
// than executing them. ``Stdin`` and ``Stdout``/``Stderr`` are the FDs the
// child commands inherit (so sudo can prompt the user on the controlling TTY).
type Options struct {
	DryRun bool
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultOptions wires the child commands to the parent process's TTY.
func DefaultOptions() Options {
	return Options{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Installer is the per-source interface. Every package manager (apt, flatpak,
// snap, vscode) has its own implementation.
type Installer interface {
	// Source name (matches snapshot.Source values).
	Source() string
	// Available reports whether the corresponding tool is on PATH.
	Available() bool
	// Install runs the install commands for ``items`` and reports per-item
	// outcomes via ``rep``.
	Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult
}

// run executes ``cmd args...`` honouring DryRun. Output is wired to the user's
// terminal; sudo prompts therefore work.
func run(opts Options, name string, args ...string) error {
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "  $ %s %s\n", name, joinArgs(args))
		return nil
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	return cmd.Run()
}

func runCtx(ctx context.Context, opts Options, name string, args ...string) error {
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "  $ %s %s\n", name, joinArgs(args))
		return nil
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = opts.Stdin
	cmd.Stdout = opts.Stdout
	cmd.Stderr = opts.Stderr
	return cmd.Run()
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// Filter returns the subset of ``items`` matching ``source``.
func Filter(items []snapshot.Package, source string) []snapshot.Package {
	out := make([]snapshot.Package, 0)
	for _, p := range items {
		if p.Source == source && !p.Unverified {
			out = append(out, p)
		}
	}
	return out
}
