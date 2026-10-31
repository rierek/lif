package tui

// overlay_shortcuts.go — the `?` Help-Overlay: a floating, modalPanel-based
// overview rendered from keys.helpGroups() (keymap.go, Single-Source,
// Drift-Guarded by TestHelpGroupsCoverEveryBindingExactlyOnce) -- labels and
// descriptions come straight from key.Binding.Help(), so it can never drift
// from the real bindings.

import (
	"fmt"
	"slices"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// helpEntry is one row of the help: a key label and its description, in a
// group. Keys come from keys.helpGroups() (Single-Source) and from the Views
// section below; nothing here is a second key table.
type helpEntry struct{ group, key, desc string }

// helpViews describes every view and layout in one line with the keys that
// reach it. Key labels come from the bindings, so a remap cannot leave them
// stale. The long form is wiki/reference/tui/views.md.
func helpViews() []helpEntry {
	layouts := keys.LayoutNext.Help().Key + "/" + keys.LayoutPrev.Help().Key
	palette := keys.Palette.Help().Key
	const g = "Views"
	return []helpEntry{
		{g, keys.Roadmap.Help().Key, "Roadmap: milestones and their items as a tree"},
		{g, keys.FlatList.Help().Key, "Flat list: all items in one list"},
		{g, keys.Ready.Help().Key, "Ready: items ready to work on, in one list"},
		{g, keys.Waves.Help().Key, "Waves: the children of an item as columns that follow blocked_by; also palette entry view: waves (graph)"},
		{g, keys.Picker.Help().Key, "Lobby: pick a repository"},
		{g, palette, "Tag management: command palette, entry view: tags"},
		{g, palette, "Settings: command palette, entry view: settings"},
		{g, layouts, "Layout roadmap: list at full width, the start layout"},
		{g, layouts, "Layout filter: filter bar above the list"},
		{g, layouts, "Layout filter+details: filter bar, list and details of the item"},
		{g, layouts, "Layout details: details of the item at full width"},
	}
}

// helpEntries is the whole help content in display order: Views first, then
// every keys.helpGroups() group, with the dynamic Tasks group (tui.actions)
// third among those.
func (m model) helpEntries() []helpEntry {
	groups := keys.helpGroups()
	var taskBinds []keybind.Binding
	for _, a := range m.taskActions {
		if a.Key != "" {
			taskBinds = append(taskBinds, keybind.NewBinding(keybind.WithKeys(a.Key), keybind.WithHelp(a.Key, a.Label)))
		}
	}
	if len(taskBinds) > 0 {
		groups = slices.Insert(groups, 2, helpGroup{"Tasks", taskBinds})
	}
	out := helpViews()
	for _, g := range groups {
		for _, b := range g.bindings {
			h := b.Help()
			out = append(out, helpEntry{g.title, h.Key, h.Desc})
		}
	}
	return out
}

// filterHelpEntries keeps the entries whose key, description or group name
// contains term (case-insensitive substring, no ranking).
func filterHelpEntries(es []helpEntry, term string) []helpEntry {
	if term == "" {
		return es
	}
	term = strings.ToLower(term)
	var out []helpEntry
	for _, e := range es {
		if strings.Contains(strings.ToLower(e.key+"\x00"+e.desc+"\x00"+e.group), term) {
			out = append(out, e)
		}
	}
	return out
}

// helpWidth is the box width; helpHeight the number of content rows that fit
// the terminal (box = rows + border 2 + title + search line + footer, one row
// margin). 0 means no limit (terminal size unknown).
func (m model) helpWidth() int { return clampModalWidth(72, m.width) }
func (m model) helpHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(3, m.height-6)
}

