package picker

import "github.com/charmbracelet/bubbles/key"

// Keymap groups all picker key bindings.
type Keymap struct {
	Up       key.Binding
	Down     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Toggle   key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Prev     key.Binding
	Next     key.Binding
	All      key.Binding
	None     key.Binding
	Invert   key.Binding
	Quit     key.Binding
}

// DefaultKeymap returns the picker's standard bindings.
func DefaultKeymap() Keymap {
	return Keymap{
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("PgUp", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("PgDn", "page down")),
		Home:     key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("Home", "top")),
		End:      key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("End", "bottom")),

		Toggle: key.NewBinding(key.WithKeys(" ", "enter"), key.WithHelp("Space", "toggle")),

		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "focus")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("S-Tab", "focus back")),

		Prev: key.NewBinding(key.WithKeys("p", "P", "left"), key.WithHelp("P/←", "previous")),
		Next: key.NewBinding(key.WithKeys("n", "N", "right"), key.WithHelp("N/→", "next")),

		All:    key.NewBinding(key.WithKeys("a", "A"), key.WithHelp("A", "all")),
		None:   key.NewBinding(key.WithKeys("z", "Z", "0"), key.WithHelp("Z", "none")),
		Invert: key.NewBinding(key.WithKeys("i", "I"), key.WithHelp("I", "invert")),

		Quit: key.NewBinding(key.WithKeys("q", "Q", "ctrl+c", "esc"), key.WithHelp("Q", "quit")),
	}
}
