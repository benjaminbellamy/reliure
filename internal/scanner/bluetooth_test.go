// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseBluetoothName(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{`[General]
Name=Magic Keyboard
Class=0x002540
`, "Magic Keyboard"},
		{`[General]
Name=Sony WH-1000XM4
SupportedTechnologies=BR/EDR;

[LinkKey]
Key=ABCDEF
`, "Sony WH-1000XM4"},
		{`[LinkKey]
Name=Ignored — wrong section
`, ""},
		{`# comment
[General]
# Name=NotThisOne
Name=Real Name
`, "Real Name"},
		{``, ""},
	}
	for _, c := range cases {
		if got := parseBluetoothName([]byte(c.body)); got != c.want {
			t.Errorf("parseBluetoothName(%q) = %q, want %q", c.body, got, c.want)
		}
	}
}

func TestLooksLikeMAC(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"AA:BB:CC:DD:EE:FF", true},
		{"aa:bb:cc:dd:ee:ff", true},
		{"01:23:45:67:89:AB", true},
		{"cache", false},
		{"AA:BB:CC:DD:EE", false},
		{"AA-BB-CC-DD-EE-FF", false},
		{"AA:BB:CC:DD:EE:FG", false}, // G not hex
		{"", false},
	}
	for _, c := range cases {
		if got := looksLikeMAC(c.s); got != c.want {
			t.Errorf("looksLikeMAC(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestBluetoothScannerScan(t *testing.T) {
	dir := t.TempDir()
	prev := btRoot
	btRoot = dir
	defer func() { btRoot = prev }()

	adapter := filepath.Join(dir, "AA:BB:CC:DD:EE:FF")
	if err := os.MkdirAll(adapter, 0o755); err != nil {
		t.Fatal(err)
	}
	// Cache dir at adapter level — must be ignored.
	if err := os.MkdirAll(filepath.Join(adapter, "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	devices := map[string]string{
		"11:22:33:44:55:66": "[General]\nName=Magic Keyboard\nClass=0x002540\n",
		"77:88:99:AA:BB:CC": "[General]\nName=Sony Speaker\n",
		"DD:EE:FF:00:11:22": "[LinkKey]\nKey=NOTGENERAL\n", // no Name= → falls back to MAC
	}
	for mac, body := range devices {
		dDir := filepath.Join(adapter, mac)
		if err := os.MkdirAll(dDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dDir, "info"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pkgs, err := BluetoothScanner{}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("got %d packages, want 3", len(pkgs))
	}
	wantNames := map[string]bool{
		"Magic Keyboard":    false,
		"Sony Speaker":      false,
		"DD:EE:FF:00:11:22": false, // fallback name
	}
	for _, p := range pkgs {
		if _, ok := wantNames[p.Name]; !ok {
			t.Errorf("unexpected name: %q", p.Name)
			continue
		}
		wantNames[p.Name] = true
		decoded, err := base64.StdEncoding.DecodeString(p.Payload)
		if err != nil || len(decoded) == 0 {
			t.Errorf("payload for %q: %v / len=%d", p.Name, err, len(decoded))
		}
	}
	for n, hit := range wantNames {
		if !hit {
			t.Errorf("missing expected name %q", n)
		}
	}
}

// TestBluetoothScannerDedupesByName exercises both flavours of duplicate:
// (1) Classic BT — the same device MAC paired with two adapters (built-in
// + USB dongle or an old adapter dir); (2) BLE — the same physical
// device paired with two adapters under different resolvable random
// addresses (Logi M650 etc.). The scanner should keep one entry per
// friendly name, preferring the adapter whose dir has the most recent
// mtime.
func TestBluetoothScannerDedupesByName(t *testing.T) {
	dir := t.TempDir()
	prev := btRoot
	btRoot = dir
	defer func() { btRoot = prev }()

	old := filepath.Join(dir, "AA:AA:AA:AA:AA:AA")
	fresh := filepath.Join(dir, "BB:BB:BB:BB:BB:BB")

	// Classic BT speaker — same MAC under both adapters.
	classicMAC := "11:22:33:44:55:66"
	// BLE mouse — different RPA per adapter, same friendly name.
	bleOldMAC := "D7:A4:70:37:27:EC"
	bleFreshMAC := "D7:A4:70:37:27:ED"

	mk := func(adapter, mac, name string) {
		dir := filepath.Join(adapter, mac)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "info"),
			[]byte("[General]\nName="+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk(old, classicMAC, "Sony Speaker")
	mk(fresh, classicMAC, "Sony Speaker")
	mk(old, bleOldMAC, "Logi M650")
	mk(fresh, bleFreshMAC, "Logi M650")

	now := time.Now()
	if err := os.Chtimes(old, now.Add(-2*time.Hour), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(fresh, now, now); err != nil {
		t.Fatal(err)
	}

	pkgs, err := BluetoothScanner{}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 {
		for _, p := range pkgs {
			t.Logf("got: %s · %s", p.Name, p.Evidence)
		}
		t.Fatalf("got %d packages, want 2 (one per name)", len(pkgs))
	}
	for _, p := range pkgs {
		// Each kept entry must come from the fresh adapter dir.
		if filepath.Dir(filepath.Dir(p.Evidence)) != fresh {
			t.Errorf("%q kept from wrong adapter: %q", p.Name, p.Evidence)
		}
	}
}
