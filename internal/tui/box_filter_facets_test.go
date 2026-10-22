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

// --- Filter-menu key machine (keyFilterMenu / `f` open) ---

// TestFilterMenuUpDownMovesCursorSpaceTogglesRow guards menu navigation +
// checkbox toggling end-to-end through Update.
func TestFilterMenuUpDownMovesCursorSpaceTogglesRow(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))
	if !m.filterOpen {
		t.Fatal("f did not open the filter menu")
	}
	if len(m.filterItems) == 0 {
		t.Fatal("filterItems empty after opening the filter menu")
	}
	if m.filterMenu.cursor != 0 {
		t.Fatalf("filterMenu.cursor after open = %d, want 0", m.filterMenu.cursor)
	}

	m = step(t, m, keyMsg(tea.KeyDown))
	if m.filterMenu.cursor != 1 {
		t.Fatalf("filterMenu.cursor after down = %d, want 1", m.filterMenu.cursor)
	}

	it := m.filterItems[1]
	m = step(t, m, keyMsg(tea.KeySpace))
	if !m.facetOn(it) {
		t.Fatalf("facet %+v not toggled on after space", it)
	}
	m = step(t, m, keyMsg(tea.KeySpace))
	if m.facetOn(it) {
		t.Fatalf("facet %+v still on after second space toggle", it)
	}

	// "x" is the Toggle binding's second key (alongside space) -- verify it
	// toggles too, distinct from the uppercase "X" FilterClear binding.
	m = step(t, m, runeMsg('x'))
	if !m.facetOn(it) {
		t.Fatalf("facet %+v not toggled on by lowercase x", it)
	}
}

// TestFilterMenuEscAndFCloseWithoutClearing guards that both
// close keys (esc/f) leave the toggled facet state untouched -- only
// X (a separate binding) clears.
func TestFilterMenuEscAndFCloseWithoutClearing(t *testing.T) {
	closeKeys := []tea.KeyMsg{keyMsg(tea.KeyEsc), runeMsg('f')}
	for _, closeKey := range closeKeys {
		m := fixtureModel(t, fixtureBeans())
		m = step(t, m, runeMsg('f'))
		it := m.filterItems[0]
		m = step(t, m, keyMsg(tea.KeySpace))
		if !m.facetOn(it) {
			t.Fatalf("setup: facet %+v not toggled on", it)
		}

		m = step(t, m, closeKey)
		if m.filterOpen {
			t.Fatalf("close key %v did not close the filter menu", closeKey)
		}
		if !m.facetOn(it) {
			t.Fatalf("close key %v cleared the toggled facet -- closing must not clear filter state", closeKey)
		}
	}
}

// TestFilterMenuXClearsWithoutClosing guards devd parity: X inside the open
// menu clears every facet but leaves the menu open (view_browse_project.go's
// keyTreeFilter FilterClear case does not touch treeFilterOpen).
func TestFilterMenuXClearsWithoutClosing(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))
	it := m.filterItems[0]
	m = step(t, m, keyMsg(tea.KeySpace))
	if !m.facetOn(it) {
		t.Fatal("setup: facet not toggled on")
	}

	m = step(t, m, runeMsg('X'))
	if !m.filterOpen {
		t.Fatal("X inside the menu closed it -- devd parity requires it to stay open")
	}
	if m.filterActive() {
		t.Fatal("X inside the menu did not clear the facet")
	}
}

// --- Querformat Tab-Kategorien (bt-2p9m, PO-Review E11 Runde 2,
// epic-E12-plan.md Item 7) ---

// TestFilterMenuOpensWithFirstTabActiveAndFirstElementSelected guards the
// PO-Klickpfad's first step verbatim: "f für Filter, erster tab aktiviert"
// -- opening the menu selects tab 0 (Status, facetHead order) with its
// first row already under the cursor.
func TestFilterMenuOpensWithFirstTabActiveAndFirstElementSelected(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))

	if !m.filterOpen {
		t.Fatal("f did not open the filter menu")
	}
	if m.filterTab != 0 {
		t.Fatalf("filterTab after open = %d, want 0 (first tab active)", m.filterTab)
	}
	if m.filterMenu.cursor != 0 {
		t.Fatalf("filterMenu.cursor after open = %d, want 0 (first element pre-selected)", m.filterMenu.cursor)
	}
	if m.filterItems[0].facet != "status" {
		t.Fatalf("filterItems[0].facet = %q, want %q (Status is the first facetHead tab)", m.filterItems[0].facet, "status")
	}
}

