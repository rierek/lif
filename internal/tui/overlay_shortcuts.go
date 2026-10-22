package tui

// overlay_shortcuts.go — the `?` Help-Overlay (E5 Task 2, bean bt-wpn9, Port
// devd overlay_shortcuts.go, ~/dev/DeveloperDashboard/apps/
// cli-go/internal/tui/overlay_shortcuts.go): a floating, modalPanel-based
// full-screen-ish overview rendered from keys.helpGroups() (keymap.go,
// Single-Source, already Drift-Guarded since E1 Task 7's
// TestHelpGroupsCoverEveryBindingExactlyOnce) -- the key labels/descriptions
// come straight from key.Binding.Help(), so this view can never drift from
// the real bindings the way a hand-maintained parallel doc could.
//
// devd also carries a shortcutMarkdown() sibling (external docs/shortcuts.md
// generator, DD2-5) -- OUT OF SCOPE here: beans-tui has no external shortcut
// doc to keep in sync (design-spec.md never asks for one), only the in-app
// overlay itself. Port only what's asked for (YAGNI), same discipline as
// keymap.go's own doc comment on the E1 Task 7 scope cut.

import (
	"slices"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// helpBox renders the floating shortcut overview from keys.helpGroups() --
// Port devd helpBox VERBATIM (modalPanel-based, key-label column width
// determined GLOBALLY across every group so the Tasten-Label column lines
// up group-to-group, not just within one).
func (m model) helpBox() string {
	groups := keys.helpGroups()
	var taskBinds []keybind.Binding
	for _, a := range m.taskActions {
		if a.Key != "" {
			taskBinds = append(taskBinds, keybind.NewBinding(keybind.WithKeys(a.Key), keybind.WithHelp(a.Key, a.Label)))
		}
	}
	if len(taskBinds) > 0 {
		// Third, not last: the box is taller than a small terminal and
		// clips at the bottom, where a late group would never show.
		groups = slices.Insert(groups, 2, helpGroup{"Tasks", taskBinds})
	}

	// Spaltenbreite für das Tasten-Label global bestimmen (ANSI-sicher).
	keyW := 0
	for _, g := range groups {
		for _, b := range g.bindings {
			if w := lipgloss.Width(b.Help().Key); w > keyW {
				keyW = w
			}
		}
	}

	var b strings.Builder
	for _, g := range groups {
		b.WriteString("\n" + theme.Accent.Render(g.title) + "\n")
		for _, bind := range g.bindings {
			h := bind.Help()
			pad := strings.Repeat(" ", keyW-lipgloss.Width(h.Key))
			b.WriteString("  " + theme.Header.Render(h.Key) + pad + "  " + theme.Dim.Render(h.Desc) + "\n")
		}
	}

	return modalPanel("Keyboard shortcuts", b.String(), "esc/?/q: close", clampModalWidth(54, m.width), theme.Mauve)
}

// keyHelp handles input while the Help-Overlay is open (full capture, same
// precedent as keyFilterMenu/keyPalette/keyForm/keyOverlay): esc/?/q close
// it (devd footer hint "esc/?/q: close", ported literally above), every
// other key is a no-op -- Help stays open, nothing leaks through to
// keyNodeAction/openPalette/the quit-confirm switch underneath (a
// deliberate deviation from devd, which closes Help on ANY key; see
// overlay_shortcuts_test.go's file doc comment for the rationale).
func (m model) keyHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?", "q":
		m.helpOpen = false
		return m, nil
	}
	return m, nil
}
