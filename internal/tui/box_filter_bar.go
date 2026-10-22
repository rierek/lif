package tui

// box_filter_bar.go — the persistent jira-style filter chip row (S3, design-
// spec.md D02/D08): four titled boxes (Type/Status/Priority/Tags) across the
// Browse view's full inner width, reusing S1's dropdownBox via S2e's gridRow
// (box_dropdown.go/box_detail_form.go) — the SAME widget the Detail-Pane's
// scalar grid already uses, so the chip row and the detail boxes read as one
// consistent widget language (D01's own rationale). Experiment-only, gated
// at the ONE call site (view_browse_repo.go).
//
// D07/D08: the four chips carry NO per-chip hotkey badge — `f` still opens
// the existing floating filter overlay (box_filter_facets.go's
// treeFilterBox) to actually SET a facet; this bar is a read-only status
// strip, not yet mouse-/key-addressable per chip (a later slice per the
// design spec).

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// filterBarActive is the D08 "gesetzter Filter = Peach" salience style —
// local to this file since no shared theme.PeachStyle exists yet (only
// theme.Chevron pins Peach to a specific glyph use, box_dropdown.go).
var filterBarActive = lipgloss.NewStyle().Foreground(theme.Peach)

// filterBarHeight is filterBar's fixed rendered height: gridRow's four
// dropdownBox cells are always exactly 3 lines tall (top border/value/bottom
// border, box_dropdown.go's dropdownBox), so the bar itself never varies --
// S6 (bean tbd, B6 "filter-bar click offset") needs this exact number too,
// so it is a named const here rather than re-deriving lipgloss.Height(
// m.filterBar(...)) at every click-geometry call site (Golden-Rule-Drift-
// Schutz, mirrors headerBlockLines' own const precedent, mouse.go).
const filterBarHeight = 3

// filterBar renders the four-chip Type/Status/Priority/Tags row at exactly
// `width` cells (gridRow's own contract). A facet with active values shows
// them (joinFilterKeys, box_filter_facets.go) in theme.Peach — the shared
// D08 "gesetzter Filter = Peach" salience convention; an empty facet shows
// "(any)" in theme.Muted (Hint-grey).
func (m model) filterBar(width int) string {
	chip := func(mp map[string]bool) string {
		if len(mp) > 0 {
			return filterBarActive.Render(joinFilterKeys(mp))
		}
		return theme.Muted.Render("(any)")
	}

	cells := []scalarCell{
		{"Type", chip(m.filterType), ""},
		{"Status", chip(m.filterStatus), ""},
		{"Priority", chip(m.filterPriority), ""},
		{"Tags", chip(m.filterTag), ""},
	}
	return gridRow(cells, width, -1) // -1: the filter bar has no field cursor (bt-1o4g)
}
