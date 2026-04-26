![reliure](reliure.svg)

# reliure
Format, Reinstall, Restore. A new clean system every 6 months.

> **reliure** \\ʁə.ljyʁ\\ *féminin*  
> French, binding (the spine of a book where the pages are held together)

---

**Reliure** scans your Linux system, builds a YAML list of everything installed (apt packages, flatpaks, snaps, VS Code extensions, GNOME Shell extensions, GNOME settings, plus clues from your shell history and manual installs), and lets you reinstall it on a fresh system through a slick interactive picker.

It's a single static binary built in Go.

### Highlights

- **Single static binary** (~5 MB, `linux/amd64`). No Python, no `dialog`, no `whiptail`, no curl-piped scripts on the new system.
- **Slick TUI** built on the Charm stack (Bubble Tea + Bubbles + Lip Gloss). Multi-page wizard, six labelled buttons, all keyboard-driven, with a `back` option from every confirmation.
- **Smart scanning**: picks up apt, flatpak, snap, VS Code, GNOME Shell extensions, GNOME dconf settings, and inference sources (shell history + manual installs in `/opt`, `/usr/local/bin`, `~/.local/bin`, third-party APT repos, runtime fingerprints).
- **OS-vs-user diff**: packages that arrived with the original Ubuntu install are flagged `[os]` (heuristic: `/var/log/installer/*` mtime ± dpkg.log burst detection) so you can focus on what *you* added.
- **Printable HTML report**: `reliure report <snapshot.yaml>` emits a styled standalone HTML — open in a browser and Cmd/Ctrl-P → Save as PDF.

---

## How to use it

The full clean-reinstall workflow:

1. **Install reliure** — without cloning the repo:

    ```bash
    curl -sSfL https://raw.githubusercontent.com/benjaminbellamy/reliure/main/install.sh | sh
    ```

   This downloads the latest `linux/amd64` binary into `~/.local/bin/`.

2. **Run the backup:**

    ```bash
    reliure
    ```

   The system gets scanned, you walk through a multi-page checkbox picker (one page per source: apt, flatpak, snap, vscode, gnome-ext, plus an "inferred" review page), and a YAML snapshot lands in `~/.config/reliure/snapshots/reliure-YYYYMMDD.yaml`. Optionally a `reliure-gnome-YYYYMMDD.dconf` is dumped into `~/Documents/`.

3. **Back up your data** with whatever tool you usually use (DéjaDup, Borg, rsync, …). Make sure `~/Documents/` and `~/.config/reliure/` are included so the snapshot and dconf file go with it.

4. **Format and reinstall** Ubuntu (or your derivative).

5. **Restore your data backup**, again with your usual tool.

6. **Reinstall reliure on the new system** (the install one-liner above), then run the restore:

    ```bash
    reliure restore ~/.config/reliure/snapshots/reliure-20260426.yaml
    ```

   The picker reopens — this time with everything **unchecked by default**. You tick what you want, hit `[Apply ✓]`, confirm, and reliure runs the install commands. Items already installed at the snapshot's version are tagged `[installed]` and silently skipped; items at a different version show `[installed: X]` so you can choose to upgrade.

You have a brand new clean computer.

---

## The picker

Built on the Charm stack (Bubble Tea + Bubbles + Lip Gloss). Multi-page wizard, one page per source, with the section name as a high-contrast colored bar at the top of each page.

**Keys (work regardless of which control is focused):**

| Key       | What it does                                  |
|-----------|-----------------------------------------------|
| `↑` / `↓` | navigate items                                |
| `Space`   | toggle current item                           |
| `A`       | select **A**ll (current page)                 |
| `Z`       | select none (**Z**ero)                        |
| `I`       | **I**nvert selection                          |
| `P` / `←` | **P**revious page                             |
| `N` / `→` | **N**ext page (becomes `Apply ✓` on the last) |
| `Tab`     | move focus between list and buttons           |
| `Q` / `Esc` | quit / abort                                |

**Buttons** (always visible, all clickable via `Tab` + `Enter`):

```
[P] Previous   [N] Next/Apply   [A] Select all   [Z] Select none   [I] Invert   [Q] Quit
```

**Item badges** appear next to each entry to tell you what's what:

