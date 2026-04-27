package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// ManualScanner surfaces things that look like they were installed outside
// any package manager: top-level dirs in /opt, executables in /usr/local/bin
// and ~/.local/bin (excluding pipx-managed ones), third-party APT repos, and
// well-known fingerprint dirs (~/.nvm, ~/.ollama, /etc/docker, …).
//
// Every entry is ``unverified=true``; the user prunes false positives.
type ManualScanner struct{}

func (ManualScanner) Name() string    { return snapshot.SourceManual }
func (ManualScanner) Available() bool { return true }

// builtinAptSources are the Ubuntu-shipped sources files; not "third-party".
var builtinAptSources = map[string]struct{}{
	"ubuntu":          {},
	"ubuntu-archive":  {},
	"ubuntu-security": {},
}

type fingerprint struct {
	Path     string
	Name     string
	Evidence string
	Hint     string
}

func fingerprints() []fingerprint {
	home, _ := os.UserHomeDir()
	return []fingerprint{
		{home + "/.nvm", "nvm", "directory ~/.nvm",
			"curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash"},
		{home + "/.ollama", "ollama", "directory ~/.ollama",
			"curl -fsSL https://ollama.com/install.sh | sh"},
		{"/etc/docker", "docker", "directory /etc/docker",
			"https://docs.docker.com/engine/install/ubuntu/"},
		{"/var/lib/docker", "docker", "directory /var/lib/docker",
			"https://docs.docker.com/engine/install/ubuntu/"},
		{home + "/.rustup", "rustup", "directory ~/.rustup",
			"curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"},
		{home + "/.deno", "deno", "directory ~/.deno",
			"curl -fsSL https://deno.land/install.sh | sh"},
		{home + "/.bun", "bun", "directory ~/.bun",
			"curl -fsSL https://bun.sh/install | bash"},
		{home + "/.sdkman", "sdkman", "directory ~/.sdkman",
			"curl -s https://get.sdkman.io | bash"},
		{home + "/.volta", "volta", "directory ~/.volta",
			"curl https://get.volta.sh | bash"},
		{home + "/.pyenv", "pyenv", "directory ~/.pyenv",
			"curl https://pyenv.run | bash"},
	}
}

func (ManualScanner) Scan(ctx context.Context) ([]snapshot.Package, error) {
	results := []snapshot.Package{}
	results = append(results, scanOpt()...)
	results = append(results, scanUsrLocalBin(ctx)...)
	results = append(results, scanUserBins(ctx)...)
	results = append(results, scanThirdPartyRepos()...)
	results = append(results, scanFingerprints()...)

	// Dedupe by ID (e.g. docker fingerprint hits twice).
	seen := map[string]struct{}{}
	uniq := results[:0]
	for _, p := range results {
		if _, hit := seen[p.ID]; hit {
			continue
		}
		seen[p.ID] = struct{}{}
		uniq = append(uniq, p)
	}
	sort.Slice(uniq, func(i, j int) bool { return uniq[i].ID < uniq[j].ID })
	return uniq, nil
}

// --- /opt ---

func scanOpt() []snapshot.Package {
	entries, err := os.ReadDir("/opt")
	if err != nil {
		return nil
	}
	out := []snapshot.Package{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := "/opt/" + e.Name()
		out = append(out, snapshot.Package{
			ID:          "manual:opt:" + e.Name(),
			Name:        e.Name(),
			Source:      snapshot.SourceManual,
			DetectedVia: "filesystem",
			Evidence:    full,
			Unverified:  true,
			RestoreHint: "installed under " + full + "; reinstall via its vendor's method",
			Notes:       "Found in /opt — likely a vendor installer",
		})
	}
	return out
}

// --- /usr/local/bin ---

func scanUsrLocalBin(_ context.Context) []snapshot.Package {
	const dir = "/usr/local/bin"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []snapshot.Package{}
	for _, e := range entries {
		if isAppImage(e.Name()) {
			continue
		}
		full := dir + "/" + e.Name()
		if !isExecutableFile(full) {
			continue
		}
		out = append(out, snapshot.Package{
			ID:          "manual:usr-local-bin:" + e.Name(),
			Name:        e.Name(),
			Source:      snapshot.SourceManual,
			DetectedVia: "filesystem",
			Evidence:    full,
			Unverified:  true,
			RestoreHint: "binary at " + full +
				"; reinstall via its original installer (typical: a curl-bash one-liner from the vendor's website)",
		})
	}
	return out
}

// --- ~/.local/bin and ~/bin ---

func scanUserBins(ctx context.Context) []snapshot.Package {
	pipxManaged := pipxBinaries(ctx)
	goManaged := GoBinBasenames()
	home, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin")}
	out := []snapshot.Package{}
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if _, skip := pipxManaged[e.Name()]; skip {
				continue
			}
			if _, skip := goManaged[e.Name()]; skip {
				continue
			}
			if isAppImage(e.Name()) {
				continue
			}
			full := filepath.Join(d, e.Name())
			if !isExecutableFile(full) {
				continue
			}
			out = append(out, snapshot.Package{
				ID:          "manual:user-bin:" + e.Name(),
				Name:        e.Name(),
				Source:      snapshot.SourceManual,
				DetectedVia: "filesystem",
				Evidence:    full,
				Unverified:  true,
				RestoreHint: "binary at " + full + "; reinstall via its original method",
			})
		}
	}
	return out
}

func pipxBinaries(ctx context.Context) map[string]struct{} {
	out := map[string]struct{}{}
	if !Have("pipx") {
		return out
	}
	body, err := RunCmd(ctx, "pipx", "list", "--short")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			out[fields[0]] = struct{}{}
		}
	}
	return out
}

func isExecutableFile(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if !info.Mode().IsRegular() {
		// Resolve symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			info, err = os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				return false
			}
		} else {
			return false
		}
	}
	return info.Mode().Perm()&0o111 != 0
}

// --- /etc/apt/sources.list.d ---

func scanThirdPartyRepos() []snapshot.Package {
	const dir = "/etc/apt/sources.list.d"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	out := []snapshot.Package{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".list" && ext != ".sources" {
			continue
		}
		stem := strings.TrimSuffix(e.Name(), ext)
		if _, builtin := builtinAptSources[stem]; builtin {
			continue
		}
		full := filepath.Join(dir, e.Name())
		out = append(out, snapshot.Package{
			ID:          "manual:apt-repo:" + stem,
			Name:        stem,
			Source:      snapshot.SourceManual,
			DetectedVia: "apt-sources",
			Evidence:    full,
			Unverified:  true,
			RestoreHint: "third-party APT source from " + e.Name() +
				"; on the new system re-add the GPG key and source line, then `sudo apt update`",
			Notes: "Third-party APT repo (signals a vendor install — Docker, NodeSource, etc.)",
		})
	}
	return out
}

// --- known fingerprint dirs ---

func scanFingerprints() []snapshot.Package {
	out := []snapshot.Package{}
	for _, fp := range fingerprints() {
		if _, err := os.Stat(fp.Path); err != nil {
			continue
		}
		out = append(out, snapshot.Package{
			ID:          "manual:fingerprint:" + fp.Name,
			Name:        fp.Name,
			Source:      snapshot.SourceManual,
			DetectedVia: "fingerprint",
			Evidence:    fp.Evidence,
			Unverified:  true,
			RestoreHint: fp.Hint,
		})
	}
	return out
}
