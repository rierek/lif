package tui

// box_picker_filter_test.go — TDD coverage for the shared Picker-Filter
// (bean bt-a3a8, PO-Nebenbefund N7): the Blocking-Picker (`r`) and the
// Parent-Picker (`a`) both grew a live text search plus a Type/Status/
// Priority/Tags chip strip, ported from the Tag-Picker's own always-focused
// textinput pattern (box_picker_tag.go, bean bt-9ipw D01).
//
// The four hard acceptance criteria from the bean drive this file:
//  1. typing filters the candidate list live (title OR ID substring)
//  2. the chip strip's facets actually narrow the list
//  3. the picker's filter state NEVER mutates the global Browse filter
//  4. plain letters stay TYPEABLE inside the search field (no action
//     collision -- the bt-9ipw/D01 lesson: `x`/`s`/`t` must reach the input)
//
// Facet cycling therefore rides on ctrl-chords (ctrl+t/n/p/g), which a
// textinput can never consume as literal characters.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// typeString feeds each rune of s through Update as a separate KeyRunes
// message -- the real per-keystroke round-trip, not a SetValue shortcut, so
// the "every non-reserved key belongs to the input" contract is genuinely
// exercised.
func typeString(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = step(t, m, runeMsg(r))
	}
	return m
}

// filterBeansFixture is a deliberately heterogeneous set: distinct titles,
// types, statuses, priorities and tags, so every facet has something to
// narrow on and no two criteria are accidentally coextensive.
func filterBeansFixture() []data.Bean {
	return []data.Bean{
		{ID: "ms-1", Title: "Alpha Milestone", Status: "todo", Type: "milestone", Priority: "normal", Tags: []string{"infra"}},
		{ID: "ep-1", Title: "Beta Epic", Status: "todo", Type: "epic", Priority: "high", Parent: "ms-1", Tags: []string{"ui"}},
		{ID: "tk-1", Title: "Gamma Task", Status: "in-progress", Type: "task", Priority: "high", Parent: "ep-1", Tags: []string{"ui", "infra"}},
		{ID: "tk-2", Title: "Delta Task", Status: "todo", Type: "task", Priority: "low", Parent: "ep-1"},
		{ID: "bug-9", Title: "Gamma Bug", Status: "todo", Type: "bug", Priority: "normal", Parent: "ep-1", Tags: []string{"ui"}},
	}
}

// --- Blocking-Picker: live text search ---

// TestBlockingPickerSearchNarrowsByTitle is acceptance criterion 1: typing
// "gamma" must leave only the two Gamma-titled candidates on screen.
func TestBlockingPickerSearchNarrowsByTitle(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	if len(m.blockFiltered) != 4 {
		t.Fatalf("unfiltered blockFiltered = %d, want 4 (every bean but ms-1)", len(m.blockFiltered))
	}

	m = typeString(t, m, "gamma")

	ids := pickerItemIDs(m.blockFiltered)
	if len(m.blockFiltered) != 2 || !ids["tk-1"] || !ids["bug-9"] {
		t.Fatalf("after typing \"gamma\": blockFiltered = %+v, want exactly tk-1 + bug-9", m.blockFiltered)
	}
	// The FULL list is untouched -- filtering is a view, not a rebuild.
	if len(m.blockItems) != 4 {
		t.Fatalf("blockItems = %d, want 4 (search must not shrink the source list)", len(m.blockItems))
	}
}

// TestBlockingPickerSearchMatchesID guards the ID half of the substring
// predicate (mirrors beanMatchesSearch's own title-OR-ID contract).
func TestBlockingPickerSearchMatchesID(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	m = typeString(t, m, "bug-9")

	if len(m.blockFiltered) != 1 || m.blockFiltered[0].id != "bug-9" {
		t.Fatalf("after typing an ID: blockFiltered = %+v, want exactly bug-9", m.blockFiltered)
	}
}

// TestBlockingPickerLettersStayTypeable is acceptance criterion 4 and the
// bean's single most important fallstrick (bt-9ipw/D01): `x` is
// keys.Toggle's second alias, `s`/`t` are global action keys. Inside the
// picker's focused search field all three must reach the textinput as
// literal characters -- and specifically `x` must NOT toggle anything.
func TestBlockingPickerLettersStayTypeable(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	before := len(m.blockPending)
	m = typeString(t, m, "xst")

	if got := m.blockFilter.input.Value(); got != "xst" {
		t.Fatalf("search field value = %q, want %q -- x/s/t must stay typeable", got, "xst")
	}
	if len(m.blockPending) != before {
		t.Fatalf("blockPending changed while typing (%d -> %d): a letter fired a toggle", before, len(m.blockPending))
	}
}

