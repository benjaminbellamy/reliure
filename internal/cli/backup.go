package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/benjaminbellamy/reliure/internal/picker"
	"github.com/benjaminbellamy/reliure/internal/settings"
	"github.com/benjaminbellamy/reliure/internal/snapshot"
	"github.com/benjaminbellamy/reliure/internal/tui"
)

// BackupCmd is the default command. It scans the system, optionally captures
// GNOME settings, lets the user pick what to keep via the curses-equivalent
// picker, and writes a YAML snapshot.
type BackupCmd struct {
	Output           string   `short:"o" help:"Snapshot YAML path." placeholder:"PATH"`
	Sources          []string `name:"source" help:"Restrict to these scanners (comma-separated)." placeholder:"NAME"`
	IncludeInference bool     `name:"include-inference" help:"Also run inference scanners (history, manual)." default:"true" negatable:""`
	Exclude          []string `help:"Drop entries whose id matches the glob (repeatable)." placeholder:"PATTERN"`
	NoTUI            bool     `name:"no-tui" help:"Skip the picker; keep everything scanned."`
	Edit             bool     `help:"Open the snapshot in $EDITOR after writing."`
}

// Run executes the backup workflow.
func (c *BackupCmd) Run(ctx context.Context) error {
	theme := tui.DefaultTheme()
	tui.Banner(os.Stdout, theme,
		theme.Title.Render(fmt.Sprintf("Reliure %s — System State Backup", Version)),
		theme.Subtitle.Render(Tagline),
		"",
		theme.Subtitle.Render("Scan the system, choose what to keep, and write a snapshot."),
		theme.Subtitle.Render("On a fresh install, run "+theme.Title.Render("reliure restore <snapshot.yaml>")+"."),
		"",
		theme.Muted.Render(Copyright+" — GPLv3+"),
	)

	// 1. Scan
	tui.Section(os.Stdout, theme, "Scanning")
	snap, err := runScan(ctx, theme, scanOpts{
		Sources:          c.Sources,
		IncludeInference: c.IncludeInference,
		Exclude:          c.Exclude,
	})
	if err != nil {
		return err
	}
	direct, inferred := 0, 0
	for _, p := range snap.Packages {
		if p.Unverified {
			inferred++
		} else {
			direct++
		}
	}
	fmt.Println("  " + theme.Muted.Render(
		fmt.Sprintf("found %d package(s) — %d direct, %d inferred", len(snap.Packages), direct, inferred)))

	fmt.Println()
	if !tui.PressEnter(theme, "Press Enter to continue, Q to quit now…") {
		fmt.Println("  " + theme.Subtitle.Render("aborted"))
		return nil
	}

	// 2. GNOME settings — captured automatically when dconf is available.
	// No prompt, no flag: the dconf file is always written alongside the
	// snapshot. Restore still asks before applying.
	backupDir := defaultBackupDir()
	_ = os.MkdirAll(backupDir, 0o755)
	var gnomeFile string
	if settings.GNOMEAvailable() {
		path := filepath.Join(backupDir, settings.DefaultDconfFilename(snap.Meta.Date))
		n, err := settings.DumpGNOME(ctx, path)
		if err != nil {
			fmt.Println("  " + theme.Warn.Render("could not dump dconf: "+err.Error()))
		} else if n == 0 {
			fmt.Println("  " + theme.Subtitle.Render("dconf produced no output — skipped"))
		} else {
			gnomeFile = path
			fmt.Println("  " + theme.OK.Render(fmt.Sprintf("✓ GNOME settings: %s (%d bytes)", path, n)))
		}
	}

	// 3. Save the YAML so the user has it even if they Quit the picker.
	snapshotPath := c.Output
	if snapshotPath == "" {
		snapshotPath = defaultSnapshotPath()
	}
	if err := snapshot.Dump(snap, snapshotPath); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	fmt.Println()
	tui.Section(os.Stdout, theme, "Snapshot")
	fmt.Println("  " + theme.OK.Render("✓ ") + snapshotPath)

	// 4. Edit (optional)
	if c.Edit {
		if err := openInEditor(snapshotPath); err != nil {
			fmt.Println("  " + theme.Warn.Render("editor: "+err.Error()))
		} else {
			reloaded, err := snapshot.Load(snapshotPath)
			if err == nil {
				snap = reloaded
			}
		}
	}

	// 5. Picker (mandatory unless --no-tui)
	if !c.NoTUI {
		if !tui.IsStdinTTY() {
			fmt.Println("  " + theme.Subtitle.Render("(no TTY — keeping every scanned item)"))
		} else {
			// Initial selection: every user-installed item kept; OS packages
			// off by default (the fresh OS will come with them anyway, and
			// keeping them out makes the snapshot focus on user choices).
			checked := func(p snapshot.Package) bool { return !p.OSPackage }
			for {
				pages := buildPickerPages(snap, checked, nil)
				res, err := picker.Run("Reliure backup", pages)
				if err != nil {
					if errors.Is(err, picker.ErrAborted) {
						fmt.Println("  " + theme.Warn.Render("aborted — snapshot left untouched."))
						return nil
					}
					return err
				}
				selected := applyPickerSelections(snap.Packages, res.Selections)
				fmt.Println()
				fmt.Println("  " + theme.Subtitle.Render(
					fmt.Sprintf("→ %d of %d package(s) will be kept in the snapshot.",
						len(selected), len(snap.Packages))))
				switch tui.AskConfirm(theme, "Save snapshot with this selection?", tui.ConfirmYes) {
				case tui.ConfirmYes:
					snap.Packages = selected
					if err := snapshot.Dump(snap, snapshotPath); err != nil {
						return fmt.Errorf("re-save snapshot: %w", err)
					}
					fmt.Println("  " + theme.OK.Render(fmt.Sprintf("✓ kept %d package(s)", len(snap.Packages))))
				case tui.ConfirmNo:
					fmt.Println("  " + theme.Subtitle.Render("aborted — snapshot left untouched."))
					return nil
				case tui.ConfirmBack:
					// Re-launch the picker preserving the user's selection.
					selectedIDs := map[string]struct{}{}
					for _, p := range selected {
						selectedIDs[p.ID] = struct{}{}
					}
					checked = func(p snapshot.Package) bool {
						_, ok := selectedIDs[p.ID]
						return ok
					}
					continue
				}
				break
			}
		}
	}

	// 6. Done — copy reminder
	fmt.Println()
	tui.Banner(os.Stdout, theme,
		theme.Title.Render("Done."),
		"",
		"  Snapshot:    "+theme.Title.Render(snapshotPath),
		gnomeLine(theme, gnomeFile),
		"",
		"  Back up "+theme.Title.Render(backupDir)+" with your usual tool (DéjaDup, Borg, …).",
		"  On the new system:  "+theme.Title.Render("reliure restore "+filepath.Base(snapshotPath)),
	)
	return nil
}

func gnomeLine(theme tui.Theme, path string) string {
	if path == "" {
		return ""
	}
	return "  GNOME .dconf: " + theme.Title.Render(path)
}

func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
