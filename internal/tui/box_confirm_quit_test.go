package tui

// box_confirm_quit_test.go — TDD coverage for B08 (bean bt-1u0t, epic-E8-
// plan.md Task 5): the quit-confirm text becomes a question (A1) and
// keyConfirmQuit's enter branch turns into a two-stage cascade via the
// Lobby (A2). Pattern: fixtureModel/step (update_test.go) round-trip
// through the real m.Update, same precedent as every other Update-test in
// this package.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestQuitBoxTextIsQuestion guards A1: the confirm text is a QUESTION
// ("Really quit lif?"), not a statement ("Really quit lif.") -- the modal
// prompts a decision, it does not assert a fact.
func TestQuitBoxTextIsQuestion(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24
	m.confirmQuit = true

	out := m.quitBox()
	if !strings.Contains(out, "Really quit lif?") {
		t.Fatalf("quitBox() does not contain the question form, got:\n%s", out)
	}
	if strings.Contains(out, "Really quit lif.") {
		t.Fatalf("quitBox() still contains the old statement form, got:\n%s", out)
	}
}

// TestKeyConfirmQuitEnterGoesToLobbyWhenReposConfiguredAndNotInLobby guards
// A2's stage 1: Browse/Backlog + at least one configured repo -> enter on
// the quit-confirm opens the Lobby instead of quitting (bean bt-ntoz B08
// case 1).
func TestKeyConfirmQuitEnterGoesToLobbyWhenReposConfiguredAndNotInLobby(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24
	m.settings.Repos = []string{"/repo/a"}
	// fixtureModel/newModel defaults to viewBrowseRepo (types.go newModel).

	m = step(t, m, runeMsg('q'))
	if !m.confirmQuit {
		t.Fatal("setup: q did not open the quit-confirm")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.confirmQuit {
		t.Fatal("confirmQuit must be cleared once the cascade resolves (Lobby or Quit)")
	}
	if nm.view != viewLobby {
		t.Fatalf("view = %v, want viewLobby (stage 1 of the cascade)", nm.view)
	}
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("enter must NOT quit when repos are configured and not already in the Lobby -- it must stop at the Lobby (stage 1)")
		}
	}
}

// TestKeyConfirmQuitEnterQuitsWhenAlreadyInLobby guards A2's stage 2: once
// already IN the Lobby, enter on the quit-confirm quits for real (bean
// bt-ntoz B08 case 2) -- regardless of Settings.Repos, since m.view ==
// viewLobby already short-circuits the cascade's first condition.
func TestKeyConfirmQuitEnterQuitsWhenAlreadyInLobby(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24
	m.settings.Repos = []string{"/repo/a"}
	m.view = viewLobby
	m.confirmQuit = true

	_, cmd := m.Update(keyMsg(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on the quit-confirm must return a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("enter's Cmd did not resolve to tea.QuitMsg once already in the Lobby (stage 2)")
	}
}

// TestKeyConfirmQuitEnterQuitsWhenNoReposConfigured guards the documented
// Randfall (bean bt-1u0t / bt-ntoz B08, design-spec.md §15 PF-16): a fresh
// start with NO configured repos would make the Lobby stop empty/pointless,
// so the cascade skips it and quits directly, same as before A2.
func TestKeyConfirmQuitEnterQuitsWhenNoReposConfigured(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24
	// m.settings.Repos is nil by default (fixtureModel/newModel never loads
	// config.yaml) -- this IS the Randfall.

	m = step(t, m, runeMsg('q'))
	if !m.confirmQuit {
		t.Fatal("setup: q did not open the quit-confirm")
	}

	_, cmd := m.Update(keyMsg(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on the quit-confirm must return a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("enter's Cmd did not resolve to tea.QuitMsg for the no-repos Randfall")
	}
}

// TestQuitBoxHintTextContextSensitive guards the Planner's hint-text
// add-on (over B08's literal wording, bean bt-1u0t): the modal's own hint
// line names the actual next step -- "go to lobby" while stage 1 of the
// cascade still applies, "quit" once enter would really exit (Lobby already
// reached, or the no-repos Randfall).
func TestQuitBoxHintTextContextSensitive(t *testing.T) {
	t.Run("stage 1: repos configured, not in Lobby -> go to lobby hint", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m.width, m.height = 80, 24
		m.settings.Repos = []string{"/repo/a"}
		m.confirmQuit = true

		out := m.quitBox()
		if !strings.Contains(out, "go to lobby") {
			t.Fatalf("quitBox() hint does not mention 'go to lobby', got:\n%s", out)
		}
	})

	t.Run("stage 2: already in Lobby -> quit hint", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m.width, m.height = 80, 24
		m.settings.Repos = []string{"/repo/a"}
		m.view = viewLobby
		m.confirmQuit = true

		out := m.quitBox()
		if strings.Contains(out, "go to lobby") {
			t.Fatalf("quitBox() hint still says 'go to lobby' while already in the Lobby, got:\n%s", out)
		}
		if !strings.Contains(out, "quit") {
			t.Fatalf("quitBox() hint does not mention 'quit', got:\n%s", out)
		}
	})

	t.Run("randfall: no repos configured -> quit hint", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m.width, m.height = 80, 24
		m.confirmQuit = true

		out := m.quitBox()
		if strings.Contains(out, "go to lobby") {
			t.Fatalf("quitBox() hint says 'go to lobby' despite no configured repos (Randfall), got:\n%s", out)
		}
		if !strings.Contains(out, "quit") {
			t.Fatalf("quitBox() hint does not mention 'quit', got:\n%s", out)
		}
	})
}
