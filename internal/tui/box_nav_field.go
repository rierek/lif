package tui

// box_nav_field.go — the box-form FIELD CURSOR (bean bt-1o4g, epic bt-vy1q,
// PO-Nebenbefund N8): keyboard-first navigation between detailBoxForm's boxes
// (box_detail_form.go) while the Detail pane is focused (up/down).
//
// THREE deliberate single-source decisions, all Golden-Rule-Drift-Schutz:
//
//  1. boxFormFieldOrder is the ONE field table. It carries each field's
//     (row, col) POSITION in detailBoxForm's own fixed layout (D12: Title /
//     Status|Type|Priority / Parent|Tags / Body / Relations / History), so the
//     render (which box gets focused=true), the navigation (which field is
//     left/right/up/down of which) and the mouse hit-test (mouse.go's
//     detailBoxFormClickRow, which now resolves a click through
//     boxFormFieldAt below) all read the SAME positions. There is no second,
//     independently-maintained order that could drift out of sync with the
//     rendered layout.
//
//  2. enter goes through activateBoxFormTarget (mouse.go) -- the identical
//     dispatch a mouse click already takes, which itself mirrors keyNodeAction's
//     s/o/u/a/t/e branches. One vocabulary (boxFormTarget*), one dispatch.
//
//  3. Feld-Fokus und Scroll (bean bt-ze10's viewport) are ONE movement, not
//     two competing ones -- see boxFormNav's own doc comment for the rule.

import (
	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/pkg/item"
)

// boxFormPageUp/boxFormPageDown page the focused Detail pane by one FULL
// screen (bean bt-adkn, PO-Entscheidung 2026-07-20: pgup/pgdn -- ctrl+i is
// Tab (0x09, indistinguishable), ctrl+k is the Command-Palette, both
// excluded; pgup/pgdn mean exactly "blaettern" and collide with nothing,
// TestKeymapNoCtrlSQ is the precedent for hard terminal-collision guards).
//
// Standalone keybind.Bindings, NOT keyMap fields: keymap_test.go's
// TestHelpGroupsCoverEveryBindingExactlyOnce reflects over keyMap fields only,
// and these are Detail-region-local keys, not global Help entries.
//
// Both route through the SAME adjustBoxFormScroll mutation point up/down and
// the wheel use (keyDetailFocus, update.go), one screen == boxFormScrollBounds'
// reported height, so line-wise and page-wise scrolling can never drift on the
// reset/clamp rules (SSTD load-bearing constraint).
var (
	boxFormPageUp   = keybind.NewBinding(keybind.WithKeys("pgup"), keybind.WithHelp("pgup", "page up"))
	boxFormPageDown = keybind.NewBinding(keybind.WithKeys("pgdown"), keybind.WithHelp("pgdn", "page down"))
)

// boxFormRow* index detailBoxForm's block slice (boxFormBlocks,
// box_detail_form.go) -- one entry per rendered layout row.
const (
	boxFormRowTitle = iota
	boxFormRowScalarsA
	boxFormRowScalarsB
	boxFormRowBody
	boxFormRowRelations
	boxFormRowHistory
)

// boxFormField is one navigable box: its position in detailBoxForm's fixed
// layout plus the boxFormTarget* action enter/click fires on it.
type boxFormField struct {
	name   string
	target string
	row    int
	col    int
}

// boxFormFieldOrder is the fixed navigation order, in detailBoxForm's own
// render order (bean bt-1o4g Ziel 2). Title/Body/Relations/History all carry
// boxFormTargetEditor: `e` opens the whole-bean $EDITOR regardless of section
// context (D01, update.go's keyNodeAction doc comment "egal an welcher
// Stelle"), the same collapse detailBoxFormClickRow already made for clicks.
var boxFormFieldOrder = []boxFormField{
	{"title", boxFormTargetEditor, boxFormRowTitle, 0},
	{"status", boxFormTargetStatus, boxFormRowScalarsA, 0},
	{"type", boxFormTargetType, boxFormRowScalarsA, 1},
	{"priority", boxFormTargetPriority, boxFormRowScalarsA, 2},
	{"parent", boxFormTargetParent, boxFormRowScalarsB, 0},
	{"tags", boxFormTargetTags, boxFormRowScalarsB, 1},
	{"body", boxFormTargetEditor, boxFormRowBody, 0},
	{"relations", boxFormTargetEditor, boxFormRowRelations, 0},
	{"history", boxFormTargetEditor, boxFormRowHistory, 0},
}

