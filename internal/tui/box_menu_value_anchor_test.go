package tui

// box_menu_value_anchor_test.go — TDD coverage for Slice C (bt-f0y9,
// "feld-verankertes Inline-Dropdown", D09 revidiert) + its B01/B02 fix round
// (Reviewer, 2026-07-20): the value menu (Status/Type/Priority) opens
// field-anchored (placeOverlayAt + boxFormFieldRect) instead of centered
// (placeOverlay). Every render-grounded assertion
// here locates a REAL landmark (the popup's own bottom-border corner) in
// ansi.Strip(m.View()) and compares its position to what boxFormFieldRect
// (Slice A) + valueMenuPopupY (this slice's own pure function, unit-tested
// separately below) compute -- never a hand-typed pixel guess, and never a
// "re-render with overlay closed" baseline for the WHOLE view (footer text
// itself depends on m.overlay, footer_context.go's overlayLocalBindings --
// that would silently compare against a DIFFERENT chrome, not just a
// different overlay layer). The B01 chrome-integrity test below DOES use a
// before/after comparison, but scoped ONLY to the rows the fix guarantees
// are untouched (above boxFormPopupChromeFloor) -- those rows are NOT
// overlay-dependent (outer border/breadcrumb/divider), unlike the footer.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// --- valueMenuPopupY (pure function) ---

func TestValueMenuPopupYFitsBelow(t *testing.T) {
	got := valueMenuPopupY(5, 3, 10, 30, 0)
	if want := 8; got != want {
		t.Fatalf("valueMenuPopupY(fits below) = %d, want %d", got, want)
	}
}

func TestValueMenuPopupYFlipsAboveWhenBelowOverflowsAndFullyFitsRespectingFloor(t *testing.T) {
	// anchorY=20, anchorH=3 -> below=23, +fgH(10)=33 > canvasH(25): overflow.
	// above = 20-10 = 10 >= minY(3): flips, fits fully without crossing the
	// chrome floor.
	got := valueMenuPopupY(20, 3, 10, 25, 3)
	if want := 10; got != want {
		t.Fatalf("valueMenuPopupY(flip, full fit) = %d, want %d", got, want)
	}
}

// TestValueMenuPopupYNeverCrossesChromeFloor is the B01 regression at the
// pure-function level (Reviewer, 2026-07-20): the OLD behavior flipped
// whenever there was "any room above" and let the result go negative
// (anchorY=10, fgH=12 -> above=-2), which spliced the popup over the app's
// own title/border at 80x24. The FIX only flips when the result would not
// cross minY -- here it can't (-2 < minY=3), so it must stay BELOW instead,
// even though that means clipping at the bottom.
func TestValueMenuPopupYNeverCrossesChromeFloor(t *testing.T) {
	got := valueMenuPopupY(10, 3, 12, 20, 3)
	if want := 13; got != want { // below = anchorY+anchorH = 13, NOT flipped
		t.Fatalf("valueMenuPopupY(would-cross-floor) = %d, want %d (must stay below, never flip into the chrome floor)", got, want)
	}
	if got < 3 {
		t.Fatalf("valueMenuPopupY(would-cross-floor) = %d, crosses minY=3 -- chrome floor violated", got)
	}
}

func TestValueMenuPopupYNoRoomAboveFallsBackToBelow(t *testing.T) {
	// anchorY == minY: no room above at all -- falls back to "below",
	// clipped at the bottom like any other overflow.
	got := valueMenuPopupY(3, 3, 10, 8, 3)
	if want := 6; got != want {
		t.Fatalf("valueMenuPopupY(no room above) = %d, want %d", got, want)
	}
}

// --- boxFormFieldIndexForGroup ---

func TestBoxFormFieldIndexForGroupMatchesFieldOrder(t *testing.T) {
	cases := []struct{ group, field string }{
		{"status", "status"},
		{"type", "type"},
		{"priority", "priority"},
	}
	for _, tc := range cases {
		if got, want := boxFormFieldIndexForGroup(tc.group), fieldIdx(t, tc.field); got != want {
			t.Fatalf("boxFormFieldIndexForGroup(%q) = %d, want %d", tc.group, got, want)
		}
	}
}