| Badge              | Meaning                                                                  |
|--------------------|--------------------------------------------------------------------------|
| `[essential]`      | flagged in the YAML as essential                                         |
| `[os]`             | arrived with the original OS install (don't need to reinstall)           |
| `[installed]`      | already installed at the snapshot's version (restore-time only)          |
| `[installed: X]`   | installed at a *different* version — you may want to upgrade             |
| `[unverified]`     | inferred via heuristics — review carefully                               |

**Confirmation prompt** after the picker:

```
  Save snapshot with this selection?  [Y/n/b] (b = back to picker):
```

`b` re-opens the picker with your current ticks preserved — keep editing, no need to start over. The same `[Y/n/b]` prompt fires before installs on restore.

---

## CLI

```
reliure                          # interactive backup workflow (default)
reliure backup [flags]           # explicit alias for the default
reliure snapshot [flags]         # scan + write YAML, no picker, no installs
reliure restore <snapshot.yaml>  # picker + run installs
reliure report  <snapshot.yaml>  # render a styled HTML report (printable to PDF)
reliure list    <snapshot.yaml>  # v0.2 stub
reliure diff    <a> <b>          # v0.2 stub
reliure version
```

**`backup` flags**

| Flag                        | What it does                                                  |
|-----------------------------|---------------------------------------------------------------|
| `-o, --output PATH`         | snapshot YAML path (default: `~/.config/reliure/snapshots/reliure-YYYYMMDD.yaml`) |
| `--source NAME`             | restrict to these scanners (repeatable)                        |
| `--no-include-inference`    | skip the history + manual-install scanners                    |
| `--exclude PATTERN`         | drop entries whose `id` matches the glob (repeatable)         |
| `--edit`                    | open the snapshot in `$EDITOR` after writing                  |
| `--no-tui`                  | skip the picker; keep everything scanned                      |
| `--no-gnome`                | skip the GNOME settings dump                                  |

**`restore` flags**

| Flag                  | What it does                                              |
|-----------------------|-----------------------------------------------------------|
| `--dry-run`           | print commands without executing                           |
| `--essential-only`    | non-interactive: install items flagged `essential: true`  |
| `--source NAME`       | restrict to these sources (repeatable)                     |
| `--no-gnome`          | skip the GNOME dconf prompt                                |
| `--gnome-file PATH`   | explicit dconf path                                        |
| `-y, --yes`           | skip the confirmation prompts (and the GNOME yes/no)       |

**`report` flags**

| Flag             | What it does                                              |
|------------------|-----------------------------------------------------------|
| `-o, --output PATH` | output HTML path (default: `./reliure-report-YYYYMMDD.html` in cwd) |

---

## Reports

`reliure report` turns a snapshot into a styled, standalone HTML document — useful as an archival artefact or a printable PDF.

```bash
reliure report ~/.config/reliure/snapshots/reliure-20260426.yaml
# → ./reliure-report-20260426.html
xdg-open reliure-report-20260426.html      # or just double-click
# Then Cmd/Ctrl-P → "Save as PDF" for a printable copy
```

Layout: a header with the date / host / OS / package counts, a two-column table of contents, one section per source with a name/version/tags table, and a footer with the GPL line. Light/dark adaptive. The print stylesheet hides the TOC, sets 1.8 cm page margins, and prevents row breaks across pages — Cmd/Ctrl-P → Save as PDF gives a clean PDF.

---

## What gets scanned

| Source            | How                                                                                                                               |
|-------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| `apt`             | `apt-mark showmanual` (manually-installed only). Each package also gets `os_package: true` if it landed during the original OS install (signal: `/var/log/installer/*` mtime, cross-checked against the dpkg-log install burst). |
| `flatpak`         | `flatpak list --app`                                                                                                              |
| `snap`            | `snap list` (infrastructure snaps filtered)                                                                                       |
| `vscode`          | `code --list-extensions`                                                                                                          |
| `gnome-ext`       | `~/.local/share/gnome-shell/extensions/` and `/usr/share/gnome-shell/extensions/` — restore via the Extensions app (URL surfaced) |
| GNOME settings    | `dconf dump /org/gnome/` (optional, prompted at backup time)                                                                      |
| shell history     | `~/.bash_history`, `~/.zsh_history` for install commands (`apt`, `snap`, `flatpak`, `pip`, `pipx`, `cargo`, `npm`, VS Code)        |
| manual installs   | `/opt/`, `/usr/local/bin/`, `~/.local/bin/`, `/etc/apt/sources.list.d/`, plus fingerprint dirs (`~/.nvm`, `~/.ollama`, `/etc/docker`, `~/.rustup`, `~/.deno`, `~/.bun`, `~/.pyenv`, …) |

History and manual results are flagged `unverified: true` — they appear on a separate review page in the picker, default off, **never auto-installed**.

---

## Build from source

Requires Go 1.23+.

```bash
git clone https://github.com/benjaminbellamy/reliure.git
cd reliure
go mod tidy
make build         # → ./bin/reliure
make build-static  # CGO-disabled, stripped — what release builds use
make test
```

The release pipeline (`.goreleaser.yaml`) builds the published `linux/amd64` binary on every `v*` tag push via GitHub Actions.

---

## License

Copyright (C) 2026 Benjamin Bellamy.

This program is free software: you can redistribute it and/or modify it under the terms of the [GNU General Public License v3 or later](LICENSE) as published by the Free Software Foundation.
