// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// BluetoothInstaller writes paired-device ``info`` files back into BlueZ's
// keystore (``/var/lib/bluetooth/<adapter>/<device>/``) so the new system
// inherits the link keys without re-pairing each keyboard / mouse /
// speaker. After the batch, a single ``sudo systemctl restart bluetooth``
// makes BlueZ re-read the store.
//
// Same-machine reinstall is the design centre: the adapter MAC is a
// hardware identifier on the controller and survives an OS reformat, so
// the destination path matches the source path one-to-one. If the user is
// restoring on different hardware, the adapter dir won't exist on the new
// system and the device entry is skipped with a hint — same pattern as
// the mounts installer when a disk isn't attached.
type BluetoothInstaller struct{}

func (BluetoothInstaller) Source() string { return snapshot.SourceBluetooth }
func (BluetoothInstaller) Available() bool {
	_, err := os.Stat(scanner.BluetoothRoot())
	return err == nil
}

func (b BluetoothInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("bluetooth")
	rep.SectionTotal(len(items))
	results := make([]ItemResult, 0, len(items))

	wroteAny := false
	for _, item := range items {
		adapterMAC, deviceMAC := bluetoothMACs(item)
		if adapterMAC == "" || deviceMAC == "" {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing adapter/device MAC in evidence path %q", item.Evidence)}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		adapterDir := filepath.Join(scanner.BluetoothRoot(), adapterMAC)
		if _, err := os.Stat(adapterDir); err != nil {
			rep.Note("  • %s — adapter %s not present on this system, skipped (different hardware?)",
				item.Name, adapterMAC)
			r := ItemResult{Package: item, Outcome: OutcomeSkipped}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		target := scanner.BluetoothInfoPath(adapterMAC, deviceMAC)
		if _, err := os.Stat(target); err == nil {
			r := ItemResult{Package: item, Outcome: OutcomeAlreadyInstalled}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		decoded, err := base64.StdEncoding.DecodeString(item.Payload)
		if err != nil || len(decoded) == 0 {
			r := ItemResult{Package: item, Outcome: OutcomeFailed,
				Err: fmt.Errorf("missing or invalid payload — snapshot was scanned without sudo")}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		// Stage to a tempfile, then place atomically via ``sudo install``.
		// The device subdirectory needs to exist first — BlueZ uses 700/root
		// for both the adapter and device dirs, so we mkdir with the same
		// mode under sudo.
		tmp, err := os.CreateTemp("", "reliure-bt-*.info")
		if err != nil {
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: err}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		_, werr := tmp.Write(decoded)
		_ = tmp.Close()
		if werr != nil {
			_ = os.Remove(tmp.Name())
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: werr}
			rep.Result(r)
			results = append(results, r)
			continue
		}

		deviceDir := filepath.Dir(target)
		if err := runCtx(ctx, opts, "sudo",
			"install", "-d", "-m", "700", "-o", "root", "-g", "root", deviceDir); err != nil {
			_ = os.Remove(tmp.Name())
			r := ItemResult{Package: item, Outcome: OutcomeFailed, Err: err}
			rep.Result(r)
			results = append(results, r)
			continue
		}
		err = runCtx(ctx, opts, "sudo",
			"install", "-m", "600", "-o", "root", "-g", "root",
			tmp.Name(), target)
		_ = os.Remove(tmp.Name())
		out := OutcomeInstalled
		if err != nil {
			out = OutcomeFailed
		} else {
			wroteAny = true
		}
		r := ItemResult{Package: item, Outcome: out, Err: err}
		rep.Result(r)
		results = append(results, r)
	}

	// One restart covers the whole batch — far cheaper than per-file.
	if wroteAny {
		if err := runCtx(ctx, opts, "sudo", "systemctl", "restart", "bluetooth"); err != nil && !opts.DryRun {
			rep.Warn("systemctl restart bluetooth failed: %v", err)
		}
	}
	return results
}

// bluetoothMACs extracts (adapterMAC, deviceMAC) from the recorded
// ``Evidence`` path (``/var/lib/bluetooth/<adapter>/<device>/info``).
// Returns ("","") when the path doesn't match the expected shape — the
// caller turns that into a Failed outcome.
func bluetoothMACs(p snapshot.Package) (string, string) {
	if p.Evidence == "" {
		return "", ""
	}
	base := filepath.Base(p.Evidence)
	if base != "info" {
		return "", ""
	}
	deviceDir := filepath.Dir(p.Evidence)
	adapterDir := filepath.Dir(deviceDir)
	device := filepath.Base(deviceDir)
	adapter := filepath.Base(adapterDir)
	if !isMAC(adapter) || !isMAC(device) {
		return "", ""
	}
	return adapter, device
}

// isMAC is a stripped-down 17-char ``XX:XX:XX:XX:XX:XX`` check, mirroring
// scanner.looksLikeMAC. Duplicated here to avoid widening the scanner
// package's exported surface.
func isMAC(s string) bool {
	if len(s) != 17 {
		return false
	}
	for i, r := range s {
		if i%3 == 2 {
			if r != ':' {
				return false
			}
			continue
		}
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}