func TestBoxFormFieldIndexForGroupUnknownReturnsNegativeOne(t *testing.T) {
	if got := boxFormFieldIndexForGroup("bogus"); got != -1 {
		t.Fatalf("boxFormFieldIndexForGroup(bogus) = %d, want -1", got)
	}
}

// --- valueMenuContentWidth (B02 fix: content-sized, not fixed-40) ---

func TestValueMenuContentWidthSizesToLongestLine(t *testing.T) {
	title := "Set status"
	body := "short\nmedium line here\nx"
	got := valueMenuContentWidth(title, body)
	want := ansi.StringWidth("medium line here") + 2
	if got != want {
		t.Fatalf("valueMenuContentWidth = %d, want %d", got, want)
	}
}

func TestValueMenuBoxWidthIsUnderOldFixedFortyAndMatchesContent(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-2")
	m.detailFocus = true
	m = step(t, m, runeMsg('s'))

	fg := m.valueMenuBox()
	fgLines := strings.Split(fg, "\n")
	fgW := maxLineWidth(fgLines)
	// fgW = modalBox's TOTAL rendered width = content-width param + 2
	// (Border, overlay.go's own Width()-is-content-only contract, confirmed
	// render-grounded in this slice's own debugging). The OLD fixed
	// preference was 40 -> fgW=42; content-sized must be strictly narrower.
	if fgW >= 42 {
		t.Fatalf("valueMenuBox() total width = %d, want < 42 (old fixed-40 preference) -- B02 fix did not shrink to content", fgW)
	}
	if got, want := fgW, valueMenuContentWidth(valueMenuTitle(m.menuItems), rawValueMenuBody(m))+2; got != want {
		t.Fatalf("valueMenuBox() total width = %d, want %d (valueMenuContentWidth+2 border)", got, want)
	}
}

// rawValueMenuBody rebuilds valueMenuBox's OWN body string (the part BEFORE
// modalPanel/modalBox wrap it) -- duplicated here (not exported from
// box_menu_value.go, which returns only the fully-wrapped box) purely to let
// this ONE width-assertion test verify valueMenuBox() actually used
// valueMenuContentWidth on the SAME body it renders, not a hand-typed guess.
func rawValueMenuBody(m model) string {
	var b strings.Builder
	b.WriteString("enter:apply  esc/" + valueMenuGroupKey(m.valueMenuGroup()).Help().Key + ":cancel\n")
	lastGroup := ""
	for i, it := range m.menuItems {
		if it.group != lastGroup {
			b.WriteString("\n" + valueMenuGroupHead[it.group] + "\n")
			lastGroup = it.group
		}
		label := it.value
		if cur := m.idx.ByID[m.mutTarget]; cur != nil && valueMenuIsCurrent(cur, it) {
			label += " (current)"
		}
		prefix := "  "
		if i == m.menu.cursor {
			prefix = "▸ "
		}
		b.WriteString(prefix + label + "\n")
	}
	return b.String()
}

// --- Render-grounded: anchored placement, one test per trigger path ---

// anchorModel builds a fixtureModel at the given size, tk-2
// focused (Task Two, ms-1->ep-1->tk-2 ancestry expanded, box_menu_value_test.
// go's focusBean), Detail-Pane focused (tab-equivalent) -- the state every
// one of the four trigger paths needs at minimum.
func anchorModel(t *testing.T, width, height int) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: width, Height: height})
	m = focusBean(m, "tk-2")
	m.detailFocus = true
	return m
}

// cellAt returns the rune (as a string, ANSI-safe) at the given column of an
// already ansi.Strip()'d line -- ansi.Cut(l, col, col+1), mirroring Slice A's
// own box_nav_field_test.go corner-check pattern (never strings.Index, which
// would find the FIRST occurrence of a glyph ANYWHERE on the row -- the
// background chrome (e.g. the Tree pane's own Search sub-box) can render the
// SAME "╰"/"╭" box-drawing characters at an earlier column than the popup's
// own corner).
func cellAt(l string, col int) string {
	return ansi.Cut(l, col, col+1)
}

