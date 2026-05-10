package cli

import (
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/installer"
	"github.com/benjaminbellamy/reliure/internal/picker"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// buildPickerPages converts a snapshot's package list into picker pages
// (one per source). ``checked`` is called per-package to determine the
// initial checkbox state — backup defaults to always-true, restore to
// always-false, and the "back to picker" path uses the user's previous
// selection. ``installed`` provides per-source installed-version maps so we
// can decorate items with [installed] / [installed: X] badges (used at
// restore time).
func buildPickerPages(s *snapshot.Snapshot, checked func(snapshot.Package) bool, installed *installer.InstalledVersions) []picker.Page {
	type srcDef struct {
		key     string // snapshot.Source value
		title   string
		install map[string]string // optional: installed-version map
		idFn    func(snapshot.Package) string
	}
	noInst := map[string]string{}
	apt, fp, sn, vs := noInst, noInst, noInst, noInst
	if installed != nil {
		apt, fp, sn, vs = installed.Apt, installed.Flatpak, installed.Snap, installed.VSCode
	}
	defs := []srcDef{
		{key: snapshot.SourceMounts, title: "mounted disks (fstab)", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceApt, title: "apt", install: apt,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceFlatpak, title: "flatpak", install: fp,
			idFn: func(p snapshot.Package) string {
				if p.AppID != "" {
					return p.AppID
				}
				return p.Name
			}},
		{key: snapshot.SourceSnap, title: "snap", install: sn,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceVSCode, title: "vscode", install: vs,
			idFn: func(p snapshot.Package) string {
				if p.ExtensionID != "" {
					return p.ExtensionID
				}
				return p.Name
			}},
		{key: snapshot.SourceGnomeExt, title: "gnome extensions", install: noInst,
			idFn: func(p snapshot.Package) string {
				if p.AppID != "" {
					return p.AppID
				}
				return p.Name
			}},
		{key: snapshot.SourcePip, title: "pip", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourcePipx, title: "pipx", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceCargo, title: "cargo", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceNpm, title: "npm", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceGo, title: "go binaries", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceOllama, title: "ollama models", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceAppImage, title: "appimages (manual re-download)", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceWifi, title: "wifi networks", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceVPN, title: "vpn connections", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
		{key: snapshot.SourceBluetooth, title: "bluetooth devices", install: noInst,
			idFn: func(p snapshot.Package) string { return p.Name }},
	}

	pages := []picker.Page{}
	for _, def := range defs {
		items := []picker.Item{}
		for _, p := range s.Packages {
			if p.Source != def.key || p.Unverified {
				continue
			}
			items = append(items, makePickerItem(p, def.idFn(p), def.install, checked(p)))
		}
		if len(items) == 0 {
			continue
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		pages = append(pages, picker.Page{ID: def.key, Title: def.title, Items: items})
	}

	// Inferred entries — surfaced as a separate "review" page; checked per
	// the predicate (backup may default to true; restore always false).
	inferred := []picker.Item{}
	for _, p := range s.Packages {
		if !p.Unverified {
			continue
		}
		label := p.Source + ":" + p.Name
		var badges []picker.Badge
		if p.Evidence != "" {
			badges = append(badges, picker.Badge{Kind: picker.BadgeNeutral, Text: p.Evidence})
		}
		inferred = append(inferred, picker.Item{
			ID:      p.ID,
			Label:   label,
			Checked: checked(p),
			Badges:  badges,
		})
	}
	if len(inferred) > 0 {
		sort.Slice(inferred, func(i, j int) bool { return inferred[i].Label < inferred[j].Label })
		pages = append(pages, picker.Page{
			ID:    "inferred",
			Title: "inferred (review carefully — not auto-installed)",
			Items: inferred,
		})
	}

	return pages
}

func makePickerItem(p snapshot.Package, identity string, installed map[string]string, defaultChecked bool) picker.Item {
	label := identity
	var badges []picker.Badge
	if p.Essential {
		badges = append(badges, picker.Badge{Kind: picker.BadgeEssential, Text: "[essential]"})
	}
	if p.OSPackage {
		badges = append(badges, picker.Badge{Kind: picker.BadgeOS, Text: "[os]"})
	}
	snapVer := strings.TrimSpace(p.Version)
	if snapVer == "" {
		snapVer = strings.TrimSpace(p.Branch)
	}
	insVer := installed[identity]
	switch {
	case insVer != "" && (snapVer == "" || insVer == snapVer):
		badges = append(badges, picker.Badge{Kind: picker.BadgeInstalled, Text: "[installed]"})
	case insVer != "":
		badges = append(badges, picker.Badge{Kind: picker.BadgeInstalled, Text: "[installed: " + insVer + "]"})
	}
	if snapVer != "" {
		badges = append(badges, picker.Badge{Kind: picker.BadgeVersion, Text: snapVer})
	}
	return picker.Item{
		ID:      p.ID,
		Label:   label,
		Checked: defaultChecked,
		Badges:  badges,
	}
}

// applyPickerSelections returns the subset of ``packages`` whose IDs are in
// the picker result.
func applyPickerSelections(packages []snapshot.Package, sel map[string][]string) []snapshot.Package {
	keep := map[string]struct{}{}
	for _, ids := range sel {
		for _, id := range ids {
			keep[id] = struct{}{}
		}
	}
	out := make([]snapshot.Package, 0, len(packages))
	for _, p := range packages {
		if _, ok := keep[p.ID]; ok {
			out = append(out, p)
		}
	}
	return out
}