// TestFilterMenuTabSwitchesCategoryAndSelectsFirstElement guards the
// PO-Klickpfad's second+third step: "mit tab/shift-tab andere Filter
// wählen" then "im fokussierten Tab ist das erste Element immer aktiv".
func TestFilterMenuTabSwitchesCategoryAndSelectsFirstElement(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))

	m = step(t, m, keyMsg(tea.KeyTab))
	if m.filterTab != 1 {
		t.Fatalf("filterTab after tab = %d, want 1 (Type)", m.filterTab)
	}
	wantFacet := m.filterItems[m.filterMenu.cursor].facet
	if wantFacet != "type" {
		t.Fatalf("active facet after tab = %q, want %q", wantFacet, "type")
	}
	start, _ := filterFacetRange(m.filterItems, "type")
	if m.filterMenu.cursor != start {
		t.Fatalf("filterMenu.cursor after tab = %d, want %d (Type's first element)", m.filterMenu.cursor, start)
	}

	// shift+tab from tab 1 (Type) goes back to tab 0 (Status).
	m = step(t, m, keyMsg(tea.KeyShiftTab))
	if m.filterTab != 0 {
		t.Fatalf("filterTab after shift+tab = %d, want 0 (Status)", m.filterTab)
	}

	// shift+tab from tab 0 wraps around to the LAST tab (Archive).
	m = step(t, m, keyMsg(tea.KeyShiftTab))
	facets := filterFacetOrder(m.filterItems)
	lastIdx := len(facets) - 1
	if m.filterTab != lastIdx {
		t.Fatalf("filterTab after wrap-around shift+tab = %d, want %d (last tab)", m.filterTab, lastIdx)
	}
	if facets[m.filterTab] != "archive" {
		t.Fatalf("wrapped-to facet = %q, want %q", facets[m.filterTab], "archive")
	}
}

// TestFilterMenuArrowsStayWithinActiveTabBounds guards the PO-Klickpfad's
// own explicit scope limit: "Pfeil-rauf/runter direkt die Auswahl
// navigieren" INSIDE the focused tab only -- up/down must never cross into
// a neighboring facet's rows.
func TestFilterMenuArrowsStayWithinActiveTabBounds(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))

	_, end := filterFacetRange(m.filterItems, "status")

	// Down past the last Status row must clamp at the last Status row, not
	// spill into Type.
	for i := 0; i < end+5; i++ {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if got := m.filterItems[m.filterMenu.cursor].facet; got != "status" {
		t.Fatalf("cursor facet after overshooting down = %q, want %q (must not cross tabs)", got, "status")
	}
	if m.filterMenu.cursor != end-1 {
		t.Fatalf("filterMenu.cursor after overshooting down = %d, want %d (clamped at last Status row)", m.filterMenu.cursor, end-1)
	}

	// Up past the first Status row must clamp at 0, not go negative.
	for i := 0; i < 10; i++ {
		m = step(t, m, keyMsg(tea.KeyUp))
	}
	if m.filterMenu.cursor != 0 {
		t.Fatalf("filterMenu.cursor after overshooting up = %d, want 0 (clamped at first Status row)", m.filterMenu.cursor)
	}
}

// TestFilterMenuTabDoesNotLeakToGlobalFocusToggle guards the plan's own
// verified claim (epic-E12-plan.md Item 7 "Kein Tastenkonflikt"):
// handleKey checks m.filterOpen (Full-Capture) BEFORE the global
// FocusIn/FocusOut case, so tab/shift+tab inside the open filter menu must
// change the CATEGORY tab, never m.detailFocus.
func TestFilterMenuTabDoesNotLeakToGlobalFocusToggle(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.detailFocus = false
	m = step(t, m, runeMsg('f'))

	m = step(t, m, keyMsg(tea.KeyTab))
	if m.detailFocus {
		t.Fatal("tab inside the open filter menu leaked into the global FocusIn toggle (m.detailFocus)")
	}
	if !m.filterOpen {
		t.Fatal("tab inside the open filter menu unexpectedly closed it")
	}

	m = step(t, m, keyMsg(tea.KeyShiftTab))
	if m.detailFocus {
		t.Fatal("shift+tab inside the open filter menu leaked into the global FocusOut toggle (m.detailFocus)")
	}
}

