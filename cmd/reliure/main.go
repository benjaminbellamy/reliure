// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy
//
// reliure — Format, Reinstall, Restore. A new clean system every 6 months.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version. See LICENSE for the full text.

// Command reliure scans your Linux system, builds a YAML list of installed
// software, and (separately) drives an interactive picker + installer to
// restore that list onto a fresh system.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/benjaminbellamy/reliure/internal/cli"
)

// version is overridden at build time:
//
//	go build -ldflags="-X main.version=v1.2.3" ./cmd/reliure
var version = "dev"

func main() {
	cli.Version = version

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel the context on SIGINT/SIGTERM so child processes (sudo, dconf,
	// etc.) get a chance to clean up.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		cancel()
	}()

	os.Exit(cli.Execute(ctx))
}