// TestBlockingPickerSpaceStillToggles proves the narrowing above did not
// cost the picker its multi-select: space toggles the cursored row of the
// FILTERED list, i.e. "toggles what's on screen" (same semantics
// toggleTagPending gained under bt-9ipw/D01).
func TestBlockingPickerSpaceStillToggles(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	m = typeString(t, m, "bug-9")
	m = step(t, m, runeMsg(' '))

	if !m.blockPending["bug-9"] {
		t.Fatalf("space did not toggle the cursored filtered row; blockPending = %+v", m.blockPending)
	}
}

// TestBlockingPickerSelectionAfterFilterHitsRightBean is the bean's own
// "Auswahl nach Filterung trifft den richtigen Bean" criterion, driven end
// to end: filter down to a single row, toggle it, and confirm the pending
// diff names exactly that bean -- never the row that USED to sit at cursor
// index 0 of the unfiltered list.
func TestBlockingPickerSelectionAfterFilterHitsRightBean(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	unfilteredFirst := m.blockItems[0].id
	m = typeString(t, m, "delta")
	if len(m.blockFiltered) != 1 || m.blockFiltered[0].id != "tk-2" {
		t.Fatalf("blockFiltered = %+v, want exactly tk-2", m.blockFiltered)
	}
	if unfilteredFirst == "tk-2" {
		t.Skip("fixture no longer distinguishes filtered from unfiltered cursor position")
	}

	m = step(t, m, runeMsg(' '))

	if !m.blockPending["tk-2"] {
		t.Fatalf("blockPending = %+v, want tk-2 toggled", m.blockPending)
	}
	if m.blockPending[unfilteredFirst] {
		t.Fatalf("toggle hit the UNFILTERED cursor row %q -- cursor is not tracking the filtered list", unfilteredFirst)
	}
}

// --- Blocking-Picker: facet chips ---

// TestBlockingPickerFacetCycleNarrows is acceptance criterion 2: ctrl+t
// steps the Type chip through "(any)" -> the type enum, and the candidate
// list follows it.
func TestBlockingPickerFacetCycleNarrows(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	// Step the Type chip until it lands on "task".
	for i := 0; i < len(data.TypeValues())+1; i++ {
		if m.blockFilter.facetType == "task" {
			break
		}
		m = step(t, m, keyMsg(tea.KeyCtrlT))
	}
	if m.blockFilter.facetType != "task" {
		t.Fatalf("Type chip = %q after cycling, want %q", m.blockFilter.facetType, "task")
	}

	ids := pickerItemIDs(m.blockFiltered)
	if len(m.blockFiltered) != 2 || !ids["tk-1"] || !ids["tk-2"] {
		t.Fatalf("Type=task filtered = %+v, want exactly tk-1 + tk-2", m.blockFiltered)
	}
}

// TestBlockingPickerFacetCycleWrapsToAny guards the cycle's "(any)" origin
// and wrap-around: N+1 presses return the chip to unset, and the full list
// with it.
func TestBlockingPickerFacetCycleWrapsToAny(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	if m.blockFilter.facetType != "" {
		t.Fatalf("Type chip = %q on open, want unset", m.blockFilter.facetType)
	}
	for i := 0; i < len(data.TypeValues())+1; i++ {
		m = step(t, m, keyMsg(tea.KeyCtrlT))
	}
	if m.blockFilter.facetType != "" {
		t.Fatalf("Type chip = %q after a full cycle, want back to unset", m.blockFilter.facetType)
	}
	if len(m.blockFiltered) != 4 {
		t.Fatalf("blockFiltered = %d after wrapping to (any), want 4", len(m.blockFiltered))
	}
}

// TestBlockingPickerFacetAndSearchCombine proves the two halves AND rather
// than replace each other: "gamma" alone yields tk-1+bug-9, plus Status=
// in-progress yields tk-1 only.
func TestBlockingPickerFacetAndSearchCombine(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	m = typeString(t, m, "gamma")
	for i := 0; i < len(data.StatusValues())+1; i++ {
		if m.blockFilter.facetStatus == "in-progress" {
			break
		}
		m = step(t, m, keyMsg(tea.KeyCtrlN))
	}
	if m.blockFilter.facetStatus != "in-progress" {
		t.Fatalf("Status chip = %q, want in-progress", m.blockFilter.facetStatus)
	}

	if len(m.blockFiltered) != 1 || m.blockFiltered[0].id != "tk-1" {
		t.Fatalf("search AND facet = %+v, want exactly tk-1", m.blockFiltered)
	}
}

