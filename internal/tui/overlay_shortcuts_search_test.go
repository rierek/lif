package tui

// overlay_shortcuts_search_test.go — U6 (bean beans-rbwc): search, Views
// section and scrolling of the `?` Help-Overlay.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func helpModel() model {
	return model{helpOpen: true, width: 80, height: 24}
}

func typeHelp(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = step(t, m, runeMsg(r))
	}
	return m
}

func helpGroupOf(es []helpEntry, group string) []helpEntry {
	var out []helpEntry
	for _, e := range es {
		if e.group == group {
			out = append(out, e)
		}
	}
	return out
}

// AC1: the overlay shows a search field.
func TestHelpShowsSearchField(t *testing.T) {
	out := ansi.Strip(helpModel().helpBox())
	if !strings.Contains(out, "/ ") || !strings.Contains(out, "search") {
		t.Errorf("helpBox() lacks a search field:\n%s", out)
	}
}

// AC2: key, description and group name match, ignoring case.
func TestFilterHelpEntriesMatchesKeyDescGroup(t *testing.T) {
	all := helpModel().helpEntries()
	// description
	for _, e := range filterHelpEntries(all, "RELOAD") {
		if !strings.Contains(strings.ToLower(e.desc), "reload") {
			t.Errorf("desc match returned %+v", e)
		}
	}
	if len(filterHelpEntries(all, "RELOAD")) == 0 {
		t.Fatal("no entry matches description refresh")
	}
	// key
	if got := filterHelpEntries(all, "ctrl+c"); len(got) == 0 {
		t.Error("no entry matches key ctrl+c")
	}
	// group name: every entry of the group is returned
	want := helpGroupOf(all, "Dialogs")
	if len(want) == 0 {
		t.Fatal("no Dialogs entries")
	}
	got := helpGroupOf(filterHelpEntries(all, "dIaLoGs"), "Dialogs")
	if len(got) != len(want) {
		t.Errorf("group match returned %d of %d Dialogs entries", len(got), len(want))
	}
	if len(filterHelpEntries(all, "")) != len(all) {
		t.Error("empty term must keep every entry")
	}
}

// AC2 live + group header only for groups with matches.
func TestHelpSearchNarrowsLive(t *testing.T) {
	m := typeHelp(t, step(t, helpModel(), runeMsg('/')), "dialogs")
	out := ansi.Strip(m.helpBox())
	if !strings.Contains(out, "Dialogs") {
		t.Errorf("group Dialogs missing:\n%s", out)
	}
	for _, other := range []string{"Navigation", "Actions", "Picker filter"} {
		if strings.Contains(out, other) {
			t.Errorf("group %q shown for term dialogs:\n%s", other, out)
		}
	}
}

// AC3.
func TestHelpNoMatchLine(t *testing.T) {
	m := typeHelp(t, step(t, helpModel(), runeMsg('/')), "zzqqxx")
	lines := m.helpLines()
	if len(lines) != 1 || !strings.Contains(ansi.Strip(lines[0]), "No matching") {
		t.Errorf("want one no-match line, got %q", lines)
	}
}

// AC4: every view and layout has a one-line description with its keys.
func TestHelpViewsSection(t *testing.T) {
	views := helpGroupOf(helpModel().helpEntries(), "Views")
	wantKeys := []string{
		keys.Roadmap.Help().Key, keys.FlatList.Help().Key, keys.Ready.Help().Key,
		keys.Picker.Help().Key, keys.Palette.Help().Key,
		keys.LayoutNext.Help().Key + "/" + keys.LayoutPrev.Help().Key,
	}
	for _, k := range wantKeys {
		found := false
		for _, e := range views {
			if e.key == k && e.desc != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("Views section has no described entry for key %q: %+v", k, views)
		}
	}
	for _, name := range []string{"Roadmap", "Flat", "Ready", "Lobby", "Tag management", "Layout roadmap", "Layout filter:", "Layout filter+details", "Layout details"} {
		found := false
		for _, e := range views {
			if strings.HasPrefix(e.desc, name) {
				found = true
			}
		}
		if !found {
			t.Errorf("Views section lacks %q", name)
		}
	}
	// searchable
	if len(filterHelpEntries(helpModel().helpEntries(), "lobby")) == 0 {
		t.Error("Views not searchable")
	}
	// tab belongs to the focus ring (U2), not to this section
	for _, e := range views {
		if strings.Contains(e.desc, "tab") {
			t.Errorf("Views entry mentions tab: %+v", e)
		}
	}
}

