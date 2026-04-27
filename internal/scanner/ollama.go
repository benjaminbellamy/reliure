// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// OllamaScanner enumerates locally pulled Ollama models.
//
// Output of ``ollama list``:
//
//	NAME                       ID              SIZE      MODIFIED
//	llama3.2:latest            a80c4f17acd5    2.0 GB    7 weeks ago
//	qwen3:8b                   500a1f067a9f    5.2 GB    3 weeks ago
//	kimi-k2.6:cloud            a90cd0d1590c    -         5 days ago
//
// The ``model:tag`` string in NAME is the canonical install identity — it's
// what gets passed to ``ollama pull``. Cloud entries (SIZE = ``-``) are
// included as-is; ``ollama pull`` handles them when the user is authenticated.
type OllamaScanner struct{}

func (OllamaScanner) Name() string    { return snapshot.SourceOllama }
func (OllamaScanner) Available() bool { return Have("ollama") }

var ollamaColRE = regexp.MustCompile(`\s{2,}`)

func (OllamaScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "ollama", "list")
	if err != nil {
		return nil, err
	}
	pkgs := []snapshot.Package{}
	for _, row := range parseOllamaList(out) {
		pkgs = append(pkgs, snapshot.Package{
			ID:      "ollama:" + row.name,
			Name:    row.name,
			Source:  snapshot.SourceOllama,
			Version: row.size,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

type ollamaRow struct{ name, size string }

func parseOllamaList(out string) []ollamaRow {
	rows := []ollamaRow{}
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "NAME") {
			continue
		}
		fields := ollamaColRE.Split(line, -1)
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		row := ollamaRow{name: fields[0]}
		if len(fields) >= 3 && fields[2] != "-" {
			row.size = fields[2]
		}
		rows = append(rows, row)
	}
	return rows
}

// OllamaInstalledModels returns the set of model:tag identifiers currently
// pulled. Used by the installer to skip ``ollama pull`` when a model is
// already present. Empty map on any error (missing tool, parse failure).
func OllamaInstalledModels() map[string]struct{} {
	out := map[string]struct{}{}
	if !Have("ollama") {
		return out
	}
	body, err := RunCmd(context.Background(), "ollama", "list")
	if err != nil {
		return out
	}
	for _, row := range parseOllamaList(body) {
		out[row.name] = struct{}{}
	}
	return out
}
