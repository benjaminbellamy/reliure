package scanner

import (
	"context"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// infrastructureSnaps are always installed automatically as bases / runtimes
// and don't reflect user choices, so we drop them from the snapshot.
var infrastructureSnaps = map[string]struct{}{
	"bare":                       {},
	"core":                       {},
	"core18":                     {},
	"core20":                     {},
	"core22":                     {},
	"core24":                     {},
	"snapd":                      {},
	"snapd-desktop-integration": {},
}

// SnapScanner enumerates installed snaps. ``snap list`` has no --json mode,
// so we parse the columnar output (skipping the "Name Version Rev Tracking…"
// header).
type SnapScanner struct{}

func (SnapScanner) Name() string    { return snapshot.SourceSnap }
func (SnapScanner) Available() bool { return Have("snap") }

func (SnapScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "snap", "list")
	if err != nil {
		return nil, err
	}

	pkgs := []snapshot.Package{}
	for i, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		// Skip the header line (only when it's the first non-empty line).
		if i == 0 && fields[0] == "Name" {
			continue
		}
		name, version, _, tracking := fields[0], fields[1], fields[2], fields[3]
		if _, infra := infrastructureSnaps[name]; infra {
			continue
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:      "snap:" + name,
			Name:    name,
			Source:  snapshot.SourceSnap,
			Version: version,
			Branch:  tracking, // used as --channel on restore
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}
