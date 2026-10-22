package tui

// box_menu_value_click_test.go — TDD coverage for Slice D (bt-f0y9,
// "feld-verankertes Inline-Dropdown", D09 revidiert): a left-click on the
// anchored value menu's own item row applies that value IMMEDIATELY and
// closes (the SAME applyValueMenuSelection enter already fires), a click
// outside the popup closes WITHOUT mutating -- the actual mouse-native
// payoff D09 was written for (handleMouse's blanket overlay guard,
// mouse.go:181-185, otherwise swallows every click while ANY overlay is
// open). Every click coordinate is found by rendering the REAL View() and
// locating a known landmark (never hand-derived from the hit-test formula
// itself -- mouse_test.go's own doc-stamp precedent).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// valueMenuItemClickAt finds substr (an item's own value text, e.g.
// "completed") in m's real rendered View() and returns a real left-click
// tea.MouseMsg at that screen coordinate -- restricted to rows at/after the
// popup's own top (valueMenuPopupRect) so a coincidental match in the
// background chrome underneath can never be picked instead (the popup fully
// overwrites its own rect, so any match strictly inside it IS the popup's
// own text).
func valueMenuItemClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	x, y, w, h, ok := m.valueMenuPopupRect()
	if !ok {
		t.Fatalf("setup: valueMenuPopupRect() ok=false")
	}
	for row, l := range screenLines(m) {
		if row < y || row >= y+h {
			continue
		}
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		col := ansi.StringWidth(l[:i])
		if col < x || col >= x+w {
			continue
		}
		return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: col, Y: row}
	}
	t.Fatalf("substr %q not found inside the popup's own rect (x=%d,y=%d,w=%d,h=%d) in the rendered View()", substr, x, y, w, h)
	return tea.MouseMsg{}
}

// TestValueMenuMouseClickOnItemAppliesSelectionAndCloses guards Slice D's
// core contract at 80 AND 120 columns: clicking the "completed" row in the
// open Status menu sets menu.cursor to it, applies it (the SAME
// applyValueMenuSelection enter already drives -- proven here via the
// REAL data.Client's error message pointed at a nonexistent repo dir,
// mirrors box_menu_value_test.go's own TestValueMenuEnterAppliesCursored
// ValueAndCloses precedent) and closes the overlay.
func TestValueMenuMouseClickOnItemAppliesSelectionAndCloses(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			m.client = &data.Client{RepoDir: "/nonexistent-bt-f0y9-slice-d-scratch-dir"}
			m = step(t, m, runeMsg('s'))
			if m.overlay != overlayValueMenu {
				t.Fatalf("setup: overlay = %v, want overlayValueMenu", m.overlay)
			}

			msg := valueMenuItemClickAt(t, m, "completed")
			tm, cmd := m.handleMouse(msg)
			nm, ok := tm.(model)
			if !ok {
				t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
			}
			if nm.overlay != overlayNone {
				t.Fatalf("overlay after click = %v, want overlayNone", nm.overlay)
			}
			if cmd == nil {
				t.Fatal("click on an item must return a Cmd (mutateCmd)")
			}

			res := cmd()
			mdm, ok := res.(mutationDoneMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want mutationDoneMsg", res)
			}
			if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif update") {
				t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves SetStatus dispatched)", mdm.err, "lif update")
			}
		})
	}
}

// TestValueMenuMouseClickOutsidePopupClosesWithoutMutation guards the other
// half of Slice D's contract: a click OUTSIDE the popup's own rect (e.g. on
// the Tree pane, still visible to the left of/behind the popup) closes the
// menu with NO mutation Cmd -- mirrors esc's own keyValueMenu branch.
func TestValueMenuMouseClickOutsidePopupClosesWithoutMutation(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			m = step(t, m, runeMsg('s'))
			if m.overlay != overlayValueMenu {
				t.Fatalf("setup: overlay = %v, want overlayValueMenu", m.overlay)
			}

			x, _, _, _, ok := m.valueMenuPopupRect()
			if !ok {
				t.Fatalf("setup: valueMenuPopupRect() ok=false")
			}
			if x < 2 {
				t.Fatalf("setup: popup x=%d too close to the left edge for an outside-click fixture", x)
			}
			// Column 1 is inside the outer border/Tree pane, always strictly
			// left of the popup's own x (>=2 here) -- a real "outside" click.
			msg := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 1, Y: 5}
			tm, cmd := m.handleMouse(msg)
			nm, ok := tm.(model)
			if !ok {
				t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
			}
			if nm.overlay != overlayNone {
				t.Fatalf("overlay after outside click = %v, want overlayNone", nm.overlay)
			}
			if cmd != nil {
				t.Fatal("click outside the popup must NOT return a mutation Cmd")
			}
			if nm.mutTarget != m.mutTarget {
				t.Fatalf("mutTarget changed by an outside click that must not mutate: got %q, want unchanged %q", nm.mutTarget, m.mutTarget)
			}
		})
	}
}

// --- valueMenuItemRow / valueMenuBodyAndItemRows (pure geometry) ---

// TestValueMenuItemRowMatchesRenderedRows is the render-grounded cross-check
// for valueMenuItemRow (mirrors Slice A's own boxFormFieldRect corner-check
// pattern): for every item, the row valueMenuItemRow(m, i) returns must
// contain that exact item's OWN value text in the real rendered popup.
func TestValueMenuItemRowMatchesRenderedRows(t *testing.T) {
	m := anchorModel(t, 100, 40)
	m = step(t, m, runeMsg('s'))
	x, y, _, _, ok := m.valueMenuPopupRect()
	if !ok {
		t.Fatalf("setup: valueMenuPopupRect() ok=false")
	}
	lines := screenLines(m)
	for i, it := range m.menuItems {
		row := valueMenuItemRow(m, i)
		if row < 0 {
			t.Fatalf("valueMenuItemRow(%d) = %d, want >= 0", i, row)
		}
		screenRow := y + row
		if screenRow < 0 || screenRow >= len(lines) {
			t.Fatalf("item %d (%s): screen row %d out of range", i, it.value, screenRow)
		}
		l := lines[screenRow]
		if !strings.Contains(l, it.value) {
			t.Fatalf("item %d: row %d (screen row %d) = %q, want it to contain %q", i, row, screenRow, l, it.value)
		}
		// Sanity: the match must fall within the popup's own column span.
		idx := strings.Index(l, it.value)
		if col := ansi.StringWidth(l[:idx]); col < x {
			t.Fatalf("item %d (%s) match at col=%d, want >= popup x=%d", i, it.value, col, x)
		}
	}
}
