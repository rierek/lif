package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// navFixture: ms-1 {ep-1a {tk-1}, ep-1b {tk-2}}, ms-2 {ep-2a {tk-3}} (the roadmap drops childless epics/milestones). All collapsed except
// the synthetic roots (fixtureModel).
func navFixture(t *testing.T) model {
	t.Helper()
	mk := func(id, typ, parent string) *item.Item {
		return &item.Item{ID: id, Title: id, Status: "todo", Type: typ, Priority: "normal", Parent: parent}
	}
	m := fixtureModel(t, []*item.Item{
		mk("ms-1", "milestone", ""), mk("ep-1a", "epic", "ms-1"), mk("ep-1b", "epic", "ms-1"), mk("tk-1", "task", "ep-1a"), mk("tk-2", "task", "ep-1b"),
		mk("ms-2", "milestone", ""), mk("ep-2a", "epic", "ms-2"), mk("tk-3", "task", "ep-2a"),
	})
	m.expanded = map[string]bool{}
	m.view, m.flatView = viewBrowseRepo, false
	return m
}

func keys2(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = step(t, m, runeMsg(r))
	}
	return m
}

func TestSiblingJump(t *testing.T) {
	m := navFixture(t)
	m.expanded = map[string]bool{"ms-1": true, "ms-2": true, "ep-1a": true, "ep-1b": true, "ep-2a": true}
	m.cursorID = "ep-1a" // tk-1 (child) sits between ep-1a and ep-1b

	m = keys2(t, m, "]")
	if m.cursorID != "ep-1b" {
		t.Fatalf("] from ep-1a: cursor %q, want ep-1b (expanded child skipped)", m.cursorID)
	}
	m = keys2(t, m, "]")
	if m.cursorID != "ep-1b" {
		t.Fatalf("] at last epic: cursor %q, want stay on ep-1b (never leaves the parent)", m.cursorID)
	}
	m = keys2(t, m, "[")
	m = keys2(t, m, "[")
	if m.cursorID != "ep-1a" {
		t.Fatalf("[ twice: cursor %q, want ep-1a (stops at first sibling)", m.cursorID)
	}

	m.cursorID = "ms-1" // expanded subtree is skipped
	if m = keys2(t, m, "]"); m.cursorID != "ms-2" {
		t.Fatalf("] from ms-1: cursor %q, want ms-2", m.cursorID)
	}
	if m = keys2(t, m, "]"); m.cursorID != "ms-2" {
		t.Fatalf("] at last milestone: cursor %q, want stay on ms-2", m.cursorID)
	}
}

func TestSiblingJumpNoopInFlatList(t *testing.T) {
	m := navFixture(t)
	m.flatView = true
	m.cursorID = "ep-1a"
	if m = keys2(t, m, "]"); m.cursorID != "ep-1a" {
		t.Fatalf("] in flat list moved cursor to %q", m.cursorID)
	}
}

func TestFoldKeys(t *testing.T) {
	m := navFixture(t)
	rows := func(m model) int { return len(m.visibleNodes()) }
	start := rows(m) // milestones (+ roots) only

	m = keys2(t, m, "zR")
	if got := rows(m); got != start+6 { // 3 epics + 3 tasks
		t.Fatalf("zR: %d rows, want %d", got, start+6)
	}
	if !m.expanded["ep-1a"] || !m.expanded["ms-2"] {
		t.Fatalf("zR left nodes closed: %v", m.expanded)
	}

	m = keys2(t, m, "zM")
	if rows(m) != start || len(m.expanded) != 0 {
		t.Fatalf("zM: %d rows, want start state %d; expanded=%v", rows(m), start, m.expanded)
	}

	m.cursorID = "ms-1"
	m = keys2(t, m, "zo")
	if !m.expanded["ms-1"] || !m.expanded["ep-1a"] || m.expanded["ms-2"] {
		t.Fatalf("zo on ms-1 must open it and its subtree only: %v", m.expanded)
	}
	m = keys2(t, m, "zc")
	if len(m.expanded) != 0 || m.cursorID != "ms-1" {
		t.Fatalf("zc on ms-1 must close the whole subtree: %v cursor=%q", m.expanded, m.cursorID)
	}
}

func TestFoldCollapseAllMovesHiddenCursorToAncestor(t *testing.T) {
	m := navFixture(t)
	m = keys2(t, m, "zR")
	m.cursorID = "tk-1"
	m = keys2(t, m, "zM")
	if m.cursorID != "ms-1" {
		t.Fatalf("cursor %q, want ms-1 (nearest visible ancestor)", m.cursorID)
	}
}

func TestFoldPrefixLivesOneKey(t *testing.T) {
	m := navFixture(t)
	m.cursorID = "ms-1"
	m = keys2(t, m, "zx") // unknown completion: discarded
	m = keys2(t, m, "o")
	if m.expanded["ms-1"] {
		t.Fatal("stale z prefix completed a later o")
	}
}

func TestFoldSequenceInOneRead(t *testing.T) {
	m := navFixture(t)
	m = step(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("zR")}) // terminals may deliver both keys at once
	if !m.expanded["ms-1"] || !m.expanded["ep-1a"] {
		t.Fatalf("zR in one read did not expand: %v", m.expanded)
	}
}
