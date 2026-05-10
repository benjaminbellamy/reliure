// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFstabLine(t *testing.T) {
	cases := []struct {
		in       string
		want     bool
		spec     string
		mp       string
		fstype   string
		options  string
	}{
		{"# comment", false, "", "", "", ""},
		{"", false, "", "", "", ""},
		{"   ", false, "", "", "", ""},
		{"only two fields", false, "", "", "", ""},
		{"UUID=abc /data ext4 defaults 0 2", true, "UUID=abc", "/data", "ext4", "defaults"},
		{"\t/dev/sdb1\t/mnt/photos\text4\trw,noatime\t0\t2", true, "/dev/sdb1", "/mnt/photos", "ext4", "rw,noatime"},
		{"//server/share /mnt/smb cifs rw,credentials=/etc/foo 0 0", true, "//server/share", "/mnt/smb", "cifs", "rw,credentials=/etc/foo"},
	}
	for _, c := range cases {
		got, ok := parseFstabLine(c.in)
		if ok != c.want {
			t.Errorf("parseFstabLine(%q): ok=%v, want %v", c.in, ok, c.want)
			continue
		}
		if !ok {
			continue
		}
		if got.spec != c.spec || got.mountpoint != c.mp || got.fstype != c.fstype || got.options != c.options {
			t.Errorf("parseFstabLine(%q) = (%q, %q, %q, %q), want (%q, %q, %q, %q)",
				c.in, got.spec, got.mountpoint, got.fstype, got.options,
				c.spec, c.mp, c.fstype, c.options)
		}
	}
}

func TestIsSystemMount(t *testing.T) {
	cases := []struct {
		mp, fstype string
		want       bool
	}{
		{"/", "ext4", true},
		{"/boot", "ext4", true},
		{"/boot/efi", "vfat", true},
		{"/data", "ext4", false},
		{"/home/ben/Documents", "ext4", false},
		{"/mnt/photos", "btrfs", false},
		{"/mnt/smb", "cifs", false},
		{"none", "swap", true},
		{"/swap.img", "swap", true},
		{"/dev/shm", "tmpfs", true},
		{"/proc", "proc", true},
		{"/sys", "sysfs", true},
		{"/var/snap/foo", "squashfs", true},
		{"/srv/nfs", "nfs", false},
		{"/srv/nfs", "nfs4", false},
	}
	for _, c := range cases {
		got := isSystemMount(fstabEntry{mountpoint: c.mp, fstype: c.fstype})
		if got != c.want {
			t.Errorf("isSystemMount(%q, %q) = %v, want %v", c.mp, c.fstype, got, c.want)
		}
	}
}

func TestEnsureNofail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "nofail"},
		{"defaults", "defaults,nofail"},
		{"defaults,nofail", "defaults,nofail"},
		{"nofail", "nofail"},
		{"rw,noatime", "rw,noatime,nofail"},
		{"rw,nofail,noatime", "rw,nofail,noatime"},
		{"rw, nofail ,noatime", "rw, nofail ,noatime"}, // trim respected
	}
	for _, c := range cases {
		got := EnsureNofail(c.in)
		if got != c.want {
			t.Errorf("EnsureNofail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRewriteFstabLineWithNofail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"# comment", "# comment"},
		{"", ""},
		{
			"UUID=abc /data ext4 defaults 0 2",
			"UUID=abc\t/data\text4\tdefaults,nofail\t0\t2",
		},
		{
			"UUID=abc /data ext4 defaults,nofail 0 2",
			"UUID=abc /data ext4 defaults,nofail 0 2",
		},
	}
	for _, c := range cases {
		got := RewriteFstabLineWithNofail(c.in)
		if got != c.want {
			t.Errorf("RewriteFstabLineWithNofail(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFstabLineMountpoint(t *testing.T) {
	if got := FstabLineMountpoint("UUID=abc /data ext4 defaults 0 2"); got != "/data" {
		t.Errorf("FstabLineMountpoint = %q, want /data", got)
	}
	if got := FstabLineMountpoint("# comment"); got != "" {
		t.Errorf("FstabLineMountpoint(comment) = %q, want empty", got)
	}
}

func TestMountsScannerScan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fstab")
	body := `# /etc/fstab
UUID=root-uuid / ext4 defaults 0 1
UUID=boot-uuid /boot ext4 defaults 0 1
UUID=efi-uuid /boot/efi vfat defaults 0 1
/swap.img none swap sw 0 0
tmpfs /tmp tmpfs defaults 0 0

UUID=ca59b4b3-b908-48cf-a18e-da0c4f1b7579 /home/ben/Documents ext4 defaults,nofail 0 2
//server/share /mnt/smb cifs credentials=/etc/foo 0 0
nas:/exports/photos /mnt/nas nfs defaults 0 0
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	prev := fstabPath
	fstabPath = path
	defer func() { fstabPath = prev }()

	pkgs, err := MountsScanner{}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 3 {
		for _, p := range pkgs {
			t.Logf("got: %s (%s)", p.Name, p.Evidence)
		}
		t.Fatalf("got %d packages, want 3", len(pkgs))
	}
	wantMps := map[string]bool{
		"/home/ben/Documents": false,
		"/mnt/smb":            false,
		"/mnt/nas":            false,
	}
	for _, p := range pkgs {
		if _, ok := wantMps[p.Name]; !ok {
			t.Errorf("unexpected mountpoint: %q", p.Name)
			continue
		}
		wantMps[p.Name] = true
		// Payload must round-trip the exact original line.
		decoded, err := base64.StdEncoding.DecodeString(p.Payload)
		if err != nil {
			t.Errorf("payload for %q: %v", p.Name, err)
			continue
		}
		if FstabLineMountpoint(string(decoded)) != p.Name {
			t.Errorf("payload for %q does not round-trip its mountpoint", p.Name)
		}
	}
	for mp, hit := range wantMps {
		if !hit {
			t.Errorf("missing expected mountpoint %q", mp)
		}
	}
}
