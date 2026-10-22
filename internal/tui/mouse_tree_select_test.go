package tui

// mouse_tree_select_test.go — bean bt-vpvu (PO-Befund #14: "Im Tree lassen
// sich beans mit der Maus nicht auswaehlen"). Two halves:
//
//  1. the REGRESSION half: the reported failure was a scrolled Tree whose
//     hit-test windowed against a different height than the renderer did
//     (the box-form's filter bar was reclaimed from bodyH by viewBrowseRepo but
//     not by treeClickRow). Both starts clamp to 0 at the top of an unscrolled
//     list, which is why it presented as "sometimes works".
//  2. the BEHAVIOUR half: single click selects only, double click toggles
//     expansion -- the PO's requested semantics, which differ from the ported
//     devd behaviour (single click used to expand a closed node).
//
// Every click goes through a real tea.MouseMsg roundtrip at a coordinate
// located in the REAL rendered View() (treeClickAt), never one derived from
// the click formula itself -- deriving it would make the test circular, the
// trap detailClickRow's own doc comment names. The clock is injected, so no
// assertion depends on wall time.

import (
	"testing"
	"time"

	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// fixedClock returns a clock the test drives by hand, plus the knob to
// advance it. bean bt-vpvu explicitly requires the double-click time source to
// be injectable -- a wall clock here would make these tests flaky under load.
func fixedClock() (func() time.Time, func(time.Duration)) {
	base := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	cur := base
	return func() time.Time { return cur }, func(d time.Duration) { cur = cur.Add(d) }
}

func clickTree(t *testing.T, m model, substr string) model {
	t.Helper()
	msg := treeClickAt(t, m, substr)
	tm, _ := m.handleMouse(msg)
	mm, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse did not return a model, got %T", tm)
	}
	return mm
}

// --- 1. regression: the scrolled window ---

// TestScrolledTreeClickSelectsTheRowUnderThePointer is the PO's #14 in test
// form: 60 beans, cursor parked deep enough that the window is scrolled, flag
// ON. Before the fix this selected the wrong bean (or none at all).
func TestScrolledTreeClickSelectsTheRowUnderThePointer(t *testing.T) {
	m := longTreeModel(t)

	// Row 045 is the cursor; 040 is visible above it in the scrolled window.
	m2 := clickTree(t, m, "Row 040")
	if m2.cursorID != "gld-r040" {
		t.Fatalf("click on the \"Row 040\" row selected %q, want gld-r040 (scrolled window)", m2.cursorID)
	}
}

func TestScrolledTreeClickSelectsTheRowUnderThePointerFlagOff(t *testing.T) {
	m := longTreeModel(t)
	m2 := clickTree(t, m, "Row 050")
	if m2.cursorID != "gld-r050" {
		t.Fatalf("click on the \"Row 050\" row selected %q, want gld-r050 (scrolled window, flag off)", m2.cursorID)
	}
}

// TestScrolledTreeClickHitsEveryVisibleRow walks EVERY rendered bean row and
// asserts the click lands on it -- a single-row spot check would pass against
// an off-by-N window whose error happens to be zero at that one row.
func TestScrolledTreeClickHitsEveryVisibleRow(t *testing.T) {
	m := longTreeModel(t)

	for i := 30; i < 60; i++ {
		title := "Row " + pad3(i)
		id := "gld-r" + pad3(i)
		if !renderedInLeftPane(m, title) {
			continue // not in the current window
		}
		if got := clickTree(t, m, title).cursorID; got != id {
			t.Errorf("click on %q selected %q, want %q", title, got, id)
		}
	}
}

func pad3(i int) string {
	s := []byte{'0', '0', '0'}
	s[2] = byte('0' + i%10)
	s[1] = byte('0' + (i/10)%10)
	s[0] = byte('0' + (i/100)%10)
	return string(s)
}

func renderedInLeftPane(m model, substr string) bool {
	head, localKeys := m.browseRepoChrome(m.width - 2)
	_, lw, _, originX, _ := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, false)
	for _, l := range screenLines(m) {
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		if ansi.StringWidth(l[:i]) < originX+lw {
			return true
		}
	}
	return false
}

// --- 2. behaviour: single click selects, double click toggles ---

func TestSingleClickSelectsWithoutExpanding(t *testing.T) {
	clock, _ := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.cursorID = "ms-1"

	m2 := clickTree(t, m, "Milestone One")
	if m2.cursorID != "ms-1" {
		t.Fatalf("single click did not select: cursorID = %q", m2.cursorID)
	}
	if m2.expanded["ms-1"] {
		t.Errorf("a SINGLE click expanded ms-1 — the PO asked for select-only on a single click (bean bt-vpvu)")
	}
}

func TestDoubleClickExpandsThenCollapses(t *testing.T) {
	clock, advance := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.cursorID = "ms-1"

	m = clickTree(t, m, "Milestone One")
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Milestone One")
	if !m.expanded["ms-1"] {
		t.Fatalf("double click did not expand ms-1")
	}

	// A second double click (two FRESH clicks) collapses it again.
	advance(2 * time.Second)
	m = clickTree(t, m, "Milestone One")
	if !m.expanded["ms-1"] {
		t.Fatalf("the first click of the second pair must not toggle anything")
	}
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Milestone One")
	if m.expanded["ms-1"] {
		t.Errorf("the second double click did not collapse ms-1")
	}
}

// TestTripleClickDoesNotToggleTwice guards the pairing rule: after a double
// click fires, the counter resets, so a third rapid click starts a NEW pair
// instead of immediately undoing what the second one just did.
func TestTripleClickDoesNotToggleTwice(t *testing.T) {
	clock, advance := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.cursorID = "ms-1"

	m = clickTree(t, m, "Milestone One")
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Milestone One") // expands
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Milestone One") // must NOT collapse
	if !m.expanded["ms-1"] {
		t.Errorf("a third rapid click collapsed ms-1 again — a double click must consume its pair")
	}
}

func TestSlowSecondClickIsNotADoubleClick(t *testing.T) {
	clock, advance := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.cursorID = "ms-1"

	m = clickTree(t, m, "Milestone One")
	advance(doubleClickInterval + time.Millisecond)
	m = clickTree(t, m, "Milestone One")
	if m.expanded["ms-1"] {
		t.Errorf("two clicks further apart than doubleClickInterval toggled expansion")
	}
}

func TestDoubleClickOnDifferentBeansIsNotADoubleClick(t *testing.T) {
	clock, advance := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.expanded["ms-1"] = true
	m.cursorID = "ms-1"

	m = clickTree(t, m, "Milestone One")
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Epic One")
	if !m.expanded["ms-1"] {
		t.Errorf("a click on a DIFFERENT bean collapsed the previous one")
	}
	if m.cursorID != "ep-1" {
		t.Errorf("the second click did not select ep-1: %q", m.cursorID)
	}
}

func TestDoubleClickOnLeafIsHarmless(t *testing.T) {
	clock, advance := fixedClock()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.clock = clock
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true

	m = clickTree(t, m, "Task Two")
	advance(50 * time.Millisecond)
	m = clickTree(t, m, "Task Two")
	if m.cursorID != "tk-2" {
		t.Errorf("double click on a leaf lost the selection: %q", m.cursorID)
	}
}
