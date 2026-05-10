// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/base64"
	"os"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// fstabPath is overridden in tests.
var fstabPath = "/etc/fstab"

// MountsScanner reads /etc/fstab and emits a Package per user-configured
// mount. Skips system entries (root, /boot*, swap, virtual filesystems) so
// the snapshot only carries data disks the user actually attached
// themselves — those are the ones a fresh install won't recreate.
//
// The full fstab line is preserved verbatim in Package.Payload (base64) so
// the restore installer can append it back idempotently. /etc/fstab is
// world-readable, so this scanner runs unprivileged.
type MountsScanner struct{}

func (MountsScanner) Name() string    { return snapshot.SourceMounts }
func (MountsScanner) Available() bool {
	_, err := os.Stat(fstabPath)
	return err == nil
}

func (MountsScanner) Scan(_ context.Context) ([]snapshot.Package, error) {
	data, err := os.ReadFile(fstabPath)
	if err != nil {
		return nil, err
	}
	pkgs := []snapshot.Package{}
	for _, line := range strings.Split(string(data), "\n") {
		entry, ok := parseFstabLine(line)
		if !ok {
			continue
		}
		if isSystemMount(entry) {
			continue
		}
		pkgs = append(pkgs, snapshot.Package{
			ID:       "mounts:" + entry.mountpoint,
			Name:     entry.mountpoint,
			Source:   snapshot.SourceMounts,
			Evidence: entry.spec,
			Payload:  base64.StdEncoding.EncodeToString([]byte(entry.raw)),
		})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// fstabEntry captures the four fields reliure cares about. raw is the
// untrimmed source line for verbatim round-trip.
type fstabEntry struct {
	spec       string // UUID=…, LABEL=…, /dev/…, host:/path, //host/share, …
	mountpoint string
	fstype     string
	options    string
	raw        string
}

// parseFstabLine returns (entry, true) for a real entry, (_, false) for
// blank/comment lines or malformed rows. fstab is whitespace-separated; the
// 5th and 6th fields (dump/pass) are integers we don't need.
func parseFstabLine(line string) (fstabEntry, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return fstabEntry{}, false
	}
	fields := strings.Fields(trimmed)
	if len(fields) < 4 {
		return fstabEntry{}, false
	}
	return fstabEntry{
		spec:       fields[0],
		mountpoint: fields[1],
		fstype:     fields[2],
		options:    fields[3],
		raw:        line,
	}, true
}

// virtualFstypes are kernel-managed pseudo-filesystems and swap. The new
// OS install creates these itself; a snapshot entry would either fight the
// installer or, in swap's case, point at a swap file/partition that no
// longer exists.
var virtualFstypes = map[string]bool{
	"swap": true, "proc": true, "sysfs": true, "tmpfs": true,
	"devpts": true, "devtmpfs": true, "cgroup": true, "cgroup2": true,
	"mqueue": true, "pstore": true, "efivarfs": true, "bpf": true,
	"securityfs": true, "debugfs": true, "tracefs": true, "fusectl": true,
	"configfs": true, "binfmt_misc": true, "hugetlbfs": true,
	"rpc_pipefs": true, "nfsd": true, "autofs": true, "squashfs": true,
	"ramfs": true, "none": true,
}

// isSystemMount returns true for entries the new OS install will recreate
// itself — root, kernel/EFI partitions, virtual filesystems, swap.
func isSystemMount(e fstabEntry) bool {
	if e.mountpoint == "/" {
		return true
	}
	if e.mountpoint == "/boot" || strings.HasPrefix(e.mountpoint, "/boot/") {
		return true
	}
	if virtualFstypes[strings.ToLower(e.fstype)] {
		return true
	}
	return false
}

// EnsureNofail returns the options column with ``nofail`` added when not
// already present. Used by the mounts installer at restore time so a
// missing disk doesn't drop the new system into an emergency shell at
// boot. nofail is appended at the end.
func EnsureNofail(options string) string {
	if options == "" {
		return "nofail"
	}
	for _, p := range strings.Split(options, ",") {
		if strings.TrimSpace(p) == "nofail" {
			return options
		}
	}
	return options + ",nofail"
}

// FstabLineMountpoint extracts the mountpoint from a single fstab line.
// Returns "" if the line is a comment, blank, or malformed. Used by the
// installer to dedupe against an existing fstab.
func FstabLineMountpoint(line string) string {
	e, ok := parseFstabLine(line)
	if !ok {
		return ""
	}
	return e.mountpoint
}

// RewriteFstabLineWithNofail returns ``line`` with nofail injected into the
// options column. If the line is malformed, returned unchanged.
func RewriteFstabLineWithNofail(line string) string {
	e, ok := parseFstabLine(line)
	if !ok {
		return line
	}
	if strings.Contains(","+e.options+",", ",nofail,") {
		return line
	}
	// Replace the options field while preserving original whitespace as much
	// as possible. Simplest: rebuild from fields, normalising to single tabs
	// — this matches the shape of a hand-edited fstab and stays valid.
	fields := strings.Fields(strings.TrimSpace(line))
	fields[3] = EnsureNofail(fields[3])
	return strings.Join(fields, "\t")
}
