// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package report renders a snapshot as a styled standalone HTML document.
// The HTML is print-friendly: open it in a browser and use Cmd/Ctrl-P →
// "Save as PDF" for a beautiful printable artefact.
package report

import (
	_ "embed"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

//go:embed report.html.tmpl
var htmlTemplate string

// View is the data model the HTML template renders against.
type View struct {
	Snapshot  *snapshot.Snapshot
	Sections  []Section
	Stats     Stats
	Generated string
	Version   string
	Tagline   string
	Copyright string
}

// Section groups items by source for the report's body.
type Section struct {
	ID    string
	Title string
	Items []Item
	Total int
}

// Item is one row in a section's table.
type Item struct {
	Name     string
	Version  string
	Subtitle string  // contextual extra (e.g. "flathub", evidence)
	Badges   []Badge
}

// Badge is a styled tag rendered next to an item.
type Badge struct {
	Kind string // matches a CSS class: "essential", "os", "unverified"
	Text string
}

// Stats is the at-a-glance count block at the top of the report.
type Stats struct {
	Total      int
	Essential  int
	OS         int
	Unverified int
	BySource   map[string]int
}

var sectionOrder = []struct {
	id, title, source string
}{
	{"mounts", "Mounted disks (fstab)", snapshot.SourceMounts},
	{"apt", "apt", snapshot.SourceApt},
	{"flatpak", "flatpak", snapshot.SourceFlatpak},
	{"snap", "snap", snapshot.SourceSnap},
	{"vscode", "VS Code extensions", snapshot.SourceVSCode},
	{"gnome-ext", "GNOME Shell extensions", snapshot.SourceGnomeExt},
	{"pip", "pip", snapshot.SourcePip},
	{"pipx", "pipx", snapshot.SourcePipx},
	{"cargo", "cargo", snapshot.SourceCargo},
	{"npm", "npm", snapshot.SourceNpm},
	{"go", "Go binaries (go install)", snapshot.SourceGo},
	{"ollama", "Ollama models", snapshot.SourceOllama},
	{"appimage", "AppImages (manual re-download)", snapshot.SourceAppImage},
	{"wifi", "Wi-Fi networks", snapshot.SourceWifi},
	{"vpn", "VPN connections", snapshot.SourceVPN},
	{"bluetooth", "Bluetooth devices", snapshot.SourceBluetooth},
	{"udev", "Hardware permission rules (udev)", snapshot.SourceUdev},
	{"history", "Inferred from shell history", snapshot.SourceHistory},
	{"manual", "Manual installs", snapshot.SourceManual},
}

// RenderHTML writes a complete standalone HTML document for ``snap`` to ``w``.
func RenderHTML(snap *snapshot.Snapshot, version, tagline, copyright string, w io.Writer) error {
	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(w, buildView(snap, version, tagline, copyright))
}

func buildView(snap *snapshot.Snapshot, version, tagline, copyright string) View {
	bySource := map[string][]snapshot.Package{}
	for _, p := range snap.Packages {
		bySource[p.Source] = append(bySource[p.Source], p)
	}

	sections := []Section{}
	for _, def := range sectionOrder {
		raw := bySource[def.source]
		if len(raw) == 0 {
			continue
		}
		// Stable order within section: by name.
		sort.Slice(raw, func(i, j int) bool { return raw[i].Name < raw[j].Name })
		section := Section{ID: def.id, Title: def.title, Total: len(raw)}
		for _, p := range raw {
			section.Items = append(section.Items, makeItem(p))
		}
		sections = append(sections, section)
	}

	stats := Stats{Total: len(snap.Packages), BySource: map[string]int{}}
	for _, p := range snap.Packages {
		stats.BySource[p.Source]++
		if p.Essential {
			stats.Essential++
		}
		if p.OSPackage {
			stats.OS++
		}
		if p.Unverified {
			stats.Unverified++
		}
	}

	return View{
		Snapshot:  snap,
		Sections:  sections,
		Stats:     stats,
		Generated: time.Now().Format("2006-01-02 15:04 MST"),
		Version:   version,
		Tagline:   tagline,
		Copyright: copyright,
	}
}

func makeItem(p snapshot.Package) Item {
	name := p.Name
	switch {
	case p.Source == snapshot.SourceFlatpak && p.AppID != "":
		name = p.AppID
	case p.Source == snapshot.SourceVSCode && p.ExtensionID != "":
		name = p.ExtensionID
	case p.Source == snapshot.SourceGnomeExt && p.AppID != "":
		name = p.AppID
	}

	item := Item{Name: name, Version: p.Version}
	if item.Version == "" {
		item.Version = p.Branch
	}

	subtitleParts := []string{}
	switch p.Source {
	case snapshot.SourceFlatpak:
		if p.Name != "" && p.Name != p.AppID {
			subtitleParts = append(subtitleParts, p.Name)
		}
		if p.Remote != "" {
			subtitleParts = append(subtitleParts, p.Remote)
		}
	case snapshot.SourceSnap:
		if p.Branch != "" {
			subtitleParts = append(subtitleParts, p.Branch)
		}
	case snapshot.SourceGnomeExt:
		if p.Name != "" && p.Name != p.AppID {
			subtitleParts = append(subtitleParts, p.Name)
		}
		if p.URL != "" {
			subtitleParts = append(subtitleParts, p.URL)
		}
	case snapshot.SourceManual, snapshot.SourceHistory:
		if p.Source == snapshot.SourceHistory {
			subtitleParts = append(subtitleParts, p.Source+" · "+p.Name)
		}
		if p.DetectedVia != "" {
			subtitleParts = append(subtitleParts, p.DetectedVia)
		}
		if p.Evidence != "" {
			subtitleParts = append(subtitleParts, p.Evidence)
		}
	}
	item.Subtitle = strings.Join(subtitleParts, " · ")

	if p.Essential {
		item.Badges = append(item.Badges, Badge{Kind: "essential", Text: "essential"})
	}
	if p.OSPackage {
		item.Badges = append(item.Badges, Badge{Kind: "os", Text: "os"})
	}
	if p.Unverified {
		item.Badges = append(item.Badges, Badge{Kind: "unverified", Text: "unverified"})
	}
	return item
}