// TestFilterMenuSpaceTogglesCorrectFacetAfterTabSwitch guards that toggling
// (space/x, unchanged toggleFacet) after switching categories still writes
// to the RIGHT facet map -- a regression here would mean the Querformat
// rendering changed but the underlying facet semantics silently broke.
func TestFilterMenuSpaceTogglesCorrectFacetAfterTabSwitch(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyTab)) // -> Type tab, cursor on Type's first row

	it := m.filterItems[m.filterMenu.cursor]
	if it.facet != "type" {
		t.Fatalf("setup: active facet = %q, want %q", it.facet, "type")
	}
	m = step(t, m, keyMsg(tea.KeySpace))
	if !m.facetOn(it) {
		t.Fatalf("space after tab-switch did not toggle %+v on", it)
	}
	if len(m.filterStatus) != 0 {
		t.Fatalf("toggling the Type row leaked into filterStatus: %v", m.filterStatus)
	}
}

// --- bean bt-8d35: the Filter-Strip as a REGION ---

// TestFilterStripEnterAppliesAndHoldsFocus is the PO's Flow #3 verbatim:
// "Anwenden: enter -> Filter wirkt, **Fokus bleibt im Strip** (ausdrücklicher
// PO-Wunsch)".
func TestFilterStripEnterAppliesAndHoldsFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyTab)) // Type tab
	it := m.filterItems[m.filterMenu.cursor]

	m = step(t, m, keyMsg(tea.KeyEnter))
	if !m.filterOpen {
		t.Fatal("enter closed the Filter-Strip -- the focus must STAY in the region (bt-8d35 Flow #3)")
	}
	if !m.facetOn(it) {
		t.Fatal("enter did not apply the cursored facet")
	}

	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.facetOn(it) {
		t.Fatal("a second enter did not un-apply the facet -- enter must apply the cursored value (multi-select)")
	}
	if !m.filterOpen {
		t.Fatal("the second enter closed the Filter-Strip")
	}
}

// TestFilterStripEscLeavesTheRegion pins bt-8d35's other half for the Strip:
// esc (and only esc / the toggle key f) leaves the region.
func TestFilterStripEscLeavesTheRegion(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('f'))
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.filterOpen {
		t.Fatal("esc did not leave the Filter-Strip region")
	}
}

// --- Querformat rendering (treeFilterBox) ---

// TestTreeFilterBoxShowsTabBarAndOnlyActiveFacetRows guards the actual
// Querformat rendering contract: a tab bar naming every category, but the
// row list below shows ONLY the active tab's items -- Type/Priority/Tag/
// Archive values must not appear while Status is active.
//
// Uses a beans set WITH a tag (fixtureBeans() itself has none, so the Tag
// facet would be legitimately absent -- filterFacetOrder deliberately never
// reports a phantom tab for a zero-row facet, see its own doc comment) so
// this test genuinely exercises all five tabs, matching real-world usage.
func TestTreeFilterBoxShowsTabBarAndOnlyActiveFacetRows(t *testing.T) {
	beans := []*item.Item{
		{ID: "tk-1", Title: "Task One", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"backend"}},
	}
	m := fixtureModel(t, beans)
	m = step(t, m, runeMsg('f'))
	out := stripHint(m.treeFilterBox())

	for _, tabLabel := range []string{"Status", "Type", "Priority", "Tags", "Archive"} {
		if !strings.Contains(out, tabLabel) {
			t.Errorf("treeFilterBox() tab bar missing %q:\n%s", tabLabel, out)
		}
	}
	if !strings.Contains(out, "todo") {
		t.Errorf("treeFilterBox() with Status active must show its own rows (todo):\n%s", out)
	}
	if strings.Contains(out, "milestone") {
		t.Errorf("treeFilterBox() with Status active must NOT show Type rows (milestone):\n%s", out)
	}

	m = step(t, m, keyMsg(tea.KeyTab)) // -> Type
	out = stripHint(m.treeFilterBox())
	if !strings.Contains(out, "milestone") {
		t.Errorf("treeFilterBox() with Type active must show its own rows (milestone):\n%s", out)
	}
	if strings.Contains(out, "todo") {
		t.Errorf("treeFilterBox() with Type active must NOT show Status rows (todo):\n%s", out)
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
	m = setSearchQuery(m, "alpha")              // matches both by title
	m.filterType = map[string]bool{"bug": true} // excludes "a"

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"b"}) {
		t.Fatalf("combined search+facet nodes = %v, want [b] (AND-combined)", got)
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
