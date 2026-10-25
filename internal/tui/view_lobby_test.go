package tui

// view_lobby_test.go — E5 Task 6 (bean bt-zhwl): TDD coverage for the Lobby
// V1 view + Repo-Picker `p` + the design-decision-d startup-trigger
// invariant at the model layer.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/config"
)

// lobbyFixtureModel builds a model already positioned in the Lobby, with
// Settings.Repos set and repoList/repoSearch primed the way openLobby()
// itself would prime them (mirrors fixtureModel's own "already past its
// initial transition" convention, update_test.go). openLobby() reloads
// Settings from disk (bean bt-zhwl's own "lädt Settings.Repos neu, falls
// seit Start geändert" acceptance wording -- caught missing during this
// task's own tmux smoke test) -- so repos must be seeded via a REAL,
// isolated config.yaml (t.Setenv("HOME", ...) + config.SaveUserSettings),
// not just a direct model-field assignment that openLobby's own reload
// would immediately clobber.
func lobbyFixtureModel(t *testing.T, repos []string) model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	if err := config.SaveUserSettings(repos, "", "", 0); err != nil {
		t.Fatalf("SaveUserSettings: %v", err)
	}
	m := newModel(nil, "/tmp/bt-fixture-repo")
	nm, _ := m.openLobby()
	return nm.(model)
}

func TestLobbyShowsConfiguredRepos(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha", "/tmp/repo-beta"})
	out := m.viewLobby()
	if !strings.Contains(out, "/tmp/repo-alpha") {
		t.Errorf("viewLobby() output missing repo-alpha:\n%s", out)
	}
	if !strings.Contains(out, "/tmp/repo-beta") {
		t.Errorf("viewLobby() output missing repo-beta:\n%s", out)
	}
}

// TestRepoPickerBodyShowsSlugAndPath guards bt-d3ps (epic-E13-plan.md Item
// 4, PO-Redefinition Grilling 2026-07-17): each row renders
// "<slug> — <path>", not just the raw path -- the slug comes from
// data.RepoSlug (beans.prefix trimmed, or the dir basename fallback).
func TestRepoPickerBodyShowsSlugAndPath(t *testing.T) {
	dir := t.TempDir()
	yml := "lif:\n    prefix: lt-\n"
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(yml), 0o644); err != nil {
		t.Fatalf("write .lif.yml: %v", err)
	}
	m := lobbyFixtureModel(t, []string{dir})
	// A generous content width (bypassing repoPickerWidth's own 72-cell
	// production cap, view_lobby.go's own doc-stamp) -- this test targets
	// repoPickerBody's LABEL CONSTRUCTION, not its truncation behavior
	// (macOS t.TempDir() paths routinely exceed the production cap, which
	// would make this assertion width-flaky for reasons unrelated to what
	// it guards).
	out := m.repoPickerBody(300)
	want := "lt — " + dir
	if !strings.Contains(out, want) {
		t.Errorf("repoPickerBody() missing %q:\n%s", want, out)
	}
}

// TestRepoPickerBodySlugFallsBackToDirBaseWithoutBeansYML guards the
// fallback half: a config-registered repo with no (readable) .beans.yml
// still renders a row, using data.RepoSlug's dir-basename fallback -- a
// stale/misconfigured entry must never blank the row.
func TestRepoPickerBodySlugFallsBackToDirBaseWithoutBeansYML(t *testing.T) {
	dir := t.TempDir() // no .beans.yml written
	m := lobbyFixtureModel(t, []string{dir})
	out := m.repoPickerBody(300) // see TestRepoPickerBodyShowsSlugAndPath's own doc-stamp
	want := filepath.Base(dir) + " — " + dir
	if !strings.Contains(out, want) {
		t.Errorf("repoPickerBody() missing %q:\n%s", want, out)
	}
}

