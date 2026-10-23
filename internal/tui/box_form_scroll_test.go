package tui

// box_form_scroll_test.go — TDD coverage for F1 (jira-style-ui experiment,
// bean bt-ze10, epic bt-vy1q): the box-form Detail-Pane's own scroll
// viewport. Before this slice, detailBoxForm (box_detail_form.go) rendered
// Title+scalar-grid+Body/Relations/History as ONE tall string with no
// windowing of its own -- a long Body just got cut off by renderPane's
// Golden-Rule-#1 line cap, Relations/History unreachable. Mirrors
// mouse_boxform_test.go's own render-grounded pattern (screenLines/
// clickPaneGeometry) for the wheel/render assertions, and update_test.go's
// step()/keyMsg() pattern for the keyboard assertions.

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// boxFormLongBodyBeans is fixtureBeans() (update_test.go) with tk-1 AND tk-2
// given an 80-item markdown list Body ("- BODYLINE01".."- BODYLINE80") --
// glamour (glowRender, accordion.go) renders a compact list one item per
// line, so this reliably overflows any reasonable Detail-Pane height
// (Golden-Rule-Drift-Schutz reason for BOTH leaf tasks getting one: several
// tests below re-focus from tk-2 onto tk-1 to exercise the "selection
// changed" reset -- if only tk-2 overflowed, adjustBoxFormScroll(tk-1, ...)
// would itself clamp to 0 regardless of the reset logic under test, a false
// negative).
func boxFormLongBodyBeans() []*item.Item {
	var body strings.Builder
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&body, "- BODYLINE%02d\n", i)
	}
	beans := fixtureBeans()
	for i := range beans {
		if beans[i].ID == "tk-1" || beans[i].ID == "tk-2" {
			beans[i].Body = body.String()
		}
	}
	return beans
}

// boxFormScrollModel builds a fixtureModel with a 100x30
// frame, and tk-2 (long body) focused (Browse/Tree view, ancestors
// expanded via focusBean, box_menu_value_test.go) -- detailFocus is left
// false (Tree-focused default); callers that need Detail focus set
// m.detailFocus = true themselves, mirroring update_test.go's own pattern
// (e.g. TestActivateDetailFieldJump).
func boxFormScrollModel(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, boxFormLongBodyBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-2")
	return m
}

// requireOverflow fails the test unless b's box-form actually overflows the
// Detail pane's own height budget -- every test below depends on this
// (Akzeptanz "Langer Body: Detail-Pane scrollt"), so a fixture/geometry
// regression that accidentally makes the form fit must fail LOUDLY here,
// not silently pass a scroll test that no longer exercises any scrolling.
func requireOverflow(t *testing.T, m model, b *item.Item) (total, height int) {
	t.Helper()
	total, height = boxFormScrollBounds(m, b)
	if total <= height {
		t.Fatalf("fixture must overflow the Detail pane (total=%d, height=%d) for this test to be meaningful", total, height)
	}
	return total, height
}

// --- Keyboard: up/down only scrolls when the Detail pane is focused ---

// TestBoxFormScrollDownUpWhenDetailFocused guards the core drive path: while
// m.detailFocus is true, down/up eventually move
// model.boxFormScroll and record its owning bean -- Akzeptanz "Offset
// clamped" + the bean's own Test list item 1.
//
// REVISED by bean bt-1o4g (PO-Nebenbefund N8, keyboard-first): down/up are no
// longer a CONTEXT-FREE ±1 viewport scroll. They now drive the box-form FIELD
// CURSOR (boxFormNav, box_nav_field.go), which drags this offset along so the
// focused field stays visible -- the first few presses therefore walk the
// short scalar boxes (which already fit, offset stays 0) before any scrolling
// happens at all. What bt-ze10 actually guarantees, and what this test now
// asserts, is unchanged in substance: repeated `down` scrolls the viewport
// (and `up` scrolls it back) rather than leaving tall content unreachable.
// The exact-±1-per-press assertion moved to box_nav_field_test.go's
// TestBoxFormDownScrollsThroughATallField, where it belongs to the tall-field
// reveal rule.
func TestBoxFormScrollDownUpWhenDetailFocused(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)

	m.detailFocus = true
	for i := 0; i < 10 && m.boxFormScroll == 0; i++ {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if m.boxFormScroll <= 0 {
		t.Fatalf("boxFormScroll after 10 detail-focused downs = %d, want > 0", m.boxFormScroll)
	}
	if m.boxFormScrollBean != b.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q (offset must record its owning bean)", m.boxFormScrollBean, b.ID)
	}

	deeper := m
	deeper = step(t, deeper, keyMsg(tea.KeyDown))
	if deeper.boxFormScroll <= m.boxFormScroll {
		t.Fatalf("boxFormScroll after another down = %d, want > %d", deeper.boxFormScroll, m.boxFormScroll)
	}

	back := step(t, deeper, keyMsg(tea.KeyUp))
	if back.boxFormScroll >= deeper.boxFormScroll {
		t.Fatalf("boxFormScroll after up = %d, want < %d", back.boxFormScroll, deeper.boxFormScroll)
	}
}

