// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package settings backs up and restores desktop-environment settings.
//
// Currently only GNOME (dconf) is supported. The dump format is plain text:
// reviewable, editable, diff-friendly, round-trippable via ``dconf load``.
package settings

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const dconfPath = "/org/gnome/"

// GNOMEAvailable reports whether ``dconf`` is on PATH.
func GNOMEAvailable() bool {
	_, err := exec.LookPath("dconf")
	return err == nil
}

// DumpGNOME runs ``dconf dump /org/gnome/`` and writes the result to ``output``.
// Returns the number of bytes written, or 0 if dconf is unavailable / output empty.
func DumpGNOME(ctx context.Context, output string) (int, error) {
	if !GNOMEAvailable() {
		return 0, nil
	}
	cmd := exec.CommandContext(ctx, "dconf", "dump", dconfPath)
	body, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("dconf dump: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return 0, nil
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(output, body, 0o644); err != nil {
		return 0, err
	}
	return len(body), nil
}

// LoadGNOME pipes the file at ``input`` through ``dconf load /org/gnome/``.
func LoadGNOME(ctx context.Context, input string) error {
	if !GNOMEAvailable() {
		return fmt.Errorf("dconf is not installed (try: sudo apt install -y dconf-cli)")
	}
	body, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}
	cmd := exec.CommandContext(ctx, "dconf", "load", dconfPath)
	cmd.Stdin = strings.NewReader(string(body))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DefaultDconfFilename returns "reliure-gnome-YYYYMMDD.dconf" given a snapshot
// date in ISO 8601.
func DefaultDconfFilename(snapshotDateISO string) string {
	short := snapshotDateISO
	if len(short) >= 10 {
		short = short[:10]
	}
	compact := strings.ReplaceAll(short, "-", "")
	return "reliure-gnome-" + compact + ".dconf"
}
