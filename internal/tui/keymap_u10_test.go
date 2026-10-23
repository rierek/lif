package tui

// keymap_u10_test.go — beans-u4uu (U10): one consistent key map. The tests
// drive real key presses through Update and assert the effect; none of them
// re-implements dispatch.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// u10Browse is the shared start state: roadmap view, focused task, list layout.
func u10Browse(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	return m.setLayout(layoutRoadmap)
}

func u10Flat(t *testing.T) model { m := u10Browse(t); m.flatView = true; return m }

// u10Ready is the Ready view over a model whose Ready set holds tk-1 and tk-2.
func u10Ready(t *testing.T) model {
	m := u10Browse(t)
	m.readyIDs = map[string]bool{"tk-1": true, "tk-2": true}
	return step(t, m, runeMsg('R'))
}

type viewState struct {
	view viewID
	flat bool
}

func stateOf(m model) viewState { return viewState{m.view, m.flatView} }

var (
	stRoadmap = viewState{viewBrowseRepo, false}
	stFlat    = viewState{viewBrowseRepo, true}
	stReady   = viewState{viewReady, false}
)

// SC-01, AC1-4: the view keys, from every view.
func TestViewKeysSwitchFromEveryView(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // P reloads the settings file
	starts := map[string]func(*testing.T) model{"roadmap": u10Browse, "flat": u10Flat, "ready": u10Ready}
	for name, start := range starts {
		for key, want := range map[rune]viewState{'M': stRoadmap, 'L': stFlat, 'R': stReady} {
			t.Run(name+"/"+string(key), func(t *testing.T) {
				if got := stateOf(step(t, start(t), runeMsg(key))); got != want {
					t.Fatalf("state after %c = %+v, want %+v", key, got, want)
				}
			})
		}
		t.Run(name+"/P", func(t *testing.T) {
			if got := step(t, start(t), runeMsg('P')).view; got != viewLobby {
				t.Fatalf("view after P = %v, want the Lobby", got)
			}
		})
	}
}

// AC5: esc leaves flat list and Ready for the Roadmap; Roadmap esc stays.
func TestEscReturnsToRoadmap(t *testing.T) {
	for _, tc := range []struct {
		name   string
		start  func(*testing.T) model
		layout layoutMode
	}{
		{"flat", u10Flat, layoutRoadmap},
		{"ready", u10Ready, layoutRoadmap},
		{"flat detail layout", u10Flat, layoutDetail},
		{"ready detail layout", u10Ready, layoutDetail}, // U1 review finding: used to be a trap
		{"ready filter+detail layout", u10Ready, layoutFilterDetail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.start(t).setLayout(tc.layout)
			if got := stateOf(step(t, m, keyMsg(tea.KeyEsc))); got != stRoadmap {
				t.Fatalf("state after esc = %+v, want roadmap", got)
			}
		})
	}
	if got := stateOf(step(t, u10Browse(t), keyMsg(tea.KeyEsc))); got != stRoadmap {
		t.Fatalf("esc in roadmap changed the view to %+v", got)
	}
}

// AC5: an active search or filter is cancelled first; the view stays.
func TestEscCancelsFilterBeforeLeavingView(t *testing.T) {
	for name, start := range map[string]func(*testing.T) model{"flat": u10Flat, "ready": u10Ready} {
		t.Run(name, func(t *testing.T) {
			m := setSearchQuery(start(t), "Task")
			m = step(t, m, keyMsg(tea.KeyEsc))
			if m.searchQuery != "" {
				t.Fatalf("searchQuery = %q after first esc, want cleared", m.searchQuery)
			}
			if got := stateOf(m); got == stRoadmap {
				t.Fatal("first esc left the view although a search was active")
			}
			if got := stateOf(step(t, m, keyMsg(tea.KeyEsc))); got != stRoadmap {
				t.Fatalf("second esc: state = %+v, want roadmap", got)
			}
		})
	}
}

// AC6-10: item keys open their menus on the focused item, from both
// layouts that keep node keys reachable.
func TestItemKeysOpenTheirPickers(t *testing.T) {
	for key, want := range map[rune]overlayID{
		't': overlayValueMenu, 'p': overlayValueMenu, 's': overlayValueMenu,
		'g': overlayTagPicker, 'a': overlayParentPicker, 'b': overlayBlockingPicker,
		'd': overlayDeleteConfirm,
	} {
		for name, start := range map[string]func(*testing.T) model{"roadmap": u10Browse, "flat": u10Flat, "ready": u10Ready} {
			t.Run(name+"/"+string(key), func(t *testing.T) {
				m := step(t, start(t), runeMsg(key))
				if m.overlay != want {
					t.Fatalf("overlay after %c = %v, want %v", key, m.overlay, want)
				}
			})
		}
	}
	for key, group := range map[rune]string{'t': "type", 'p': "priority", 's': "status"} {
		if got := step(t, u10Browse(t), runeMsg(key)).valueMenuGroup(); got != group {
			t.Errorf("%c opens value menu %q, want %q", key, got, group)
		}
	}
}

