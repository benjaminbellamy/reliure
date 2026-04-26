package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/benjaminbellamy/reliure/internal/installer"
	"github.com/benjaminbellamy/reliure/internal/picker"
	"github.com/benjaminbellamy/reliure/internal/settings"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/tui"
)

// RestoreCmd reads a snapshot YAML and runs the picker + per-source
// installers.
type RestoreCmd struct {
	Snapshot       string   `arg:"" help:"Path to the snapshot YAML." type:"existingfile"`
	DryRun         bool     `name:"dry-run" help:"Print commands without executing them."`
	EssentialOnly  bool     `name:"essential-only" help:"Skip the picker and install only items flagged essential."`
	Sources        []string `name:"source" help:"Restrict to these sources." placeholder:"NAME"`
	NoGnome        bool     `name:"no-gnome" help:"Don't ask about GNOME settings."`
	GnomeFile      string   `name:"gnome-file" help:"Explicit path to the dconf file (default: alongside the snapshot)."`
	Yes            bool     `short:"y" help:"Skip the final \"install N packages?\" confirmation."`
}

// Run executes the restore workflow.
func (c *RestoreCmd) Run(ctx context.Context) error {
	theme := tui.DefaultTheme()
	snap, err := snapshot.Load(c.Snapshot)
	if err != nil {
		return err
	}

	tui.Banner(os.Stdout, theme,
		theme.Title.Render(fmt.Sprintf("Reliure %s — Restore", Version)),
		theme.Subtitle.Render(Tagline),
		"",
		"  Snapshot: "+theme.Subtitle.Render(c.Snapshot),
		"  Source:   "+theme.Subtitle.Render(snap.Meta.Hostname+" · "+snap.Meta.OS),
		"  Packages: "+theme.Subtitle.Render(fmt.Sprintf("%d total", len(snap.Packages))),
		"",
		theme.Subtitle.Render("Sudo will be requested when the first install command runs."),
		"",
		theme.Muted.Render(Copyright+" — GPLv3+"),
	)

	// Optionally restrict by source.
	if len(c.Sources) > 0 {
		allowed := map[string]struct{}{}
		for _, s := range c.Sources {
			allowed[s] = struct{}{}
		}
		filtered := snap.Packages[:0]
		for _, p := range snap.Packages {
			if _, ok := allowed[p.Source]; ok {
				filtered = append(filtered, p)
			}
		}
		snap.Packages = filtered
	}

	// Pick what to install.
	var toInstall []snapshot.Package
	switch {
	case c.EssentialOnly:
		for _, p := range snap.Packages {
			if p.Essential && !p.Unverified {
				toInstall = append(toInstall, p)
			}
		}
	case !tui.IsStdinTTY():
		return fmt.Errorf("restore needs a TTY for the picker (or pass --essential-only)")
	default:
		installed := installer.LoadInstalledVersions(ctx)
		// Initial selection: nothing checked (restore default).
		checked := func(snapshot.Package) bool { return false }
	pickLoop:
		for {
			pages := buildPickerPages(snap, checked, &installed)
			res, err := picker.Run("Reliure restore", pages)
			if err != nil {
				if errors.Is(err, picker.ErrAborted) {
					fmt.Println("  " + theme.Warn.Render("aborted"))
					return nil
				}
				return err
			}
			toInstall = applyPickerSelections(snap.Packages, res.Selections)
			fmt.Println()
			fmt.Println("  " + theme.OK.Render(fmt.Sprintf("→ %d package(s) selected", len(toInstall))))
			if c.Yes {
				break pickLoop
			}
			question := "Proceed with installation?"
			if c.DryRun {
				question = "Show what would run (dry run)?"
			}
			switch tui.AskConfirm(theme, question, tui.ConfirmYes) {
			case tui.ConfirmYes:
				break pickLoop
			case tui.ConfirmNo:
				fmt.Println("  " + theme.Subtitle.Render("aborted"))
				return nil
			case tui.ConfirmBack:
				selectedIDs := map[string]struct{}{}
				for _, p := range toInstall {
					selectedIDs[p.ID] = struct{}{}
				}
				checked = func(p snapshot.Package) bool {
					_, ok := selectedIDs[p.ID]
					return ok
				}
				continue
			}
		}
	}

	if len(toInstall) == 0 {
		fmt.Println("  " + theme.Subtitle.Render("nothing selected — nothing to do"))
		return c.maybeApplyGNOME(ctx, theme, snap)
	}

	// Run installers per source.
	rep := &tui.StreamReporter{W: os.Stdout, Theme: theme}
	opts := installer.DefaultOptions()
	opts.DryRun = c.DryRun

	installers := []installer.Installer{
		installer.AptInstaller{},
		installer.FlatpakInstaller{},
		installer.SnapInstaller{},
		installer.VSCodeInstaller{},
		installer.GNOMEExtInstaller{},
		installer.PipInstaller{},
		installer.PipxInstaller{},
		installer.CargoInstaller{},
		installer.NpmInstaller{},
	}
	for _, ins := range installers {
		if !ins.Available() {
			continue
		}
		items := installer.Filter(toInstall, ins.Source())
		ins.Install(ctx, items, opts, rep)
	}

	stats := rep.Stats()
	fmt.Println()
	tui.Section(os.Stdout, theme, "Summary")
	fmt.Printf("  installed:         %d\n", stats.Installed)
	fmt.Printf("  already installed: %d\n", stats.Skipped)
	fmt.Printf("  failed:            %d\n", stats.Failed)
	if c.DryRun {
		fmt.Println("  " + theme.Subtitle.Render("(dry run — nothing was actually installed)"))
	}

	if err := c.maybeApplyGNOME(ctx, theme, snap); err != nil {
		return err
	}
	if stats.Failed > 0 {
		return fmt.Errorf("%d install(s) failed", stats.Failed)
	}
	return nil
}

// maybeApplyGNOME asks (or, with --no-gnome, doesn't) whether to apply a
// dconf file alongside the snapshot.
func (c *RestoreCmd) maybeApplyGNOME(ctx context.Context, theme tui.Theme, snap *snapshot.Snapshot) error {
	if c.NoGnome {
		return nil
	}
	dconfPath := c.GnomeFile
	if dconfPath == "" {
		dconfPath = filepath.Join(filepath.Dir(c.Snapshot),
			settings.DefaultDconfFilename(snap.Meta.Date))
	}
	if _, err := os.Stat(dconfPath); err != nil {
		return nil // not present, silent
	}
	if !tui.IsStdinTTY() && !c.Yes {
		return nil
	}
	fmt.Println()
	if !c.Yes {
		if !tui.AskYesNo(theme, "Apply GNOME settings from "+filepath.Base(dconfPath)+"?", true) {
			return nil
		}
	}
	if c.DryRun {
		fmt.Println("  " + theme.Muted.Render("$ dconf load /org/gnome/ < "+dconfPath))
		fmt.Println("  " + theme.OK.Render("✓ GNOME settings (dry-run)"))
		return nil
	}
	if err := settings.LoadGNOME(ctx, dconfPath); err != nil {
		fmt.Println("  " + theme.Err.Render("✗ GNOME settings: "+err.Error()))
		return err
	}
	fmt.Println("  " + theme.OK.Render("✓ GNOME settings applied"))
	return nil
}