// TestBlockingPickerTagFacetNarrows covers the dynamic (repo-collected)
// fourth chip.
func TestBlockingPickerTagFacetNarrows(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-2")
	m = step(t, m, runeMsg('r'))

	for i := 0; i < len(m.tagFilterOptions())+1; i++ {
		if m.blockFilter.facetTag == "infra" {
			break
		}
		m = step(t, m, keyMsg(tea.KeyCtrlG))
	}
	if m.blockFilter.facetTag != "infra" {
		t.Fatalf("Tag chip = %q, want infra", m.blockFilter.facetTag)
	}

	ids := pickerItemIDs(m.blockFiltered)
	if len(m.blockFiltered) != 2 || !ids["ms-1"] || !ids["tk-1"] {
		t.Fatalf("Tag=infra filtered = %+v, want exactly ms-1 + tk-1", m.blockFiltered)
	}
}

// TestBlockingPickerPriorityFacetNarrows covers the third chip (ctrl+p).
func TestBlockingPickerPriorityFacetNarrows(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	for i := 0; i < len(data.PriorityValues())+1; i++ {
		if m.blockFilter.facetPriority == "low" {
			break
		}
		m = step(t, m, keyMsg(tea.KeyCtrlP))
	}
	if m.blockFilter.facetPriority != "low" {
		t.Fatalf("Priority chip = %q, want low", m.blockFilter.facetPriority)
	}
	if len(m.blockFiltered) != 1 || m.blockFiltered[0].id != "tk-2" {
		t.Fatalf("Priority=low filtered = %+v, want exactly tk-2", m.blockFiltered)
	}
}

// --- Global-filter isolation (acceptance criterion 3) ---

// TestBlockingPickerDoesNotMutateGlobalFilter is the bean's explicit
// "(Test!)" criterion: neither typing nor cycling every chip may touch the
// four global m.filter* maps or m.searchQuery -- otherwise merely OPENING a
// picker would silently re-filter the Browse view behind it.
func TestBlockingPickerDoesNotMutateGlobalFilter(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))

	m = typeString(t, m, "gamma")
	m = step(t, m, keyMsg(tea.KeyCtrlT))
	m = step(t, m, keyMsg(tea.KeyCtrlN))
	m = step(t, m, keyMsg(tea.KeyCtrlP))
	m = step(t, m, keyMsg(tea.KeyCtrlG))

	assertGlobalFilterUntouched(t, m)
}

// TestParentPickerDoesNotMutateGlobalFilter is the same guard for `a`.
func TestParentPickerDoesNotMutateGlobalFilter(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('a'))

	m = typeString(t, m, "alpha")
	m = step(t, m, keyMsg(tea.KeyCtrlT))
	m = step(t, m, keyMsg(tea.KeyCtrlN))
	m = step(t, m, keyMsg(tea.KeyCtrlP))
	m = step(t, m, keyMsg(tea.KeyCtrlG))

	assertGlobalFilterUntouched(t, m)
}

func assertGlobalFilterUntouched(t *testing.T, m model) {
	t.Helper()
	if len(m.filterType) != 0 || len(m.filterStatus) != 0 || len(m.filterPriority) != 0 || len(m.filterTag) != 0 {
		t.Fatalf("picker-local filtering leaked into the GLOBAL facets: type=%v status=%v prio=%v tag=%v",
			m.filterType, m.filterStatus, m.filterPriority, m.filterTag)
	}
	if m.searchQuery != "" {
		t.Fatalf("picker search leaked into the global searchQuery = %q", m.searchQuery)
	}
	if m.filterActive() {
		t.Fatal("filterActive() true after picker-local filtering only")
	}
}

// --- Parent-Picker ---