// boxFormRowCols reports how many columns a layout row has -- derived from
// boxFormFieldOrder itself rather than restating D12's 3|2 grouping, so the
// grid can never disagree with the table above.
func boxFormRowCols(row int) int {
	n := 0
	for _, f := range boxFormFieldOrder {
		if f.row == row {
			n++
		}
	}
	return n
}

// boxFormCursorRow returns the layout row the field cursor sits in, or -1 for
// no cursor (the SAME derivation boxFormBlocks' own focusRow uses, box_detail_
// form.go). Lets renderDetailBox resolve the Body header's focus color
// without restating the cursor->row rule (bean bt-adkn Rework B3).
func boxFormCursorRow(cursor int) int {
	if cursor >= 0 && cursor < len(boxFormFieldOrder) {
		return boxFormFieldOrder[cursor].row
	}
	return -1
}

// boxFormFieldIndex resolves a (row, col) position back to its index in
// boxFormFieldOrder, or -1 when the position carries no field.
func boxFormFieldIndex(row, col int) int {
	for i, f := range boxFormFieldOrder {
		if f.row == row && f.col == col {
			return i
		}
	}
	return -1
}

// boxFormFieldAt maps a Detail-pane content row/column (relative to the
// pane's own top-left, i.e. AFTER mouse.go's origin/filter-bar corrections)
// to a boxFormFieldOrder index, or -1 for "no field here" (the 1-col gap
// between grid boxes). accW is the pane's box width, the same value
// detailBoxForm renders at -- gridColWidths (box_detail_form.go) is the ONE
// place column boundaries are computed, shared with gridRow's real render.
//
// The three scalar rows are exactly 3 lines tall each (dropdownBox's fixed
// top/value/bottom shape), so rows 0-8 map arithmetically. Everything past
// them (Body/Relations/History panels, unbounded height) collapses onto the
// Body field: its target is boxFormTargetEditor, the SAME action all three
// panels fire, so this is not a mis-mapping -- it is the pre-existing
// detailBoxFormClickRow default branch, expressed through the field table.
func boxFormFieldAt(row, col, accW int) int {
	switch {
	case row < 0:
		return -1
	case row < 3:
		return boxFormFieldIndex(boxFormRowTitle, 0)
	case row < 6:
		c := gridColAt(gridColWidths(boxFormRowCols(boxFormRowScalarsA), accW), col)
		if c < 0 {
			return -1
		}
		return boxFormFieldIndex(boxFormRowScalarsA, c)
	case row < 9:
		c := gridColAt(gridColWidths(boxFormRowCols(boxFormRowScalarsB), accW), col)
		if c < 0 {
			return -1
		}
		return boxFormFieldIndex(boxFormRowScalarsB, c)
	default:
		return boxFormFieldIndex(boxFormRowBody, 0)
	}
}

// boxFormMoveField steps the cursor one field in dir ("up"/"down"/"left"/
// "right"), returning the new index. left/right walk the columns of the
// CURRENT row; up/down walk rows keeping the column where the target row has
// one and clamping to its last column otherwise (so `down` from Priority --
// row A col 2 -- lands on Tags, row B's last column). Edges are no-ops
// (no wrap-around): the PO's mental model is a form, not a carousel.
func boxFormMoveField(cur int, dir string) int {
	if cur < 0 || cur >= len(boxFormFieldOrder) {
		cur = 0
	}
	f := boxFormFieldOrder[cur]
	switch dir {
	case "left":
		if next := boxFormFieldIndex(f.row, f.col-1); next >= 0 {
			return next
		}
	case "right":
		if next := boxFormFieldIndex(f.row, f.col+1); next >= 0 {
			return next
		}
	case "up", "down":
		row := f.row - 1
		if dir == "down" {
			row = f.row + 1
		}
		if row < boxFormRowTitle || row > boxFormRowHistory {
			return cur
		}
		col := f.col
		if n := boxFormRowCols(row); col >= n {
			col = n - 1
		}
		if next := boxFormFieldIndex(row, col); next >= 0 {
			return next
		}
	}
	return cur
}

// boxFormEffectiveCursor returns the field cursor for b, defaulting to the
// FIRST field (Title) whenever the selection has moved to a different bean
// since the cursor was last written (bean bt-1o4g "Cursor resettet bei
// Bean-Wechsel"). Same derived-reset doctrine as boxFormEffectiveScroll
// (mouse.go): one lazy check where the value is READ beats N reset call sites
// at every cursor-move/click/jump that can change the selection. Also the
// defensive clamp point -- boxFormCursor survives watch-reloads untouched.
func boxFormEffectiveCursor(m model, b *item.Item) int {
	if b == nil || b.ID != m.boxFormCursorBean {
		return 0
	}
	if m.boxFormCursor < 0 || m.boxFormCursor >= len(boxFormFieldOrder) {
		return 0
	}
	return m.boxFormCursor
}

