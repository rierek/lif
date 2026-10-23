package tui

// mouse_filter_bar_test.go — TDD coverage for bt-wtqs (epic bt-vy1q): a
// click on one of the persistent filter-strip's four chips (Type/Status/
// Priority/Tags, box_filter_bar.go) must open the SAME
// shared facet-filter menu `f` opens (openFilterMenu, update.go), seeded to
// the clicked chip's own facet -- NOT openValueMenu (that mutates the
// FOCUSED bean, a completely different action; the Ursache-Tabelle in the
// bean explicitly flags this as the one wrong-action risk). Render-grounded
// throughout (screenLines/filterBarClickAt), mirrors mouse_test.go's/
// mouse_boxform_test.go's own "never hand-derive the click coordinate"
// discipline.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// filterBarClickAt finds substr's first occurrence WITHIN the filter strip's
// own screen-row span ([originY, originY+filterBarHeight), the RAW
// clickPaneGeometry originY BEFORE treeClickRow's/detailBoxFormClickRow's own
// +filterBarHeight correction -- that correction is what makes THOSE two
// callers skip past the strip; here we want the opposite, a click INSIDE it)
// -- the mirror image of mouse_boxform_test.go's boxFormClickAt, which
// deliberately SKIPS these same rows to dodge the chip labels' own text
// ("Status"/"Type"/"Priority"/"Tags" also appear as box-form Detail-Pane box
// labels a few rows further down, box_detail_form.go's rowA/rowB -- searching
// the WHOLE screen for these words would risk matching the wrong widget).
func filterBarClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	head, localKeys := m.browseRepoChrome(m.width - 2)
	_, _, _, originX, rawOriginY := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, layoutFilterDetail)
	// clickPaneGeometry's raw originY assumes content starts 1 row after a
	// bordered box's own top border sitting directly below the divider --
	// the bordered box IS the filter bar itself
	// (view_browse_repo.go's content build order: head+div, THEN
	// filterBarRow, THEN the body), so the strip's own top-border row is
	// originY-1, not originY (verified via a real View() render: rawOriginY
	// lands on the strip's MIDDLE (value) row, one past its top border).
	stripTop := rawOriginY - 1
	for y, l := range screenLines(m) {
		if y < stripTop || y >= stripTop+filterBarHeight {
			continue // outside the strip's own 3 rows
		}
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		col := ansi.StringWidth(l[:i])
		return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: col + originX, Y: y}
	}
	t.Fatalf("substr %q not found in the filter strip's own rows of the rendered View()", substr)
	return tea.MouseMsg{}
}