// TestParentPickerSearchNarrows mirrors the Blocking test for `a`, and adds
// the Parent-Picker's own twist: the synthetic "(No parent)" clear row is
// an ACTION, not a candidate, so it stays pinned first no matter what is
// typed (documented design decision -- clearing a parent must never require
// emptying the search box first).
func TestParentPickerSearchNarrows(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('a'))

	m = typeString(t, m, "beta")

	if len(m.parentFiltered) < 1 || m.parentFiltered[0].id != "" {
		t.Fatalf("parentFiltered[0] = %+v, want the pinned \"(No parent)\" row", m.parentFiltered)
	}
	ids := pickerItemIDs(m.parentFiltered)
	if !ids["ep-1"] {
		t.Fatalf("parentFiltered = %+v, want ep-1 (title \"Beta Epic\")", m.parentFiltered)
	}
	if ids["ms-1"] {
		t.Fatalf("parentFiltered = %+v, ms-1 must not survive the \"beta\" query", m.parentFiltered)
	}
}

// TestParentPickerSelectionAfterFilterHitsRightBean drives `a` end to end:
// filter, then Enter, and assert the mutation targets the FILTERED row.
// With no client wired the mutation itself cannot run, so the assertion is
// on the row the cursor resolved to before dispatch.
func TestParentPickerSelectionAfterFilterHitsRightBean(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('a'))

	m = typeString(t, m, "beta")
	m = step(t, m, keyMsg(tea.KeyDown)) // off the pinned clear row, onto the first real candidate

	it, ok := m.parentPickerCursorItem()
	if !ok || it.id != "ep-1" {
		t.Fatalf("cursor item = %+v (ok=%v), want ep-1", it, ok)
	}
}

// TestParentPickerLettersStayTypeable repeats the collision guard for `a`.
func TestParentPickerLettersStayTypeable(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('a'))

	m = typeString(t, m, "xst")

	if got := m.parentFilter.input.Value(); got != "xst" {
		t.Fatalf("search field value = %q, want %q", got, "xst")
	}
	if m.overlay != overlayParentPicker {
		t.Fatalf("overlay = %v after typing, want the picker to stay open", m.overlay)
	}
}

// TestParentPickerFacetNarrows covers the chip strip on `a`. tk-1 is a task,
// so every non-task type is eligible; Type=milestone must leave the pinned
// clear row plus ms-1 alone.
func TestParentPickerFacetNarrows(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('a'))

	for i := 0; i < len(data.TypeValues())+1; i++ {
		if m.parentFilter.facetType == "milestone" {
			break
		}
		m = step(t, m, keyMsg(tea.KeyCtrlT))
	}
	if m.parentFilter.facetType != "milestone" {
		t.Fatalf("Type chip = %q, want milestone", m.parentFilter.facetType)
	}

	ids := pickerItemIDs(m.parentFiltered)
	if !ids["ms-1"] || ids["ep-1"] {
		t.Fatalf("parentFiltered = %+v, want ms-1 but not ep-1", m.parentFiltered)
	}
}

// --- Reset-on-open ---

// TestPickerFilterResetsOnReopen guards that a picker never opens carrying
// the previous session's query/chips (wholesale-replace-on-open convention,
// openTagPicker's own doc-stamp).
func TestPickerFilterResetsOnReopen(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m = focusBeanFull(m, "ms-1")

	m = step(t, m, runeMsg('r'))
	m = typeString(t, m, "gamma")
	m = step(t, m, keyMsg(tea.KeyCtrlT))
	m = step(t, m, keyMsg(tea.KeyEsc))

	m = step(t, m, runeMsg('r'))
	if got := m.blockFilter.input.Value(); got != "" {
		t.Fatalf("reopened picker carries query %q, want empty", got)
	}
	if m.blockFilter.facetType != "" {
		t.Fatalf("reopened picker carries Type chip %q, want unset", m.blockFilter.facetType)
	}
	if len(m.blockFiltered) != 4 {
		t.Fatalf("reopened blockFiltered = %d, want the full 4", len(m.blockFiltered))
	}
}

// --- Rendering ---

// --- bean bt-6nuz: overlay title, framed search field, footer styling ---

// TestBlockingPickerBoxTitleIsRelations guards PO finding #7: the overlay
// heading must read "Relations", matching the Detail-View's own RELATIONS
// panel. Only the DISPLAY label moved -- blockItems/blockPending and every
// other internal identifier deliberately keep the "blocking" domain word.
func TestBlockingPickerBoxTitleIsRelations(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('r'))

	out := ansi.Strip(m.blockingPickerBox())
	if !strings.Contains(out, "Relations") {
		t.Errorf("blocking picker title is not %q:\n%s", "Relations", out)
	}
	if strings.Contains(out, "Blocking blocked") || strings.Contains(out, "─ Blocking ─") {
		t.Errorf("blocking picker still shows the old %q heading:\n%s", "Blocking", out)
	}
}

