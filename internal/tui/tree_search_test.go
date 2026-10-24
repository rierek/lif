package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rierek/lif/pkg/item"
)

// searchFixture: ms-1 {ep-1a {tk-1 "zz", tk-x}, ep-1b {tk-2}}, ms-2 {ep-2a {tk-3 "zz"}}.
// Everything collapsed; searching "zz" hits tk-1 and tk-3 in different epics.
func searchFixture(t *testing.T) model {
	t.Helper()
	mk := func(id, typ, parent, title string) *item.Item {
		return &item.Item{ID: id, Title: title, Status: "todo", Type: typ, Priority: "normal", Parent: parent}
	}
	m := fixtureModel(t, []*item.Item{
		mk("ms-1", "milestone", "", "ms-1"), mk("ep-1a", "epic", "ms-1", "ep-1a"), mk("ep-1b", "epic", "ms-1", "ep-1b"),
		mk("tk-1", "task", "ep-1a", "zz one"), mk("tk-x", "task", "ep-1a", "tk-x"), mk("tk-2", "task", "ep-1b", "tk-2"),
		mk("ms-2", "milestone", "", "ms-2"), mk("ep-2a", "epic", "ms-2", "ep-2a"), mk("tk-3", "task", "ep-2a", "zz three"),
	})
	m.expanded = map[string]bool{}
	m.view, m.flatView = viewBrowseRepo, false
	return keyMsgSeq(t, keys2(t, m, "/zz"), keyMsg(tea.KeyEnter))
}

func keyMsgSeq(t *testing.T, m model, ks ...tea.KeyMsg) model {
	t.Helper()
	for _, k := range ks {
		m = step(t, m, k)
	}
	return m
}

func searchRows(m model) (ids []string, ctx map[string]bool) {
	ctx = map[string]bool{}
	for _, n := range m.visibleNodes() {
		if n.section != "" {
			continue
		}
		ids = append(ids, n.id)
		ctx[n.id] = n.context
	}
	return
}

func TestSearchExpandsAncestorsOfHits(t *testing.T) {
	m := searchFixture(t)
	ids, ctx := searchRows(m)
	want := []string{"ms-1", "ep-1a", "tk-1", "ms-2", "ep-2a", "tk-3"}
	if len(ids) != len(want) {
		t.Fatalf("rows %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("rows %v, want %v", ids, want)
		}
	}
	for _, id := range []string{"ms-1", "ep-1a", "ms-2", "ep-2a"} {
		if !ctx[id] {
			t.Errorf("%s: ancestor without own hit must be context (muted)", id)
		}
	}
	for _, id := range []string{"tk-1", "tk-3"} {
		if ctx[id] {
			t.Errorf("%s: hit must not be context", id)
		}
	}
}

func TestSearchCursorOnlyOnHits(t *testing.T) {
	m := searchFixture(t)
	if m.cursorID != "tk-1" {
		t.Fatalf("cursor after search %q, want first hit tk-1", m.cursorID)
	}
	m = keyMsgSeq(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "tk-3" {
		t.Fatalf("down: %q, want tk-3 (context rows skipped)", m.cursorID)
	}
	m = keyMsgSeq(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "tk-3" {
		t.Fatalf("down at end: %q, want stay on tk-3", m.cursorID)
	}
	m = keyMsgSeq(t, m, keyMsg(tea.KeyUp), keyMsg(tea.KeyUp))
	if m.cursorID != "tk-1" {
		t.Fatalf("up x2: %q, want tk-1", m.cursorID)
	}
	if m = keys2(t, m, "]"); m.cursorID != "tk-1" {
		t.Fatalf("] from tk-1: %q, want stay (tk-x is no hit)", m.cursorID)
	}
}

func TestSearchClearRestoresExpandState(t *testing.T) {
	m := searchFixture(t)
	m.expanded = map[string]bool{"ms-2": true}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.treeSearchActive() {
		t.Fatalf("esc must clear the search")
	}
	ids, _ := searchRows(m)
	want := []string{"ms-1", "ms-2", "ep-2a"}
	if len(ids) != len(want) {
		t.Fatalf("rows after clear %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("rows after clear %v, want %v", ids, want)
		}
	}
}
