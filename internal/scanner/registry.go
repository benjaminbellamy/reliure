package scanner

// Registry returns scanner instances by name.
type Registry struct {
	direct    map[string]Scanner
	inference map[string]Scanner
}

// DefaultRegistry returns the registry with all built-in scanners wired up.
// Scanners are deterministic in iteration order via AllNames.
func DefaultRegistry() *Registry {
	return &Registry{
		direct: map[string]Scanner{
			"apt":       AptScanner{},
			"flatpak":   FlatpakScanner{},
			"snap":      SnapScanner{},
			"vscode":    VSCodeScanner{},
			"gnome-ext": GNOMEExtScanner{},
			"pip":       PipScanner{},
			"pipx":      PipxScanner{},
			"cargo":     CargoScanner{},
			"npm":       NpmScanner{},
			"ollama":    OllamaScanner{},
			"go":        GoBinScanner{},
			"appimage":  AppImageScanner{},
			"wifi":      WifiScanner{},
			"vpn":       VPNScanner{},
			"mounts":    MountsScanner{},
			"bluetooth": BluetoothScanner{},
			"udev":      UdevScanner{},
		},
		inference: map[string]Scanner{
			"history": HistoryScanner{},
			"manual":  ManualScanner{},
		},
	}
}

// directOrder is the canonical iteration order for direct scanners.
var directOrder = []string{"mounts", "apt", "flatpak", "snap", "vscode", "gnome-ext", "pip", "pipx", "cargo", "npm", "go", "ollama", "appimage", "wifi", "vpn", "bluetooth", "udev"}

// inferenceOrder is the canonical iteration order for inference scanners.
var inferenceOrder = []string{"history", "manual"}

// Get returns a scanner instance by name. Returns nil for unknown names.
func (r *Registry) Get(name string) Scanner {
	if s, ok := r.direct[name]; ok {
		return s
	}
	if s, ok := r.inference[name]; ok {
		return s
	}
	return nil
}

// AllNames returns the names of all known scanners. ``includeInference``
// adds inference sources at the end of the list.
func (r *Registry) AllNames(includeInference bool) []string {
	out := append([]string{}, directOrder...)
	if includeInference {
		out = append(out, inferenceOrder...)
	}
	return out
}

// Selected returns the scanner instances to run, given an optional name
// filter and whether to include inference sources.
func (r *Registry) Selected(names []string, includeInference bool) []Scanner {
	allowed := map[string]struct{}{}
	if names != nil {
		for _, n := range names {
			allowed[n] = struct{}{}
		}
	}
	pick := func(n string, src map[string]Scanner) Scanner {
		if names == nil {
			return src[n]
		}
		if _, ok := allowed[n]; ok {
			return src[n]
		}
		return nil
	}
	out := []Scanner{}
	for _, n := range directOrder {
		if s := pick(n, r.direct); s != nil {
			out = append(out, s)
		}
	}
	if includeInference {
		for _, n := range inferenceOrder {
			if s := pick(n, r.inference); s != nil {
				out = append(out, s)
			}
		}
	}
	return out
}
