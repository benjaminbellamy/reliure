package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// VSCodeInstaller installs extensions one at a time via ``code --install-extension``.
type VSCodeInstaller struct{}

func (VSCodeInstaller) Source() string  { return snapshot.SourceVSCode }
func (VSCodeInstaller) Available() bool { return have("code") }

func (v VSCodeInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("vscode")
	results := make([]ItemResult, 0, len(items))
	for _, p := range items {
		ext := p.ExtensionID
		if ext == "" {
			ext = p.Name
		}
		if vscodeInstalled(ext) {
			r := ItemResult{Package: p, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "code", "--install-extension", ext, "--force")
		out := OutcomeInstalled
		if err != nil {
			out = OutcomeFailed
		}
		r := ItemResult{Package: p, Outcome: out, Err: err}
		rep.Result(r)
		results = append(results, r)
	}
	return results
}

func vscodeInstalled(ext string) bool {
	out, err := exec.Command("code", "--list-extensions").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), ext) {
			return true
		}
	}
	return false
}
