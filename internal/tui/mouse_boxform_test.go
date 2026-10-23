package tui

// mouse_boxform_test.go — TDD coverage for S6 (jira-style-ui experiment,
// "mouse for box-form"): B6's filter-bar +3 click-Y offset
// only), box-form Detail-Pane click-to-edit (a click on a field box fires
// the SAME action its own hotkey does), and the flat-list row click (S5's
// Nested/Flat Browse toggle `G`, view_browse_flat.go). Mirrors mouse_test.go's
// own render-grounded click pattern throughout (screenLines/leftPaneClickAt/
// rightPaneClickAt) -- no coordinate is ever hand-derived from the click
// formula itself (that would be circular, detailClickRow's own doc comment
// precedent, mouse.go).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// --- B6: filter-bar +3 click offset (Tree pane) ---

// TestBoxFormFilterBarOffsetTreeClickSelectsCorrectRow mirrors
// TestClickSetsTreeCursor (mouse_test.go) verbatim, EXCEPT the box-form is
// set first: the persistent filter bar (box_filter_bar.go) now sits between
// the header divider and the Tree/Detail split, pushing every rendered row
// down by filterBarHeight (3) lines. treeClickAt locates "Task Two" in the
// REAL rendered View() (already reflecting the +3 shift) -- without B6's
// originY += filterBarHeight correction (view_browse_repo.go's treeClickRow),
// this click would resolve 3 rows too far and select the WRONG node (or
// none).
func TestBoxFormFilterBarOffsetTreeClickSelectsCorrectRow(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "ms-1"

	msg := treeClickAt(t, m, "Task Two")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.cursorID != "tk-2" {
		t.Fatalf("box-form click on \"Task Two\" row: cursorID = %q, want tk-2 (B6 filter-bar +3 offset)", m2.cursorID)
	}
}

// --- Box-form Detail-Pane click-to-edit ---

// boxFormClickAt is rightPaneClickAt (mouse_test.go) scoped to Browse's own
// chrome, restricted further to Y >= the FIRST row past the filter bar (i.e.
// the box-form's own content, not the filter bar's four chips one-column-
// left-of-boundary text, which happens to share label words like "Status"
// with the box-form's own Status box). substr should be a badge/landmark
// string unique to the target box (e.g. "(s)"/"(o)"/"(u)" — the hotkey badge
// boxBottomBorder renders, box_dropdown.go).
func boxFormClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	head, localKeys := m.browseRepoChrome(m.width - 2)
	_, lw, _, originX, originY := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, layoutFilterDetail)
	boundary := originX + lw
	filterBarBottom := originY + filterBarHeight // filter bar occupies [originY, originY+filterBarHeight)
	for y, l := range screenLines(m) {
		if y < filterBarBottom {
			continue // skip the filter bar's own 3 rows -- avoid matching its "Status"/"Type"/"Priority"/"Tags" chip labels
		}
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		col := ansi.StringWidth(l[:i])
		if col < boundary {
			continue
		}
		return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: col, Y: y}
	}
	t.Fatalf("substr %q not found in the box-form Detail-Pane of the rendered View()", substr)
	return tea.MouseMsg{}
}

// boxFormFocusModel builds a fixtureModel with and tk-2 focused
// (Browse/Tree view, no detailFocus needed -- focusedBean() resolves via
// m.cursorID regardless, update.go).
func boxFormFocusModel(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-2")
	return m
}

// TestBoxFormClickOnStatusBoxOpensValueMenuSeededToStatus guards the Status
// box's click-to-edit: a click on its (s) hotkey badge fires the SAME
// m.openValueMenu("status") keys.Status already fires (update.go:772-773).
func TestBoxFormClickOnStatusBoxOpensValueMenuSeededToStatus(t *testing.T) {
	m := boxFormFocusModel(t)

	msg := boxFormClickAt(t, m, "(s)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu (click on Status box)", m2.overlay)
	}
	if len(m2.menuItems) == 0 || m2.menuItems[0].group != "status" {
		t.Fatalf("menuItems = %+v, want group=status seeded", m2.menuItems)
	}
}

// TestBoxFormClickOnTypeBoxOpensValueMenuSeededToType mirrors the Status
// case for the Type box's own (t) hotkey badge -- proves the click-X column
// bucketing (gridColAt/gridColWidths, mouse.go/box_detail_form.go) resolves
// to a DIFFERENT column than Status, not just "any click in rowA works".
func TestBoxFormClickOnTypeBoxOpensValueMenuSeededToType(t *testing.T) {
	m := boxFormFocusModel(t)

	msg := boxFormClickAt(t, m, "(t)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu (click on Type box)", m2.overlay)
	}
	if len(m2.menuItems) == 0 || m2.menuItems[0].group != "type" {
		t.Fatalf("menuItems = %+v, want group=type seeded", m2.menuItems)
	}
}

// TestBoxFormClickOnTagsBoxOpensTagPicker guards rowB's Tags box (a
// DIFFERENT row than rowA, and openTagPicker's own two-return-value
// signature, unlike openValueMenu's single-model return).
func TestBoxFormClickOnTagsBoxOpensTagPicker(t *testing.T) {
	m := boxFormFocusModel(t)

	msg := boxFormClickAt(t, m, "(g)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayTagPicker {
		t.Fatalf("overlay = %v, want overlayTagPicker (click on Tags box)", m2.overlay)
	}
}

// --- Flat-list row click (S5 Nested/Flat Browse toggle) ---

// TestFlatClickSelectsRow guards Task 3 ("Flat-mode row click"): a click on
// a flat-list row (mirrors TestClickSetsTreeCursor's own render-grounded
// pattern) sets flatList's index cursor to that row -- flatSelected()
// resolves to the clicked bean.
func TestFlatClickSelectsRow(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.flatView = true
	vis := m.flatVisible()
	if len(vis) < 2 {
		t.Fatalf("setup: need >=2 flat-visible beans, got %d", len(vis))
	}
	m.flatList.setLen(len(vis))
	m.flatList.cursor = 0

	msg := treeClickAt(t, m, "Task Two") // treeClickAt = browseRepoChrome-scoped leftPaneClickAt, view-agnostic to Tree-vs-Flat
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	sel := m2.flatSelected()
	if sel == nil || sel.ID != "tk-2" {
		gotID := "<nil>"
		if sel != nil {
			gotID = sel.ID
		}
		t.Fatalf("flat click on \"Task Two\" row: flatSelected().ID = %q, want tk-2", gotID)
	}
}

// TestFlatClickWithBoxFormOffsetSelectsRow combines S6's two flat-mode
// requirements: the +3 filter-bar offset AND flatClickRow's own
// windowStart/row math, both at once -- guards flatClickRow's own
// filter-bar branch (view_browse_flat.go), the flat-list analog of the
// Tree test above.
func TestFlatClickWithBoxFormOffsetSelectsRow(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.flatView = true
	vis := m.flatVisible()
	if len(vis) < 2 {
		t.Fatalf("setup: need >=2 flat-visible beans, got %d", len(vis))
	}
	m.flatList.setLen(len(vis))
	m.flatList.cursor = 0

	msg := treeClickAt(t, m, "Task Two")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	sel := m2.flatSelected()
	if sel == nil || sel.ID != "tk-2" {
		gotID := "<nil>"
		if sel != nil {
			gotID = sel.ID
		}
		t.Fatalf("flat click on \"Task Two\" row: flatSelected().ID = %q, want tk-2 (B6 offset)", gotID)
	}
}
