package tui

// box_filter_facets_test.go — TDD coverage for Facetten-Filter `f`/`X` (E2
// Task 4, bean bt-9ldr, design-spec.md §6 US-05): buildFilterItems (fixed
// Status/Type/Priority + dynamic Tag facets), the I01 copy-on-write
// convention applied to the new facet maps (mirrors
// TestSetExpandedDoesNotMutateSharedMapAcrossModelCopies, update_test.go),
// the floating filter-menu key machine (keyFilterMenu), and the combined
// search+facet AND predicate (beanMatches). Reuses fixtureBeans/
// fixtureModel/step/keyMsg/runeMsg/nodeIDs/equalStrings from update_test.go
// (same package).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- buildFilterItems / tagFilterOptions ---

// TestBuildFilterItemsCoversStatusTypePriorityAndDynamicTags guards the
// fixed facet counts (5 status values, 5 beans types, 5 priorities -- the
// full beans enums, design-spec.md §4) plus the dynamic, deduped Tag facet
// collected from the currently loaded beans.
func TestBuildFilterItemsCoversStatusTypePriorityAndDynamicTags(t *testing.T) {
	beans := []*item.Item{
		{ID: "b1", Title: "One", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"backend", "urgent"}},
		{ID: "b2", Title: "Two", Status: "in-progress", Type: "bug", Priority: "high", Tags: []string{"backend"}},
	}
	m := fixtureModel(t, beans)
	items := m.buildFilterItems()

	count := func(facet string) int {
		n := 0
		for _, it := range items {
			if it.facet == facet {
				n++
			}
		}
		return n
	}
	if n := count("status"); n != 5 {
		t.Errorf("status facet count = %d, want 5", n)
	}
	if n := count("type"); n != 5 {
		t.Errorf("type facet count = %d, want 5", n)
	}
	if n := count("priority"); n != 5 {
		t.Errorf("priority facet count = %d, want 5", n)
	}

	gotTags := map[string]bool{}
	tagCount := 0
	for _, it := range items {
		if it.facet == "tag" {
			gotTags[it.value] = true
			tagCount++
		}
	}
	if tagCount != 2 {
		t.Errorf("tag facet count = %d, want 2 (deduped backend+urgent), items=%v", tagCount, items)
	}
	if !gotTags["backend"] || !gotTags["urgent"] {
		t.Errorf("tag facets = %v, want backend+urgent present", gotTags)
	}
}

// TestTagFilterOptionsDeterministicAcrossCalls guards the fix for the plan's
// own ERRATUM (box_filter_facets.go doc comment): idx.ByID is a Go map with
// no defined iteration order, so a naive "range idx.ByID" walk would make
// the tag facet order flicker between calls. Repeated calls against the SAME
// model must return byte-identical order.
func TestTagFilterOptionsDeterministicAcrossCalls(t *testing.T) {
	beans := make([]*item.Item, 0, 10)
	for i := 0; i < 10; i++ {
		beans = append(beans, &item.Item{
			ID: string(rune('a' + i)), Title: "Bean", Status: "todo", Type: "task", Priority: "normal",
			Tags: []string{string(rune('z' - i))},
		})
	}
	m := fixtureModel(t, beans)

	first := m.tagFilterOptions()
	for i := 0; i < 5; i++ {
		got := m.tagFilterOptions()
		if !equalStrings(got, first) {
			t.Fatalf("tagFilterOptions() call %d = %v, want %v (must be deterministic)", i, got, first)
		}
	}
}

// --- I01 copy-on-write (facet maps) ---

// TestFacetToggleUsesCopyOnWrite mirrors
// TestSetExpandedDoesNotMutateSharedMapAcrossModelCopies (update_test.go):
// toggling a facet on one model copy must not mutate a sibling copy's map --
// same I01 regression shape, applied to the new filter facet fields.
func TestFacetToggleUsesCopyOnWrite(t *testing.T) {
	base := model{filterStatus: map[string]bool{}}
	copy1 := base // struct copy -- map HEADER copied, backing array still shared pre-fix
	copy2 := copy1.toggleFacet(ffItem{facet: "status", value: "todo"})

	if copy1.filterStatus["todo"] {
		t.Error("copy1.filterStatus was mutated by copy2's toggleFacet call -- facet maps must be copy-on-write (I01)")
	}
	if !copy2.filterStatus["todo"] {
		t.Error("copy2.filterStatus should carry the new toggle state")
	}
}

// --- filterActive / clearFacets ---