// TestBoxFormScrollUnaffectedWhenTreeFocused guards the negative: the SAME
// down keypress while the Tree has focus (m.detailFocus == false, the
// default) must move the Tree cursor, not model.boxFormScroll -- Akzeptanz
// bean bt-ze10 Test list item 1's "does NOT change" half.
func TestBoxFormScrollUnaffectedWhenTreeFocused(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)
	if m.detailFocus {
		t.Fatal("setup: expected Tree-focused (detailFocus == false)")
	}

	// tk-2 sorts LAST among the fixture's two tasks (fixtureBeans' own doc
	// comment: "tk-1 sorts before tk-2"), so `up` -- not `down` -- is the
	// direction guaranteed to actually move the Tree cursor here.
	before := m.cursorID
	m = step(t, m, keyMsg(tea.KeyUp))
	if m.boxFormScroll != 0 {
		t.Fatalf("tree-focused up changed boxFormScroll to %d, want 0 (unaffected)", m.boxFormScroll)
	}
	if m.boxFormScrollBean != "" {
		t.Fatalf("boxFormScrollBean = %q, want unset (never written while Tree-focused)", m.boxFormScrollBean)
	}
	if m.cursorID == before {
		t.Fatalf("tree-focused up left cursorID unchanged (%q) -- the keypress should have moved the Tree cursor instead of scrolling", before)
	}
}

// --- Clamping ---

// TestBoxFormScrollClampsAtBothEnds guards Akzeptanz "Offset clamped (kein
// Scroll ueber Anfang/Ende hinaus)": a large negative delta floors at 0, a
// large positive delta ceilings at total-height, and repeated over-scrolling
// past either end stays pinned rather than drifting further.
func TestBoxFormScrollClampsAtBothEnds(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	wantMax := total - height

	mNeg := m.adjustBoxFormScroll(b, -5)
	if mNeg.boxFormScroll != 0 {
		t.Fatalf("adjustBoxFormScroll(b, -5) from a fresh 0 baseline = %d, want clamped 0", mNeg.boxFormScroll)
	}
	mNeg2 := mNeg.adjustBoxFormScroll(b, -1)
	if mNeg2.boxFormScroll != 0 {
		t.Fatalf("adjustBoxFormScroll(b, -1) already at 0 = %d, want still 0 (floor)", mNeg2.boxFormScroll)
	}

	mMax := m.adjustBoxFormScroll(b, total+50)
	if mMax.boxFormScroll != wantMax {
		t.Fatalf("adjustBoxFormScroll(b, total+50) = %d, want clamped ceiling %d", mMax.boxFormScroll, wantMax)
	}
	mMax2 := mMax.adjustBoxFormScroll(b, 10)
	if mMax2.boxFormScroll != wantMax {
		t.Fatalf("adjustBoxFormScroll(b, 10) already at the ceiling = %d, want still %d", mMax2.boxFormScroll, wantMax)
	}
}

// --- Mouse wheel ---

