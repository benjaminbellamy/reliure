package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// FlatpakInstaller installs apps one at a time. It also ensures the flathub
// remote is registered before running any per-app install.
type FlatpakInstaller struct{}

func (FlatpakInstaller) Source() string  { return snapshot.SourceFlatpak }
func (FlatpakInstaller) Available() bool { return have("flatpak") }

func (f FlatpakInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("flatpak")
	rep.SectionTotal(len(items))

	if err := ensureFlathub(ctx, opts, rep); err != nil {
		rep.Warn("could not ensure flathub remote: %v", err)
	}

	results := make([]ItemResult, 0, len(items))
	for _, p := range items {
		appID := p.AppID
		if appID == "" {
			appID = p.Name
		}
		remote := p.Remote
		if remote == "" {
			remote = "flathub"
		}
		if flatpakInstalled(appID) {
			r := ItemResult{Package: p, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err := runCtx(ctx, opts, "flatpak", "install", "-y", "--noninteractive", remote, appID)
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

func ensureFlathub(ctx context.Context, opts Options, rep Reporter) error {
	if !have("flatpak") {
		return nil
	}
	out, err := exec.Command("flatpak", "remotes", "--columns=name").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == "flathub" {
				return nil
			}
		}
	}
	rep.Note("adding flathub remote")
	return runCtx(ctx, opts, "flatpak", "remote-add", "--if-not-exists",
		"flathub", "https://flathub.org/repo/flathub.flatpakrepo")
}

func flatpakInstalled(appID string) bool {
	return exec.Command("flatpak", "info", appID).Run() == nil
}
