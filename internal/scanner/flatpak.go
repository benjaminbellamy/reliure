package scanner

import (
	"context"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// FlatpakScanner enumerates installed flatpak applications. ``--app`` excludes
// runtimes/SDKs (which are pulled in as dependencies, not user-facing apps).
type FlatpakScanner struct{}

func (FlatpakScanner) Name() string    { return snapshot.SourceFlatpak }
func (FlatpakScanner) Available() bool { return Have("flatpak") }

func (FlatpakScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "flatpak", "list", "--app",
		"--columns=application,name,branch,origin")
	if err != nil {
		return nil, err
	}

	pkgs := []snapshot.Package{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "application\t") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		appID, name, branch, remote := parts[0], parts[1], parts[2], parts[3]
		display := name
		if display == "" {
			display = appID
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:     "flatpak:" + appID,
			Name:   display,
			Source: snapshot.SourceFlatpak,
			AppID:  appID,
			Remote: remote,
			Branch: branch,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].AppID < pkgs[j].AppID })
	return pkgs, nil
}