func TestLobbyFilterNarrowsBySearch(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha", "/tmp/repo-beta"})

	m = step(t, m, runeMsg('/'))
	for _, r := range "beta" {
		nm, _ := m.keyLobby(runeMsg(r))
		m = nm.(model)
	}
	if m.repoQuery != "beta" {
		t.Fatalf("repoQuery = %q, want %q", m.repoQuery, "beta")
	}
	filtered := m.filteredRepos()
	if len(filtered) != 1 || filtered[0] != "/tmp/repo-beta" {
		t.Fatalf("filteredRepos() = %v, want exactly [/tmp/repo-beta]", filtered)
	}

	out := m.viewLobby()
	if strings.Contains(out, "repo-alpha") {
		t.Errorf("viewLobby() still shows repo-alpha after filtering to %q:\n%s", m.repoQuery, out)
	}
	if !strings.Contains(out, "repo-beta") {
		t.Errorf("viewLobby() missing repo-beta after filtering to %q:\n%s", m.repoQuery, out)
	}
}

// TestLobbyQueryDoesNotSwallowArrowAliasLetters guards bean bt-l8e7 (E12
// Item 3, found in bt-9ipw's own review): keyLobby's nav intercept must key
// off the raw tea.KeyUp/tea.KeyDown KeyType, NOT navKey's letter-alias table
// (keys.Up binds "i", keys.Down binds "k", vim-style) -- otherwise a repo
// query starting with "i"/"k" (e.g. "ide") could never be typed, exactly
// mirroring TestTagPickerArrowKeysDoNotLeakIntoTypedText's own rationale
// (box_picker_tag_test.go, bt-9ipw). Typing the literal runes must land in
// repoQuery untouched.
func TestLobbyQueryDoesNotSwallowArrowAliasLetters(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha", "/tmp/repo-beta"})

	m = step(t, m, runeMsg('/'))
	for _, r := range "ide" {
		nm, _ := m.keyLobby(runeMsg(r))
		m = nm.(model)
	}
	if got := m.repoQuery; got != "ide" {
		t.Fatalf("repoQuery = %q, want %q -- i/k must stay literal, not be swallowed as up/down", got, "ide")
	}
}

// TestLobbyArrowKeysStillNavigateRepoList is the fix's own counter-guard:
// the raw-KeyType switch (replacing navKey's alias table) must keep moving
// m.repoList.cursor exactly as before -- the fix must not regress the
// acceptance criterion it did NOT touch.
func TestLobbyArrowKeysStillNavigateRepoList(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha", "/tmp/repo-beta"})
	if m.repoList.cursor != 0 {
		t.Fatalf("setup: repoList.cursor = %d, want 0", m.repoList.cursor)
	}

	nm, _ := m.keyLobby(tea.KeyMsg{Type: tea.KeyDown})
	m = nm.(model)
	if m.repoList.cursor != 1 {
		t.Fatalf("after KeyDown: repoList.cursor = %d, want 1", m.repoList.cursor)
	}

	nm, _ = m.keyLobby(tea.KeyMsg{Type: tea.KeyUp})
	m = nm.(model)
	if m.repoList.cursor != 0 {
		t.Fatalf("after KeyUp: repoList.cursor = %d, want 0", m.repoList.cursor)
	}
}