// TestPickerBoxesFrameTheSearchField guards PO finding #8: the search field
// used to be the one bare, unframed row in an overlay otherwise built from
// boxed fields -- it fell out optically against the chip strip right above
// it. It now wears the same boxTopBorder/boxBottomBorder frame, labelled
// "Search".
func TestPickerBoxesFrameTheSearchField(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-1")

	for _, tc := range []struct {
		name string
		key  rune
		box  func(model) string
	}{
		{"blocking", 'r', model.blockingPickerBox},
		{"parent", 'a', model.parentPickerBox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := step(t, m, runeMsg(tc.key))
			out := ansi.Strip(tc.box(mm))
			var framed bool
			lines := strings.Split(out, "\n")
			for i, line := range lines {
				if !strings.Contains(line, pickerFilterPlaceholder) {
					continue
				}
				if i == 0 || i+1 >= len(lines) {
					t.Fatalf("%s: placeholder line has no room for a frame:\n%s", tc.name, out)
				}
				if strings.Contains(lines[i-1], "╭") && strings.Contains(lines[i+1], "╰") &&
					strings.HasPrefix(strings.TrimSpace(line), "│") {
					framed = true
				}
			}
			if !framed {
				t.Errorf("%s picker search field is not framed like the other fields:\n%s", tc.name, out)
			}
		})
	}
}

// TestPickerHintUsesBindingStyling guards PO finding #9: the overlay's own
// hint line was one flat Muted string ("space:toggle  enter:save"), while
// every other footer in the app renders Key in Teal and action in Subtext
// (renderBindings, view.go). It now comes from the SAME
// blockingPickerLocalBindings/parentPickerLocalBindings accessor the OUTER
// Footer Zone 3 uses (bean bt-z4w7's derive-from-the-active-binding rule),
// so the two surfaces cannot drift.
func TestPickerHintUsesBindingStyling(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-1")

	for _, tc := range []struct {
		name string
		key  rune
		box  func(model) string
		want string
	}{
		{"blocking", 'r', model.blockingPickerBox, renderBindings(blockingPickerLocalBindings())},
		{"parent", 'a', model.parentPickerBox, renderBindings(parentPickerLocalBindings())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := step(t, m, runeMsg(tc.key))
			if !strings.Contains(tc.box(mm), tc.want) {
				t.Errorf("%s picker hint is not the renderBindings-styled local set:\n%s", tc.name, tc.box(mm))
			}
		})
	}
}

// TestPickerHintOmitsKeysAlreadyBadgedInline guards the second half of #9
// (and bt-fy5d's own rule): the four facet chords ^t/^n/^p/^g are already
// rendered as badges in the chip strip's own frames, so repeating them in
// the hint line above is exactly the duplication bt-fy5d removed from the
// main footer.
func TestPickerHintOmitsKeysAlreadyBadgedInline(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-1")
	m = step(t, m, runeMsg('r'))

	out := ansi.Strip(m.blockingPickerBox())
	for _, stale := range []string{"^t/^n/^p/^g:facets", "type:filter", "esc:discard", "space:toggle"} {
		if strings.Contains(out, stale) {
			t.Errorf("hint still carries the flat pre-bt-6nuz fragment %q:\n%s", stale, out)
		}
	}
	// The chips themselves keep their badges -- only the hint's copy went.
	if !strings.Contains(out, "^t") {
		t.Errorf("the Type chip lost its own inline ^t badge:\n%s", out)
	}
}

// TestPickerBoxesRenderFilterChrome asserts both overlays actually SHOW the
// new chrome: the four chip labels plus the search field's placeholder.
func TestPickerBoxesRenderFilterChrome(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 120, 40
	m = focusBeanFull(m, "tk-1")

	for _, tc := range []struct {
		name string
		key  rune
		box  func(model) string
	}{
		{"blocking", 'r', model.blockingPickerBox},
		{"parent", 'a', model.parentPickerBox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := step(t, m, runeMsg(tc.key))
			out := tc.box(mm)
			for _, want := range []string{"Type", "Status", "Priority", "Tags"} {
				if !strings.Contains(out, want) {
					t.Errorf("%s picker box missing chip label %q:\n%s", tc.name, want, out)
				}
			}
			if !strings.Contains(out, pickerFilterPlaceholder) {
				t.Errorf("%s picker box missing the search field placeholder:\n%s", tc.name, out)
			}
		})
	}
}

