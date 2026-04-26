// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package scanner finds installed software via package managers and
// inference sources (shell history, filesystem fingerprints).
package scanner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// Scanner is implemented by every source detector. Implementations should
// be safe to call without root privileges: scanning is read-only.
type Scanner interface {
	// Name returns the source identifier embedded in resulting Package.Source
	// values (e.g. "apt", "flatpak", "history").
	Name() string

	// Available reports whether this scanner can run on the current system
	// (e.g. ``flatpak`` is on PATH).
	Available() bool

	// Scan returns the discovered packages.
	Scan(ctx context.Context) ([]snapshot.Package, error)
}

// Result is the outcome of running a single scanner.
type Result struct {
	Source        string
	Packages      []snapshot.Package
	Errors        []string
	SkippedReason string
}

// OK reports whether the scan completed without errors or skip.
func (r Result) OK() bool {
	return r.SkippedReason == "" && len(r.Errors) == 0
}

// Run is a helper that catches panics, applies a default timeout, and wraps
// the result in a Result.
func Run(ctx context.Context, s Scanner) Result {
	if !s.Available() {
		return Result{Source: s.Name(), SkippedReason: s.Name() + " not available"}
	}
	defer func() { _ = recover() }()

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	pkgs, err := s.Scan(ctx)
	if err != nil {
		return Result{Source: s.Name(), Errors: []string{fmt.Sprintf("%s: %v", s.Name(), err)}}
	}
	return Result{Source: s.Name(), Packages: pkgs}
}

// Have reports whether ``cmd`` is on PATH.
func Have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RunCmd executes ``cmd args...`` with the given context and returns stdout.
// stderr is discarded; non-zero exit returns an error wrapping the raw output.
func RunCmd(ctx context.Context, cmd string, args ...string) (string, error) {
	c := exec.CommandContext(ctx, cmd, args...)
	out, err := c.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("%s exit %d: %s", cmd, ee.ExitCode(), string(ee.Stderr))
		}
		return "", err
	}
	return string(out), nil
}
