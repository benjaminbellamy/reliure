// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// GoBinScanner enumerates ``go install``-ed binaries in $GOBIN (or
// $GOPATH/bin, default ~/go/bin) by running ``go version -m <bin>``:
//
//	/home/u/go/bin/staticcheck: go1.21.0
//		path	honnef.co/go/tools/cmd/staticcheck
//		mod	honnef.co/go/tools	v0.4.6	h1:abc…
//		dep	…
//
// We capture the ``path`` line as the canonical install identity (it's what
// ``go install <path>@<version>`` takes) and the ``mod`` version for restore
// pinning.
type GoBinScanner struct{}

func (GoBinScanner) Name() string    { return snapshot.SourceGo }
func (GoBinScanner) Available() bool { return Have("go") && GoBinDir() != "" }

func (GoBinScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	dir := GoBinDir()
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}
	paths := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if !isExecutableFile(full) {
			continue
		}
		paths = append(paths, full)
	}
	if len(paths) == 0 {
		return nil, nil
	}
	// Single batched `go version -m` for all binaries — much faster than
	// one subprocess per file.
	args := append([]string{"version", "-m"}, paths...)
	out, err := exec.CommandContext(ctx, "go", args...).Output()
	if err != nil {
		return nil, nil
	}
	pkgs := []snapshot.Package{}
	for _, info := range parseGoVersionM(string(out)) {
		if info.path == "" {
			continue // not a Go binary, or built without module info
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:      "go:" + info.path,
			Name:    info.path,
			Source:  snapshot.SourceGo,
			Version: info.version,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// GoBinDir resolves the directory ``go install`` writes binaries to.
// Honours $GOBIN, then $GOPATH/bin, then ~/go/bin. Returns "" if the home
// directory is unavailable and no env var resolves a path.
func GoBinDir() string {
	if b := strings.TrimSpace(os.Getenv("GOBIN")); b != "" {
		return b
	}
	if gp := strings.TrimSpace(os.Getenv("GOPATH")); gp != "" {
		// GOPATH may be colon-separated; first wins.
		first := strings.SplitN(gp, string(os.PathListSeparator), 2)[0]
		if first != "" {
			return filepath.Join(first, "bin")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "go", "bin")
}

type goBinInfo struct{ path, version string }

// parseGoVersionM parses the output of a multi-file ``go version -m`` call.
// Output format: each binary starts with a column-0 ``<file>: <details>``
// header, followed by tab-indented ``path``/``mod``/``dep``/``build`` lines.
// Non-Go binaries appear as a header with no following metadata.
func parseGoVersionM(out string) []goBinInfo {
	infos := []goBinInfo{}
	var cur *goBinInfo
	flush := func() {
		if cur != nil {
			infos = append(infos, *cur)
		}
	}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			flush()
			cur = &goBinInfo{}
			continue
		}
		if cur == nil {
			continue
		}
		fields := strings.Split(strings.TrimPrefix(line, "\t"), "\t")
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "path":
			cur.path = fields[1]
		case "mod":
			if len(fields) >= 3 {
				cur.version = fields[2]
			}
		}
	}
	flush()
	return infos
}

// GoBinBasenames returns the set of binary names in $GOBIN. Used by the
// manual scanner to skip Go-installed binaries, and by the installer to
// short-circuit ``go install`` when a binary is already present.
func GoBinBasenames() map[string]struct{} {
	out := map[string]struct{}{}
	dir := GoBinDir()
	if dir == "" {
		return out
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			out[e.Name()] = struct{}{}
		}
	}
	return out
}