// AC9, AC10: the labels.
func TestAssignAndBlockingLabels(t *testing.T) {
	if got := keys.Assign.Help().Desc; got != "assign parent" {
		t.Errorf("a label = %q, want %q", got, "assign parent")
	}
	if got := keys.Blocking.Help().Desc; got != "Blocking" {
		t.Errorf("b label = %q, want %q", got, "Blocking")
	}
}

// AC11: K and : open the Command Center.
func TestPaletteKeys(t *testing.T) {
	for _, k := range []rune{'K', ':'} {
		if m := step(t, u10Browse(t), runeMsg(k)); !m.paletteOpen {
			t.Errorf("%c did not open the Command Center", k)
		}
	}
}

// AC12: G, D, o, u, r are bound to nothing and do nothing.
func TestRemovedKeysAreUnbound(t *testing.T) {
	removed := []rune{'G', 'D', 'o', 'u', 'r'}
	bound := keys.boundKeys()
	for _, k := range removed {
		if bound[string(k)] {
			t.Errorf("key %c is still bound", k)
		}
		for name, start := range map[string]func(*testing.T) model{"roadmap": u10Browse, "ready": u10Ready} {
			m := start(t)
			after := step(t, m, runeMsg(k))
			if after.overlay != overlayNone || after.paletteOpen || stateOf(after) != stateOf(m) || after.View() != m.View() {
				t.Errorf("%s: key %c still has an effect", name, k)
			}
		}
	}
}

// AC13: y confirms and n cancels in every confirm dialog.
func TestConfirmDialogsAcceptYAndN(t *testing.T) {
	deleteOpen := func(t *testing.T) model {
		m := u10Browse(t)
		m.store = fixtureStore(t)
		return step(t, m, runeMsg('d'))
	}
	t.Run("delete y", func(t *testing.T) {
		m := deleteOpen(t)
		tm, cmd := m.Update(runeMsg('y'))
		if tm.(model).overlay != overlayNone || cmd == nil {
			t.Fatalf("y: overlay=%v cmd=%v, want closed overlay and a delete cmd", tm.(model).overlay, cmd)
		}
	})
	t.Run("delete n", func(t *testing.T) {
		tm, cmd := deleteOpen(t).Update(runeMsg('n'))
		if tm.(model).overlay != overlayNone || cmd != nil {
			t.Fatal("n did not cancel the delete confirm")
		}
	})
	t.Run("quit y", func(t *testing.T) {
		m := step(t, u10Browse(t), runeMsg('q'))
		if !m.confirmQuit {
			t.Fatal("q did not open the quit confirm")
		}
		tm, cmd := m.Update(runeMsg('y'))
		if tm.(model).confirmQuit {
			t.Fatal("y left the quit confirm open")
		}
		_ = cmd
	})
	t.Run("quit n", func(t *testing.T) {
		m := step(t, step(t, u10Browse(t), runeMsg('q')), runeMsg('n'))
		if m.confirmQuit {
			t.Fatal("n did not cancel the quit confirm")
		}
	})
}

// AC13: the tag-management delete confirm takes y as well.
func TestTagManagementConfirmAcceptsY(t *testing.T) {
	m := u10Browse(t)
	m.view = viewTagManagement
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "x"
	m.store = fixtureStore(t)
	tm, cmd := m.Update(runeMsg('y'))
	if tm.(model).tagMgmtDeleteConfirm || cmd == nil {
		t.Fatal("y did not confirm the tag delete")
	}
}

func lobbyModel(t *testing.T, live bool) model {
	t.Helper()
	m := u10Browse(t)
	if live {
		m.store = fixtureStore(t)
	}
	m.view = viewLobby
	m.settings.Repos = []string{"/tmp/alpha", "/tmp/quux"}
	m.repoSearch.Focus()
	return m
}