// boxFormScrollIntoView returns the smallest adjustment of off that makes the
// field spanning [start, start+h) visible in a viewport of `height` lines.
// A field TALLER than the viewport can never be fully shown: any offset that
// still lands inside the field is kept (that is what lets `down` walk through
// a long Body line by line, see boxFormNav), only offsets outside it snap
// back to the field's own head/tail.
func boxFormScrollIntoView(off, start, h, height int) int {
	if height <= 0 {
		return off
	}
	if h >= height {
		if off < start {
			return start
		}
		if tail := start + h - height; off > tail {
			if tail < start {
				tail = start
			}
			return tail
		}
		return off
	}
	if start < off {
		return start
	}
	if start+h > off+height {
		return start + h - height
	}
	return off
}

// boxFormNav is the ONE keyboard mutation point for the box-form field cursor
// and, with it, the box-form scroll offset (bean bt-ze10's viewport).
//
// THE RULE (bean bt-1o4g's explicit "a focused field that scrolls out of the
// visible area would be a bug"): field focus LEADS, the viewport FOLLOWS.
// Whatever moves the cursor, the tail of this function pulls the viewport to
// the new field (scroll-into-view) before returning.
//
// Directions:
//
//   - "next"/"prev" (bean bt-8d35, the ONLY ones a key drives today:
//     tab/shift+tab) step LINEARLY through boxFormFieldOrder and WRAP at both
//     ends -- the PO's explicit "am Ende umbrechen, NICHT in den Tree
//     zurueckfallen, sonst verliert man den Fokus versehentlich beim
//     Durchsteppen".
//   - left/right move the cursor within its grid row.
//   - up/down first REVEAL, then MOVE: while the cursored field still extends
//     past the visible window in the pressed direction, the keypress scrolls
//     the viewport by one line and leaves the cursor where it is; once the
//     field's edge in that direction is on screen, the next press moves the
//     cursor to the neighbouring field.
//
// bean bt-8d35 (PO-Entscheidung 3) took the ARROW KEYS off the four
// directional cases and gave them back to plain viewport scrolling
// (adjustBoxFormScroll, bt-ze10's state) -- bt-ze10's "walk a long Body to its
// last line with the keyboard alone" criterion is served by that scroll again,
// so the reveal-then-move half is no longer on a key. The four directional
// cases are kept rather than torn out ("kein Voll-Rueckbau", bt-8d35): they
// are the grid geometry that boxFormFieldOrder's (row, col) table encodes,
// still covered by box_nav_field_test.go one rung below the keymap.
//
// The wheel path (wheelMove, mouse.go) is deliberately NOT routed through
// here: a wheel is a viewport gesture with no focus semantics.
//
// Writes both cursor and scroll ownership (boxFormCursorBean/
// boxFormScrollBean) so the derived per-bean resets stay consistent.
func (m model) boxFormNav(b *item.Item, dir string) model {
	if b == nil {
		return m
	}
	accW, height := boxFormPaneMetrics(m, b)
	blocks := boxFormBlocks(m.idx, b, accW, -1) // cursor never changes line counts
	total := 0
	for _, blk := range blocks {
		total += lineCount(blk)
	}

	cur := boxFormEffectiveCursor(m, b)
	off := boxFormEffectiveScroll(m, b)

	switch dir {
	case "next", "prev":
		d := 1
		if dir == "prev" {
			d = -1
		}
		n := len(boxFormFieldOrder)
		cur = ((cur+d)%n + n) % n // wrap at BOTH ends (bt-8d35)
	case "left", "right":
		cur = boxFormMoveField(cur, dir)
	case "up", "down":
		start, h := boxFormFieldSpan(blocks, cur)
		switch {
		case dir == "down" && start+h > off+height:
			off++ // reveal the rest of the focused field first
		case dir == "up" && start < off:
			off--
		default:
			cur = boxFormMoveField(cur, dir)
		}
	default:
		return m
	}

	start, h := boxFormFieldSpan(blocks, cur)
	m.boxFormScroll = clampBoxFormScroll(boxFormScrollIntoView(off, start, h, height), total, height)
	m.boxFormScrollBean = b.ID
	m.boxFormCursor = cur
	m.boxFormCursorBean = b.ID
	return m
}