func TestFilterActiveWhenAnyFacetMapNonEmpty(t *testing.T) {
	m := model{}
	if m.filterActive() {
		t.Fatal("filterActive() true with every facet map empty/nil")
	}
	m.filterTag = map[string]bool{"backend": true}
	if !m.filterActive() {
		t.Fatal("filterActive() false with a non-empty facet map")
	}
}

// TestFilterClearXResetsAllFourFacetMaps guards X as a direct top-level
// reset (menu closed) -- design-spec.md: "X leert alle Facetten".
func TestFilterClearXResetsAllFourFacetMaps(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.filterStatus = map[string]bool{"todo": true}
	m.filterType = map[string]bool{"task": true}
	m.filterPriority = map[string]bool{"high": true}
	m.filterTag = map[string]bool{"backend": true}

	m = step(t, m, runeMsg('X'))

	if m.filterActive() {
		t.Fatalf("filterActive() still true after X: status=%v type=%v priority=%v tag=%v",
			m.filterStatus, m.filterType, m.filterPriority, m.filterTag)
	}
}

// --- Filter fields as focus targets (beans-f4q4, U3) ---

// filterTaggedModel is a fixtureModel whose beans carry tags, at 100x30.
func filterTaggedModel(t *testing.T) model {
	t.Helper()
	beans := []*item.Item{
		{ID: "a", Title: "Alpha", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"backend"}},
		{ID: "b", Title: "Beta", Status: "in-progress", Type: "bug", Priority: "high", Tags: []string{"web"}},
	}
	m := fixtureModel(t, beans)
	return step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
}

// TestFFromRoadmapSwitchesToFilterLayoutAndFocusesType: AC5.
func TestFFromRoadmapSwitchesToFilterLayoutAndFocusesType(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = m.setLayout(layoutRoadmap)
	m = step(t, m, runeMsg('f'))
	if m.layout != layoutFilter {
		t.Fatalf("layout after f from roadmap = %v, want layoutFilter", m.layout)
	}
	if !m.filterFocus || filterFields[m.filterField] != "type" {
		t.Fatalf("focus after f: filterFocus=%v field=%q, want Type", m.filterFocus, filterFields[m.filterField])
	}
	if m.overlay != overlayNone {
		t.Fatal("f opened an overlay; it must only move the focus")
	}
}

// TestFFocusesTypeFromEveryLayout: AC4 + SC-02. In the layouts with a filter
// bar f keeps the layout and jumps back to Type from any other field.
func TestFFocusesTypeFromEveryLayout(t *testing.T) {
	cases := []struct {
		from, want layoutMode
	}{
		{layoutRoadmap, layoutFilter},
		{layoutFilter, layoutFilter},
		{layoutFilterDetail, layoutFilterDetail},
		{layoutDetail, layoutFilter},
	}
	for _, tc := range cases {
		m := fixtureModel(t, fixtureBeans()).setLayout(tc.from)
		if tc.from.hasFilter() {
			m = m.focusFilterField("tag") // already inside the bar, on another field
		}
		m = step(t, m, runeMsg('f'))
		if m.layout != tc.want {
			t.Errorf("from layout %v: layout after f = %v, want %v", tc.from, m.layout, tc.want)
		}
		if !m.filterFocus || m.detailFocus || filterFields[m.filterField] != "type" {
			t.Errorf("from layout %v: filterFocus=%v detailFocus=%v field=%q, want focus on Type",
				tc.from, m.filterFocus, m.detailFocus, filterFields[m.filterField])
		}
	}
}

// TestFilterFieldsAreFocusTargetsInBarOrder: AC1. left/right visit every
// field of the bar, in filterFields order, and wrap.
func TestFilterFieldsAreFocusTargetsInBarOrder(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	var seen []string
	for range filterFields {
		seen = append(seen, filterFields[m.filterField])
		m = step(t, m, keyMsg(tea.KeyRight))
	}
	if !equalStrings(seen, filterFields) {
		t.Fatalf("right visits %v, want %v", seen, filterFields)
	}
	if filterFields[m.filterField] != "type" {
		t.Fatalf("right from the last field = %q, want wrap to type", filterFields[m.filterField])
	}
	m = step(t, m, keyMsg(tea.KeyLeft))
	if filterFields[m.filterField] != "archive" {
		t.Fatalf("left from the first field = %q, want wrap to archive", filterFields[m.filterField])
	}
	// The bar renders the five fields in the same order.
	bar := stripHint(m.filterBar(98))
	last := -1
	for _, label := range []string{"Type", "Status", "Priority", "Tags", "Archive"} {
		i := strings.Index(bar, label)
		if i < last {
			t.Fatalf("bar label %q out of order in:\n%s", label, bar)
		}
		last = i
	}
}

