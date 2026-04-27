package picker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Focus values: -1 means the item list is focused; 0..5 are the buttons.
const focusList = -1

const (
	btnPrev = iota
	btnNext
	btnAll
	btnNone
	btnInvert
	btnQuit
)

type buttonSpec struct {
	idx       int
	keyHelp   string // letter shown in the label
	labelLeft string // shown when not on last page
	labelLast string // shown on last page (only for "Next" → "Apply")
	primary   bool
}

var buttons = []buttonSpec{
	{idx: btnPrev, keyHelp: "P", labelLeft: "Previous", labelLast: "Previous"},
	{idx: btnNext, keyHelp: "N", labelLeft: "Next →", labelLast: "Apply ✓", primary: true},
	{idx: btnAll, keyHelp: "A", labelLeft: "Select all", labelLast: "Select all"},
	{idx: btnNone, keyHelp: "Z", labelLeft: "Select none", labelLast: "Select none"},
	{idx: btnInvert, keyHelp: "I", labelLeft: "Invert", labelLast: "Invert"},
	{idx: btnQuit, keyHelp: "Q", labelLeft: "Quit", labelLast: "Quit"},
}

type pageState struct {
	cursor   int
	checked  []bool
	viewport viewport.Model
}

type model struct {
	title  string
	pages  []Page
	states []pageState

	pi      int
	focus   int // focusList or btn*
	width   int
	height  int

	keymap Keymap
	styles Styles

	aborted bool
	done    bool
}

func newModel(title string, pages []Page) *model {
	m := &model{
		title:  title,
		pages:  pages,
		states: make([]pageState, len(pages)),
		focus:  focusList,
		keymap: DefaultKeymap(),
		styles: DefaultStyles(),
	}
	for i, p := range pages {
		st := pageState{
			checked:  make([]bool, len(p.Items)),
			viewport: viewport.New(80, 10),
		}
		for j, it := range p.Items {
			st.checked[j] = it.Checked
		}
		m.states[i] = st
	}
	return m
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) result() Result {
	r := Result{Selections: map[string][]string{}}
	for i, p := range m.pages {
		st := m.states[i]
		ids := []string{}
		for j, on := range st.checked {
			if on {
				ids = append(ids, p.Items[j].ID)
			}
		}
		r.Selections[p.ID] = ids
	}
	return r
}