// boxFormFieldRect (Slice A, bt-f0y9 "feld-verankertes Inline-Dropdown", D09
// revidiert) returns a field's absolute SCREEN rect (x, y, w, h) exactly as
// detailBoxForm renders it today -- read-only, reproduces existing geometry,
// does not change or call into any render path. Verified render-grounded
// (box_nav_field_test.go's TestBoxFormFieldRectMatchesRenderedBoxCorners
// locates the real "╭" corner in m.View() and asserts the returned x/y lands
// exactly on it, at 80 AND 120 columns) rather than derived purely from
// reading the existing click-side arithmetic -- see the ERRATUM paragraph
// below for why.
//
//   - Row span (Y): boxFormFieldSpan (box_detail_form.go) minus
//     boxFormEffectiveScroll (mouse.go), against clickPaneGeometry's originY
//     corrected for the box-form filter bar the SAME way detailBoxFormClickRow
//     is (B6, filterBarHeight, ONLY viewBrowseRepo ever shows the bar). This
//     half matches detailBoxFormClickRow's own contentRow = clickRow +
//     offset exactly, just inverted (screenRow = contentRow - offset) --
//     confirmed render-grounded, no deviation here.
//   - Pane accW: boxFormPaneMetrics (mouse.go) already branches on
//     m.fullscreen == fullscreenDetail (bt-s90e) -- reused verbatim so this
//     function and boxFormPaneMetrics' own callers (boxFormNav,
//     boxFormScrollBounds) can never disagree about the pane's width.
//   - Column span within the field's grid row: gridColWidths (box_detail_
//     form.go) + detailBoxFormGap, walked forward (colX = Σwidths[0:col] +
//     col*gap) -- gridColAt (mouse.go) is the SAME table's inverse direction.
//   - Pane left edge (X): NOT originX+lw (detailBoxFormClickRow's own click
//     boundary, mouse.go) -- see ERRATUM below. Instead: Split mode (m.
//     fullscreen != fullscreenDetail), the Detail pane's own left border sits
//     at originX+lw+2 (the Tree/Ready/Flat pane occupies lw+2 columns:
//     border+lw content+border, JoinHorizontal butts the Detail pane directly
//     against it with no gap), and detailBoxForm's own content -- the
//     accW-wide string dropdownBox/panelBox draw, which begins with its own
//     "╭"/"│" -- starts ONE column further right of that, at originX+lw+3.
//     fullscreenDetail (bt-s90e): there is no preceding Tree/Ready pane at
//     all (renderFullscreenBody's ONE full-width pane), so content starts at
//     originX+1.
//
// ERRATUM (Grounding-Kultur, checked rather than assumed): the Grounding's
// own "trivial from gridColWidths" note implicitly assumed the pane's
// content starts at originX+lw (Split) / originX (fullscreen) -- the SAME
// boundary detailBoxFormClickRow's own `msg.X < originX+lw` gate uses. A
// render-grounded probe (ansi-stripped m.View(), a real "╭" search) found
// this is off by +3 columns in Split mode (+1 in fullscreen) from where
// detailBoxForm ACTUALLY draws: detailBoxFormClickRow's own X arithmetic
// only ever gates a coarse "which pane" decision and, past that, buckets via
// gridColAt against ~14-15-cell-wide columns -- a 3-cell systematic
// undershoot never crosses a bucket boundary for any EXISTING test (all
// click near a hotkey badge, comfortably mid-box) and so was never caught.
// boxFormFieldRect exists specifically to hand Slice C/B a rect a popup gets
// ANCHORED at, where a 3-column miss would be a visibly wrong overlay
// position -- so this function computes the geometrically exact origin
// (render-verified) rather than copying detailBoxFormClickRow's imprecise
// one. The two do not silently diverge in BEHAVIOR: a click at this rect's
// own center still round-trips through detailBoxFormClickRow to the SAME
// field (verified in the same test, both at 80 and 120 columns) precisely
// because the pre-existing 3-cell slack keeps every center-click inside the
// correct bucket. detailBoxFormClickRow itself is NOT touched by this slice
// (see design-spec.md discussion under bt-f0y9 "## Notes for Slice C" for
// whether its own boundary should ever be tightened).
//
// No overflow/clip handling here (a field scrolled fully out of the pane, or
// a rect a caller would place a popup at that runs off the bottom of the
// terminal) -- that is Slice B's placeOverlayAt concern, this function only
// ever answers "where does detailBoxForm actually draw this box right now",
// on- or off-screen alike. ok=false for a nil bean or an out-of-range field
// index -- callers get a clean "no rect" rather than a garbage zero rect.
//
// boxFormPaneContentLeft and boxFormPopupChromeFloor (below) factor the
// pane-origin geometry this function's own X/Y math depends on out into
// reusable, view/fullscreen-agnostic helpers -- placeValueMenuOverlay (Slice
// C B02 fix, box_menu_value.go) needs the SAME left-edge column to clamp a
// left-aligned popup against, and the SAME "row right after the divider" as
// a hard floor no popup Y may ever cross (Slice C B01 fix) -- both MUST
// reuse this function's own geometry rather than a second, independently
// maintained copy (Golden-Rule-Drift-Schutz).

