package installer

import (
	"context"
	"os/exec"
	"strings"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// AptInstaller batches all picked items into one ``sudo apt install -y …``
// invocation, matching what the Python exporter did.
type AptInstaller struct{}

func (AptInstaller) Source() string  { return snapshot.SourceApt }
func (AptInstaller) Available() bool { return have("apt-get") && have("dpkg-query") }

func (a AptInstaller) Install(ctx context.Context, items []snapshot.Package, opts Options, rep Reporter) []ItemResult {
	if len(items) == 0 {
		return nil
	}
	rep.Section("apt")

	results := make([]ItemResult, 0, len(items))
	todo := make([]string, 0, len(items))
	idxByName := make(map[string]int, len(items))

	for _, p := range items {
		if aptInstalled(p.Name) {
			rep.Result(ItemResult{Package: p, Outcome: OutcomeAlreadyInstalled})
			results = append(results, ItemResult{Package: p, Outcome: OutcomeAlreadyInstalled})
			continue
		}
		idxByName[p.Name] = len(todo)
		todo = append(todo, p.Name)
	}
	if len(todo) == 0 {
		return results
	}

	args := append([]string{"apt", "install", "-y"}, todo...)
	rep.Note("running: sudo %s", strings.Join(args, " "))
	err := runCtx(ctx, opts, "sudo", args...)
	for _, name := range todo {
		var p snapshot.Package
		for _, q := range items {
			if q.Name == name {
				p = q
				break
			}
		}
		if err != nil {
			rep.Result(ItemResult{Package: p, Outcome: OutcomeFailed, Err: err})
			results = append(results, ItemResult{Package: p, Outcome: OutcomeFailed, Err: err})
		} else {
			rep.Result(ItemResult{Package: p, Outcome: OutcomeInstalled})
			results = append(results, ItemResult{Package: p, Outcome: OutcomeInstalled})
		}
	}
	return results
}

func aptInstalled(name string) bool {
	cmd := exec.Command("dpkg-query", "-W", "-f=${Status}", name)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "install ok installed")
}

func have(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
