package tui

// view_fullscreen_flat_test.go — B8 (bean bt-s90e, epic bt-vy1q): the Nested/
// Flat Browse toggle `G` (view_browse_flat.go) only ever reached the SPLIT
// view's left pane -- viewBrowseRepo's fullscreen branch built its list rows
// from treeRows() unconditionally, so entering the Vollbild-Liste with `v`
// silently dropped the PO back into the Tree they had just toggled away from.
//
// `G` is independent of the box-form (unlike the rest of the jira-style
// experiment), so neither are these tests -- the flag stays unset throughout.
//
// The discriminator used below is deliberately structural rather than
// cosmetic: with NO node expanded, the Tree renders the root (ms-1) alone,
// while flat mode renders every bean flatVisible() matches regardless of
// expansion state. A leaf like tk-2 appearing in the Vollbild is therefore
// proof the flat source was used, and cannot be produced by the Tree path at
// all.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// flatFullscreenModel builds a 100x30 Browse model with NOTHING expanded (so
// the Tree can only ever show ms-1), flat mode toggled on via the REAL `G`
// keypress, and the Vollbild-Liste entered via the REAL `v` keypress from the
// Tree-focused default.
func flatFullscreenModel(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded = map[string]bool{}
	m = step(t, m, runeMsg('G'))
	if !m.flatView {
		t.Fatal("setup: flatView false after `G`")
	}
	m = step(t, m, runeMsg('v'))
	if m.fullscreen != fullscreenList {
		t.Fatalf("setup: fullscreen = %v after `v` from a Tree-focused split, want fullscreenList", m.fullscreen)
	}
	return m
}

// TestFullscreenListRespectsFlatView is B8's headline criterion: `v` after `G`
// must render the FLAT list, not the Tree.
func TestFullscreenListRespectsFlatView(t *testing.T) {
	m := flatFullscreenModel(t)

	out := ansi.Strip(m.View())
	for _, id := range []string{"ms-1", "ep-1", "tk-1", "tk-2"} {
		if !strings.Contains(out, id) {
			t.Errorf("Vollbild-Liste in flat mode is missing %q -- flat mode shows every bean flatVisible() matches, independent of Tree expansion", id)
		}
	}
}

// TestFullscreenListStillTreeWithoutFlatView is the matching negative: the
// default (nested) mode must be byte-for-byte what it always was -- with
// nothing expanded, the Vollbild-Liste shows the root alone and no leaves.
// This is what keeps every pre-existing Vollbild golden valid.
func TestFullscreenListStillTreeWithoutFlatView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded = map[string]bool{}
	m = step(t, m, runeMsg('v'))
	if m.fullscreen != fullscreenList {
		t.Fatalf("setup: fullscreen = %v, want fullscreenList", m.fullscreen)
	}

	out := ansi.Strip(m.View())
	if !strings.Contains(out, "ms-1") {
		t.Fatal("nested Vollbild-Liste must show the root ms-1")
	}
	if strings.Contains(out, "tk-2") {
		t.Error("nested Vollbild-Liste shows the collapsed leaf tk-2 -- the Tree render path must be unchanged by B8")
	}
}

// TestFullscreenListFlatCursorMoves guards that flat mode's own cursor drives
// the Vollbild too: `down` routes through keyTree -> keyFlat (update.go) and
// moves flatList, not the Tree's cursorID. The dispatch already worked before
// B8 -- only the RENDER was wrong -- so this pins the pair staying in sync
// (a cursor that moves against a list the PO cannot see is the bug B8 was).
func TestFullscreenListFlatCursorMoves(t *testing.T) {
	m := flatFullscreenModel(t)
	before := m.flatList.cursor

	m = step(t, m, keyMsg(tea.KeyDown))
	if m.flatList.cursor == before {
		t.Fatalf("flatList.cursor unchanged (%d) after `down` in the Vollbild-Liste", before)
	}

	sel := m.flatSelected()
	if sel == nil {
		t.Fatal("flatSelected() == nil after moving the cursor")
	}
	// The moved-to row must be the highlighted one in the actual render --
	// flatRows marks the cursor row with a leading bar (view_browse_flat.go).
	out := ansi.Strip(m.View())
	if !strings.Contains(out, "▌") {
		t.Error("Vollbild-Liste in flat mode renders no cursor bar -- flatRows' own D08 cursor treatment must survive the Vollbild")
	}
	if !strings.Contains(out, sel.ID) {
		t.Errorf("selected bean %q is not visible in the Vollbild-Liste", sel.ID)
	}
}

// TestFullscreenListFlatEnterOpensFlatSelection closes the loop into the
// Detail-Vollbild: `enter` on the flat Vollbild-Liste must open the bean
// under flatList's cursor (focusedBean's own flatView case, update.go), not
// whatever the frozen Tree cursorID still points at.
func TestFullscreenListFlatEnterOpensFlatSelection(t *testing.T) {
	m := flatFullscreenModel(t)
	m = step(t, m, keyMsg(tea.KeyDown))

	want := m.flatSelected()
	if want == nil {
		t.Fatal("setup: flatSelected() == nil")
	}

	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.fullscreen != fullscreenDetail {
		t.Fatalf("fullscreen = %v after `enter` on the flat Vollbild-Liste, want fullscreenDetail", m.fullscreen)
	}
	if m.fullscreenBeanID != want.ID {
		t.Fatalf("fullscreenBeanID = %q, want %q (the FLAT list's selection, not the Tree cursor)", m.fullscreenBeanID, want.ID)
	}
}
