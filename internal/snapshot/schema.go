// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package snapshot defines the YAML data model and serialisation for reliure.
package snapshot

import (
	"fmt"
	"time"
)

// ReliureVersion is the schema version embedded in every snapshot.
const ReliureVersion = "1.0"

// Source identifies a package manager or inference source. Kept as a free-form
// string (not enum-typed) so future scanners can add values without breaking
// snapshots written by older binaries.
type Source = string

// Common source values.
const (
	SourceApt      Source = "apt"
	SourceFlatpak  Source = "flatpak"
	SourceSnap     Source = "snap"
	SourceVSCode   Source = "vscode"
	SourcePip      Source = "pip"
	SourcePipx     Source = "pipx"
	SourceCargo    Source = "cargo"
	SourceNpm      Source = "npm"
	SourceOllama   Source = "ollama"
	SourceGo       Source = "go"
	SourceAppImage Source = "appimage"
	SourceWifi     Source = "wifi"
	SourceVPN      Source = "vpn"
	SourceManual   Source = "manual"
	SourceHistory  Source = "history"
	SourceGnomeExt Source = "gnome-ext"
)

// Package is a single installed (or inferred) artefact.
//
// Field order matters: goccy/go-yaml emits fields in declaration order, which
// matches Python's to_dict ordering. ``essential`` and ``notes`` are always
// emitted (no omitempty) so the YAML always has a place to flip a flag or
// jot a note when editing by hand. Optional fields use omitempty.
type Package struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Source      Source `yaml:"source"`
	Version     string `yaml:"version,omitempty"`
	AppID       string `yaml:"app_id,omitempty"`
	Remote      string `yaml:"remote,omitempty"`
	Branch      string `yaml:"branch,omitempty"`
	ExtensionID string `yaml:"extension_id,omitempty"`
	Crate       string `yaml:"crate,omitempty"`
	URL         string `yaml:"url,omitempty"`
	Payload     string `yaml:"payload,omitempty"` // base64 — currently used by wifi/vpn for the .nmconnection body
	DetectedVia string `yaml:"detected_via,omitempty"`
	Evidence    string `yaml:"evidence,omitempty"`
	RestoreHint string `yaml:"restore_hint,omitempty"`
	OSPackage   bool   `yaml:"os_package,omitempty"`
	Unverified  bool   `yaml:"unverified,omitempty"`
	Essential   bool   `yaml:"essential"`
	Notes       string `yaml:"notes"`
}

// NaturalKey returns a (source, canonical-name) pair used for dedup across
// direct and inferred entries that refer to the same package. Direct and
// inferred IDs differ in form (``apt:htop`` vs ``history:apt:htop``), but
// both should be considered the same package for dedup purposes.
func (p Package) NaturalKey() [2]string {
	switch p.Source {
	case SourceFlatpak:
		if p.AppID != "" {
			return [2]string{SourceFlatpak, p.AppID}
		}
	case SourceVSCode:
		if p.ExtensionID != "" {
			return [2]string{SourceVSCode, p.ExtensionID}
		}
	}
	return [2]string{p.Source, p.Name}
}

// SnapshotMeta is the header block: when, where, what.
type SnapshotMeta struct {
	Date       string `yaml:"date"` // ISO 8601 with timezone offset
	Hostname   string `yaml:"hostname"`
	OS         string `yaml:"os"`
	OSCodename string `yaml:"os_codename"`
}

// NewMeta builds a SnapshotMeta with the current time as Date.
func NewMeta(hostname, osName, osCodename string) SnapshotMeta {
	return SnapshotMeta{
		Date:       time.Now().Format("2006-01-02T15:04:05-07:00"),
		Hostname:   hostname,
		OS:         osName,
		OSCodename: osCodename,
	}
}

// DateShort returns the YYYY-MM-DD portion of the snapshot's date.
func (m SnapshotMeta) DateShort() string {
	if len(m.Date) >= 10 {
		return m.Date[:10]
	}
	return m.Date
}

// DateCompact returns the YYYYMMDD form used in default output filenames.
func (m SnapshotMeta) DateCompact() string {
	s := m.DateShort()
	out := make([]byte, 0, 8)
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			out = append(out, s[i])
		}
	}
	return string(out)
}

// Snapshot is the top-level YAML document.
type Snapshot struct {
	ReliureVersion string       `yaml:"reliure_version"`
	Meta           SnapshotMeta `yaml:"snapshot"`
	Packages       []Package    `yaml:"packages"`
}

// New builds a fresh snapshot with the schema version pre-filled.
func New(meta SnapshotMeta) *Snapshot {
	return &Snapshot{
		ReliureVersion: ReliureVersion,
		Meta:           meta,
		Packages:       []Package{},
	}
}

// Dedupe drops inferred entries whose natural key matches any direct entry.
// Direct sources always win.
func (s *Snapshot) Dedupe() {
	seenDirect := make(map[[2]string]struct{})
	for _, p := range s.Packages {
		if !p.Unverified {
			seenDirect[p.NaturalKey()] = struct{}{}
		}
	}
	kept := s.Packages[:0]
	for _, p := range s.Packages {
		if p.Unverified {
			if _, hit := seenDirect[p.NaturalKey()]; hit {
				continue
			}
		}
		kept = append(kept, p)
	}
	s.Packages = kept
}

// DefaultRestoreScriptName is "reliure-restore-YYYYMMDD.sh", kept for
// continuity with the Python tool's filename convention even though we no
// longer emit a bash restore script.
func (s *Snapshot) DefaultRestoreScriptName() string {
	return fmt.Sprintf("reliure-restore-%s.sh", s.Meta.DateCompact())
}