// TestEnterOnFilterFieldOpensValueMenuOfThatFacet: AC2.
func TestEnterOnFilterFieldOpensValueMenuOfThatFacet(t *testing.T) {
	for i, facet := range filterFields {
		m := step(t, filterTaggedModel(t), runeMsg('f'))
		for range i {
			m = step(t, m, keyMsg(tea.KeyRight))
		}
		m = step(t, m, keyMsg(tea.KeyEnter))
		if m.overlay != overlayFilterMenu {
			t.Fatalf("field %q: enter did not open the value menu", facet)
		}
		start, end := m.filterMenuRows()
		if end <= start || m.filterItems[m.filterMenu.cursor].facet != facet {
			t.Fatalf("field %q: menu rows [%d,%d) cursor on %q", facet, start, end, m.filterItems[m.filterMenu.cursor].facet)
		}
	}
}

// TestFilterMenuWithoutValuesDoesNotOpen: a Tags field in a store without
// tags has nothing to pick.
func TestFilterMenuWithoutValuesDoesNotOpen(t *testing.T) {
	m := step(t, fixtureModel(t, fixtureBeans()), runeMsg('f'))
	m = m.focusFilterField("tag")
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.overlay != overlayNone {
		t.Fatal("enter on the Tags field without tags opened a menu")
	}
}

// TestFilterMenuToggleAppliesToListAtOnce: AC3 + SC-01. Every facet filters
// the list by the menu alone, no other overlay involved.
func TestFilterMenuToggleAppliesToListAtOnce(t *testing.T) {
	cases := []struct {
		field   string
		value   string
		visible []string // item IDs left in the list
	}{
		{"type", "bug", []string{"b"}},
		{"status", "todo", []string{"a"}},
		{"priority", "high", []string{"b"}},
		{"tag", "backend", []string{"a"}},
	}
	for _, tc := range cases {
		m := step(t, filterTaggedModel(t), runeMsg('f'))
		m = m.focusFilterField(tc.field)
		m = step(t, m, keyMsg(tea.KeyEnter))
		for m.filterItems[m.filterMenu.cursor].value != tc.value {
			m = step(t, m, keyMsg(tea.KeyDown))
		}
		m = step(t, m, keyMsg(tea.KeySpace))
		if !m.filterActive() {
			t.Fatalf("%s=%s: toggling in the menu did not set the facet", tc.field, tc.value)
		}
		var got []string
		for _, n := range m.visibleNodes() {
			if n.bean != nil {
				got = append(got, n.bean.ID)
			}
		}
		if !equalStrings(got, tc.visible) {
			t.Fatalf("%s=%s: list shows %v, want %v", tc.field, tc.value, got, tc.visible)
		}
		if m.overlay != overlayFilterMenu {
			t.Fatalf("%s=%s: toggle closed the menu", tc.field, tc.value)
		}
	}

	// Archive: a completed item shows once the toggle is on.
	beans := []*item.Item{
		{ID: "a", Title: "Alpha", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "z", Title: "Zed", Status: "completed", Type: "task", Priority: "normal"},
	}
	m := step(t, fixtureModel(t, beans), runeMsg('f'))
	m = m.focusFilterField("archive")
	m = step(t, m, keyMsg(tea.KeyEnter))
	m = step(t, m, keyMsg(tea.KeySpace))
	if !m.showArchived {
		t.Fatal("space on the Archive value did not show archived items")
	}
}

// TestFilterMenuEnterAppliesAndKeepsMenuOpen: enter toggles like space (the
// menu is a multi-select), and a second enter un-applies.
func TestFilterMenuEnterAppliesAndKeepsMenuOpen(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEnter))
	it := m.filterItems[m.filterMenu.cursor]
	m = step(t, m, keyMsg(tea.KeyEnter))
	if !m.facetOn(it) || m.overlay != overlayFilterMenu {
		t.Fatal("enter in the menu must apply the value and keep the menu open")
	}
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.facetOn(it) {
		t.Fatal("a second enter must un-apply the value")
	}
}

