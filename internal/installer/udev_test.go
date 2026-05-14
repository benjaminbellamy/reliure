// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package installer

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/benjaminbellamy/reliure/internal/snapshot"
)

// nullReporter swallows every event — the installer-under-test cares about
// the ItemResult slice it returns, not the streaming side effects.
type nullReporter struct{}

func (nullReporter) Section(string)      {}
func (nullReporter) SectionTotal(int)    {}
func (nullReporter) Result(ItemResult)   {}
func (nullReporter) Note(string, ...any) {}
func (nullReporter) Warn(string, ...any) {}

// pkgWithBody returns a udev Package whose Payload is base64(body) and
// whose Evidence is <udevTargetDir>/<name>.
func pkgWithBody(name, body string) snapshot.Package {
	return snapshot.Package{
		ID:       "udev:" + name,
		Name:     name,
		Source:   snapshot.SourceUdev,
		Evidence: filepath.Join(udevTargetDir, name),
		Payload:  base64.StdEncoding.EncodeToString([]byte(body)),
	}
}

// pointUdevDirAt swaps the installer's target dir for the test's temp
// dir, restoring it on cleanup.
func pointUdevDirAt(t *testing.T, dir string) {
	t.Helper()
	prev := udevTargetDir
	udevTargetDir = dir
	t.Cleanup(func() { udevTargetDir = prev })
}

func TestUdevInstallerAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	pointUdevDirAt(t, dir)

	body := "SUBSYSTEM==\"hidraw\", MODE=\"0660\", GROUP=\"plugdev\"\n"
	name := "50-qmk.rules"
	target := filepath.Join(dir, name)
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	opts := Options{Stdout: out, Stderr: out}

	res := UdevInstaller{}.Install(context.Background(),
		[]snapshot.Package{pkgWithBody(name, body)}, opts, nullReporter{})

	if len(res) != 1 || res[0].Outcome != OutcomeAlreadyInstalled {
		t.Fatalf("got %+v, want one OutcomeAlreadyInstalled", res)
	}
	if out.Len() != 0 {
		t.Errorf("nothing should be executed; got stdout %q", out.String())
	}
}

func TestUdevInstallerDryRun(t *testing.T) {
	dir := t.TempDir()
	pointUdevDirAt(t, dir)

	body := "ATTRS{idVendor}==\"0483\", MODE=\"0666\"\n"
	name := "99-platformio.rules"

	out := &bytes.Buffer{}
	opts := Options{DryRun: true, Stdout: out, Stderr: out}

	res := UdevInstaller{}.Install(context.Background(),
		[]snapshot.Package{pkgWithBody(name, body)}, opts, nullReporter{})

	if len(res) != 1 || res[0].Outcome != OutcomeInstalled {
		t.Fatalf("got %+v, want one OutcomeInstalled (dry-run)", res)
	}

	target := filepath.Join(dir, name)
	stdout := out.String()
	for _, want := range []string{
		"sudo install -m 644 -o root -g root",
		target,
		"sudo udevadm control --reload-rules",
		"sudo udevadm trigger",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("dry-run output missing %q\n--- output ---\n%s", want, stdout)
		}
	}
	// No file should have been written, even though it's a dry run.
	if _, err := os.Stat(target); err == nil {
		t.Errorf("dry-run wrote target file at %q", target)
	}
}

func TestUdevInstallerMissingPayload(t *testing.T) {
	dir := t.TempDir()
	pointUdevDirAt(t, dir)

	bad := snapshot.Package{
		ID:       "udev:50-empty.rules",
		Name:     "50-empty.rules",
		Source:   snapshot.SourceUdev,
		Evidence: filepath.Join(dir, "50-empty.rules"),
		Payload:  "",
	}
	res := UdevInstaller{}.Install(context.Background(),
		[]snapshot.Package{bad}, Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}},
		nullReporter{})
	if len(res) != 1 || res[0].Outcome != OutcomeFailed {
		t.Fatalf("got %+v, want one OutcomeFailed", res)
	}
}

func TestUdevInstallerMissingEvidence(t *testing.T) {
	dir := t.TempDir()
	pointUdevDirAt(t, dir)

	body := "rule\n"
	bad := snapshot.Package{
		ID:       "udev:",
		Name:     "",
		Source:   snapshot.SourceUdev,
		Evidence: "",
		Payload:  base64.StdEncoding.EncodeToString([]byte(body)),
	}
	res := UdevInstaller{}.Install(context.Background(),
		[]snapshot.Package{bad}, Options{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}},
		nullReporter{})
	if len(res) != 1 || res[0].Outcome != OutcomeFailed {
		t.Fatalf("got %+v, want one OutcomeFailed", res)
	}
}