// AC14: two presses of esc leave the Lobby.
func TestLobbyNeedsTwoEscToLeave(t *testing.T) {
	m := step(t, lobbyModel(t, true), keyMsg(tea.KeyEsc))
	if m.view != viewLobby {
		t.Fatal("one esc already left the Lobby")
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.view != viewBrowseRepo {
		t.Fatalf("view after two esc = %v, want the live repo", m.view)
	}

	// Another key between the presses resets the count.
	m = step(t, lobbyModel(t, true), keyMsg(tea.KeyEsc))
	m = step(t, m, keyMsg(tea.KeyDown))
	if m = step(t, m, keyMsg(tea.KeyEsc)); m.view != viewLobby {
		t.Fatal("esc, down, esc left the Lobby")
	}

	// No live repo: two esc quit.
	m = step(t, lobbyModel(t, false), keyMsg(tea.KeyEsc))
	_, cmd := m.Update(keyMsg(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("second esc without a live repo returned no cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("second esc without a live repo did not quit")
	}
}

// Lobby: q quits at once.
func TestLobbyQQuitsAtOnce(t *testing.T) {
	m := lobbyModel(t, true)
	tm, cmd := m.Update(runeMsg('q'))
	if tm.(model).confirmQuit || cmd == nil {
		t.Fatal("q in the Lobby opened a confirm or returned no cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("q in the Lobby did not quit")
	}
}

// AC15: the repo filter opens with / only.
func TestLobbyFilterOpensWithSlashOnly(t *testing.T) {
	m := step(t, lobbyModel(t, true), runeMsg('x'))
	if m.repoQuery != "" || m.repoFilterOpen {
		t.Fatalf("typing x without / filtered the Lobby: query=%q open=%v", m.repoQuery, m.repoFilterOpen)
	}
	m = step(t, m, runeMsg('/'))
	if !m.repoFilterOpen {
		t.Fatal("/ did not open the Lobby filter")
	}
	for _, r := range "qu" {
		m = step(t, m, runeMsg(r))
	}
	if m.repoQuery != "qu" || m.view != viewLobby {
		t.Fatalf("filter typing: query=%q view=%v, want %q in the Lobby (q must be typed, not quit)", m.repoQuery, m.view, "qu")
	}
	if got := m.filteredRepos(); len(got) != 1 || got[0] != "/tmp/quux" {
		t.Fatalf("filteredRepos = %v, want [/tmp/quux]", got)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.repoFilterOpen || m.repoQuery != "" || m.view != viewLobby {
		t.Fatal("esc in the filter must close it and clear the query, staying in the Lobby")
	}
}

func keyFromName(name string) tea.KeyMsg {
	switch name {
	case "tab":
		return keyMsg(tea.KeyTab)
	case "shift+tab":
		return keyMsg(tea.KeyShiftTab)
	case "enter":
		return keyMsg(tea.KeyEnter)
	case "esc":
		return keyMsg(tea.KeyEsc)
	case "up":
		return keyMsg(tea.KeyUp)
	case "down":
		return keyMsg(tea.KeyDown)
	}
	return runeMsg([]rune(name)[0])
}

func footerKeys(m model) []string {
	var out []string
	for _, b := range m.footerBindings() {
		out = append(out, b.Help().Key)
	}
	return out
}

func mustHave(t *testing.T, got []string, key string, want bool) {
	t.Helper()
	has := false
	for _, k := range got {
		if k == key {
			has = true
		}
	}
	if has != want {
		t.Errorf("footer %v: contains %q = %v, want %v", got, key, has, want)
	}
}

// SC-03, AC16: every key the footer lists changes something when pressed.
func TestFooterListsOnlyActingKeys(t *testing.T) {
	states := map[string]func(*testing.T) model{
		"roadmap": u10Browse, "flat": u10Flat, "ready": u10Ready,
	}
	for name, start := range states {
		for l := layoutRoadmap; l < layoutCount; l++ {
			t.Run(name+"/"+layoutName(l), func(t *testing.T) {
				m := start(t).setLayout(l)
				m.store = fixtureStore(t)
				m.width, m.height = 120, 40
				for _, b := range m.footerBindings() {
					key := b.Keys()[0]
					if key == "y" {
						continue // writes the real clipboard; context_test.go covers it
					}
					tm, cmd := m.Update(keyFromName(key))
					if cmd == nil && tm.(model).View() == m.View() {
						t.Errorf("footer lists %q (%s) but pressing it does nothing", key, b.Help().Key)
					}
				}
			})
		}
	}
}

func layoutName(l layoutMode) string {
	return [...]string{"roadmap", "filter", "filterDetail", "detail"}[l]
}

// AC16: keys that only work in some layouts or views are listed only there.
func TestFooterRespectsLayoutAndView(t *testing.T) {
	detail := footerKeys(u10Browse(t).setLayout(layoutDetail))
	for _, k := range []string{"tab", "shift+tab", "/"} {
		mustHave(t, detail, k, false) // U1 review finding: dead in layoutDetail
	}
	mustHave(t, detail, "f", true) // U3: f jumps to the filter bar from every layout
	split := footerKeys(u10Browse(t).setLayout(layoutFilterDetail))
	mustHave(t, split, "tab", true)
	mustHave(t, split, "/", true)
	list := footerKeys(u10Browse(t).setLayout(layoutRoadmap))
	mustHave(t, list, "tab", false)
	mustHave(t, list, "/", true)

	road := footerKeys(u10Browse(t))
	mustHave(t, road, "M", false)
	mustHave(t, road, "L", true)
	mustHave(t, road, "R", true)
	flat := footerKeys(u10Flat(t))
	mustHave(t, flat, "M", true)
	mustHave(t, flat, "L", false)
	ready := footerKeys(u10Ready(t))
	mustHave(t, ready, "M", true)
	mustHave(t, ready, "R", false)
	for _, k := range footerKeys(u10Browse(t)) {
		if strings.ContainsAny(k, "GDour") && len(k) == 1 {
			t.Errorf("footer lists removed key %q", k)
		}
	}
}
