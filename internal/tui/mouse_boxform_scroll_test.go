package tui

// mouse_boxform_scroll_test.go — TDD coverage for bean bt-hd42 (epic bt-vy1q):
// the box-form Detail-Pane click hit-test must be SCROLL-AWARE.
//
// The gap this file closes: bt-ze10 introduced the Detail pane's own scroll
// offset (model.boxFormScroll), the earlier mouse slice S6 computed its hit
// zones from the RAW screen row. Both correct in isolation, wrong together --
// after any scrolling, a click resolved against the UNSCROLLED layout (click
// Status, open Type). Every pre-existing box-form click test clicks at scroll
// offset 0, where screen row == content row, so the whole class was invisible.
//
// The tests below therefore always SCROLL FIRST, THEN CLICK -- the missing
// case, verbatim. They stay render-grounded (boxFormClickAt locates the target
// badge in the REAL rendered View(), mouse_boxform_test.go): the rendered view
// already reflects the offset, so a hit-test that ignores it necessarily
// disagrees with what the user sees. No coordinate is derived from the click
// formula itself (circular), the same discipline detailClickRow's own doc
// comment demands.

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// boxFormWheelScroll drives `ticks` real wheel-down tea.MouseMsg roundtrips
// through handleMouse over the Detail pane -- the mouse half of the two scroll
// input paths (wheelMove/adjustBoxFormScroll, mouse.go). Coordinates mirror
// TestBoxFormWheelOverDetailPaneScrolls' own (box_form_scroll_test.go).
func boxFormWheelScroll(t *testing.T, m model, ticks int) model {
	t.Helper()
	head, localKeys := m.browseRepoChrome(m.width - 2)
	_, lw, _, originX, originY := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, false)
	msg := tea.MouseMsg{
		Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
		X: originX + lw + 2, Y: originY + filterBarHeight + 1,
	}
	for i := 0; i < ticks; i++ {
		tm, _ := m.handleMouse(msg)
		next, ok := tm.(model)
		if !ok {
			t.Fatalf("handleMouse(wheel) did not return a model, got %T", tm)
		}
		m = next
	}
	return m
}

// TestBoxFormClickOnStatusBoxAfterScrollOpensStatus is THE regression test for
// bt-hd42. Scrolling by exactly filterBarHeight-independent 3 lines lifts the
// Title block (detailBoxForm's first 3 rendered lines, box_detail_form.go) out
// of the viewport, so the Status box now renders where Title used to be. A
// scroll-blind hit test maps that screen row back onto Title -- whose target is
// boxFormTargetEditor -- and opens the whole-bean $EDITOR instead of the Status
// value menu. Scroll-aware, the click lands on Status, exactly as rendered.
func TestBoxFormClickOnStatusBoxAfterScrollOpensStatus(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)

	m = boxFormWheelScroll(t, m, 3)
	if m.boxFormScroll != 3 {
		t.Fatalf("setup: boxFormScroll after 3 wheel ticks = %d, want 3", m.boxFormScroll)
	}
	if m.boxFormScrollBean != b.ID {
		t.Fatalf("setup: boxFormScrollBean = %q, want %q", m.boxFormScrollBean, b.ID)
	}

	msg := boxFormClickAt(t, m, "(s)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayValueMenu {
		t.Fatalf("overlay after clicking the Status box at scroll offset 3 = %v, want overlayValueMenu (the hit test must add boxFormEffectiveScroll before mapping the row)", m2.overlay)
	}
	if len(m2.menuItems) == 0 || m2.menuItems[0].group != "status" {
		t.Fatalf("menuItems = %+v, want group=status seeded (click resolved to the WRONG field after scrolling)", m2.menuItems)
	}
}

// TestBoxFormClickOnTagsBoxAfterScrollOpensTagPicker exercises the SECOND
// grid row (rowB: Parent|Tags) after scrolling -- proves the correction is a
// general row translation, not a one-row special case. At offset 3, rowB's
// content rows 6-8 render at screen rows 3-5, which a scroll-blind hit test
// reads as rowA (Status|Type|Priority) and would open a value menu instead of
// the tag picker.
func TestBoxFormClickOnTagsBoxAfterScrollOpensTagPicker(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)

	m = boxFormWheelScroll(t, m, 3)
	if m.boxFormScroll != 3 {
		t.Fatalf("setup: boxFormScroll after 3 wheel ticks = %d, want 3", m.boxFormScroll)
	}

	msg := boxFormClickAt(t, m, "(t)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayTagPicker {
		t.Fatalf("overlay after clicking the Tags box at scroll offset 3 = %v, want overlayTagPicker", m2.overlay)
	}
}

