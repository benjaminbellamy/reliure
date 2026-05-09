// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

const nmSkipHint = "needs root — re-run with `sudo` to include networks"

// nmConnection holds the parsed-enough-for-labelling view of an
// .nmconnection file plus its raw bytes (round-tripped verbatim into
// Package.Payload).
type nmConnection struct {
	file string // basename, e.g. "Home WiFi.nmconnection"
	raw  []byte
	id   string // [connection] id=
	typ  string // [connection] type=
	ssid string // [wifi] ssid=
}

// parseNMConnection extracts the fields used to label the entry. The format
// is INI-style; we only need a handful of keys, so a full parser is
// overkill. The raw bytes are preserved untouched for restore.
func parseNMConnection(name string, data []byte) nmConnection {
	nm := nmConnection{file: name, raw: data}
	section := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		switch {
		case section == "connection" && key == "id":
			nm.id = val
		case section == "connection" && key == "type":
			nm.typ = val
		case section == "wifi" && key == "ssid":
			nm.ssid = val
		}
	}
	if nm.id == "" {
		nm.id = strings.TrimSuffix(name, ".nmconnection")
	}
	return nm
}

// nmKindMatches sorts NM type strings into the coarse buckets reliure
// surfaces. Ethernet / loopback / bridge / bluetooth are deliberately
// ignored — they rarely carry secrets a user needs back after a reinstall.
func nmKindMatches(connType, kind string) bool {
	switch kind {
	case "wifi":
		return connType == "802-11-wireless" || connType == "wifi"
	case "vpn":
		return connType == "vpn" || connType == "wireguard"
	}
	return false
}

// nmListEntry is one row from ``nmcli -t -f NAME,TYPE,FILENAME,UUID connection show``.
type nmListEntry struct {
	name, typ, filename, uuid string
}

// listNMConnections asks nmcli for every known connection and where its
// keyfile lives. nmcli is the source of truth: depending on the distro the
// files live in ``/etc/NetworkManager/system-connections/`` (vanilla
// keyfile plugin) or ``/run/NetworkManager/system-connections/`` (Ubuntu's
// netplan-managed setup, where files are regenerated on boot from
// /etc/netplan/*.yaml). nmcli abstracts that away.
func listNMConnections(ctx context.Context) ([]nmListEntry, error) {
	out, err := RunCmd(ctx, "nmcli", "-t", "-f", "NAME,TYPE,FILENAME,UUID", "connection", "show")
	if err != nil {
		return nil, err
	}
	rows := []nmListEntry{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		parts := splitNMCLIRecord(line)
		if len(parts) < 4 {
			continue
		}
		rows = append(rows, nmListEntry{
			name: parts[0], typ: parts[1], filename: parts[2], uuid: parts[3],
		})
	}
	return rows, nil
}

// splitNMCLIRecord splits a ``-t``-mode nmcli row by ``:``, honouring
// ``\:`` as an embedded colon and ``\\`` as a literal backslash. nmcli
// emits these escapes when a NAME contains a colon — without unescaping
// the FILENAME column would shift.
func splitNMCLIRecord(s string) []string {
	parts := []string{}
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == ':' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	parts = append(parts, cur.String())
	return parts
}

// scanNMConnections enumerates connections of ``kind`` (``wifi`` or
// ``vpn``) via nmcli, then reads each keyfile and emits a Package with the
// full body base64-encoded into Payload. Returns SkipError when at least
// one matching file exists but is permission-denied — the typical
// unprivileged-but-NM-is-running case.
func scanNMConnections(ctx context.Context, source, kind string) ([]snapshot.Package, error) {
	rows, err := listNMConnections(ctx)
	if err != nil {
		return nil, err
	}
	pkgs := []snapshot.Package{}
	needRoot := false
	for _, r := range rows {
		if !nmKindMatches(r.typ, kind) {
			continue
		}
		if r.filename == "" {
			// Memory-only / D-Bus-only connection — nothing on disk to
			// snapshot. Skip silently.
			continue
		}
		data, err := os.ReadFile(r.filename)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				needRoot = true
				continue
			}
			// File gone or transient I/O error — skip this entry.
			continue
		}
		nm := parseNMConnection(filepath.Base(r.filename), data)
		name := nm.id
		if kind == "wifi" && nm.ssid != "" {
			name = nm.ssid
		}
		if name == "" {
			name = r.name
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:       source + ":" + r.name,
			Name:     name,
			Source:   source,
			Evidence: r.filename,
			Payload:  base64.StdEncoding.EncodeToString(data),
		})
	}
	// Any unreadable matching file means the user almost certainly wants
	// the wifi/vpn scan but didn't run with sudo. Surface a SkipError —
	// even if some files were readable — so they re-run cleanly rather
	// than ending up with a half-populated snapshot.
	if needRoot {
		return nil, &SkipError{Reason: nmSkipHint}
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// WifiScanner enumerates NetworkManager wifi connections via nmcli,
// reading the keyfiles whose paths nmcli reports. The full file text
// (including PSK) is stored base64-encoded in Package.Payload — the
// snapshot YAML therefore contains plaintext-equivalent secrets and
// should be treated as sensitive.
type WifiScanner struct{}

func (WifiScanner) Name() string    { return snapshot.SourceWifi }
func (WifiScanner) Available() bool { return Have("nmcli") }
func (WifiScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	return scanNMConnections(ctx, snapshot.SourceWifi, "wifi")
}

// VPNScanner enumerates ``type=vpn`` (OpenVPN / PPTP / etc.) and
// ``type=wireguard`` connections.
type VPNScanner struct{}

func (VPNScanner) Name() string    { return snapshot.SourceVPN }
func (VPNScanner) Available() bool { return Have("nmcli") }
func (VPNScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	return scanNMConnections(ctx, snapshot.SourceVPN, "vpn")
}
