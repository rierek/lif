package tui

// box_detail_form.go — detailBoxForm renders a bean's SCALAR fields as
// jira-style stacked/gridded titled boxes (../docs/topics/jira-style-experiment/
// design-spec.md D01/D03, mockup §6.3), reusing S1's dropdownBox primitive
// (box_dropdown.go). ADDITIVE ONLY (S2): a new pure renderer, not wired into
// the live accordion view (view_detail_bean.go's metaSectionBody/accordion
// stay untouched) -- Body/Relations/History (multi-line panels) are a later
// slice, out of scope here.
//
// S2e (D12, review B1): the responsive perRow collapse (3-up/2-up/1-up) is
// gone. The scalar grid is now FIXED: Row A = Status|Type|Priority (3 cols),
// Row B = Parent|Tags (2 cols) -- always, at any width. Columns shrink with
// the pane (dropdownBox floors at 8 cells) but never collapse to 1-up.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// detailBoxFormGap is the single space column gap between adjacent boxes in
// a row (design-spec.md §6.3 mockup: boxes separated by one blank column).
const detailBoxFormGap = 1

// scalarCell is one column of a gridRow: a dropdownBox's label/value/hotkey.
type scalarCell struct {
	label, value, hotkey string
}

// gridColWidths computes the n column widths gridRow itself lays out at
// `width` cells (S2e Change 3, review B1's own integer-division-remainder
// rule, factored out here as of S6/B6): SINGLE SOURCE for gridRow's own box-
// width loop AND mouse.go's box-form column hit-test (detailBoxFormClickRow)
// -- a click can never drift from the actual rendered column boundaries
// (Golden-Rule-Drift-Schutz, detailClickRow's own doc comment precedent,
// mouse.go).
func gridColWidths(n, width int) []int {
	if n <= 0 {
		return nil
	}
	gap := detailBoxFormGap
	avail := width - (n-1)*gap
	base := avail / n
	rem := avail % n

	widths := make([]int, n)
	for i := 0; i < n; i++ {
		w := base
		if i < rem {
			w++
		}
		widths[i] = w
	}
	return widths
}

// gridRow renders the given scalar cells as one horizontal row occupying
// EXACTLY `width` cells (S2e Change 3, review B1): len(cells) columns, one-
// space gaps between them, and the integer-division remainder spread across
// the first columns so the widths sum to width. Each cell is a dropdownBox
// at its column width.
// focusedCol (bean bt-1o4g) is the column whose box renders with
// dropdownBox's focused=true Mauve frame, or -1 for "no field cursor in this
// row" -- the ONLY behavioral difference; widths/geometry are untouched, so
// the field cursor can never shift the layout out from under the mouse
// hit-test (boxFormFieldAt, box_nav_field.go).
func gridRow(cells []scalarCell, width, focusedCol int) string {
	n := len(cells)
	if n == 0 {
		return ""
	}
	widths := gridColWidths(n, width)

	boxes := make([]string, n)
	for i, c := range cells {
		boxes[i] = dropdownBox(c.label, c.value, c.hotkey, widths[i], i == focusedCol)
	}

	gapCol := " \n \n "
	joined := boxes[0]
	for _, box := range boxes[1:] {
		joined = lipgloss.JoinHorizontal(lipgloss.Top, joined, gapCol, box)
	}
	return joined
}

// detailBoxForm renders a bean's scalar fields as jira-style titled boxes
// (design-spec.md D01/D03, mockup §6.3; D12: fixed grid, no responsive
// collapse): a full-width Title box, then a FIXED Status|Type|Priority row
// (3 cols) and a FIXED Parent|Tags row (2 cols), then full-width Body/
// Relations/History panels (S2c, design-spec.md D01/D04) built via panelBox
// (box_panel.go) reusing view_detail_bean.go's own content renderers. idx is
// needed to resolve Relations (parent/children/blocking). width = inner pane
// width in cells.
func detailBoxForm(idx *data.Index, b *data.Bean, width, cursor int) string {
	return strings.Join(boxFormBlocks(idx, b, width, cursor), "\n")
}

