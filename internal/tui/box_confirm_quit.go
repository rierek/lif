package tui

// box_confirm_quit.go — the quit-confirm modal. Ported from devd
// (~/dev/DeveloperDashboard/apps/cli-go/internal/tui/
// box_confirm_quit.go, DD2-49/DD2-174): `q` opens this confirm instead of
// quitting immediately; `enter` confirms, `esc` cancels. `ctrl+c` bypasses
// this confirm AS LONG AS it is not already open (handleKey/keyLobby route
// it straight to tea.Quit, bean bt-7jr8 task scope: ctrl+c is the hard/
// immediate kill switch, q is the soft prompt) -- once confirmQuit IS open,
// keyConfirmQuit's full capture swallows ctrl+c like every other non-enter/
// non-esc key (same precedent as every full-capture state in handleKey;
// I01, bt-1u0t Fix-Runde 1: comment precision only, behavior unchanged).

import (
	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/theme"
)

// requestQuit opens the quit-confirm instead of quitting immediately.
func (m model) requestQuit() (tea.Model, tea.Cmd) {
	m.confirmQuit = true
	return m, nil
}

// quitBoxWillGoToLobby reports whether the NEXT enter on the quit-confirm
// resolves to stage 1 of the two-stage cascade below (Lobby, not Quit):
// Browse/Backlog with at least one configured repo. SHARED by keyConfirmQuit
// (the actual dispatch) and quitBox (the hint text) so the two can never
// drift apart -- true is stage 1 (B08/A2 case 1, bean bt-ntoz), false covers
// BOTH stage 2 (already in the Lobby, case 2) and the Randfall (no repos
// configured at all, design-spec.md §15 PF-16, Planner decision documented
// in bt-1u0t's own body: a Lobby stop that could only ever render its own
// "(no repos in config.yaml...)" empty state would be a pointless detour,
// not a real intermediate stop).
func (m model) quitBoxWillGoToLobby() bool {
	return m.view != viewLobby && len(m.settings.Repos) > 0
}

// keyConfirmQuit drives the quit-confirm: esc cancels; enter resolves the
// two-stage cascade (B08/A2, bean bt-1u0t, bean bt-ntoz) per
// quitBoxWillGoToLobby above.
func (m model) keyConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Enter):
		goToLobby := m.quitBoxWillGoToLobby()
		m.confirmQuit = false
		if goToLobby {
			return m.openLobby()
		}
		return m, tea.Quit
	case keybind.Matches(msg, keys.Back):
		m.confirmQuit = false
		return m, nil
	}
	return m, nil
}

// quitBox renders the floating quit-confirm modal. The confirm text is a
// QUESTION (A1, bean bt-1u0t: was "Really quit bt." -- a statement, fixed
// to a question; the product is called lif, beans-179g). The hint line is deliberately context-sensitive
// (Planner add-on over B08's literal wording, bt-1u0t's own body): while
// enter would only land on the Lobby (stage 1), the hint says so up front
// rather than promising "quit" and then not quitting -- lean-stack's own
// "kein Ueberraschungsverhalten" principle applied to this modal's copy.
func (m model) quitBox() string {
	body := "\n" + theme.Dim.Render("Really quit lif?") + "\n\n"
	if m.quitBoxWillGoToLobby() {
		body += theme.Accent.Render("enter") + theme.Dim.Render(": go to lobby   ") +
			theme.Accent.Render("esc") + theme.Dim.Render(": cancel")
	} else {
		body += theme.Accent.Render("enter") + theme.Dim.Render(": quit   ") +
			theme.Accent.Render("esc") + theme.Dim.Render(": cancel")
	}
	return modalPanel("Quit?", body, "", 40, theme.Mauve)
}