// --- Update ---

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayoutViewports()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	st := &m.states[m.pi]
	page := m.pages[m.pi]
	n := len(page.Items)

	// Quit always wins.
	if key.Matches(msg, m.keymap.Quit) {
		m.aborted = true
		return m, tea.Quit
	}

	// Per-page bulk actions — work regardless of focus.
	if n > 0 {
		switch {
		case key.Matches(msg, m.keymap.All):
			for i := range st.checked {
				st.checked[i] = true
			}
			return m, nil
		case key.Matches(msg, m.keymap.None):
			for i := range st.checked {
				st.checked[i] = false
			}
			return m, nil
		case key.Matches(msg, m.keymap.Invert):
			for i := range st.checked {
				st.checked[i] = !st.checked[i]
			}
			return m, nil
		}
	}

	// Page navigation.
	switch {
	case key.Matches(msg, m.keymap.Prev):
		if m.pi > 0 {
			m.pi--
			m.focus = focusList
		}
		return m, nil
	case key.Matches(msg, m.keymap.Next):
		if m.pi < len(m.pages)-1 {
			m.pi++
			m.focus = focusList
			return m, nil
		}
		m.done = true
		return m, tea.Quit
	}

	// Tab cycles focus.
	if key.Matches(msg, m.keymap.Tab) {
		m.focus = nextFocus(m.focus, +1)
		return m, nil
	}
	if key.Matches(msg, m.keymap.ShiftTab) {
		m.focus = nextFocus(m.focus, -1)
		return m, nil
	}

	// Arrow / paging within the list.
	switch {
	case key.Matches(msg, m.keymap.Up):
		m.focus = focusList
		if n > 0 && st.cursor > 0 {
			st.cursor--
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.Down):
		m.focus = focusList
		if n > 0 && st.cursor < n-1 {
			st.cursor++
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.PageUp):
		m.focus = focusList
		if n > 0 {
			step := st.viewport.Height
			st.cursor -= step
			if st.cursor < 0 {
				st.cursor = 0
			}
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.PageDown):
		m.focus = focusList
		if n > 0 {
			step := st.viewport.Height
			st.cursor += step
			if st.cursor >= n {
				st.cursor = n - 1
			}
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.Home):
		m.focus = focusList
		if n > 0 {
			st.cursor = 0
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.End):
		m.focus = focusList
		if n > 0 {
			st.cursor = n - 1
		}
		m.relayoutViewports()
	case key.Matches(msg, m.keymap.Toggle):
		// Space / Enter:
		// - on list focus: toggle current item
		// - on a focused button: trigger the button
		if m.focus == focusList && n > 0 {
			st.checked[st.cursor] = !st.checked[st.cursor]
			return m, nil
		}
		switch m.focus {
		case btnPrev:
			if m.pi > 0 {
				m.pi--
				m.focus = focusList
			}
		case btnNext:
			if m.pi < len(m.pages)-1 {
				m.pi++
				m.focus = focusList
				return m, nil
			}
			m.done = true
			return m, tea.Quit
		case btnAll:
			for i := range st.checked {
				st.checked[i] = true
			}
		case btnNone:
			for i := range st.checked {
				st.checked[i] = false
			}
		case btnInvert:
			for i := range st.checked {
				st.checked[i] = !st.checked[i]
			}
		case btnQuit:
			m.aborted = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func nextFocus(cur, delta int) int {
	// Sequence: focusList → 0 → 1 → ... → 5 → focusList → ...
	if delta > 0 {
		if cur == focusList {
			return btnPrev
		}
		nxt := cur + 1
		if nxt > btnQuit {
			return focusList
		}
		return nxt
	}
	// delta < 0
	if cur == focusList {
		return btnQuit
	}
	if cur == btnPrev {
		return focusList
	}
	return cur - 1
}

// relayoutViewports updates each page's viewport size based on terminal
// dimensions and keeps the cursor visible.
func (m *model) relayoutViewports() {
	if m.width == 0 || m.height == 0 {
		return
	}
	// Layout budget: top row(1) + page title(2) + list(?) + buttons(3)
	// + spacer(1) + help(1) + frame padding(2)
	listH := m.height - 10
	if listH < 5 {
		listH = 5
	}
	for i := range m.states {
		st := &m.states[i]
		st.viewport.Width = m.width - 4
		st.viewport.Height = listH
	}
	st := &m.states[m.pi]
	if st.cursor < st.viewport.YOffset {
		st.viewport.SetYOffset(st.cursor)
	} else if st.cursor >= st.viewport.YOffset+st.viewport.Height {
		st.viewport.SetYOffset(st.cursor - st.viewport.Height + 1)
	}
}

// --- View ---

func (m *model) View() string {
	if m.aborted || m.done {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return ""
	}

	st := &m.states[m.pi]
	page := m.pages[m.pi]
	n := len(page.Items)

	// Top row: app wordmark · (spring) · selected count
	wordmark := m.styles.AppTitle.Render(m.title)
	on := 0
	for _, b := range st.checked {
		if b {
			on++
		}
	}
	countBadge := m.styles.CountBadge.Render(fmt.Sprintf(" %d / %d selected ", on, n))
	topRow := headerWithCounter(m.width-2, wordmark, countBadge)

	// Big visible page title (per-source heading) with right-aligned page
	// counter on the same bar. The -6 accounts for the outer margin and
	// PageTitle's horizontal padding (see styles.go).
	titleLeft := " ▌ " + strings.ToUpper(page.Title)
	titleRight := fmt.Sprintf("page %d / %d ", m.pi+1, len(m.pages))
	pageTitle := m.styles.PageTitle.
		Width(m.width - 2).
		Render(headerWithCounter(m.width-6, titleLeft, titleRight))

	// Item list
	listBody := m.renderList(st, page)
	st.viewport.SetContent(listBody)
	listView := m.styles.ListContainer.Width(m.width - 2).Render(st.viewport.View())

	// Help line + buttons
	help := m.renderHelp()
	buttonRow := m.renderButtons(n)

	return lipgloss.JoinVertical(lipgloss.Left,
		topRow,
		pageTitle,
		listView,
		"",
		buttonRow,
		"",
		help,
	)
}

func headerWithCounter(width int, left, right string) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *model) renderList(st *pageState, p Page) string {
	n := len(p.Items)
	if n == 0 {
		return m.styles.Subtitle.Render("  (no items in this section)")
	}
	var b strings.Builder
	for i, it := range p.Items {
		mark := "[ ]"
		markStyle := m.styles.Checkbox
		if st.checked[i] {
			mark = "[✓]"
			markStyle = m.styles.CheckboxOn
		}
		line := markStyle.Render(mark) + "  " + it.Label + m.renderBadges(it.Badges)
		if i == st.cursor && m.focus == focusList {
			line = m.styles.ItemSelected.Render("▸ " + line)
		} else {
			line = m.styles.Item.Render("  " + line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) renderBadges(badges []Badge) string {
	if len(badges) == 0 {
		return ""
	}
	var b strings.Builder
	for _, badge := range badges {
		b.WriteString("  ")
		var st lipgloss.Style
		switch badge.Kind {
		case BadgeInstalled:
			st = m.styles.BadgeInstalled
		case BadgeEssential:
			st = m.styles.BadgeEssential
		case BadgeVersion:
			st = m.styles.BadgeVersion
		case BadgeOS:
			st = m.styles.BadgeOS
		default:
			st = m.styles.Subtitle
		}
		b.WriteString(st.Render(badge.Text))
	}
	return b.String()
}

func (m *model) renderHelp() string {
	keys := []struct{ k, d string }{
		{"↑↓", "navigate"},
		{"Space", "toggle"},
		{"A/Z/I", "all/none/invert"},
		{"P/N", "prev/next"},
		{"Tab", "focus"},
		{"Q", "quit"},
	}
	parts := make([]string, 0, len(keys)*3)
	for i, k := range keys {
		if i > 0 {
			parts = append(parts, m.styles.HelpSep.Render(" · "))
		}
		parts = append(parts, m.styles.HelpKey.Render(k.k)+" "+m.styles.HelpDesc.Render(k.d))
	}
	return m.styles.StatusBar.Render(strings.Join(parts, ""))
}

func (m *model) renderButtons(n int) string {
	last := m.pi == len(m.pages)-1
	rendered := make([]string, 0, len(buttons))
	for _, b := range buttons {
		label := b.labelLeft
		if last {
			label = b.labelLast
		}
		txt := fmt.Sprintf("[%s] %s", b.keyHelp, label)
		disabled := false
		if b.idx == btnPrev && m.pi == 0 {
			disabled = true
		}
		if (b.idx == btnAll || b.idx == btnNone || b.idx == btnInvert) && n == 0 {
			disabled = true
		}
		var styled string
		switch {
		case disabled:
			styled = m.styles.ButtonDisabled.Render(txt)
		case m.focus == b.idx:
			styled = m.styles.ButtonFocus.Render(txt)
		case b.primary:
			styled = m.styles.ButtonPrimary.Render(txt)
		default:
			styled = m.styles.Button.Render(txt)
		}
		rendered = append(rendered, styled)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return lipgloss.NewStyle().Width(m.width - 2).Align(lipgloss.Center).Render(row)
}