func TestLobbySelectSwitchesRepoAndView(t *testing.T) {
	repoA := newTestRepoTUI(t)
	repoB := newTestRepoTUI(t)

	m := lobbyFixtureModel(t, []string{repoA, repoB})
	m.repoList.cursor = 1 // repoB

	nm, cmd := m.keyLobby(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(model)
	if cmd == nil {
		t.Fatal("keyLobby(enter) returned a nil cmd, want switchRepoCmd's tea.Cmd")
	}

	msg := cmd()
	rs, ok := msg.(repoSwitchedMsg)
	if !ok {
		t.Fatalf("cmd() returned %T, want repoSwitchedMsg", msg)
	}
	if rs.err != nil {
		t.Fatalf("repoSwitchedMsg.err = %v, want nil", rs.err)
	}
	if rs.repoDir != repoB {
		t.Fatalf("repoSwitchedMsg.repoDir = %q, want %q (repoB, cursor 1)", rs.repoDir, repoB)
	}
	t.Cleanup(func() { rs.store.Close() })

	final := step(t, m, msg)
	if final.view != viewBrowseRepo {
		t.Fatalf("view after repoSwitchedMsg = %v, want viewBrowseRepo", final.view)
	}
	if final.store == nil {
		t.Fatal("store is nil after a successful repo switch")
	}
	if final.repoDir != repoB {
		t.Fatalf("repoDir = %q, want %q", final.repoDir, repoB)
	}
	if final.idx == nil {
		t.Fatal("idx is nil after a successful repo switch")
	}
	if final.storeEvents == nil {
		t.Fatal("storeEvents is nil after a successful repo switch with a working watcher")
	}
}

func TestPickerKeyOpensLobbyFromAnyView(t *testing.T) {
	beans := fixtureBeans()
	// openLobby() reloads Settings from disk (its own doc-stamp) -- isolate
	// HOME so this test never depends on whatever (if anything) happens to
	// exist at the real dev machine's ~/.config/beans-tui/config.yaml.
	t.Setenv("HOME", t.TempDir())

	t.Run("from Ready", func(t *testing.T) {
		m := fixtureModel(t, beans)
		m.view = viewReady
		out := step(t, m, runeMsg('P'))
		if out.view != viewLobby {
			t.Fatalf("view after 'P' from Ready = %v, want viewLobby", out.view)
		}
	})

	t.Run("from Browse", func(t *testing.T) {
		m := fixtureModel(t, beans)
		m.view = viewBrowseRepo
		out := step(t, m, runeMsg('P'))
		if out.view != viewLobby {
			t.Fatalf("view after 'P' from Browse = %v, want viewLobby", out.view)
		}
	})
}

// TestViewLobbyFrameMatchesWidthHeight is the regression guard for the
// height/width overflow bug found live during this task's own tmux smoke
// test (viewLobby's own doc-stamp, "B01", has the full story: the first cut
// handed lipgloss.Place/outerBorder the FULL w/h instead of budgeting
// innerW/innerH first, so the rendered frame silently overflowed m.height/
// m.width by 2 in each dimension -- invisible to every OTHER test here since
// none diffed the rendered string's own dimensions against the input).
// Mirrors TestChromeNeverOverflowsWidth's own pattern (chrome_test.go) --
// every OTHER view already has an equivalent guard, the Lobby is the fourth.
func TestViewLobbyFrameMatchesWidthHeight(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha", "/tmp/a-much-longer-repo-path-beta"})
	for _, dims := range []struct{ w, h int }{{30, 24}, {60, 20}, {100, 30}, {200, 50}} {
		m.width, m.height = dims.w, dims.h
		out := m.viewLobby()
		if got := lipgloss.Height(out); got != dims.h {
			t.Errorf("w=%d h=%d: viewLobby() height = %d, want %d", dims.w, dims.h, got, dims.h)
		}
		for i, ln := range strings.Split(out, "\n") {
			if lw := lipgloss.Width(ln); lw > dims.w {
				t.Errorf("w=%d h=%d: line %d overflows (%d > %d)", dims.w, dims.h, i, lw, dims.w)
			}
		}
	}
}

// TestNoLobbyOnSingleRepoCwdMatch guards the MODEL-layer half of design
// decision d's own invariant (the cmd/tui.go trigger decision itself is
// covered independently by cmd/tui_test.go's TestDecideStartupPrioritiesInOrder,
// which cannot invoke tui.Run -- an interactive AltScreen program -- from a
// `command go test` process): newModel/Init NEVER auto-derive viewLobby from
// Settings.Repos' size themselves -- that decision is made EXCLUSIVELY by
// Run()'s own `client == nil` branch (app.go), one layer above. A model
// built with a REAL (non-nil) client -- the E1-E4 "already resolved a repo"
// shape, regardless of how many repos Settings.Repos lists -- must stay on
// viewBrowseRepo, exactly like every pre-Task-6 session.
func TestNoLobbyOnSingleRepoCwdMatch(t *testing.T) {
	m := newModel(fixtureStore(t), "/tmp/bt-fixture-repo")
	m.settings.Repos = []string{"/tmp/repo-a", "/tmp/repo-b", "/tmp/repo-c"} // >=2, would trigger Lobby at the cmd/tui.go layer if cwd had failed to resolve
	if m.view != viewBrowseRepo {
		t.Fatalf("newModel with a non-nil client started in view %v, want viewBrowseRepo -- newModel must never auto-derive the Lobby from Settings.Repos", m.view)
	}
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned a nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(beansLoadedMsg); !ok {
		t.Fatalf("Init() with a non-nil client + view=viewBrowseRepo returned %T, want beansLoadedMsg (not the Lobby's repoMetricsMsg path)", msg)
	}
}

// lobbyFixtureModelWithClient is lobbyFixtureModel plus a live client -- the
// "p from a running session" shape (keyLobby's own doc-stamp), i.e. a Lobby
// that was reached AFTER a repo was already opened. This is the exact state
// B01 (bt-1u0t Fix-Runde 1) found broken: with esc/q/ctrl+c handled
// uniformly, a live client made stage 2 of the B08 quit cascade unreachable
// (q always bounced back to Browse, ctrl+c did not even quit).
func lobbyFixtureModelWithClient(t *testing.T, repos []string) model {
	t.Helper()
	m := lobbyFixtureModel(t, repos)
	m.store = fixtureStore(t)
	return m
}

// TestLobbyCtrlCQuitsImmediatelyWithLiveClient guards the B01 fix's second
// half: ctrl+c in the Lobby quits IMMEDIATELY (no confirm), consistent with
// ctrl+c everywhere else (bean bt-7jr8: the hard/immediate kill switch) --
// the pre-fix "ctrl+c -> back to Browse" was part of the same hole.
func TestLobbyCtrlCQuitsImmediatelyWithLiveClient(t *testing.T) {
	m := lobbyFixtureModelWithClient(t, []string{"/tmp/repo-alpha"})

	nm, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("ctrl+c in the Lobby must return a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c's Cmd did not resolve to tea.QuitMsg -- ctrl+c must stay the immediate kill switch inside the Lobby too")
	}
	if mm, ok := nm.(model); ok && mm.confirmQuit {
		t.Fatal("ctrl+c must not open the quit-confirm (immediate quit, no prompt)")
	}
}