// TestFilterMenuArrowsStayWithinFieldValues: up/down never cross into
// another facet's rows.
func TestFilterMenuArrowsStayWithinFieldValues(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEnter)) // Type
	start, end := m.filterMenuRows()
	for range end + 5 {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if m.filterMenu.cursor != end-1 {
		t.Fatalf("cursor after overshooting down = %d, want %d", m.filterMenu.cursor, end-1)
	}
	for range end + 5 {
		m = step(t, m, keyMsg(tea.KeyUp))
	}
	if m.filterMenu.cursor != start {
		t.Fatalf("cursor after overshooting up = %d, want %d", m.filterMenu.cursor, start)
	}
}

// TestFilterMenuEscClosesWithoutClearingAndKeepsFocus: esc closes the menu,
// the filter stays, the focus stays on the field; a second esc leaves the
// bar for the list.
func TestFilterMenuEscClosesWithoutClearingAndKeepsFocus(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEnter))
	it := m.filterItems[m.filterMenu.cursor]
	m = step(t, m, keyMsg(tea.KeySpace))

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.overlay != overlayNone || !m.filterFocus || !m.facetOn(it) {
		t.Fatalf("after esc: overlay=%v filterFocus=%v facetOn=%v, want closed menu, field focus, facet kept", m.overlay, m.filterFocus, m.facetOn(it))
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.filterFocus || !m.facetOn(it) {
		t.Fatalf("second esc: filterFocus=%v facetOn=%v, want focus back on the list, facet kept", m.filterFocus, m.facetOn(it))
	}
}

// TestFilterClearKeepsLayoutAndClearsAllFacets: AC7, from the focused field
// and from inside the menu (the menu stays open).
func TestFilterClearKeepsLayoutAndClearsAllFacets(t *testing.T) {
	set := func(m model) model {
		m.filterStatus = map[string]bool{"todo": true}
		m.filterType = map[string]bool{"task": true}
		m.filterPriority = map[string]bool{"high": true}
		m.filterTag = map[string]bool{"backend": true}
		return m
	}
	for _, layout := range []layoutMode{layoutFilter, layoutFilterDetail} {
		m := set(filterTaggedModel(t).setLayout(layout))
		m = step(t, m, runeMsg('f'))
		m = step(t, m, runeMsg('X'))
		if m.filterActive() || m.layout != layout || !m.filterFocus {
			t.Fatalf("X on a field in layout %v: active=%v layout=%v filterFocus=%v", layout, m.filterActive(), m.layout, m.filterFocus)
		}

		m = set(m)
		m = step(t, m, keyMsg(tea.KeyEnter))
		m = step(t, m, runeMsg('X'))
		if m.filterActive() || m.layout != layout || m.overlay != overlayFilterMenu {
			t.Fatalf("X in the menu in layout %v: active=%v layout=%v overlay=%v", layout, m.filterActive(), m.layout, m.overlay)
		}
	}
}

// TestFilterFieldEscReturnsToList: esc leaves the bar and tab steps to the
// next field (the focus ring, TestFocusRingTabVisitsEveryTargetAndWraps);
// keys the field does not own never reach the list underneath.
func TestFilterFieldEscReturnsToList(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.filterFocus || m.detailFocus {
		t.Fatalf("esc: filterFocus=%v detailFocus=%v, want focus on the list", m.filterFocus, m.detailFocus)
	}

	m = step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyTab))
	if got := focusAt(m); got != "filter:"+filterFields[1] {
		t.Fatalf("tab on the first field: focus %q, want the second field", got)
	}

	m = step(t, filterTaggedModel(t), runeMsg('f'))
	before := m.cursorID
	m = step(t, m, runeMsg('s')) // would open the status menu on the list
	if m.overlay != overlayNone || m.cursorID != before {
		t.Fatalf("a list key leaked through the focused filter field: overlay=%v cursor %q -> %q", m.overlay, before, m.cursorID)
	}
}

// TestLayoutWithoutFilterBarDropsFilterFocus: v into a layout that hides the
// bar must not leave the focus on a field that is not shown.
func TestLayoutWithoutFilterBarDropsFilterFocus(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = m.setLayout(layoutDetail)
	if m.filterFocus {
		t.Fatal("filterFocus survived a layout without a filter bar")
	}
}

// TestFilterMenuTabDoesNotLeakToGlobalFocusToggle: the open menu captures
// tab, so it never reaches the global region toggle.
func TestFilterMenuTabDoesNotLeakToGlobalFocusToggle(t *testing.T) {
	m := step(t, filterTaggedModel(t), runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEnter))
	m = step(t, m, keyMsg(tea.KeyTab))
	if m.detailFocus || m.overlay != overlayFilterMenu {
		t.Fatalf("tab in the open menu: detailFocus=%v overlay=%v", m.detailFocus, m.overlay)
	}
}

