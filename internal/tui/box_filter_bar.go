package tui

// box_filter_bar.go — the persistent jira-style filter row: five titled boxes
// (Type/Status/Priority/Tags/Archive, filterFields) across the Browse view's
// full inner width, reusing S1's dropdownBox via S2e's gridRow
// (box_dropdown.go/box_detail_form.go) — the SAME widget the Detail-Pane's
// scalar grid already uses. Each box is a focus target: `f` jumps to the
// first (Type), left/right move between them, enter opens the field's value
// menu (box_filter_facets.go).

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// filterBarActive is the D08 "gesetzter Filter = Peach" salience style —
// local to this file since no shared theme.PeachStyle exists yet (only
// theme.Chevron pins Peach to a specific glyph use, box_dropdown.go).
func filterBarActive() lipgloss.Style { return lipgloss.NewStyle().Foreground(theme.Peach) }

// filterBarHeight is filterBar's fixed rendered height: gridRow's
// dropdownBox cells are always exactly 3 lines tall (top border/value/bottom
// border, box_dropdown.go's dropdownBox), so the bar itself never varies --
// S6 (bean tbd, B6 "filter-bar click offset") needs this exact number too,
// so it is a named const here rather than re-deriving lipgloss.Height(
// m.filterBar(...)) at every click-geometry call site (Golden-Rule-Drift-
// Schutz, mirrors headerBlockLines' own const precedent, mouse.go).
const filterBarHeight = 3

// filterBar renders the Type/Status/Priority/Tags/Archive row at exactly
// `width` cells (gridRow's own contract). A facet with active values shows
// them (joinFilterKeys, box_filter_facets.go) in theme.Peach — the shared
// D08 "gesetzter Filter = Peach" salience convention; an empty facet shows
// "(any)" in theme.Muted (Hint-grey). Archive shows "shown" (Peach) or
// "hidden". While the focus is in the bar, the focused field has the focus
// frame.
func (m model) filterBar(width int) string {
	chip := func(mp map[string]bool) string {
		if len(mp) > 0 {
			return filterBarActive().Render(joinFilterKeys(mp))
		}
		return theme.Muted.Render("(any)")
	}
	archive := theme.Muted.Render("hidden")
	if m.showArchived {
		archive = filterBarActive().Render("shown")
	}

	cells := []scalarCell{
		{"Type", chip(m.filterType), ""},
		{"Status", chip(m.filterStatus), ""},
		{"Priority", chip(m.filterPriority), ""},
		{"Tags", chip(m.filterTag), ""},
		{"Archive", archive, ""},
	}
	focused := -1
	if m.filterFocus {
		focused = m.filterField
	}
	return gridRow(cells, width, focused)
}