// TestLobbyHintShowsKeysThatAct guards the Lobby hint per state: command
// keys (with the two-esc exit) by default, the text-entry keys while the
// filter is open, the second-esc prompt after the first esc.
func TestLobbyHintShowsKeysThatAct(t *testing.T) {
	m := lobbyFixtureModelWithClient(t, []string{"/tmp/repo-alpha"})
	m.width, m.height = 100, 30
	for name, tc := range map[string]struct {
		m    model
		want []string
		not  []string
	}{
		"default":   {m, []string{"/ filter", "esc esc leave", "q quit"}, []string{"type filter"}},
		"armed":     {step(t, m, keyMsg(tea.KeyEsc)), []string{"esc again to leave"}, nil},
		"filtering": {step(t, m, runeMsg('/')), []string{"type filter", "esc close filter"}, []string{"q quit"}},
	} {
		out := ansi.Strip(tc.m.viewLobby())
		for _, w := range tc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s: lobby hint lacks %q:\n%s", name, w, out)
			}
		}
		for _, w := range tc.not {
			if strings.Contains(out, w) {
				t.Errorf("%s: lobby hint wrongly shows %q:\n%s", name, w, out)
			}
		}
	}
}

// TestLobbyHintAdvertisesOnlyKeysThatNavigate guards beans-hh0o: the Lobby has
// one mode, a filter that takes text, so i/k are typed into it (see
// TestLobbyQueryDoesNotSwallowArrowAliasLetters). The footer must advertise the arrow
// keys, never i/k.
func TestLobbyHintAdvertisesOnlyKeysThatNavigate(t *testing.T) {
	m := lobbyFixtureModel(t, []string{"/tmp/repo-alpha"})
	m.width, m.height = 100, 30
	out := ansi.Strip(m.viewLobby())
	if strings.Contains(out, "i/k") {
		t.Fatalf("lobby footer advertises i/k, but they edit the filter:\n%s", out)
	}
	if !strings.Contains(out, "↑↓") {
		t.Fatalf("lobby footer lacks the arrow-key hint:\n%s", out)
	}
}
