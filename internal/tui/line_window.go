package tui

// line_window.go — infrastructure for VARIABLE-HEIGHT list rows (bean bt-f68z,
// PO-Befund #13: "Titel brechen nicht um, sondern werden abgeschnitten").
//
// Before this file every list row in the app was exactly ONE terminal line, so
// "row index" and "screen line" were the same number and windowStart/
// windowAround (view_roadmap_repo.go) could window ELEMENTS directly. With
// hanging-indent title wrapping that identity is gone: one bean can occupy two
// or three screen lines. Two things break silently when that happens, and both
// have a precedent in this repo:
//
//   - a window that counts ELEMENTS cannot bound HEIGHT. parentPickerRowBudget
//     capped list elements instead of terminal lines and the overlay ran off
//     the bottom of an 80x24 screen the moment a title wrapped (found in
//     bt-a3a8's smoke, recorded in ../docs/topics/jira-style-experiment/STATE.md).
//   - a click-to-row mapping that divides screen rows by 1 lands on the wrong
//     element as soon as any earlier element is taller than one line.
//
// blockWindow therefore takes per-element LINE BLOCKS and returns both the
// windowed lines AND a line->element mapping, so the renderer and the mouse
// hit-test consume the SAME structure from the SAME call shape (the
// Golden-Rule-Drift-Schutz windowStart's own doc comment establishes: the two
// halves cannot drift because there is only one algorithm).
//
// Cursor movement is deliberately NOT touched by any of this: treeCursorMove/
// listState.move step over ELEMENTS (bean indices) and must keep doing so
// (bean bt-f68z acceptance: "up/down springen ueber beans, nicht ueber
// Zeilen"). Only rendering and hit-testing are line-aware.

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// maxRowLines caps how many terminal lines a single list row may occupy.
//
// Implementer decision (bean bt-f68z): wrapping is unbounded in principle --
// a 300-character title in a 24-column pane would otherwise produce a dozen
// lines and push every other bean off the screen, turning a readability fix
// into a usability regression. The PO's own sketch in the bean shows at most
// three lines per bean, so three is the cap; the last line ends in the normal
// "…" ellipsis (truncate, view.go) exactly like the pre-wrap behaviour, so
// nothing is ever silently lost -- it is still visibly truncated, just three
// lines later.
const maxRowLines = 3

// hangingWrap lays out one list row as `head` followed by `title`, wrapping
// the title at `width` cells with a HANGING INDENT: continuation lines are
// padded to headWidth so they start in the title's own column, not at the
// left edge (bean bt-f68z, PO sketch).
//
//	i B eq67 God-Delete/Purge crasht: ORM nullt
//	         child-FK statt Cascade (alert_configs
//
// head carries ANSI styling (status glyph, type icon, Sapphire ID); headWidth
// is its VISIBLE width, which the caller knows exactly and lipgloss.Width
// would only re-derive. title is plain text.
//
// Falls back to a single truncated line when the title budget is too small to
// wrap meaningfully (narrow pane / long ID) -- below ~minWrapBudget cells a
// "wrap" degenerates into one word per line, which is worse than the clamp it
// replaces.
func hangingWrap(head string, headWidth int, title string, width int) []string {
	budget := width - headWidth
	const minWrapBudget = 8
	if width <= 0 {
		return []string{""}
	}
	if budget < minWrapBudget {
		return []string{truncate(head+title, width)}
	}
	wrapped := strings.Split(ansi.Wrap(title, budget, " -/"), "\n")
	if len(wrapped) > maxRowLines {
		wrapped = wrapped[:maxRowLines]
		// The tail was dropped -- mark the last kept line so the truncation
		// stays VISIBLE (same "…" convention truncate() uses everywhere else).
		wrapped[maxRowLines-1] = truncate(wrapped[maxRowLines-1]+" …", budget)
	}
	lines := make([]string, 0, len(wrapped))
	pad := strings.Repeat(" ", headWidth)
	for i, w := range wrapped {
		if i == 0 {
			lines = append(lines, truncate(head+w, width))
			continue
		}
		lines = append(lines, truncate(pad+w, width))
	}
	return lines
}

// blockWindow windows per-element line blocks to at most `height` terminal
// lines, keeping the CURSOR element's block visible, and returns the flattened
// lines together with a parallel line->element index mapping.
//
// Deterministic in (blocks, height, cursor) alone -- no remembered scroll
// offset -- which is exactly what keeps windowStart jitter-free and is the
// property the mouse hit-test depends on: treeClickRow recomputes this from
// the same inputs and MUST get the same window the renderer produced.
//
// Placement mirrors windowStart's "centered when there is room, clamped at
// both edges" semantics, translated from elements to lines: grow upwards until
// roughly half the height is used above the cursor, then grow downwards to
// fill, then grow upwards again to take up any slack left at the bottom edge
// (the clamp case -- cursor on the last element).
//
// A cursor block taller than the whole window is still emitted; the result is
// simply cut to `height` lines, so the cursor's FIRST line always stays
// visible.
func blockWindow(blocks [][]string, height, cursor int) (rows []string, rowElem []int) {
	if height <= 0 || len(blocks) == 0 {
		return nil, nil
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(blocks)-1 {
		cursor = len(blocks) - 1
	}

	start, end := cursor, cursor+1
	used := len(blocks[cursor])

	// 1. upwards, until the cursor block sits roughly in the middle
	above := 0
	for start > 0 && above+len(blocks[start-1]) <= height/2 && used+len(blocks[start-1]) <= height {
		start--
		above += len(blocks[start])
		used += len(blocks[start])
	}
	// 2. downwards, filling the remaining budget
	for end < len(blocks) && used+len(blocks[end]) <= height {
		used += len(blocks[end])
		end++
	}
	// 3. upwards again -- takes up the slack when step 2 ran out of elements
	//    (cursor at/near the end of the list, the bottom-clamp case)
	for start > 0 && used+len(blocks[start-1]) <= height {
		start--
		used += len(blocks[start])
	}

	for i := start; i < end; i++ {
		for _, line := range blocks[i] {
			if len(rows) >= height {
				return rows, rowElem
			}
			rows = append(rows, line)
			rowElem = append(rowElem, i)
		}
	}
	return rows, rowElem
}

// blockWindowElemAt maps a windowed screen row (0-based, relative to the first
// windowed content line) back to its element index -- the mouse hit-test half
// of blockWindow. ok=false for a row past the last rendered line, which is the
// "clicked the empty space below the list" case every *ClickRow already
// rejects.
func blockWindowElemAt(rowElem []int, row int) (int, bool) {
	if row < 0 || row >= len(rowElem) {
		return 0, false
	}
	return rowElem[row], true
}