// AC5 / SC-02: at 80x24 the box fits and PgDn/Down reach every line.
func TestHelpScrollReachesEveryLine(t *testing.T) {
	m := helpModel()
	if h := len(strings.Split(m.helpBox(), "\n")); h > m.height {
		t.Fatalf("box is %d rows at height %d", h, m.height)
	}
	all := m.helpLines()
	if len(all) <= m.helpHeight() {
		t.Fatalf("content (%d) fits the viewport (%d): scroll untested", len(all), m.helpHeight())
	}
	seen := map[string]bool{}
	collect := func() {
		for _, l := range strings.Split(ansi.Strip(m.helpBox()), "\n") {
			seen[strings.TrimSpace(strings.Trim(l, "│ "))] = true
		}
	}
	collect()
	for range len(all) {
		m = step(t, m, keyMsg(tea.KeyPgDown))
		collect()
	}
	for _, l := range all {
		if p := strings.TrimSpace(ansi.Strip(l)); p != "" && !seen[p] {
			t.Errorf("line %q never visible", p)
		}
	}
	// Down at the end stays clamped; up/pgup come back.
	bottom := m.helpScroll
	if m = step(t, m, keyMsg(tea.KeyDown)); m.helpScroll != bottom {
		t.Error("scroll past the end")
	}
	if m = step(t, m, keyMsg(tea.KeyUp)); m.helpScroll != bottom-1 {
		t.Errorf("up: scroll %d, want %d", m.helpScroll, bottom-1)
	}
	for range len(all) {
		m = step(t, m, keyMsg(tea.KeyPgUp))
	}
	if m.helpScroll != 0 {
		t.Errorf("pgup to top: scroll %d", m.helpScroll)
	}
	if m = step(t, m, keyMsg(tea.KeyUp)); m.helpScroll != 0 {
		t.Error("scroll above the top")
	}
}

// AC6: esc clears the term, second esc closes.
func TestHelpEscClearsTermThenCloses(t *testing.T) {
	m := typeHelp(t, step(t, helpModel(), runeMsg('/')), "tag")
	if m.helpQuery != "tag" {
		t.Fatalf("query %q", m.helpQuery)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.helpQuery != "" || !m.helpOpen {
		t.Fatalf("first esc: query %q open %v", m.helpQuery, m.helpOpen)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.helpOpen {
		t.Fatal("second esc must close")
	}
	// term kept after enter, esc still clears first
	m = typeHelp(t, step(t, helpModel(), runeMsg('/')), "tag")
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.helpQuery != "tag" || m.helpSearching {
		t.Fatalf("enter: query %q searching %v", m.helpQuery, m.helpSearching)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.helpQuery != "" || !m.helpOpen {
		t.Fatal("esc after enter must clear the term first")
	}
	// reopening starts clean
	m = typeHelp(t, step(t, helpModel(), runeMsg('/')), "x")
	m = step(t, step(t, m, keyMsg(tea.KeyEsc)), keyMsg(tea.KeyEsc))
	if m.helpQuery != "" || m.helpSearching || m.helpScroll != 0 {
		t.Errorf("state left after close: %+v", m)
	}
}

// Constraint: printable keys typed in the search field never reach help-local
// or global keys.
func TestHelpSearchFieldCapturesText(t *testing.T) {
	m := typeHelp(t, step(t, helpModel(), runeMsg('/')), "q?ik/")
	if !m.helpOpen || m.helpQuery != "q?ik/" {
		t.Fatalf("open %v query %q", m.helpOpen, m.helpQuery)
	}
	if m.confirmQuit {
		t.Error("q leaked to quit confirm")
	}
	m = step(t, m, keyMsg(tea.KeyBackspace))
	if m.helpQuery != "q?ik" {
		t.Errorf("backspace: %q", m.helpQuery)
	}
	// without the entry key, text does not edit the term
	m = step(t, helpModel(), runeMsg('x'))
	if m.helpQuery != "" {
		t.Errorf("x edited the term outside the search field: %q", m.helpQuery)
	}
}

// AC7: Tasks group stays in the help and is searchable.
func TestHelpTasksSearchable(t *testing.T) {
	m := taskModel(t, helloTasks+`
tui:
  actions:
    - task: hello
      key: A
      label: greet
`)
	if len(filterHelpEntries(m.helpEntries(), "greet")) != 1 {
		t.Error("task label not searchable")
	}
	if len(helpGroupOf(filterHelpEntries(m.helpEntries(), "tasks"), "Tasks")) != 1 {
		t.Error("group name Tasks not searchable")
	}
}