// boxFormPaneContentLeft returns the column detailBoxForm's boxes actually
// start drawing at (the SAME value boxFormFieldRect's own col=0 resolves
// to, see the ERRATUM above): Split mode butts the Detail pane directly
// against the Tree/Ready/Flat pane's own lw+2-wide box (border+lw
// content+border, no gap), fullscreenDetail has no preceding pane at all.
func boxFormPaneContentLeft(m model) int {
	ww, hh := m.width, m.height
	if ww <= 0 {
		ww = 80
	}
	if hh <= 0 {
		hh = 24
	}
	innerW := ww - 2

	var head, localKeys string
	if m.view == viewReady {
		head, localKeys = m.readyChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	_, lw, _, originX, _ := clickPaneGeometry(ww, hh, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	paneLeftBorder := originX
	if m.fullscreen != fullscreenDetail {
		paneLeftBorder = originX + lw + 2
	}
	return paneLeftBorder + 1 // one column past the pane's own left border/corner
}

// boxFormPopupChromeFloor returns the LOWEST screen row (Y) any box-form
// popup may ever occupy without overwriting the app's own outer border,
// breadcrumb, or header divider (bt-f0y9 Slice C bug B01: a naive flip-up
// could compute a Y small enough to splice the popup's own middle rows
// straight over the app title). clickPaneGeometry's own originY (mouse.go)
// already documents itself as "outer top border(1) + head(lines) +
// divider(1) + the PANE'S OWN top border(1)" -- one row PAST the divider is
// exactly originY minus that last "pane's own top border" term, i.e.
// originY-1. View/fullscreen/filter-bar agnostic: none of those ever add
// rows ABOVE this point (the filter bar sits AT this
// row and below -- it is body chrome, not app chrome, safe to place a popup
// over), only below it.
func boxFormPopupChromeFloor(m model) int {
	ww, hh := m.width, m.height
	if ww <= 0 {
		ww = 80
	}
	if hh <= 0 {
		hh = 24
	}
	innerW := ww - 2

	var head, localKeys string
	if m.view == viewReady {
		head, localKeys = m.readyChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	_, _, _, _, originY := clickPaneGeometry(ww, hh, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	return originY - 1
}

func boxFormFieldRect(m model, b *item.Item, field int) (x, y, w, h int, ok bool) {
	if b == nil || field < 0 || field >= len(boxFormFieldOrder) {
		return 0, 0, 0, 0, false
	}

	ww, hh := m.width, m.height
	if ww <= 0 {
		ww = 80
	}
	if hh <= 0 {
		hh = 24
	}
	innerW := ww - 2

	var head, localKeys string
	if m.view == viewReady {
		head, localKeys = m.readyChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	_, _, _, _, originY := clickPaneGeometry(ww, hh, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	if m.view != viewReady {
		originY += filterBarHeight // B6: same correction detailBoxFormClickRow applies (mouse.go)
	}

	accW, _ := boxFormPaneMetrics(m, b) // fullscreenDetail-aware accW (bt-s90e)
	contentX := boxFormPaneContentLeft(m)

	f := boxFormFieldOrder[field]
	widths := gridColWidths(boxFormRowCols(f.row), accW)
	if f.col < 0 || f.col >= len(widths) {
		return 0, 0, 0, 0, false
	}
	colX := 0
	for i := 0; i < f.col; i++ {
		colX += widths[i] + detailBoxFormGap
	}

	blocks := boxFormBlocks(m.idx, b, accW, -1) // cursor never changes line counts
	start, height := boxFormFieldSpan(blocks, field)
	rowY := start - boxFormEffectiveScroll(m, b)

	return contentX + colX, originY + rowY, widths[f.col], height, true
}

// boxFormActivateCursor fires the cursored field's own editor -- enter's
// half of Ziel 4, dispatched through the SAME activateBoxFormTarget
// (mouse.go) a click on that box takes, so keyboard and mouse can never open
// different things for the same field.
func (m model) boxFormActivateCursor(b *item.Item) (tea.Model, tea.Cmd) {
	if b == nil {
		return m, nil
	}
	cur := boxFormEffectiveCursor(m, b)
	return m.activateBoxFormTarget(b, boxFormFieldOrder[cur].target)
}