// manyBeansFixture is big enough that both pickers must actually window
// their row list -- the short fixture above never exceeds any budget, so it
// cannot catch a vertical overflow.
func manyBeansFixture() []data.Bean {
	out := []data.Bean{{ID: "ms-1", Title: "Alpha Milestone", Status: "todo", Type: "milestone", Priority: "normal"}}
	for i := 0; i < 60; i++ {
		out = append(out, data.Bean{
			ID:     "tk-" + string(rune('a'+i%26)) + string(rune('a'+i/26)),
			Title:  "A deliberately long candidate title that will wrap at any sane picker width, number " + string(rune('a'+i%26)),
			Status: "todo", Type: "task", Priority: "normal", Parent: "ms-1",
		})
	}
	return out
}

// TestPickerBoxesFitIn24Lines is the vertical half of the bean's "Overlay
// läuft bei 80 Spalten nicht über" criterion -- the half the 80x24 tmux
// smoke actually caught. It drives a 60-candidate repo of long, WRAPPING
// titles, which is exactly the case a fixed element cap cannot bound (see
// pickerRowWindow's own ERRATUM doc-stamp).
func TestPickerBoxesFitIn24Lines(t *testing.T) {
	m := fixtureModel(t, manyBeansFixture())
	m.width, m.height = 80, 24
	m = focusBeanFull(m, "ms-1")

	for _, tc := range []struct {
		name string
		key  rune
		box  func(model) string
	}{
		{"blocking", 'r', model.blockingPickerBox},
		{"parent", 'a', model.parentPickerBox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := step(t, m, runeMsg(tc.key))
			if got := lipgloss.Height(tc.box(mm)); got > 24 {
				t.Fatalf("%s picker renders %d lines on a 24-line terminal:\n%s", tc.name, got, tc.box(mm))
			}
		})
	}
}

// TestPickerBoxesFitIn80Columns is the bean's "Overlay läuft bei 80 Spalten
// nicht über" criterion, as a structural unit guard alongside the manual
// tmux smoke: no rendered line may exceed the terminal width.
func TestPickerBoxesFitIn80Columns(t *testing.T) {
	m := fixtureModel(t, filterBeansFixture())
	m.width, m.height = 80, 24
	m = focusBeanFull(m, "tk-1")

	for _, tc := range []struct {
		name string
		key  rune
		box  func(model) string
	}{
		{"blocking", 'r', model.blockingPickerBox},
		{"parent", 'a', model.parentPickerBox},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mm := step(t, m, runeMsg(tc.key))
			for i, line := range strings.Split(tc.box(mm), "\n") {
				if w := lipgloss.Width(line); w > 80 {
					t.Fatalf("%s picker line %d is %d cells wide (>80): %q", tc.name, i, w, line)
				}
			}
		})
	}
}

// TestPickersDefaultToOpenStatuses: both pickers open on Status=open (todo,
// in-progress, draft); ^n cycles on to single statuses and finally (any),
// which shows closed items again.
func TestPickersDefaultToOpenStatuses(t *testing.T) {
	beans := append(filterBeansFixture(),
		data.Bean{ID: "done-1", Title: "Old Task", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-1"})

	m := fixtureModel(t, beans)
	m = focusBeanFull(m, "ms-1")
	m = step(t, m, runeMsg('r'))
	if m.blockFilter.facetStatus != "open" || pickerItemIDs(m.blockFiltered)["done-1"] || len(m.blockFiltered) != 4 {
		t.Fatalf("blocking picker default: chip=%q filtered=%+v, want open and no done-1", m.blockFilter.facetStatus, m.blockFiltered)
	}
	for i := 0; i < len(data.StatusValues())+1; i++ {
		m = step(t, m, keyMsg(tea.KeyCtrlN))
	}
	if m.blockFilter.facetStatus != "" || !pickerItemIDs(m.blockFiltered)["done-1"] {
		t.Fatalf("after full cycle: chip=%q filtered=%+v, want (any) incl. done-1", m.blockFilter.facetStatus, m.blockFiltered)
	}

	p := fixtureModel(t, beans)
	p = focusBeanFull(p, "tk-2")
	p = step(t, p, runeMsg('a'))
	if p.parentFilter.facetStatus != "open" || pickerItemIDs(p.parentFiltered)["done-1"] {
		t.Fatalf("parent picker default: chip=%q filtered=%+v", p.parentFilter.facetStatus, p.parentFiltered)
	}
}
