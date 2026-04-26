package scanner

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// HistoryScanner parses ``~/.bash_history`` and ``~/.zsh_history`` for install
// commands. All entries it surfaces are ``unverified=true`` and grouped under
// the inferred section in the snapshot.
type HistoryScanner struct{}

func (HistoryScanner) Name() string { return snapshot.SourceHistory }

func (HistoryScanner) Available() bool {
	for _, p := range historyFiles() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func historyFiles() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".bash_history"),
		filepath.Join(home, ".zsh_history"),
	}
}

// One regex per supported install verb. Each captures everything after the
// verb on the line for further tokenisation.
var (
	reAptInstall     = regexp.MustCompile(`^(?:sudo\s+)?apt(?:-get)?\s+install\b(.*)$`)
	reSnapInstall    = regexp.MustCompile(`^(?:sudo\s+)?snap\s+install\b(.*)$`)
	reFlatpakInstall = regexp.MustCompile(`^flatpak\s+install\b(.*)$`)
	rePipInstall     = regexp.MustCompile(`^pip3?\s+install\b(.*)$`)
	rePipxInstall    = regexp.MustCompile(`^pipx\s+install\b(.*)$`)
	reCargoInstall   = regexp.MustCompile(`^cargo\s+install\b(.*)$`)
	reNpmInstall     = regexp.MustCompile(`^npm\s+(?:install|i)\s+(?:-g|--global)\b(.*)$`)
	reVSCodeInstall  = regexp.MustCompile(`^code\s+--install-extension\s+(.*)$`)
	rePyPIName       = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_.\-]*)`)
)

// flagsWithValue: tokens whose *next* token is a value (so we skip both).
var flagsWithValue = map[string]struct{}{
	"-r": {}, "--requirement": {},
	"-c": {}, "--constraint": {},
	"-e": {}, "--editable": {},
	"-i": {}, "--index-url": {},
	"--target":           {},
	"-t":                 {}, "--target-release": {},
	"--extra-index-url":  {},
	"--root":             {},
	"--prefix":           {},
}

// knownFlatpakRemotes are recognised explicitly so a leading remote name (no
// dots) doesn't get mistaken for an app id.
var knownFlatpakRemotes = map[string]struct{}{
	"flathub":         {},
	"flathub-beta":    {},
	"gnome-nightly":   {},
	"kdeapps":         {},
	"elementary-stable": {},
}

func (HistoryScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	cmds := []string{}
	for _, p := range historyFiles() {
		cmds = append(cmds, readHistory(p)...)
	}

	// Map keyed by NaturalKey so the same install repeated in history only
	// surfaces once.
	found := map[[2]string]snapshot.Package{}
	for _, cmd := range cmds {
		for _, p := range extractFromCommand(cmd) {
			k := p.NaturalKey()
			if _, ok := found[k]; !ok {
				found[k] = p
			}
		}
	}

	out := make([]snapshot.Package, 0, len(found))
	for _, p := range found {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// stripHistoryPrefix returns the command portion of a single history line, or
// "" to skip. Handles zsh extended history (": <ts>:<dur>;cmd") and bash
// HISTTIMEFORMAT timestamp comments ("#1234567890").
func stripHistoryPrefix(line string) string {
	line = strings.TrimRight(line, "\n")
	if strings.TrimSpace(line) == "" {
		return ""
	}
	// zsh extended history
	if strings.HasPrefix(line, ": ") {
		idx := strings.Index(line, ";")
		if idx < 0 {
			return ""
		}
		return strings.TrimSpace(line[idx+1:])
	}
	// bash timestamp comment line
	if strings.HasPrefix(line, "#") {
		rest := strings.TrimSpace(line[1:])
		allDigits := rest != ""
		for _, r := range rest {
			if r < '0' || r > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return ""
		}
	}
	return strings.TrimSpace(line)
}

func readHistory(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := []string{}
	for _, raw := range strings.Split(string(data), "\n") {
		if cmd := stripHistoryPrefix(raw); cmd != "" {
			out = append(out, cmd)
		}
	}
	return out
}

// splitArgs tokenises the post-verb portion of a command, dropping flags.
// Flags listed in ``flagsWithValue`` also consume the next token. Quoting
// and shell expansion are not handled — install commands almost never use
// them for package names in practice.
func splitArgs(rest string) []string {
	out := []string{}
	skipNext := false
	for _, tok := range strings.Fields(strings.TrimSpace(rest)) {
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(tok, "-") {
			base := tok
			if i := strings.Index(tok, "="); i >= 0 {
				base = tok[:i]
			}
			if _, ok := flagsWithValue[base]; ok && !strings.Contains(tok, "=") {
				skipNext = true
			}
			continue
		}
		out = append(out, tok)
	}
	return out
}

func looksLikePathOrURL(t string) bool {
	if t == "." || t == ".." {
		return true
	}
	if strings.HasPrefix(t, "/") || strings.HasPrefix(t, "./") ||
		strings.HasPrefix(t, "../") || strings.HasPrefix(t, "~/") {
		return true
	}
	if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") ||
		strings.HasPrefix(t, "git+") || strings.HasPrefix(t, "file://") {
		return true
	}
	return false
}

func parseAptArgs(rest string) []string {
	out := []string{}
	for _, t := range splitArgs(rest) {
		if looksLikePathOrURL(t) || strings.HasSuffix(t, ".deb") {
			continue
		}
		out = append(out, t)
	}
	return out
}

func parseSimple(rest string) []string {
	out := []string{}
	for _, t := range splitArgs(rest) {
		if !looksLikePathOrURL(t) {
			out = append(out, t)
		}
	}
	return out
}

func parsePipArgs(rest string) []string {
	out := []string{}
	for _, t := range splitArgs(rest) {
		if looksLikePathOrURL(t) {
			continue
		}
		if strings.HasSuffix(t, ".txt") || strings.HasSuffix(t, ".whl") ||
			strings.HasSuffix(t, ".tar.gz") || strings.HasSuffix(t, ".zip") {
			continue
		}
		if m := rePyPIName.FindString(t); m != "" {
			out = append(out, m)
		}
	}
	return out
}

func parseNpmArgs(rest string) []string {
	out := []string{}
	for _, t := range splitArgs(rest) {
		if looksLikePathOrURL(t) {
			continue
		}
		if strings.HasSuffix(t, ".tgz") || strings.HasSuffix(t, ".tar.gz") {
			continue
		}
		// Strip @version: foo@1.0 → foo. @scope/name@1.0 → @scope/name.
		if strings.HasPrefix(t, "@") {
			parts := strings.Split(t, "@")
			if len(parts) >= 2 && strings.Contains(parts[1], "/") {
				out = append(out, "@"+parts[1])
			}
		} else {
			out = append(out, strings.SplitN(t, "@", 2)[0])
		}
	}
	return out
}

type flatpakPick struct {
	Remote string // empty if not specified
	AppID  string
}

func parseFlatpakArgs(rest string) []flatpakPick {
	args := splitArgs(rest)
	// drop urls / *.flatpak / *.flatpakref
	clean := args[:0]
	for _, a := range args {
		if looksLikePathOrURL(a) || strings.HasSuffix(a, ".flatpak") || strings.HasSuffix(a, ".flatpakref") {
			continue
		}
		clean = append(clean, a)
	}
	if len(clean) == 0 {
		return nil
	}
	remote := ""
	apps := clean
	if _, ok := knownFlatpakRemotes[clean[0]]; ok || !strings.Contains(clean[0], ".") {
		remote = clean[0]
		apps = clean[1:]
	}
	out := []flatpakPick{}
	for _, a := range apps {
		if !strings.Contains(a, ".") || strings.Contains(a, "//") {
			// Must look like a reverse-DNS app id and not be a runtime ref.
			continue
		}
		out = append(out, flatpakPick{Remote: remote, AppID: a})
	}
	return out
}

func parseVSCodeArgs(rest string) []string {
	out := []string{}
	for _, a := range splitArgs(rest) {
		if strings.Contains(a, ".") {
			out = append(out, a)
		}
	}
	return out
}

const inferredNote = "Detected in shell history — may no longer be installed"

func makeInferred(source, name, id string) snapshot.Package {
	return snapshot.Package{
		ID:          id,
		Name:        name,
		Source:      source,
		DetectedVia: "history",
		Unverified:  true,
		Notes:       inferredNote,
	}
}

func extractFromCommand(cmd string) []snapshot.Package {
	if m := reAptInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parseAptArgs(m[1]) {
			out = append(out, makeInferred("apt", n, "history:apt:"+n))
		}
		return out
	}
	if m := reSnapInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parseSimple(m[1]) {
			out = append(out, makeInferred("snap", n, "history:snap:"+n))
		}
		return out
	}
	if m := reFlatpakInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, p := range parseFlatpakArgs(m[1]) {
			pkg := makeInferred("flatpak", p.AppID, "history:flatpak:"+p.AppID)
			pkg.AppID = p.AppID
			pkg.Remote = p.Remote
			out = append(out, pkg)
		}
		return out
	}
	if m := rePipInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parsePipArgs(m[1]) {
			out = append(out, makeInferred("pip", n, "history:pip:"+n))
		}
		return out
	}
	if m := rePipxInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parseSimple(m[1]) {
			out = append(out, makeInferred("pipx", n, "history:pipx:"+n))
		}
		return out
	}
	if m := reCargoInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parseSimple(m[1]) {
			out = append(out, makeInferred("cargo", n, "history:cargo:"+n))
		}
		return out
	}
	if m := reNpmInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, n := range parseNpmArgs(m[1]) {
			out = append(out, makeInferred("npm", n, "history:npm:"+n))
		}
		return out
	}
	if m := reVSCodeInstall.FindStringSubmatch(cmd); m != nil {
		out := []snapshot.Package{}
		for _, ext := range parseVSCodeArgs(m[1]) {
			pkg := makeInferred("vscode", ext, "history:vscode:"+ext)
			pkg.ExtensionID = ext
			out = append(out, pkg)
		}
		return out
	}
	return nil
}