// boxFormStripModel builds a fixtureModel with at the given
// terminal width (bt-wtqs Akzeptanz: geometry pinned at 80 AND 120 columns --
// Maus-Off-by-one bugs in this repo (bt-hd42/bt-vpvu) lived exactly at
// boundary widths, so both must be exercised, not just a comfortable 100).
// Uses fixtureBeansTagged (box_picker_tag_test.go), NOT plain fixtureBeans --
// the Tags chip's own facet ("tag") only appears in filterFacetOrder's
// output at all when at least one loaded bean carries a tag
// (tagFilterOptions, box_filter_facets.go); plain fixtureBeans has none, which
// silently made openFilterMenuAt's facet lookup a no-op for the Tags case
// (caught live: TestFilterBarChipClickOpensFilterMenuSeededToFacet/Tags first
// failed with "seeded facet = status" -- the DEFAULT tab, not an error --
// exactly the kind of silent wrong-default this task's own bean warns
// against for the Ziel-Funktion).
func boxFormStripModel(t *testing.T, width int) model {
	t.Helper()
	m := fixtureModel(t, fixtureBeansTagged())
	m = step(t, m, tea.WindowSizeMsg{Width: width, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "tk-2"
	return m
}

// TestFilterBarChipClickOpensFilterMenuSeededToFacet is the table-driven RED
// case over all 4 chip columns (bt-wtqs Akzeptanz): each chip's OWN facet
// must become the active tab (m.filterTab), not just "some tab" -- proves
// the click-X column bucketing (gridColAt/gridColWidths) resolves each chip
// independently, not merely "any strip click opens tab 0".
func TestFilterBarChipClickOpensFilterMenuSeededToFacet(t *testing.T) {
	cases := []struct {
		label     string
		wantFacet string
	}{
		{"Type", "type"},
		{"Status", "status"},
		{"Priority", "priority"},
		{"Tags", "tag"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			m := boxFormStripModel(t, 100)

			msg := filterBarClickAt(t, m, tc.label)
			tm, _ := m.handleMouse(msg)
			m2, ok := tm.(model)
			if !ok {
				t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
			}
			if !m2.filterOpen {
				t.Fatalf("click on %q chip: filterOpen = false, want true (Filter-Overlay must open)", tc.label)
			}
			facets := filterFacetOrder(m2.filterItems)
			if m2.filterTab < 0 || m2.filterTab >= len(facets) {
				t.Fatalf("click on %q chip: filterTab = %d out of range %v", tc.label, m2.filterTab, facets)
			}
			if got := facets[m2.filterTab]; got != tc.wantFacet {
				t.Fatalf("click on %q chip: seeded facet = %q, want %q", tc.label, got, tc.wantFacet)
			}
			if m2.overlay != overlayNone {
				t.Fatalf("click on %q chip: overlay = %v, want overlayNone (openFilterMenu does not touch m.overlay -- this must NOT be openValueMenu)", tc.label, m2.overlay)
			}
		})
	}
}

// TestFilterBarChipClickGeometryAt80And120Cols pins the SAME "Status" chip
// click at the two boundary widths the bean's Akzeptanz calls out by name
// (bt-hd42/bt-vpvu precedent: off-by-one bugs surfaced exactly at these
// widths, never at a comfortable 100).
func TestFilterBarChipClickGeometryAt80And120Cols(t *testing.T) {
	for _, width := range []int{80, 120} {
		t.Run(fmt.Sprintf("w%d", width), func(t *testing.T) {
			m := boxFormStripModel(t, width)

			msg := filterBarClickAt(t, m, "Status")
			tm, _ := m.handleMouse(msg)
			m2, ok := tm.(model)
			if !ok {
				t.Fatalf("width %d: handleMouse(click) did not return a model, got %T", width, tm)
			}
			if !m2.filterOpen {
				t.Fatalf("width %d: filterOpen = false, want true", width)
			}
			facets := filterFacetOrder(m2.filterItems)
			if m2.filterTab < 0 || m2.filterTab >= len(facets) || facets[m2.filterTab] != "status" {
				t.Fatalf("width %d: seeded facet = %v (tab %d), want status", width, facets, m2.filterTab)
			}
		})
	}
}

// TestFilterBarClickRegressionTreeClickStillSelectsRow guards the regression
// half of bt-wtqs' Akzeptanz: a click that does NOT land on the strip (i.e.
// on an actual Tree row, further down) must behave exactly as before --
// mirrors TestBoxFormFilterBarOffsetTreeClickSelectsCorrectRow
// (mouse_boxform_test.go) verbatim, re-asserted here as this task's own
// regression guard for the NEW filterBarHit check placed ahead of the
// m.view switch in handleMouse.
func TestFilterBarClickRegressionTreeClickStillSelectsRow(t *testing.T) {
	m := boxFormStripModel(t, 100)
	m.cursorID = "ms-1"

	msg := treeClickAt(t, m, "Task Two")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.cursorID != "tk-2" {
		t.Fatalf("tree click below the strip: cursorID = %q, want tk-2 (strip hit-test must not swallow this click)", m2.cursorID)
	}
	if m2.filterOpen {
		t.Fatal("tree click below the strip: filterOpen = true, want false")
	}
}