// valueMenuPopupXY computes the SAME (x, y) placeValueMenuOverlay itself
// would use for wantField, given m's already-open value menu -- shared by
// every render-grounded assertion below so the "expected" position is
// always derived from the same small set of already-proven primitives
// (boxFormFieldRect, boxFormPaneContentLeft/boxFormPopupChromeFloor,
// valueMenuPopupY), never a second hand-rolled formula that could drift from
// placeValueMenuOverlay's own.
func valueMenuPopupXY(t *testing.T, m model, wantField string) (x, y, fgW, fgH int) {
	t.Helper()
	b := m.idx.ByID[m.mutTarget]
	if b == nil {
		t.Fatalf("setup: mutTarget %q not in index", m.mutTarget)
	}
	wantIdx := fieldIdx(t, wantField)
	rx, ry, _, rh, ok := boxFormFieldRect(m, b, wantIdx)
	if !ok {
		t.Fatalf("boxFormFieldRect(%s) ok=false", wantField)
	}
	fg := m.valueMenuBox()
	fgLines := strings.Split(fg, "\n")
	fgW, fgH = maxLineWidth(fgLines), len(fgLines)

	x = rx
	if fgW < m.width && x+fgW > m.width {
		x = m.width - fgW
	}
	if paneLeft := boxFormPaneContentLeft(m); x < paneLeft {
		x = paneLeft
	}
	y = valueMenuPopupY(ry, rh, fgH, m.height, boxFormPopupChromeFloor(m))
	return x, y, fgW, fgH
}

// assertValueMenuAnchoredBelowField is the shared render-grounded assertion
// every trigger-path test below drives: m (with the value menu already open,
// via whichever trigger) must have its popup's bottom-border corner at
// EXACTLY valueMenuPopupXY(wantField)'s own position -- proving
// composeOverlays' new anchored branch actually ran with the RIGHT field's
// geometry, not a leftover centered placeOverlay (which would put the
// corner at a completely different column/row for any non-trivial terminal
// size).
func assertValueMenuAnchoredBelowField(t *testing.T, m model, wantField string) {
	t.Helper()
	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	wantIdx := fieldIdx(t, wantField)
	if m.valueMenuAnchorField != wantIdx {
		t.Fatalf("valueMenuAnchorField = %d, want %d (%s)", m.valueMenuAnchorField, wantIdx, wantField)
	}

	x, y, fgW, fgH := valueMenuPopupXY(t, m, wantField)
	wantRow := y + fgH - 1

	lines := screenLines(m)
	if wantRow < 0 || wantRow >= len(lines) {
		t.Fatalf("m.View() (field=%s): wantRow=%d out of range (%d lines)", wantField, wantRow, len(lines))
	}
	if got := cellAt(lines[wantRow], x); got != "╰" {
		t.Fatalf("m.View() (field=%s): cell at (row=%d,col=%d) = %q, want \"╰\" (popup bottom-left corner; x=%d,y=%d,fgW=%d,fgH=%d, row=%q)", wantField, wantRow, x, got, x, y, fgW, fgH, lines[wantRow])
	}

	// Sanity: the anchored column must differ from the OLD centered column
	// for every size this file tests (never coincidentally equal) -- a
	// regression that silently fell back to centered would otherwise still
	// pass the row/col check above only by coincidence.
	centerX := (m.width - fgW) / 2
	if centerX < 0 {
		centerX = 0
	}
	if x == centerX {
		t.Fatalf("m.View() (field=%s): anchored x (%d) coincides with the OLD centered x -- test fixture cannot distinguish the two, widen it", wantField, x)
	}
}

// TestValueMenuAnchoredViaKeyboard guards the keyboard trigger (`s`,
// update.go's keyNodeAction) at 80 AND 120 columns, tall enough (h=40) that
// the popup fits fully below Status -- the default, no-flip case.
func TestValueMenuAnchoredViaKeyboard(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			m = step(t, m, runeMsg('s'))
			assertValueMenuAnchoredBelowField(t, m, "status")
		})
	}
}