// boxFormBlocks renders detailBoxForm's six layout rows SEPARATELY (Title /
// scalars A / scalars B / Body / Relations / History, the boxFormRow*
// constants in box_nav_field.go) instead of returning the joined string
// directly. detailBoxForm (above) just joins them with "\n" -- byte-identical
// output -- but the field cursor needs each row's own LINE SPAN to scroll a
// focused field into view (boxFormFieldSpan below, bean bt-1o4g), and deriving
// that from the same slice the render is built from is the only way the two
// can never disagree about where a field sits.
//
// cursor is an index into boxFormFieldOrder (box_nav_field.go), or -1 for no
// cursor at all -- which is the state EVERY pre-bt-1o4g call site is in
// (unfocused Detail pane, fullscreen, the flag-on goldens), so the -1 render
// is exactly the previous output, every box focused=false.
func boxFormBlocks(idx *data.Index, b *data.Bean, width, cursor int) []string {
	focusRow, focusCol := -1, -1
	if cursor >= 0 && cursor < len(boxFormFieldOrder) {
		focusRow = boxFormFieldOrder[cursor].row
		focusCol = boxFormFieldOrder[cursor].col
	}
	colIn := func(row int) int {
		if row == focusRow {
			return focusCol
		}
		return -1
	}
	on := func(row int) bool { return row == focusRow }

	// bean bt-oox1 (#1): the bean ID rides in the Title box's frame label,
	// "slim next to the title" as the PO asked -- the frame label is the one
	// slot that costs the title text no columns at all, and boxTopBorder
	// already clamps it, so a narrow pane degrades instead of overflowing.
	//
	// The FULL ID, deliberately. bt-pl5p shortened the LIST rows
	// (sproutling-eq67 -> eq67) because the repo slug repeats on every row
	// there; that same decision kept the complete, copyable ID readable in
	// the DETAIL pane, which shows exactly one bean and pays nothing for the
	// slug. Do not unify the two (TestRelationRowKeepsFullIDInDetail and
	// TestDetailBoxFormShowsBeanIDNextToTitle guard the two halves).
	titleLabel := "Title"
	if b.ID != "" {
		titleLabel = "Title · " + b.ID
	}
	title := dropdownBox(titleLabel, b.Title, "e", width, on(boxFormRowTitle))

	priority := b.Priority
	if priority == "" {
		priority = "normal"
	}
	parent := b.Parent
	if parent == "" {
		parent = theme.Dim.Render("—")
	}
	tags := tagsInline(b.Tags)
	if tags == "" {
		tags = theme.Dim.Render("—")
	}

	rowA := gridRow([]scalarCell{
		{"Status", theme.StatusStyle(b.Status).Render(b.Status), "s"},
		{"Type", theme.TypeStyle(b.Type).Render(b.Type), "o"},
		{"Priority", theme.Priority(priority), "u"},
	}, width, colIn(boxFormRowScalarsA))
	rowB := gridRow([]scalarCell{
		{"Parent", parent, "a"},
		{"Tags", tags, "t"},
	}, width, colIn(boxFormRowScalarsB))

	relationsBody, _, _ := relationsSectionBody(idx, b, width-4, false, 0)

	return []string{
		title,
		rowA,
		rowB,
		// bean bt-oox1 (#4): the Body panel is the ONE box here whose height
		// follows its content, so its bottom border can scroll out of the
		// pane -- its (e) badge rides in the TOP border instead.
		panelBoxTopHotkey("Body", bodySectionBody(b, width-4), "e", width, on(boxFormRowBody)),
		panelBox("Relations", relationsBody, "r", width, on(boxFormRowRelations)),
		panelBox("History", historieSectionBody(b, width-4), "", width, on(boxFormRowHistory)),
	}
}

// lineCount reports how many rendered lines s occupies -- boxFormBlocks'
// blocks are joined with "\n", so a block's line count IS its height in the
// joined form (no lipgloss.Height indirection needed, and no ANSI-width
// pitfalls: this counts newlines, not cells).
func lineCount(s string) int { return strings.Count(s, "\n") + 1 }

// boxFormFieldSpan returns the [start, start+height) line range the field at
// index `field` (boxFormFieldOrder, box_nav_field.go) occupies inside the
// joined detailBoxForm string -- what boxFormNav scrolls into view. Fields
// sharing a row (the scalar grid) share that row's span: they are rendered
// side by side, so their vertical extent is identical by construction.
func boxFormFieldSpan(blocks []string, field int) (start, height int) {
	if field < 0 || field >= len(boxFormFieldOrder) {
		return 0, 0
	}
	row := boxFormFieldOrder[field].row
	for i := 0; i < row && i < len(blocks); i++ {
		start += lineCount(blocks[i])
	}
	if row >= len(blocks) {
		return start, 0
	}
	return start, lineCount(blocks[row])
}

// clampBoxFormScroll bounds a box-form scroll offset to [0, max(0, total-
// height)] (bean bt-ze10, epic bt-vy1q F1) -- the SAME clamp algebra
// scrollView (view.go) applies internally when it windows content for
// render, duplicated here rather than shared because mouse.go's
// adjustBoxFormScroll needs the CLAMPED OFFSET ITSELF to store back onto
// model.boxFormScroll, not scrollView's windowed string + indicator return
// shape. Render-time windowing (renderDetailBox, view_browse_repo.go)
// still goes through scrollView directly, which self-clamps regardless --
// this function only keeps the STORED offset (and the tests asserting it)
// sane between keystrokes/wheel ticks.
func clampBoxFormScroll(offset, total, height int) int {
	maxOff := total - height
	if maxOff < 0 {
		maxOff = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}
