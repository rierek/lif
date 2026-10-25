package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// assertQuitsAtOnce sends ctrl+c and requires tea.Quit, no confirm, no lobby.
func assertQuitsAtOnce(t *testing.T, m model) {
	t.Helper()
	tm, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	nm := tm.(model)
	if nm.confirmQuit && !m.confirmQuit {
		t.Fatal("ctrl+c opened the quit confirm")
	}
	if nm.view == viewLobby && m.view != viewLobby {
		t.Fatal("ctrl+c opened the lobby")
	}
	if cmd == nil {
		t.Fatal("ctrl+c returned no cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not return tea.Quit")
	}
}

func TestCtrlCQuitsFromEveryState(t *testing.T) {
	states := map[string]func(m model) model{
		"browse":      func(m model) model { return m },
		"search":      func(m model) model { return step(t, m, runeMsg('/')) },
		"filter":      func(m model) model { return step(t, m, runeMsg('f')) },
		"palette":     func(m model) model { return step(t, m, runeMsg('K')) },
		"help":        func(m model) model { return step(t, m, runeMsg('?')) },
		"picker":      func(m model) model { return step(t, m, runeMsg('P')) },
		"ready":       func(m model) model { return step(t, m, runeMsg('R')) },
		"confirmQuit": func(m model) model { return step(t, m, runeMsg('q')) },
		"form":        func(m model) model { return step(t, m, runeMsg('c')) },
		"value menu":  func(m model) model { return step(t, m, runeMsg('s')) },
		"lobby":       func(m model) model { m.view = viewLobby; return m },
	}
	for name, setup := range states {
		t.Run(name, func(t *testing.T) {
			assertQuitsAtOnce(t, setup(fixtureModel(t, fixtureBeans())))
		})
	}
}

func TestHelpListsCtrlC(t *testing.T) {
	if keys.QuitNow.Help().Key != "ctrl+c" {
		t.Fatal("QuitNow help key is not ctrl+c")
	}
}
