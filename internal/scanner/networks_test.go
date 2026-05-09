// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package scanner

import "testing"

func TestParseNMConnectionWifi(t *testing.T) {
	body := []byte(`[connection]
id=Home WiFi
uuid=12345678-1234-1234-1234-1234567890ab
type=wifi
permissions=

[wifi]
mac-address-blacklist=
mode=infrastructure
ssid=HomeNet

[wifi-security]
auth-alg=open
key-mgmt=wpa-psk
psk=supersecret123

[ipv4]
method=auto

[ipv6]
method=auto
`)
	got := parseNMConnection("Home WiFi.nmconnection", body)
	if got.id != "Home WiFi" {
		t.Errorf("id = %q, want %q", got.id, "Home WiFi")
	}
	if got.typ != "wifi" {
		t.Errorf("type = %q, want %q", got.typ, "wifi")
	}
	if got.ssid != "HomeNet" {
		t.Errorf("ssid = %q, want %q", got.ssid, "HomeNet")
	}
	if string(got.raw) != string(body) {
		t.Error("raw should round-trip the input bytes verbatim")
	}
}

func TestParseNMConnectionVPN(t *testing.T) {
	body := []byte(`[connection]
id=work-vpn
type=vpn

[vpn]
service-type=org.freedesktop.NetworkManager.openvpn
remote=vpn.example.com
`)
	got := parseNMConnection("work-vpn.nmconnection", body)
	if got.id != "work-vpn" {
		t.Errorf("id = %q, want %q", got.id, "work-vpn")
	}
	if got.typ != "vpn" {
		t.Errorf("type = %q, want %q", got.typ, "vpn")
	}
	if got.ssid != "" {
		t.Errorf("ssid = %q, want empty for vpn", got.ssid)
	}
}

func TestParseNMConnectionFallbackID(t *testing.T) {
	// No [connection] id key → fall back to filename without the suffix.
	body := []byte(`[connection]
type=wifi
`)
	got := parseNMConnection("My-Hotspot.nmconnection", body)
	if got.id != "My-Hotspot" {
		t.Errorf("id fallback = %q, want %q", got.id, "My-Hotspot")
	}
}

func TestSplitNMCLIRecord(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a:b:c:d", []string{"a", "b", "c", "d"}},
		// Embedded colon in NAME via \:
		{`weird\:name:wifi:/path:uuid`, []string{"weird:name", "wifi", "/path", "uuid"}},
		// Embedded backslash via \\
		{`back\\slash:wifi:/p:u`, []string{`back\slash`, "wifi", "/p", "u"}},
		// Empty FILENAME (memory-only connection)
		{"lo:loopback::abc", []string{"lo", "loopback", "", "abc"}},
	}
	for _, c := range cases {
		got := splitNMCLIRecord(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitNMCLIRecord(%q): got %d fields, want %d (%q)",
				c.in, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitNMCLIRecord(%q)[%d] = %q, want %q",
					c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestNMKindMatches(t *testing.T) {
	cases := []struct {
		connType, kind string
		want           bool
	}{
		{"802-11-wireless", "wifi", true},
		{"wifi", "wifi", true},
		{"vpn", "vpn", true},
		{"wireguard", "vpn", true},
		{"802-3-ethernet", "wifi", false},
		{"vpn", "wifi", false},
		{"wifi", "vpn", false},
		{"bluetooth", "wifi", false},
	}
	for _, c := range cases {
		if got := nmKindMatches(c.connType, c.kind); got != c.want {
			t.Errorf("nmKindMatches(%q, %q) = %v, want %v", c.connType, c.kind, got, c.want)
		}
	}
}