// TestBoxFormWheelOverDetailPaneScrolls guards Akzeptanz/Test-list item 3:
// a wheel tick whose coordinates land inside the Detail pane's own
// bounds (boxFormWheelHit, mouse.go) scrolls model.boxFormScroll,
// regardless of m.detailFocus (location-based, mirrors a real mouse's
// hover-independent-of-keyboard-focus semantics) -- and a wheel tick over
// the TREE pane leaves it untouched, falling through to the pre-existing
// cursor-move path (design decision f, mouse.go).
func TestBoxFormWheelOverDetailPaneScrolls(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	requireOverflow(t, m, b)

	head, localKeys := m.browseRepoChrome(m.width - 2)
	_, lw, _, originX, originY := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, layoutFilterDetail)

	detailMsg := tea.MouseMsg{
		Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
		X: originX + lw + 2, Y: originY + filterBarHeight + 1,
	}
	tm, _ := m.handleMouse(detailMsg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel over Detail pane) did not return a model, got %T", tm)
	}
	if m2.boxFormScroll != 1 {
		t.Fatalf("wheel-down over the Detail pane: boxFormScroll = %d, want 1", m2.boxFormScroll)
	}
	if m2.boxFormScrollBean != b.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q", m2.boxFormScrollBean, b.ID)
	}

	treeMsg := tea.MouseMsg{
		Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress,
		X: originX + 2, Y: originY + 1,
	}
	tm2, _ := m.handleMouse(treeMsg)
	m3, ok := tm2.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel over Tree pane) did not return a model, got %T", tm2)
	}
	if m3.boxFormScroll != 0 {
		t.Fatalf("wheel-down over the Tree pane changed boxFormScroll to %d, want 0 (unaffected)", m3.boxFormScroll)
	}
}

// --- Reset on selection change ---

// TestBoxFormScrollResetsWhenSelectedBeanChanges guards Akzeptanz "Offset
// resets when the selected bean changes": a stale offset/owner recorded for
// one bean must never leak into a DIFFERENT bean's render, and the next
// keyboard/wheel adjustment on the new bean starts from a fresh 0 baseline
// rather than compounding the stale value.
func TestBoxFormScrollResetsWhenSelectedBeanChanges(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean() // tk-2
	requireOverflow(t, m, b)

	m = m.adjustBoxFormScroll(b, 3)
	if m.boxFormScroll != 3 || m.boxFormScrollBean != b.ID {
		t.Fatalf("setup: boxFormScroll=%d boxFormScrollBean=%q, want 3/%q", m.boxFormScroll, m.boxFormScrollBean, b.ID)
	}

	// Selection moves to a DIFFERENT bean (e.g. a Tree reselect, Relations-
	// jump, ...) -- boxFormScroll/boxFormScrollBean are still tk-2's, now
	// stale.
	m = focusBean(m, "tk-1")
	other := m.focusedBean()
	if other == nil || other.ID == b.ID {
		t.Fatalf("setup: expected a different focused bean, got %v", other)
	}
	requireOverflow(t, m, other) // tk-1 also has a long body (boxFormLongBodyBeans)

	if got := boxFormEffectiveScroll(m, other); got != 0 {
		t.Fatalf("boxFormEffectiveScroll(tk-1) after the selection moved off tk-2 = %d, want 0 (stale offset must not leak into a new bean)", got)
	}

	m = m.adjustBoxFormScroll(other, 1)
	if m.boxFormScroll != 1 {
		t.Fatalf("boxFormScroll after the FIRST adjust on the new bean = %d, want 1 (fresh baseline, not 3+1=4)", m.boxFormScroll)
	}
	if m.boxFormScrollBean != other.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q (ownership transferred to the new bean)", m.boxFormScrollBean, other.ID)
	}
}

// --- End-to-end: previously-cut-off content becomes reachable ---

// TestBoxFormScrollRevealsContentPreviouslyCutOff is the bean's own headline
// Akzeptanz criterion, exercised through the REAL render pipeline (m.View()):
// a long Body's tail (BODYLINE80) is invisible before scrolling and visible
// once scrolled to the fixture's own overflow ceiling.
func TestBoxFormScrollRevealsContentPreviouslyCutOff(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, _ := requireOverflow(t, m, b)

	before := ansi.Strip(m.View())
	if !strings.Contains(before, "BODYLINE01") {
		t.Fatal("BODYLINE01 (near the top of the Body list) must be visible before scrolling")
	}
	if strings.Contains(before, "BODYLINE80") {
		t.Fatal("BODYLINE80 (near the bottom) must NOT already be visible before scrolling -- fixture doesn't actually overflow the pane, test is not meaningful")
	}

	scrolled := m.adjustBoxFormScroll(b, total) // intentionally oversized -- clampBoxFormScroll bounds it
	after := ansi.Strip(scrolled.View())
	if !strings.Contains(after, "BODYLINE80") {
		t.Fatal("BODYLINE80 must become visible once scrolled to the end -- previously-cut-off content must be reachable (bean bt-ze10's headline Akzeptanz)")
	}
}

// --- Accordion mode (flag off) stays untouched ---
