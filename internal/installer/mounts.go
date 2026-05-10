// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

const fstabPath = "/etc/fstab"

// MountsInstaller appends user fstab entries (data disks, network mounts)
// from the snapshot to the new system's /etc/fstab — but only when the
// device spec actually resolves on the target machine. Same-disk reinstalls
// keep the same UUIDs / labels, so this is the common case.
//
// Two safety nets:
//   - We never duplicate: if any line in /etc/fstab already mounts the same
//     mountpoint, we skip and report ``already installed``.
//   - We always inject ``nofail`` into the options column. Without it, a
//     missing or late device drops the new system to an emergency shell at
//     boot. With it, missing devices are skipped and boot completes.
//
// Entries whose spec doesn't resolve at restore time (disk not attached, or
// LABEL/UUID gone) get a non-fatal "skipped" result + a hint — the user
// can attach the disk and re-run, or paste the line into fstab manually.
type MountsInstaller struct{}

func (MountsInstaller) Source() string  { return snapshot.SourceMounts }
func (MountsInstaller) Available() bool { return have("blkid") }

func (m MountsInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("mounts")
	rep.SectionTotal(len(items))

	existing := readExistingMountpoints()
	results := make([]ItemResult, 0, len(items))

	for _, item := range items {
		decoded, err := base64.StdEncoding.DecodeString(item.Payload)
		if err != nil || len(decoded) == 0 {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing or invalid payload")}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		line := strings.TrimRight(string(decoded), "\n")

		mp := scanner.FstabLineMountpoint(line)
		if mp != "" && existing[mp] {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		if !specResolves(ctx, item.Evidence) {
			rep.Note("  • %s — device %s not attached, skipped (paste line into /etc/fstab once attached)",
				item.Name, item.Evidence)
			r := ItemResult{Package: item, Outcome: OutcomeSkipped}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		safeLine := scanner.RewriteFstabLineWithNofail(line)
		if err := appendToFstab(ctx, opts, safeLine); err != nil {
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: err}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		if mp != "" {
			existing[mp] = true
		}
		r := ItemResult{Package: item, Outcome: OutcomeInstalled}
		rep.Result(r)
		results = append(results, r)
	}
	return results
}

// readExistingMountpoints returns the set of mountpoints already present in
// /etc/fstab. Used to skip duplicates without trying to install on top.
// A read error returns an empty set — the duplicate check then becomes
// best-effort, which is fine: appendToFstab still goes through sudo.
func readExistingMountpoints() map[string]bool {
	out := map[string]bool{}
	data, err := os.ReadFile(fstabPath)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		if mp := scanner.FstabLineMountpoint(line); mp != "" {
			out[mp] = true
		}
	}
	return out
}

// specResolves reports whether the device referenced by an fstab spec
// column is currently attached to the system. Network specs (NFS host:path,
// CIFS //host/share) bypass the check — we can't probe them cheaply
// pre-mount, and ``nofail`` covers the failure mode.
func specResolves(ctx context.Context, spec string) bool {
	switch {
	case strings.HasPrefix(spec, "UUID="):
		uuid := strings.TrimPrefix(spec, "UUID=")
		return blkidLookup(ctx, "-U", uuid)
	case strings.HasPrefix(spec, "LABEL="):
		label := strings.TrimPrefix(spec, "LABEL=")
		return blkidLookup(ctx, "-L", label)
	case strings.HasPrefix(spec, "PARTUUID="):
		// blkid has no -P flag; scan ``blkid`` output for the literal token.
		return blkidScanForToken(ctx, "PARTUUID", strings.TrimPrefix(spec, "PARTUUID="))
	case strings.HasPrefix(spec, "PARTLABEL="):
		return blkidScanForToken(ctx, "PARTLABEL", strings.TrimPrefix(spec, "PARTLABEL="))
	case strings.HasPrefix(spec, "/dev/"):
		_, err := os.Stat(spec)
		return err == nil
	case strings.HasPrefix(spec, "//"), strings.Contains(spec, ":/"):
		// CIFS / NFS / sshfs — assume reachable and let nofail handle the
		// boot-time miss.
		return true
	}
	return false
}

// blkidLookup runs ``blkid -U <uuid>`` or ``blkid -L <label>``. Exit 0
// means the token resolves to a device.
func blkidLookup(ctx context.Context, flag, value string) bool {
	cmd := exec.CommandContext(ctx, "blkid", flag, value)
	return cmd.Run() == nil
}

// blkidScanForToken scans ``blkid`` output for a literal ``KEY="VALUE"``
// pair. Used for PARTUUID/PARTLABEL since blkid has no direct -P lookup.
func blkidScanForToken(ctx context.Context, key, value string) bool {
	out, err := exec.CommandContext(ctx, "blkid").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), key+`="`+value+`"`)
}

// appendToFstab adds ``line`` to /etc/fstab via sudo. We use ``tee -a``
// rather than ``sh -c 'echo … >> file'`` because the redirection has to
// happen under the elevated shell, and tee gives us a single sudo prompt
// per call without a nested shell.
func appendToFstab(ctx context.Context, opts Options, line string) error {
	payload := "\n" + line + "\n"
	if opts.DryRun {
		fmt.Fprintf(opts.Stdout, "  $ printf %q | sudo tee -a %s\n", payload, fstabPath)
		return nil
	}
	cmd := exec.CommandContext(ctx, "sudo", "tee", "-a", fstabPath)
	cmd.Stdin = strings.NewReader(payload)
	cmd.Stdout = nil // tee echoes stdin to stdout; suppress the noise
	cmd.Stderr = opts.Stderr
	return cmd.Run()
}