// TestFilterMenuScrollsLongTagFacet: the Tags field can hold many values;
// the popup shows a window that always contains the cursor row.
func TestFilterMenuScrollsLongTagFacet(t *testing.T) {
	var beans []*item.Item
	for i := range 40 {
		beans = append(beans, &item.Item{ID: fmt.Sprintf("t%02d", i), Title: "T", Status: "todo", Type: "task", Priority: "normal", Tags: []string{fmt.Sprintf("tag%02d", i)}})
	}
	m := step(t, fixtureModel(t, beans), tea.WindowSizeMsg{Width: 100, Height: 24})
	m = step(t, m, runeMsg('f'))
	m = m.focusFilterField("tag")
	m = step(t, m, keyMsg(tea.KeyEnter))
	for range 30 {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	first, last := m.filterMenuWindow()
	if last-first != m.filterMenuMaxRows() {
		t.Fatalf("window shows %d rows, want the cap %d", last-first, m.filterMenuMaxRows())
	}
	if m.filterMenu.cursor < first || m.filterMenu.cursor >= last {
		t.Fatalf("cursor %d outside window [%d,%d)", m.filterMenu.cursor, first, last)
	}
	if box := stripHint(m.filterMenuBox()); !strings.Contains(box, m.filterItems[m.filterMenu.cursor].label) {
		t.Fatalf("popup does not show the cursored value:\n%s", box)
	}
}

// TestFilterMenuPopupSitsBelowItsFieldAndClicksToggle: the popup is drawn
// where filterMenuRect says (directly under the focused field), a click on a
// value row toggles it and a click outside closes the menu.
func TestFilterMenuPopupSitsBelowItsFieldAndClicksToggle(t *testing.T) {
	for _, width := range []int{80, 120} {
		m := step(t, filterTaggedModel(t), tea.WindowSizeMsg{Width: width, Height: 30})
		m = step(t, m, runeMsg('f'))
		m = m.focusFilterField("archive") // rightmost field: exercises the right-edge shift
		m = step(t, m, keyMsg(tea.KeyEnter))

		x, y, w, _ := m.filterMenuRect()
		_, barY, _ := filterBarOrigin(m)
		if y != barY+filterBarHeight {
			t.Fatalf("width %d: popup top %d, want directly below the bar (%d)", width, y, barY+filterBarHeight)
		}
		if x+w > width {
			t.Fatalf("width %d: popup [%d,%d) passes the right edge", width, x, x+w)
		}
		lines := screenLines(m)
		if row := lines[y+filterMenuFirstRow]; !strings.Contains(row, "Show archived") {
			t.Fatalf("width %d: popup first value row %d reads %q, want the Archive value", width, y+filterMenuFirstRow, row)
		}

		click := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: x + 3, Y: y + filterMenuFirstRow}
		tm, _ := m.handleMouse(click)
		m2 := tm.(model)
		if !m2.showArchived || m2.overlay != overlayFilterMenu {
			t.Fatalf("width %d: click on the value row: showArchived=%v overlay=%v", width, m2.showArchived, m2.overlay)
		}
		tm, _ = m2.handleMouse(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 0, Y: 0})
		if tm.(model).overlay != overlayNone {
			t.Fatalf("width %d: click outside did not close the menu", width)
		}
	}
}

// --- Combined search + facet AND (beanMatches / visibleNodes) ---

