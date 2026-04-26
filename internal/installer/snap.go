package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// SnapInstaller installs snaps one at a time, passing --channel when the
// snapshot recorded a non-stable tracking value.
type SnapInstaller struct{}

func (SnapInstaller) Source() string  { return snapshot.SourceSnap }
func (SnapInstaller) Available() bool { return have("snap") }

func (s SnapInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("snap")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))
	for _, p := range items {
		if snapInstalled(p.Name) {
			r := ItemResult{Package: p, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		args := []string{"snap", "install", p.Name}
		if p.Branch != "" && p.Branch != "latest/stable" {
			args = append(args, "--channel="+p.Branch)
		}
		err := runCtx(ctx, opts, "sudo", args...)
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

func snapInstalled(name string) bool {
	out, err := exec.Command("snap", "list", name).Output()
	if err != nil {
		return false
	}
	// Header line + at least one data line means it's installed.
	return strings.Count(string(out), "\n") >= 2
}