// helpLines renders the filtered content, one string per row: group headers
// only for groups with matches, a no-match line when nothing matches. The
// key column is as wide as the widest shown key; long descriptions wrap with
// a hanging indent so one entry row never breaks the row count.
func (m model) helpLines() []string {
	es := filterHelpEntries(m.helpEntries(), m.helpQuery)
	if len(es) == 0 {
		return []string{"  " + theme.Dim.Render("No matching key bindings")}
	}
	keyW := 0
	for _, e := range es {
		keyW = max(keyW, lipgloss.Width(e.key))
	}
	descW := max(10, m.helpWidth()-6-keyW)
	var lines []string
	group := ""
	for _, e := range es {
		if e.group != group {
			if group != "" {
				lines = append(lines, "")
			}
			lines = append(lines, theme.Accent.Render(e.group))
			group = e.group
		}
		pad := strings.Repeat(" ", keyW-lipgloss.Width(e.key))
		for i, l := range strings.Split(wrapText(e.desc, descW), "\n") {
			if i == 0 {
				lines = append(lines, "  "+theme.Header.Render(e.key)+pad+"  "+theme.Dim.Render(l))
			} else {
				lines = append(lines, strings.Repeat(" ", keyW+4)+theme.Dim.Render(l))
			}
		}
	}
	return lines
}

// helpBox renders the floating help: search line, the visible window of
// helpLines(), and a footer with the keys (and scroll position when the
// content is taller than the window).
func (m model) helpBox() string {
	lines := m.helpLines()
	footer := "/: search · ↑/↓/pgup/pgdn: scroll · esc/?/q: close"
	if m.helpSearching {
		footer = "type to search · enter: done · esc: clear"
	}
	if h := m.helpHeight(); h > 0 && len(lines) > h {
		off := min(max(m.helpScroll, 0), len(lines)-h)
		footer += fmt.Sprintf(" · %d-%d/%d", off+1, off+h, len(lines))
		lines = lines[off : off+h]
	}
	search := theme.Dim.Render("press / to search")
	switch {
	case m.helpSearching:
		search = theme.Accent.Render("/ ") + theme.Header.Render(m.helpQuery) + theme.Accent.Render("▏")
	case m.helpQuery != "":
		search = theme.Accent.Render("/ ") + theme.Header.Render(m.helpQuery)
	}
	return modalPanel("Keyboard shortcuts", search+"\n"+strings.Join(lines, "\n"), footer, m.helpWidth(), theme.Mauve)
}

// closeHelp closes the help and drops search and scroll state.
func (m model) closeHelp() model {
	m.helpOpen, m.helpQuery, m.helpSearching, m.helpScroll = false, "", false, 0
	return m
}

// scrollHelp moves the window by delta rows, clamped to the content.
func (m model) scrollHelp(delta int) model {
	if m.helpHeight() > 0 {
		m.helpScroll = min(max(m.helpScroll+delta, 0), max(0, len(m.helpLines())-m.helpHeight()))
	}
	return m
}

// keyHelp handles input while the Help-Overlay is open (full capture). `/`
// focuses search (printable keys are text, enter keeps the term, esc clears
// it); otherwise esc/?/q closes and ↑/↓/i/k/pgup/pgdn scroll.
func (m model) keyHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		if m.helpSearching || m.helpQuery != "" {
			m.helpQuery, m.helpSearching, m.helpScroll = "", false, 0
			return m, nil
		}
		return m.closeHelp(), nil
	case tea.KeyUp:
		return m.scrollHelp(-1), nil
	case tea.KeyDown:
		return m.scrollHelp(1), nil
	case tea.KeyPgUp:
		return m.scrollHelp(-m.helpHeight()), nil
	case tea.KeyPgDown:
		return m.scrollHelp(m.helpHeight()), nil
	}
	if m.helpSearching {
		switch msg.Type {
		case tea.KeyEnter:
			m.helpSearching = false
		case tea.KeyBackspace:
			if r := []rune(m.helpQuery); len(r) > 0 {
				m.helpQuery, m.helpScroll = string(r[:len(r)-1]), 0
			}
		case tea.KeyRunes, tea.KeySpace:
			m.helpQuery, m.helpScroll = m.helpQuery+string(msg.Runes), 0
		}
		return m, nil
	}
	switch {
	case keybind.Matches(msg, keys.Help), msg.String() == "q":
		return m.closeHelp(), nil
	case keybind.Matches(msg, keys.Search):
		m.helpSearching = true
	case keybind.Matches(msg, keys.Up):
		return m.scrollHelp(-1), nil
	case keybind.Matches(msg, keys.Down):
		return m.scrollHelp(1), nil
	}
	return m, nil
}
