package picker

import "github.com/charmbracelet/lipgloss"

// Styles bundles all the lipgloss styles used by the picker. Group them
// together so it's easy to see (and tweak) the visual identity in one place.
type Styles struct {
	App            lipgloss.Style
	AppTitle       lipgloss.Style // top-of-screen "Reliure" wordmark
	PageTitle      lipgloss.Style // big per-source heading
	Subtitle       lipgloss.Style
	PageBadge      lipgloss.Style
	CountBadge     lipgloss.Style
	ListContainer  lipgloss.Style
	Item           lipgloss.Style
	ItemSelected   lipgloss.Style
	Checkbox       lipgloss.Style
	CheckboxOn     lipgloss.Style
	BadgeInstalled lipgloss.Style
	BadgeEssential lipgloss.Style
	BadgeVersion   lipgloss.Style
	BadgeOS        lipgloss.Style
	HelpKey        lipgloss.Style
	HelpDesc       lipgloss.Style
	HelpSep        lipgloss.Style
	Button         lipgloss.Style
	ButtonFocus    lipgloss.Style
	ButtonDisabled lipgloss.Style
	ButtonPrimary  lipgloss.Style
	StatusBar      lipgloss.Style
	HeaderRule     lipgloss.Style // horizontal divider under the heading
}

// DefaultStyles returns the picker's default visual theme. Adaptive colours
// keep things readable on both light and dark terminals.
func DefaultStyles() Styles {
	primary := lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#B294FF"}
	accent := lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#7CE38B"}
	muted := lipgloss.AdaptiveColor{Light: "#7E7E87", Dark: "#9595A2"}
	soft := lipgloss.AdaptiveColor{Light: "#E5E5EE", Dark: "#3A3A45"}
	warn := lipgloss.AdaptiveColor{Light: "#D5752E", Dark: "#FFB454"}
	subtle := lipgloss.AdaptiveColor{Light: "#A0A0AC", Dark: "#7A7A88"}

	return Styles{
		App:      lipgloss.NewStyle().Padding(0, 1),
		AppTitle: lipgloss.NewStyle().Foreground(primary).Bold(true),
		PageTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Bold(true).
			Padding(0, 2).
			MarginTop(1).
			MarginBottom(1),
		Subtitle: lipgloss.NewStyle().Foreground(muted),
		HeaderRule: lipgloss.NewStyle().Foreground(soft),

		PageBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primary).
			Bold(true).
			Padding(0, 1),
		CountBadge: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(accent).
			Bold(true).
			Padding(0, 1),

		ListContainer: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(soft).
			Padding(0, 1),

		Item:         lipgloss.NewStyle().PaddingLeft(2),
		ItemSelected: lipgloss.NewStyle().PaddingLeft(2).Foreground(primary).Bold(true),

		Checkbox:   lipgloss.NewStyle().Foreground(muted),
		CheckboxOn: lipgloss.NewStyle().Foreground(accent).Bold(true),

		BadgeInstalled: lipgloss.NewStyle().Foreground(subtle).Italic(true),
		BadgeEssential: lipgloss.NewStyle().Foreground(warn).Bold(true),
		BadgeVersion:   lipgloss.NewStyle().Foreground(muted),
		BadgeOS:        lipgloss.NewStyle().Foreground(subtle).Italic(true),

		HelpKey:  lipgloss.NewStyle().Foreground(primary).Bold(true),
		HelpDesc: lipgloss.NewStyle().Foreground(muted),
		HelpSep:  lipgloss.NewStyle().Foreground(soft),

		Button: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(soft).
			Padding(0, 2),
		ButtonFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primary).
			Foreground(primary).
			Bold(true).
			Padding(0, 2),
		ButtonDisabled: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(soft).
			Foreground(subtle).
			Padding(0, 2),
		ButtonPrimary: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Foreground(accent).
			Bold(true).
			Padding(0, 2),

		StatusBar: lipgloss.NewStyle().Foreground(muted).Padding(0, 1),
	}
}
