package scanner

import (
	"context"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// VSCodeScanner enumerates VS Code extensions. Each line of output is a
// publisher.name extension id.
type VSCodeScanner struct{}

func (VSCodeScanner) Name() string    { return snapshot.SourceVSCode }
func (VSCodeScanner) Available() bool { return Have("code") }

func (VSCodeScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	out, err := RunCmd(ctx, "code", "--list-extensions")
	if err != nil {
		return nil, err
	}
	pkgs := []snapshot.Package{}
	for _, line := range strings.Split(out, "\n") {
		id := strings.TrimSpace(line)
		if id == "" || !strings.Contains(id, ".") {
			continue
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:          "vscode:" + id,
			Name:        id,
			Source:      snapshot.SourceVSCode,
			ExtensionID: id,
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].ExtensionID < pkgs[j].ExtensionID })
	return pkgs, nil
}