// TestTreeCombinesSearchAndFacetsWithAnd guards that an active search AND an
// active facet combine with AND semantics: a bean matching the search text
// but excluded by the facet must not be visible; one matching both is.
func TestTreeCombinesSearchAndFacetsWithAnd(t *testing.T) {
	beans := []*item.Item{
		{ID: "a", Title: "Alpha Task", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "b", Title: "Alpha Bug", Status: "todo", Type: "bug", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.expanded[unscheduledRootID] = true        // parentless items sit under the Unscheduled root
	m = setSearchQuery(m, "alpha")              // matches both by title
	m.filterType = map[string]bool{"bug": true} // excludes "a"

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{unscheduledRootID, "b"}) {
		t.Fatalf("combined search+facet nodes = %v, want [unscheduled b] (AND-combined)", got)
	}
}

// TestBeanMatchesFacetsEmptyMapsImposeNoConstraint guards the base case:
// every facet map empty -> everything matches.
func TestBeanMatchesFacetsEmptyMapsImposeNoConstraint(t *testing.T) {
	m := model{}
	b := &item.Item{ID: "x", Status: "todo", Type: "task", Priority: "normal"}
	if !m.beanMatchesFacets(b) {
		t.Fatal("beanMatchesFacets with all facet maps empty must match everything")
	}
}

// TestBeanMatchesFacetsTagRequiresAnyOverlap guards the Tag facet's OR-
// within-facet semantics (a bean matches if ANY of its tags is selected).
func TestBeanMatchesFacetsTagRequiresAnyOverlap(t *testing.T) {
	m := model{filterTag: map[string]bool{"backend": true}}
	hit := &item.Item{ID: "a", Tags: []string{"frontend", "backend"}}
	miss := &item.Item{ID: "b", Tags: []string{"frontend"}}
	if !m.beanMatchesFacets(hit) {
		t.Error("bean with a matching tag among several must match the tag facet")
	}
	if m.beanMatchesFacets(miss) {
		t.Error("bean with no matching tag must not match the tag facet")
	}
}

// --- wideModalWidth (T5, bean bt-4mo9, B06) ---

// TestWideModalWidthScalesWithTerminal guards the actual fix behind B06: PO
// verbatim "Aber die Breite muss viel weiter werden" -- unlike
// clampModalWidth (a pure shrink-only clamp of a FIXED preference, never
// grows), wideModalWidth scales UP with the terminal (~85%), with a floor of
// 60 (never narrower than the old fixed 48-ish picker width) and a ceiling
// of termW-4 (clampModalWidth's own 2-column margin convention), plus the
// absolute floor of 24 clampModalWidth itself already established.
func TestWideModalWidthScalesWithTerminal(t *testing.T) {
	cases := []struct {
		name  string
		termW int
		want  int
	}{
		{"wide terminal, plain 85%", 120, 102},      // 120*85/100=102, well under both floor/ceiling
		{"very wide terminal, plain 85%", 200, 170}, // 200*85/100=170
		{"floor-60 boundary: 85% would be 59, floor wins", 70, 60},
		{"ceiling termW-4: floor(60) exceeds it, ceiling wins", 50, 46},
		{"absolute floor 24: ceiling itself is below 24", 20, 24},
		{"zero terminal width: no ceiling guard applies, floor 60 wins", 0, 60},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := wideModalWidth(c.termW); got != c.want {
				t.Errorf("wideModalWidth(%d) = %d, want %d", c.termW, got, c.want)
			}
		})
	}
}

// TestWideModalWidthNeverNarrowerThanOldFixedPicker guards the B06 PO-
// complaint directly against a regression: the picker's old fixed width
// (clampModalWidth(48, termW)) must never exceed wideModalWidth's own floor
// on any terminal wide enough to hold it -- otherwise the "fix" could
// silently render NARROWER than before on some width.
func TestWideModalWidthNeverNarrowerThanOldFixedPicker(t *testing.T) {
	for _, termW := range []int{48, 52, 64, 80, 100, 120, 200} {
		old := clampModalWidth(48, termW)
		got := wideModalWidth(termW)
		if got < old {
			t.Errorf("wideModalWidth(%d) = %d, narrower than the old clampModalWidth(48,%d) = %d", termW, got, termW, old)
		}
	}
}

// --- I02 (optional, E2-T3-Review finding carried into bean bt-9ldr): empty
// filter-match render must not panic. ---

// TestEmptyFacetMatchViewRendersWithoutPanic guards that a facet filter
// matching zero beans renders cleanly -- the Tree pane simply shows no rows
// below its (still-rendered) search/filter head row. The real assertion is
// implicit: if View()/visibleNodes() panicked, this test would fail with a
// stack trace rather than a plain t.Fatal. (PF-10, bean bt-uyzf: the Tree
// pane no longer renders a "Tree" title line -- the Breadcrumb's "Browse"
// title carries the view identity now, and the search head row is the
// pane's own still-rendered signal that its frame survived.)
func TestEmptyFacetMatchViewRendersWithoutPanic(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 100, 30
	m.filterStatus = map[string]bool{"scrapped": true} // no fixture bean has this status

	if n := len(m.visibleNodes()); n != 0 {
		t.Fatalf("visibleNodes() = %d nodes, want 0 (no fixture bean is scrapped)", n)
	}
	out := m.View()
	if !strings.Contains(out, "─ Search ") {
		t.Error("View() with zero filtered matches lost the Tree pane's search head row entirely")
	}
}
