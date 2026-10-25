package tui

// box_form_page.go — the box-form Detail pane's PAGE indicator (bean bt-adkn,
// jira-style-ui experiment, epic bt-vy1q). pgup/pgdn (handleKey's global
// paging checkpoint, update.go -- focus-independent, Rework B1) page the
// viewport by one full screen through adjustBoxFormScroll (mouse.go); this file
// turns the resulting scroll offset into "page X of N" and renders it as hell/
// dunkel dots. Rework B3 (PO "Sticky Body-Kopfzeile"): the dots ride in the
// BODY panel's own title row (boxTopBorderBadges, box_dropdown.go), which
// renderDetailBox pins to the top of the viewport while the body is paged,
// so the indicator stays visible WHILE paging (Akzeptanz) -- no longer on the
// outer pane frame.
//
// A page == the pane's visible line budget (the same `height` boxFormScroll
// Bounds clamps against and pgdn steps by), so a dot boundary lines up exactly
// with a page-down stop -- the indicator and the paging can never disagree.

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// Page-indicator glyphs: hell (current page) vs. dunkel (the rest). Filled/
// hollow circles read as "here / not here" independent of color, so the
// indicator still parses on a mono terminal where the theme greys collapse.
const (
	pageDotFilled = "●"
	pageDotEmpty  = "○"
)

// boxFormPageIndex maps a scroll offset to (current page index, page count) for
// a box-form of `total` lines windowed to `height` lines. count is
// ceil(total/height); the current page is offset/height, EXCEPT at the clamped
// ceiling (offset >= total-height) it is always the LAST page -- otherwise a
// total that is not a whole multiple of height would leave the last dot
// unreachable (paging clamps at total-height, which floor-divides to the
// second-to-last page). Returns count 1 (=> caller shows no indicator) when it
// all fits (total <= height).
func boxFormPageIndex(total, height, offset int) (page, count int) {
	if height < 1 {
		height = 1
	}
	count = (total + height - 1) / height // ceil
	if count < 1 {
		count = 1
	}
	maxOff := total - height
	if maxOff < 0 {
		maxOff = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOff {
		offset = maxOff
	}
	if offset >= maxOff {
		return count - 1, count
	}
	page = offset / height
	if page > count-1 {
		page = count - 1
	}
	return page, count
}

// boxFormPageBadge renders the page indicator for page/count within maxWidth
// cells: hell dot for the current page, dunkel for the rest (theme greys only,
// bean bt-adkn -- Subtext = hell, Surface = dunkel; no hex literals). Empty
// when count <= 1 (it fits, no indicator wanted) or maxWidth cannot hold even
// the compact form. A very long body (more pages than cells) falls back from
// the dot row to a themed "n/N" so the indicator stays visible rather than
// being dropped.
func boxFormPageBadge(page, count, maxWidth int) string {
	if count <= 1 || maxWidth < 1 {
		return ""
	}
	hell := lipgloss.NewStyle().Foreground(theme.Subtext)
	dunkel := lipgloss.NewStyle().Foreground(theme.Surface)

	if count <= maxWidth { // one cell per dot
		var b strings.Builder
		for i := 0; i < count; i++ {
			if i == page {
				b.WriteString(hell.Render(pageDotFilled))
			} else {
				b.WriteString(dunkel.Render(pageDotEmpty))
			}
		}
		return b.String()
	}

	// FIXED width (bean bt-adkn US-02, 2nd PO-Reject: "(e) springt weiterhin"):
	// pad the page number to count's digit width so the badge is the SAME width
	// on every page (" 1/27", "12/27", "100/102"). A variable-width "n/N" grows
	// as the page number crosses a digit boundary mid-paging, which either
	// shifts the (e) badge or -- at a narrow width near the fit threshold --
	// drops the indicator entirely on the wider pages. A constant width does
	// neither: it fits (or not) identically for every page.
	countStr := fmt.Sprintf("%d", count)
	txt := fmt.Sprintf("%*d/%s", len(countStr), page+1, countStr)
	if lipgloss.Width(txt) > maxWidth {
		return ""
	}
	return hell.Render(txt)
}
