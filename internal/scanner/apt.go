// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// AptScanner enumerates manually-installed apt packages (apt-mark showmanual)
// and looks up their versions via dpkg-query. It deliberately ignores
// auto-installed dependencies — that list is typically thousands of entries
// long and useless for restoration.
//
// Each package is also tagged ``OSPackage=true`` if dpkg's install log
// timestamps the package as part of the day-zero install burst (i.e. it
// arrived with the OS, not later). See aptOSPackages.
type AptScanner struct{}

func (AptScanner) Name() string    { return snapshot.SourceApt }
func (AptScanner) Available() bool { return Have("apt-mark") && Have("dpkg-query") }

func (AptScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	manualOut, err := RunCmd(ctx, "apt-mark", "showmanual")
	if err != nil {
		return nil, err
	}

	manual := map[string]struct{}{}
	for _, line := range strings.Split(manualOut, "\n") {
		name := strings.TrimSpace(line)
		if name != "" {
			manual[name] = struct{}{}
		}
	}
	if len(manual) == 0 {
		return nil, nil
	}

	versions, _ := aptVersions(ctx)
	osSet := aptOSPackages() // best-effort; empty set on failure

	names := make([]string, 0, len(manual))
	for n := range manual {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]snapshot.Package, 0, len(names))
	for _, n := range names {
		_, isOS := osSet[n]
		out = append(out, snapshot.Package{
			ID:        "apt:" + n,
			Name:      n,
			Source:    snapshot.SourceApt,
			Version:   versions[n],
			OSPackage: isOS,
		})
	}
	return out, nil
}

// aptVersions returns name → version for every installed apt package.
func aptVersions(ctx context.Context) (map[string]string, error) {
	out, err := RunCmd(ctx, "dpkg-query", "-W", "-f=${Package} ${Version}\n")
	if err != nil {
		return nil, err
	}
	versions := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		name, ver, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		versions[name] = ver
	}
	return versions, nil
}

// osBurstGap is the time window that separates the OS install burst from
// later user-driven installs. The OS installer lands hundreds of packages
// within a few minutes; the next install happens hours or days later.
const osBurstGap = 10 * time.Minute

// minOSBurstSize is the smallest install burst we'll consider an "OS install".
// A real OS install is hundreds of packages; a user running ``apt install
// foo`` that pulls in 5 deps shouldn't be mistaken for one.
const minOSBurstSize = 50

// aptOSPackages returns the set of packages that arrived during the OS
// install. Heuristic: prefer ``/var/log/installer/version``'s mtime as the
// anchor (Ubuntu's live installer writes it at install time), and pull every
// dpkg ``install`` event within ±2 h of that timestamp. If that's not
// available, fall back to "first contiguous burst of ≥ 50 installs separated
// by gaps shorter than 10 min."
//
// Returns an empty set when neither signal is conclusive — preferred over
// guessing wrong.
func aptOSPackages() map[string]struct{} {
	out := map[string]struct{}{}
	events := readDpkgInstalls()
	if len(events) == 0 {
		return out
	}
	sort.Slice(events, func(i, j int) bool { return events[i].t.Before(events[j].t) })

	// Anchor on the installer's manifest file mtime when present.
	if anchor, ok := osInstallAnchor(); ok {
		const window = 2 * time.Hour
		for _, e := range events {
			diff := e.t.Sub(anchor)
			if diff < 0 {
				diff = -diff
			}
			if diff <= window {
				out[e.name] = struct{}{}
			}
		}
		if len(out) >= minOSBurstSize {
			return out
		}
		out = map[string]struct{}{}
	}

	// Fallback: first burst of ≥ minOSBurstSize installs.
	for _, b := range burstsOf(events, osBurstGap) {
		if len(b) >= minOSBurstSize {
			for _, e := range b {
				out[e.name] = struct{}{}
			}
			return out
		}
	}
	return out
}

// osInstallAnchor returns the OS-install timestamp from a file the live
// installer drops, when available. Different Ubuntu installers leave
// different breadcrumbs:
//   - Debian Installer: ``/var/log/installer/{version,lsb-release,syslog}``
//   - subiquity / autoinstall (Ubuntu 23+): ``curtin-install.log``,
//     ``cloud-init-output.log`` (the dir itself's mtime is also reliable)
//
// We probe in order of specificity and fall back to the dir mtime.
func osInstallAnchor() (time.Time, bool) {
	for _, p := range []string{
		"/var/log/installer/version",
		"/var/log/installer/lsb-release",
		"/var/log/installer/syslog",
		"/var/log/installer/curtin-install.log",
		"/var/log/installer/cloud-init-output.log",
		"/var/log/installer",
	} {
		if info, err := os.Stat(p); err == nil {
			return info.ModTime(), true
		}
	}
	return time.Time{}, false
}

// burstsOf groups events into bursts separated by gaps strictly larger than
// ``gap``.
func burstsOf(events []dpkgInstall, gap time.Duration) [][]dpkgInstall {
	if len(events) == 0 {
		return nil
	}
	out := [][]dpkgInstall{{events[0]}}
	for i := 1; i < len(events); i++ {
		if events[i].t.Sub(events[i-1].t) > gap {
			out = append(out, []dpkgInstall{events[i]})
		} else {
			out[len(out)-1] = append(out[len(out)-1], events[i])
		}
	}
	return out
}

type dpkgInstall struct {
	t    time.Time
	name string
}

// readDpkgInstalls collects all ``install`` events from /var/log/dpkg.log*
// (including rotated .gz files), keeping the earliest occurrence per package.
func readDpkgInstalls() []dpkgInstall {
	paths, _ := filepath.Glob("/var/log/dpkg.log*")

	earliest := map[string]time.Time{}
	for _, p := range paths {
		for _, ev := range readDpkgFile(p) {
			if t, seen := earliest[ev.name]; !seen || ev.t.Before(t) {
				earliest[ev.name] = ev.t
			}
		}
	}
	out := make([]dpkgInstall, 0, len(earliest))
	for n, t := range earliest {
		out = append(out, dpkgInstall{t: t, name: n})
	}
	return out
}

func readDpkgFile(path string) []dpkgInstall {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return nil
		}
		defer gr.Close()
		r = gr
	}

	events := []dpkgInstall{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		// Format: "2024-04-25 13:14:15 install vim:amd64 <none> 9.0.0001-1ubuntu1"
		line := sc.Text()
		fields := strings.SplitN(line, " ", 5)
		if len(fields) < 4 || fields[2] != "install" {
			continue
		}
		t, err := time.Parse("2006-01-02 15:04:05", fields[0]+" "+fields[1])
		if err != nil {
			continue
		}
		name := fields[3]
		if i := strings.Index(name, ":"); i > 0 {
			name = name[:i] // strip arch suffix (vim:amd64 → vim)
		}
		if name == "" {
			continue
		}
		events = append(events, dpkgInstall{t: t, name: name})
	}
	return events
}
