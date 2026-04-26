package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/tui"
)

// SnapshotCmd is the no-frills "scan + write YAML" path. No picker, no
// settings, no banners — just the YAML. Useful for cron / scripted use.
type SnapshotCmd struct {
	Output           string   `short:"o" help:"Output path." placeholder:"PATH"`
	Sources          []string `name:"source" help:"Restrict to these scanners." placeholder:"NAME"`
	IncludeInference bool     `name:"include-inference" help:"Also run inference scanners."`
	Exclude          []string `help:"Drop entries whose id matches the glob (repeatable)." placeholder:"PATTERN"`
}

// Run executes the snapshot workflow.
func (c *SnapshotCmd) Run(ctx context.Context) error {
	theme := tui.DefaultTheme()
	snap, err := runScan(ctx, theme, scanOpts{
		Sources:          c.Sources,
		IncludeInference: c.IncludeInference,
		Exclude:          c.Exclude,
	})
	if err != nil {
		return err
	}
	out := c.Output
	if out == "" {
		out = defaultSnapshotPath()
	}
	if err := snapshot.Dump(snap, out); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	fmt.Fprintln(os.Stdout, "wrote", out, fmt.Sprintf("(%d packages)", len(snap.Packages)))
	return nil
}
