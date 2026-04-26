// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

// Package system reads OS / host metadata (hostname, /etc/os-release).
package system

import (
	"bufio"
	"os"
	"strings"
)

// Hostname returns the system's hostname, or "unknown" on error.
func Hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}

// OSRelease parses /etc/os-release into a map. Returns an empty map if the
// file is absent or unreadable.
func OSRelease() map[string]string {
	out := map[string]string{}
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}

// PrettyName returns PRETTY_NAME (or NAME) from /etc/os-release.
func PrettyName() string {
	r := OSRelease()
	if v := r["PRETTY_NAME"]; v != "" {
		return v
	}
	if v := r["NAME"]; v != "" {
		return v
	}
	return "Unknown"
}

// Codename returns VERSION_CODENAME (or UBUNTU_CODENAME) from /etc/os-release.
func Codename() string {
	r := OSRelease()
	if v := r["VERSION_CODENAME"]; v != "" {
		return v
	}
	if v := r["UBUNTU_CODENAME"]; v != "" {
		return v
	}
	return ""
}
