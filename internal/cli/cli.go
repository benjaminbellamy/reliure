// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package cli wires the kong-driven command tree to the per-command
// implementations in sibling files.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

// Tagline is the project's one-liner; printed at the top of every banner.
const Tagline = "Format, Reinstall, Restore. A new clean system every 6 months."

// Copyright + license short-form. Long-form is in LICENSE.
const (
	Copyright = "Copyright (C) 2026 Benjamin Bellamy"
	License   = "License: GPLv3+ — GNU GPL version 3 or later <https://gnu.org/licenses/gpl.html>"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// CLI is the kong root.
type CLI struct {
	Backup   BackupCmd   `cmd:"" default:"withargs" help:"Scan the system, optionally pick what to keep, and write a snapshot. (default if no command)"`
	Snapshot SnapshotCmd `cmd:"" help:"Scan the system and write a snapshot YAML. No picker."`
	Restore  RestoreCmd  `cmd:"" help:"Read a snapshot YAML and run the picker + installs."`
	Report   ReportCmd   `cmd:"" help:"Render a snapshot as a styled, printable HTML report."`
	List     ListCmd     `cmd:"" help:"Print a styled tabular view of a snapshot."`
	Diff     DiffCmd     `cmd:"" help:"Compare two snapshots — show what was added/removed/changed."`
	Version  VersionCmd  `cmd:"" help:"Print the binary version."`
}

// VersionCmd is the ``reliure version`` subcommand.
type VersionCmd struct{}

// Run prints version, copyright, and license.
func (VersionCmd) Run() error {
	fmt.Printf("reliure %s\n%s\n\n%s\n%s\n", Version, Tagline, Copyright, License)
	return nil
}

// Execute parses os.Args and runs the chosen subcommand.
func Execute(ctx context.Context) int {
	var cli CLI
	desc := fmt.Sprintf("reliure %s — %s\n\n%s\n%s",
		Version, Tagline, Copyright, License)
	parser, err := kong.New(&cli,
		kong.Name("reliure"),
		kong.Description(desc),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cli init:", err)
		return 1
	}
	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}
	kctx.BindTo(ctx, (*context.Context)(nil))
	if err := kctx.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}