// TestValueMenuAnchoredViaFieldCursorEnter guards the field-cursor Enter
// trigger (boxFormActivateCursor -> activateBoxFormTarget, box_nav_field.go/
// mouse.go): cursor parked on "type", enter opens Type's menu anchored to
// Type's own box, not Status'.
func TestValueMenuAnchoredViaFieldCursorEnter(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			b := m.focusedBean()
			m.boxFormCursor = fieldIdx(t, "type")
			m.boxFormCursorBean = b.ID
			m = step(t, m, keyMsg(tea.KeyEnter))
			assertValueMenuAnchoredBelowField(t, m, "type")
		})
	}
}

// TestValueMenuAnchoredViaMouseClick guards the mouse-click trigger
// (mouseBoxFormDetailClick -> detailBoxFormClickRow -> activateBoxFormTarget,
// mouse.go): clicking Priority's own "(p)" hotkey badge anchors to Priority.
func TestValueMenuAnchoredViaMouseClick(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			msg := boxFormClickAt(t, m, "(p)")
			tm, _ := m.handleMouse(msg)
			m2, ok := tm.(model)
			if !ok {
				t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
			}
			assertValueMenuAnchoredBelowField(t, m2, "priority")
		})
	}
}

// TestValueMenuAnchoredViaPalette guards the Command-Center trigger
// (dispatchPalette, overlay_palette.go): actionID "type" opens the SAME
// m.openValueMenu("type") the `o` key does, anchored to Type.
func TestValueMenuAnchoredViaPalette(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(width2str(width), func(t *testing.T) {
			m := anchorModel(t, width, 40)
			tm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "type", label: "set type"})
			m2, ok := tm.(model)
			if !ok {
				t.Fatalf("dispatchPalette did not return a model, got %T", tm)
			}
			assertValueMenuAnchoredBelowField(t, m2, "type")
		})
	}
}

// --- B01 regression: flip must never corrupt the app's own chrome ---

// TestValueMenuFlipDoesNotCorruptChrome is the B01 regression (Reviewer,
// 2026-07-20): at 80x24, Status sits at y=10 with a 12-line popup -- below
// overflows the canvas, and the OLD flip-up logic computed y=-2, splicing
// the popup's own middle rows straight over the app's outer border/
// breadcrumb/divider (canvas row 0). Every row from 0 up to
// boxFormPopupChromeFloor(m) (exclusive) must be BYTE-IDENTICAL to the
// pre-open baseline -- those rows are NOT overlay-dependent (only the
// FOOTER is, footer_context.go's overlayLocalBindings), so this before/
// after comparison is safe, unlike a whole-view baseline compare would be.
func TestValueMenuFlipDoesNotCorruptChrome(t *testing.T) {
	m := anchorModel(t, 80, 24)
	baseline := screenLines(m) // overlay still closed

	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Fatalf("setup: overlay = %v, want overlayValueMenu", m.overlay)
	}

	floor := boxFormPopupChromeFloor(m)
	if floor <= 0 {
		t.Fatalf("setup: boxFormPopupChromeFloor = %d, want > 0 for this test to be meaningful", floor)
	}
	lines := screenLines(m)
	for row := 0; row < floor; row++ {
		if row >= len(baseline) || row >= len(lines) {
			t.Fatalf("setup: row %d out of range (baseline=%d, lines=%d)", row, len(baseline), len(lines))
		}
		if lines[row] != baseline[row] {
			t.Fatalf("row %d (above chrome floor %d) corrupted by the popup:\n  got:  %q\n  want: %q (unchanged app chrome)", row, floor, lines[row], baseline[row])
		}
	}
}

// TestValueMenuFlipStaysBelowWhenFullFitAboveIsImpossible documents (render-
// grounded) WHY the flip branch is effectively unreachable for Status/Type/
// Priority at realistic sizes, now that B01 requires a FULL fit above the
// chrome floor: the popup instead stays below and clips at the bottom,
// which is exactly what TestValueMenuFlipDoesNotCorruptChrome above proves
// is safe.
func TestValueMenuFlipStaysBelowWhenFullFitAboveIsImpossible(t *testing.T) {
	m := anchorModel(t, 80, 24)
	m = step(t, m, runeMsg('s'))

	b := m.idx.ByID[m.mutTarget]
	rx, ry, _, rh, ok := boxFormFieldRect(m, b, fieldIdx(t, "status"))
	_ = rx
	if !ok {
		t.Fatalf("boxFormFieldRect ok=false")
	}
	fg := m.valueMenuBox()
	fgH := len(strings.Split(fg, "\n"))
	floor := boxFormPopupChromeFloor(m)
	gotY := valueMenuPopupY(ry, rh, fgH, m.height, floor)
	wantBelow := ry + rh
	if gotY != wantBelow {
		t.Fatalf("valueMenuPopupY = %d, want %d (below -- a full fit above the chrome floor is impossible here: anchorY=%d, fgH=%d, floor=%d)", gotY, wantBelow, ry, fgH, floor)
	}
}

