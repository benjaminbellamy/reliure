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
	"time"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// btRoot is the BlueZ paired-device store; overridden in tests.
var btRoot = "/var/lib/bluetooth"

const btSkipHint = "needs root — re-run with `sudo` to include bluetooth devices"

// BluetoothScanner reads paired Bluetooth devices from BlueZ's keystore at
// /var/lib/bluetooth/<adapter-mac>/<device-mac>/. The ``info`` file holds
// the link key plus the device's friendly name; reliure stores its full
// body base64-encoded in Package.Payload so restore can re-pair without
// putting the device back into pairing mode.
//
// /var/lib/bluetooth itself is mode 700 root, so this scanner needs root
// at scan time. When run unprivileged it returns SkipError so the user
// gets a "re-run with sudo" hint instead of a red error line — same
// pattern as the wifi/vpn scanners.
//
// Same-machine reinstall is the design centre: the adapter MAC is a
// hardware identifier baked into the controller, so it survives an OS
// reinstall and the directory layout matches one-to-one across the
// reformat.
type BluetoothScanner struct{}

func (BluetoothScanner) Name() string { return snapshot.SourceBluetooth }
func (BluetoothScanner) Available() bool {
	_, err := os.Stat(btRoot)
	return err == nil
}

func (BluetoothScanner) Scan(_ context.Context) ([]snapshot.Package, error) {
	adapters, err := os.ReadDir(btRoot)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, &SkipError{Reason: btSkipHint}
		}
		return nil, err
	}
	// Group candidates by friendly name. The same physical device may
	// appear under multiple adapters, in two flavours: Classic Bluetooth
	// devices (speakers, older keyboards) keep a fixed MAC across adapters
	// — easy to dedup. BLE devices (Logi M650, Apple Magic Mouse, …) use
	// resolvable random addresses, so the same mouse paired with two
	// adapters lands at two different MACs with two different link keys.
	// MAC-based dedup misses those. Name-based dedup catches both cases:
	// two info files labelled "Logi M650" are interchangeable from the
	// user's mental model, so we keep the one from the adapter whose dir
	// has the most recent mtime. Nameless devices fall back to the MAC so
	// they never collide with each other.
	type cand struct {
		pkg     snapshot.Package
		freshAt time.Time
	}
	freshest := map[string]cand{}
	needRoot := false
	for _, ad := range adapters {
		if !ad.IsDir() || !looksLikeMAC(ad.Name()) {
			continue
		}
		adapterDir := filepath.Join(btRoot, ad.Name())
		adapterInfo, _ := os.Stat(adapterDir)
		var adapterMtime time.Time
		if adapterInfo != nil {
			adapterMtime = adapterInfo.ModTime()
		}
		devices, err := os.ReadDir(adapterDir)
		if err != nil {
			if errors.Is(err, os.ErrPermission) {
				needRoot = true
			}
			continue
		}
		for _, dv := range devices {
			if !dv.IsDir() || !looksLikeMAC(dv.Name()) {
				continue
			}
			infoPath := filepath.Join(adapterDir, dv.Name(), "info")
			data, err := os.ReadFile(infoPath)
			if err != nil {
				if errors.Is(err, os.ErrPermission) {
					needRoot = true
				}
				continue
			}
			deviceMAC := dv.Name()
			parsedName := parseBluetoothName(data)
			displayName := parsedName
			if displayName == "" {
				displayName = deviceMAC
			}
			// Dedup key: the friendly name when we have one, else the
			// device MAC. Two nameless devices keep distinct keys; two
			// "Logi M650" entries collapse to one.
			dedupKey := parsedName
			if dedupKey == "" {
				dedupKey = deviceMAC
			}
			pkg := snapshot.Package{
				ID:       "bluetooth:" + deviceMAC,
				Name:     displayName,
				Source:   snapshot.SourceBluetooth,
				Evidence: infoPath,
				Payload:  base64.StdEncoding.EncodeToString(data),
			}
			if existing, ok := freshest[dedupKey]; !ok || adapterMtime.After(existing.freshAt) {
				freshest[dedupKey] = cand{pkg: pkg, freshAt: adapterMtime}
			}
		}
	}
	// /var/lib/bluetooth is mode 700 root; an unprivileged caller hits
	// permission-denied at the first ReadDir and gets nothing back. The
	// SkipError path turns this into a clean "re-run with sudo" hint.
	if needRoot && len(freshest) == 0 {
		return nil, &SkipError{Reason: btSkipHint}
	}
	pkgs := make([]snapshot.Package, 0, len(freshest))
	for _, c := range freshest {
		pkgs = append(pkgs, c.pkg)
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs, nil
}

// parseBluetoothName extracts ``Name=`` from the ``[General]`` section of
// a BlueZ info file. The file is INI-style; we only need one key, so a
// full parser is overkill.
func parseBluetoothName(data []byte) string {
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
		if section != "General" {
			continue
		}
		if eq := strings.IndexByte(line, '='); eq > 0 {
			key := strings.TrimSpace(line[:eq])
			if key == "Name" {
				return strings.TrimSpace(line[eq+1:])
			}
		}
	}
	return ""
}

// looksLikeMAC reports whether ``s`` is a 17-character ``XX:XX:XX:XX:XX:XX``
// MAC. Used to filter stray entries (BlueZ also drops a ``cache`` dir at
// the adapter level alongside device dirs).
func looksLikeMAC(s string) bool {
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

// BluetoothInfoPath returns the canonical install destination for the
// device with ``deviceMAC`` paired to ``adapterMAC``. Used by the
// bluetooth installer.
func BluetoothInfoPath(adapterMAC, deviceMAC string) string {
	return filepath.Join(btRoot, adapterMAC, deviceMAC, "info")
}

// BluetoothRoot returns the BlueZ root directory (``/var/lib/bluetooth``
// in production, overridable in tests).
func BluetoothRoot() string { return btRoot }