// TestBoxFormClickUnaffectedAtZeroScroll guards the no-regression half: with
// the offset at its 0 default, the corrected hit test must resolve EXACTLY as
// before (screen row == content row). Cheap insurance that the fix is a
// translation, not a re-mapping.
func TestBoxFormClickUnaffectedAtZeroScroll(t *testing.T) {
	m := boxFormScrollModel(t)
	if m.boxFormScroll != 0 {
		t.Fatalf("setup: boxFormScroll = %d, want 0", m.boxFormScroll)
	}

	msg := boxFormClickAt(t, m, "(s)")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.overlay != overlayValueMenu {
		t.Fatalf("overlay at scroll offset 0 = %v, want overlayValueMenu (unchanged behaviour)", m2.overlay)
	}
	if len(m2.menuItems) == 0 || m2.menuItems[0].group != "status" {
		t.Fatalf("menuItems = %+v, want group=status seeded", m2.menuItems)
	}
}

// TestBoxFormClickSameAfterWheelAndKeyboardScroll is the bean's third
// acceptance item: the two INPUT paths that can move the offset -- the wheel
// (wheelMove/adjustBoxFormScroll, mouse.go) and the keyboard (boxFormNav,
// box_nav_field.go) -- must leave the click hit test in the same state. Both
// write the same model.boxFormScroll/boxFormScrollBean pair, so a hit test
// reading it through boxFormEffectiveScroll cannot tell them apart; this pins
// that property so a future second offset (or a path that forgets to record
// its owning bean) fails loudly.
//
// The keyboard path cannot be driven to an ARBITRARY offset -- boxFormNav
// couples scroll to the field cursor and snaps a tall field into view -- so
// the test takes the offset the keyboard reaches and replays exactly that many
// wheel ticks, then compares the hit test across the WHOLE pane.
func TestBoxFormClickSameAfterWheelAndKeyboardScroll(t *testing.T) {
	base := boxFormScrollModel(t)
	b := base.focusedBean()
	_, height := requireOverflow(t, base, b)

	kb := base
	kb.detailFocus = true
	for i := 0; i < 20 && kb.boxFormScroll == 0; i++ {
		kb = step(t, kb, keyMsg(tea.KeyDown))
	}
	if kb.boxFormScroll <= 0 {
		t.Fatalf("keyboard drive never scrolled (boxFormScroll = %d) -- the equivalence assertion below would be vacuous", kb.boxFormScroll)
	}

	wheel := boxFormWheelScroll(t, base, kb.boxFormScroll)
	if wheel.boxFormScroll != kb.boxFormScroll {
		t.Fatalf("wheel offset %d != keyboard offset %d -- the two paths must reach the same viewport state", wheel.boxFormScroll, kb.boxFormScroll)
	}

	head, localKeys := base.browseRepoChrome(base.width - 2)
	_, lw, rw, originX, originY := clickPaneGeometry(base.width, base.height, head, localKeys, base.statusLine(base.width-2), base.settings.Layout.TreeWidth, false)
	originY += filterBarHeight

	for row := 0; row < height; row++ {
		for _, col := range []int{0, 2, rw / 3, rw / 2, rw - 3} {
			msg := tea.MouseMsg{
				Button: tea.MouseButtonLeft, Action: tea.MouseActionPress,
				X: originX + lw + col, Y: originY + row,
			}
			wTarget, wOK := detailBoxFormClickRow(wheel, msg)
			kTarget, kOK := detailBoxFormClickRow(kb, msg)
			if wOK != kOK || wTarget != kTarget {
				t.Fatalf("hit test disagrees at (row=%d, col=%d): wheel=%q/%v, keyboard=%q/%v", row, col, wTarget, wOK, kTarget, kOK)
			}
		}
	}
}