// --- B02 regression: Type and Priority must sit at DIFFERENT, field-near x ---

// TestValueMenuTypeAndPriorityPopupsSitAtDifferentFieldNearColumns is the
// B02 regression (Reviewer, 2026-07-20): the OLD fixed-width-40 popup
// clamped BOTH Type (anchor x=46) and Priority (anchor x=62) onto the SAME
// column (38) at 80 columns, because a popup far wider than either field's
// own box ran off the right edge from both anchors alike. The content-sized
// popup (B02 fix, valueMenuContentWidth) is narrow enough that Type needs no
// clamp at all and Priority needs, at most, a small one -- the two MUST
// render at visibly different, field-near columns.
func TestValueMenuTypeAndPriorityPopupsSitAtDifferentFieldNearColumns(t *testing.T) {
	mType := anchorModel(t, 80, 40)
	mType = step(t, mType, runeMsg('t'))
	assertValueMenuAnchoredBelowField(t, mType, "type")
	typeX, _, _, _ := valueMenuPopupXY(t, mType, "type")

	mPriority := anchorModel(t, 80, 40)
	mPriority = step(t, mPriority, runeMsg('p'))
	assertValueMenuAnchoredBelowField(t, mPriority, "priority")
	priorityX, _, _, _ := valueMenuPopupXY(t, mPriority, "priority")

	if typeX == priorityX {
		t.Fatalf("Type popup x (%d) == Priority popup x (%d) -- B02 not fixed, both still clamped onto the same column", typeX, priorityX)
	}

	bType := mType.idx.ByID[mType.mutTarget]
	typeRectX, _, _, _, ok := boxFormFieldRect(mType, bType, fieldIdx(t, "type"))
	if !ok {
		t.Fatalf("boxFormFieldRect(type) ok=false")
	}
	if typeX != typeRectX {
		t.Fatalf("Type popup x = %d, want == its own field x %d (left-aligned, no overflow expected here)", typeX, typeRectX)
	}

	bPriority := mPriority.idx.ByID[mPriority.mutTarget]
	priorityRectX, _, _, _, ok := boxFormFieldRect(mPriority, bPriority, fieldIdx(t, "priority"))
	if !ok {
		t.Fatalf("boxFormFieldRect(priority) ok=false")
	}
	// Priority's popup must stay NEAR its own field, not fall back to some
	// unrelated fixed column: within one field-width of the field's own x.
	if d := priorityRectX - priorityX; d < 0 || d > 20 {
		t.Fatalf("Priority popup x = %d, field x = %d -- not field-near (delta=%d, want [0,20])", priorityX, priorityRectX, d)
	}
}

// --- layoutDetail ---

// TestValueMenuAnchoredInLayoutDetail guards layoutDetail's full-width
// geometry, which boxFormFieldRect handles (lw == 0): `v` from the split
// reaches layoutDetail, then `s` opens Status' menu anchored to the field
// rect, not centered.
func TestValueMenuAnchoredInLayoutDetail(t *testing.T) {
	m := anchorModel(t, 100, 30)
	m = step(t, m, runeMsg('v'))
	if m.layout != layoutDetail {
		t.Fatalf("setup: layout = %v after `v`, want layoutDetail", m.layout)
	}

	m = step(t, m, runeMsg('s'))
	assertValueMenuAnchoredBelowField(t, m, "status")
}

// --- Flag off: accordion mode stays centered ---

func maxLineWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		if lw := ansi.StringWidth(l); lw > w {
			w = lw
		}
	}
	return w
}

func width2str(w int) string {
	switch w {
	case 80:
		return "w=80"
	case 120:
		return "w=120"
	default:
		return "w=other"
	}
}
