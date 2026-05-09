// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package cli

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/benjaminbellamy/reliure/internal/scanner"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// dropPrivilegesIfSudo demotes the current process from root back to the
// user identified by $SUDO_UID / $SUDO_GID / $SUDO_USER, restores
// HOME/USER/LOGNAME, and prepends the user's well-known dev bin dirs
// (~/.cargo/bin, ~/.local/bin, ~/go/bin, …) to PATH so child commands like
// ``cargo`` and ``code`` can resolve again. Returns (dropped, err): false
// without error when reliure isn't running under sudo (genuine root or
// normal user — both no-op).
//
// Privilege drop is required because ``sudo reliure backup`` would
// otherwise break the user-space scanners: ``code`` self-aborts as uid 0,
// cargo/pipx/npm aren't on sudo's secure_path, and history files are read
// from $HOME (which sudo points at /root). After this returns, the process
// is the original user, with sudo only used implicitly during the wifi/vpn
// scanners that run BEFORE this is called (see runScan).
//
// Go ≥1.16 implements syscall.Setuid/Setgid as process-wide on Linux, so
// this is safe to call from any goroutine.
func dropPrivilegesIfSudo() (bool, error) {
	if os.Geteuid() != 0 {
		return false, nil
	}
	sudoUidStr := os.Getenv("SUDO_UID")
	sudoGidStr := os.Getenv("SUDO_GID")
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUidStr == "" || sudoGidStr == "" || sudoUser == "" {
		// Genuine root login — leave as-is.
		return false, nil
	}
	uid, err := strconv.Atoi(sudoUidStr)
	if err != nil {
		return false, fmt.Errorf("parse $SUDO_UID: %w", err)
	}
	gid, err := strconv.Atoi(sudoGidStr)
	if err != nil {
		return false, fmt.Errorf("parse $SUDO_GID: %w", err)
	}
	u, err := user.Lookup(sudoUser)
	if err != nil {
		return false, fmt.Errorf("user lookup %q: %w", sudoUser, err)
	}

	// Replace root's group set with the user's actual groups before dropping
	// uid — only root can call setgroups, and we don't want to keep root's
	// secondary group capabilities under the user's uid. Best-effort: a
	// failure here isn't fatal (read-only scans rarely need supplementary
	// groups).
	if gids := userGIDs(u); len(gids) > 0 {
		_ = syscall.Setgroups(gids)
	}

	// Order matters: setgid first (still privileged), then setuid (which
	// drops the ability to setgid).
	if err := syscall.Setgid(gid); err != nil {
		return false, fmt.Errorf("setgid: %w", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return false, fmt.Errorf("setuid: %w", err)
	}

	os.Setenv("HOME", u.HomeDir)
	os.Setenv("USER", sudoUser)
	os.Setenv("LOGNAME", sudoUser)
	os.Setenv("PATH", composeUserPath(u.HomeDir, os.Getenv("PATH")))
	return true, nil
}

// userGIDs returns the integer GIDs in the user's group set, silently
// dropping malformed entries. Empty if the lookup itself failed (which
// happens on systems without /etc/group readable, etc.).
func userGIDs(u *user.User) []int {
	groups, err := u.GroupIds()
	if err != nil {
		return nil
	}
	out := make([]int, 0, len(groups))
	for _, g := range groups {
		if i, err := strconv.Atoi(g); err == nil {
			out = append(out, i)
		}
	}
	return out
}

// composeUserPath prepends the user's standard dev bin directories — only
// those that actually exist — to ``existing``. Sudo's secure_path stripped
// these out; without restoration, cargo/pipx/go binaries are unfindable.
func composeUserPath(home, existing string) string {
	candidates := []string{
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".deno", "bin"),
		filepath.Join(home, ".npm-global", "bin"),
		"/usr/local/go/bin",
		"/snap/bin",
	}
	parts := []string{}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		if _, err := os.Stat(c); err != nil {
			continue
		}
		seen[c] = true
		parts = append(parts, c)
	}
	if existing != "" {
		parts = append(parts, existing)
	}
	return strings.Join(parts, ":")
}

// partitionRootScanners splits scanners into those that need root at scan
// time (currently wifi + vpn) and the rest. runScan uses this to schedule
// the privileged ones before dropPrivilegesIfSudo is called.
func partitionRootScanners(scs []scanner.Scanner) (root, user []scanner.Scanner) {
	for _, s := range scs {
		switch s.Name() {
		case snapshot.SourceWifi, snapshot.SourceVPN:
			root = append(root, s)
		default:
			user = append(user, s)
		}
	}
	return
}
