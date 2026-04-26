package installer

import (
	"context"
	"os/exec"
	"strings"
)

// InstalledVersions returns name/id → installed version maps for each source.
// Used by the picker to decorate items with [installed] / [installed: X]
// badges. All map values are best-effort — a missing key just means "not
// known to be installed."
type InstalledVersions struct {
	Apt     map[string]string
	Flatpak map[string]string
	Snap    map[string]string
	VSCode  map[string]string
}

// LoadInstalledVersions queries every package manager that's available and
// returns a populated InstalledVersions. Errors per source are swallowed —
// missing tools just mean an empty map for that source.
func LoadInstalledVersions(ctx context.Context) InstalledVersions {
	return InstalledVersions{
		Apt:     loadAptVersions(ctx),
		Flatpak: loadFlatpakVersions(ctx),
		Snap:    loadSnapVersions(ctx),
		VSCode:  loadVSCodeVersions(ctx),
	}
}

func loadAptVersions(ctx context.Context) map[string]string {
	out := map[string]string{}
	if !have("dpkg-query") {
		return out
	}
	body, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Package}\t${Version}\n").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(body), "\n") {
		name, ver, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		out[name] = ver
	}
	return out
}

func loadFlatpakVersions(ctx context.Context) map[string]string {
	out := map[string]string{}
	if !have("flatpak") {
		return out
	}
	body, err := exec.CommandContext(ctx, "flatpak", "list", "--app", "--columns=application,branch").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(body), "\n") {
		appID, branch, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		out[strings.TrimSpace(appID)] = strings.TrimSpace(branch)
	}
	return out
}

func loadSnapVersions(ctx context.Context) map[string]string {
	out := map[string]string{}
	if !have("snap") {
		return out
	}
	body, err := exec.CommandContext(ctx, "snap", "list").Output()
	if err != nil {
		return out
	}
	for i, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if i == 0 && fields[0] == "Name" {
			continue
		}
		out[fields[0]] = fields[1]
	}
	return out
}

func loadVSCodeVersions(ctx context.Context) map[string]string {
	out := map[string]string{}
	if !have("code") {
		return out
	}
	body, err := exec.CommandContext(ctx, "code", "--list-extensions", "--show-versions").Output()
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(body), "\n") {
		id, ver, ok := strings.Cut(strings.TrimSpace(line), "@")
		if !ok || id == "" {
			continue
		}
		out[id] = ver
	}
	return out
}
