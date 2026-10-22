package tui

// search_test.go — TDD coverage for Suche `/` (E2 Task 3, bean bt-4ep2,
// design-spec.md §6 V2): the local live filter (title+ID substring,
// case-insensitive) over the flattened tree, DD2-178 ancestor/collapse
// parity (port devd view_browse_project.go:161-241), the search-input key
// machine (port devd keyTreeSearch, view_browse_project.go:1073-1097), and
// the async Bleve half (searchCmd/searchBleveResultMsg, staleness-guarded
// instead of debounced -- see messages.go/update.go doc comments for the
// "why"). Reuses fixtureBeans/fixtureModel/step/keyMsg/runeMsg/nodeIDs/
// equalStrings from update_test.go (same package).

import (
	"github.com/rierek/lif/pkg/item"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Local live filter (flattenTreeFiltered / beanMatchesSearch) ---

// TestLocalSearchFiltersTreeByTitleSubstringCaseInsensitive guards the basic
// filter: only beans whose title contains the (lower-cased) query survive,
// case-insensitively; already-expanded ancestors of a match stay visible as
// context, siblings that don't match (and don't contain a match) are
// dropped.
func TestLocalSearchFiltersTreeByTitleSubstringCaseInsensitive(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m = setSearchQuery(m, "TWO") // case-insensitive, matches "Task Two" (tk-2) only

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1", "ep-1", "tk-2"}) {
		t.Fatalf("filtered nodes = %v, want [ms-1 ep-1 tk-2]", got)
	}
}

// TestLocalSearchMatchesBeanIDSubstring guards the "+ID" half of the filter
// (task brief: "title+ID substring, case-insensitive") -- the plan's own
// beanMatchesSearch snippet only checked Title; the ID half is required
// here explicitly, so it gets its own dedicated coverage.
func TestLocalSearchMatchesBeanIDSubstring(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "ms-1") // ID substring -- "Milestone One" the TITLE does not contain "ms-1"

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1"}) {
		t.Fatalf("filtered nodes = %v, want [ms-1] (search must match bean ID substring, not just title)", got)
	}
}

// TestLocalSearchPreservesAncestorPathOfMatch guards that an expanded
// ancestor whose OWN title doesn't match stays visible when one of its
// descendants does -- the match's path must render as context.
func TestLocalSearchPreservesAncestorPathOfMatch(t *testing.T) {
	beans := []*item.Item{
		{ID: "root-1", Title: "Container", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "leaf-1", Title: "Findme", Status: "todo", Type: "task", Priority: "normal", Parent: "root-1"},
	}
	m := fixtureModel(t, beans)
	m.expanded["root-1"] = true
	m = setSearchQuery(m, "findme")

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"root-1", "leaf-1"}) {
		t.Fatalf("filtered nodes = %v, want [root-1 leaf-1] (root-1 must stay visible as leaf-1's expanded ancestor even though root-1's own title doesn't match)", got)
	}
}

// TestLocalSearchCollapsedAncestorHidesMatchingDescendant guards DD2-178
// parity (devd view_browse_project.go:215-238, PO decision carried over
// verbatim per the plan's Port-Referenzen): a manually collapsed ancestor
// stays collapsed even though it contains a match -- it renders as a single
// context row, its matching child stays hidden, and it is NEVER
// auto-expanded.
func TestLocalSearchCollapsedAncestorHidesMatchingDescendant(t *testing.T) {
	beans := []*item.Item{
		{ID: "root-1", Title: "Container", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "leaf-1", Title: "Findme", Status: "todo", Type: "task", Priority: "normal", Parent: "root-1"},
	}
	m := fixtureModel(t, beans) // root-1 NOT expanded
	m = setSearchQuery(m, "findme")

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"root-1"}) {
		t.Fatalf("filtered nodes = %v, want [root-1] only -- collapsed ancestor must hide the matching descendant, never auto-expand", got)
	}
}

// TestLocalSearchOrphanBucketOmittedWhenNoMatch guards that the synthetic
// "(orphaned)" root goes through the SAME predicate as the real tree: with
// zero matching orphans/cycle-beans, it must be omitted entirely (not shown
// as an empty collapsed bucket).
func TestLocalSearchOrphanBucketOmittedWhenNoMatch(t *testing.T) {
	beans := append(fixtureBeans(), &item.Item{
		ID: "orph-1", Title: "Orphaned Task", Status: "todo", Type: "task", Priority: "normal", Parent: "missing",
	})
	m := fixtureModel(t, beans)
	m.expanded[orphanRootID] = true
	m = setSearchQuery(m, "zzz-no-match-anywhere")

	for _, n := range m.visibleNodes() {
		if n.orphan {
			t.Fatal("orphan root must be omitted entirely when no orphan/cycle bean matches the query")
		}
	}
}

// TestLocalSearchOrphanBucketShowsMatchingOrphan is the positive
// counterpart: a matching orphan renders exactly like today, under the
// (still expanded) orphan root.
func TestLocalSearchOrphanBucketShowsMatchingOrphan(t *testing.T) {
	beans := append(fixtureBeans(), &item.Item{
		ID: "orph-1", Title: "Findable Orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing",
	})
	m := fixtureModel(t, beans)
	m.expanded[orphanRootID] = true
	m = setSearchQuery(m, "findable")

	found := false
	for _, n := range m.visibleNodes() {
		if n.id == "orph-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("matching orphan must remain visible under the (still expanded) orphan root")
	}
}

// --- Search-input key machine (keySearchInput / keyTree's `/` case) ---

// TestSearchTypingUpdatesQueryLiveAndResetsCursor guards live filtering:
// `/` opens the input, then every subsequent rune keystroke updates
// m.searchQuery immediately (not just on commit) and resets the cursor to
// the filtered list's first row (mirrors devd's treeCursor=0 on every
// keystroke, view_browse_project.go:1073-1097).
func TestSearchTypingUpdatesQueryLiveAndResetsCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "tk-2" // parked away from what the filtered list's row 0 will be

	m = step(t, m, runeMsg('/'))
	if !m.searchActive {
		t.Fatal("/ must open the search input")
	}

	m = step(t, m, runeMsg('o'))
	if m.searchQuery != "o" {
		t.Fatalf("searchQuery after 1 rune = %q, want \"o\"", m.searchQuery)
	}
	nodes := m.visibleNodes()
	if len(nodes) == 0 || m.cursorID != nodes[0].id {
		t.Fatalf("cursorID after keystroke = %q, want first filtered row %q", m.cursorID, nodes[0].id)
	}

	m = step(t, m, runeMsg('n'))
	m = step(t, m, runeMsg('e'))
	if m.searchQuery != "one" {
		t.Fatalf("searchQuery after 3 runes = %q, want \"one\"", m.searchQuery)
	}
}

// TestSearchEnterCommitsQueryStaysActiveInputBlurred guards the commit path:
// enter blurs the input (searchActive=false) but the query/filter stays
// live -- it is not cleared.
func TestSearchEnterCommitsQueryStaysActiveInputBlurred(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	m = step(t, m, runeMsg('o'))
	m = step(t, m, runeMsg('n'))
	m = step(t, m, runeMsg('e'))
	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.searchActive {
		t.Fatal("enter must blur the search input (searchActive=false)")
	}
	if m.searchInput.Focused() {
		t.Fatal("enter must blur m.searchInput")
	}
	if m.searchQuery != "one" {
		t.Fatalf("searchQuery after enter = %q, want \"one\" (committed, not cleared)", m.searchQuery)
	}
	if !m.treeSearchActive() {
		t.Fatal("a committed query must keep the filter active")
	}
}

// TestSearchEscWhileTypingCancelsAndClearsQuery guards esc-cascade Rung 1:
// esc while the input still has focus cancels AND clears the query (not
// just blur+keep, unlike enter).
func TestSearchEscWhileTypingCancelsAndClearsQuery(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	m = step(t, m, runeMsg('o'))
	m = step(t, m, runeMsg('n'))
	if m.searchQuery != "on" {
		t.Fatalf("setup: searchQuery = %q, want \"on\"", m.searchQuery)
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.searchActive {
		t.Fatal("esc while typing must exit search-typing mode")
	}
	if m.searchQuery != "" {
		t.Fatalf("esc while typing must clear the query, got %q", m.searchQuery)
	}
	if m.searchInput.Value() != "" {
		t.Fatalf("esc while typing must clear the input value, got %q", m.searchInput.Value())
	}
}

// TestSearchEscAfterCommitClearsQuery guards esc-cascade Rung 2 (bean
// bt-4ep2: "Suche abbrechen -> Query+Filter leeren"): once the query is
// committed (input blurred, not typing), esc clears the committed query.
// Task 4 extends this same esc path to also clear the facet filter maps
// (not present yet in Task 3's scope).
func TestSearchEscAfterCommitClearsQuery(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	m = step(t, m, runeMsg('o'))
	m = step(t, m, runeMsg('n'))
	m = step(t, m, runeMsg('e'))
	m = step(t, m, keyMsg(tea.KeyEnter))
	if !m.treeSearchActive() {
		t.Fatal("setup: expected an active committed query")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.treeSearchActive() {
		t.Fatal("esc after commit must clear the committed query")
	}
	if m.searchQuery != "" {
		t.Fatalf("searchQuery after esc-clear = %q, want empty", m.searchQuery)
	}
}

// --- Async Bleve half (searchCmd / searchBleveResultMsg / staleness guard) ---

// TestSearchBleveFiresOnlyAtThreeOrMoreChars guards the threshold: below 3
// chars no dispatch happens (searchBleveLoading stays false); the keystroke
// that brings the query to length 3 dispatches one.
func TestSearchBleveFiresOnlyAtThreeOrMoreChars(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	m = step(t, m, runeMsg('a'))
	m = step(t, m, runeMsg('b'))
	if m.searchBleveLoading {
		t.Fatal("2 chars must not dispatch a Bleve search yet")
	}
	if m.searchBleveFor != "" {
		t.Fatalf("searchBleveFor = %q at 2 chars, want unset", m.searchBleveFor)
	}

	m = step(t, m, runeMsg('c'))
	if !m.searchBleveLoading {
		t.Fatal("the keystroke reaching 3 chars must dispatch a Bleve search (searchBleveLoading)")
	}
}

// TestSearchCmdTagsResultWithQuery guards searchCmd's message tagging
// directly (Store.Search() tags the query
// on BOTH its success and its error path).
func TestSearchCmdTagsResultWithQuery(t *testing.T) {
	c := fixtureStore(t)
	msg := searchCmd(c, "abc")()
	res, ok := msg.(searchBleveResultMsg)
	if !ok {
		t.Fatalf("searchCmd()() = %T, want searchBleveResultMsg", msg)
	}
	if res.query != "abc" {
		t.Errorf("searchBleveResultMsg.query = %q, want \"abc\"", res.query)
	}
}

// TestSearchBleveStaleResultDiscardedWhenQueryChangedMeanwhile guards the
// staleness guard (chosen over a debounce timer, see messages.go doc
// comment): a result tagged for a query that no longer matches the model's
// CURRENT searchQuery must be a no-op, not a panic.
func TestSearchBleveStaleResultDiscardedWhenQueryChangedMeanwhile(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "abcd") // query has already moved on past "abc"

	m = step(t, m, searchBleveResultMsg{query: "abc", ids: []string{"tk-1"}})

	if m.searchBleveFor != "" {
		t.Fatalf("stale result must not update searchBleveFor, got %q", m.searchBleveFor)
	}
	if m.searchBleveIDs != nil {
		t.Fatalf("stale result must not update searchBleveIDs, got %v", m.searchBleveIDs)
	}
}

// TestSearchIDSubstringHitStaysVisibleAfterBleveResultArrives guards I01 (E2-
// T3-Review finding, PFLICHT carried into bean bt-9ldr/Task 4): once a Bleve
// response for the CURRENT query lands, beanMatchesSearch must UNION the
// local ID-substring match with the Bleve result set, not replace it
// outright -- Bleve's full-text index covers title+body (client.go Search
// doc comment) but not necessarily an arbitrary ID substring, so a bean that
// only matched by ID must not flicker out of the tree the instant the async
// Bleve response arrives (its Bleve result set may legitimately be empty for
// that bean).
func TestSearchIDSubstringHitStaysVisibleAfterBleveResultArrives(t *testing.T) {
	beans := []*item.Item{
		{ID: "tk-1", Title: "Something Else Entirely", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m = setSearchQuery(m, "tk-1") // matches the ID substring only, NOT the title
	if !m.beanMatchesSearch(m.idx.ByID["tk-1"]) {
		t.Fatal("setup: ID-substring match must hold before any Bleve response arrives")
	}

	// The (simulated) Bleve title+body search finds nothing for this bean --
	// its title/body contain no token resembling "tk-1" -- so the response
	// arrives empty, tagged for the current query.
	m = step(t, m, searchBleveResultMsg{query: "tk-1", ids: nil})
	if m.searchBleveFor != "tk-1" {
		t.Fatalf("setup: searchBleveFor = %q, want \"tk-1\" (authoritative Bleve response must have applied)", m.searchBleveFor)
	}

	if !m.beanMatchesSearch(m.idx.ByID["tk-1"]) {
		t.Fatal("ID-substring match must stay visible (UNIONed) once the authoritative Bleve response arrives, not disappear")
	}
}

// TestSearchBleveResultAppliedWhenQueryStillCurrent is the positive
// counterpart: a result tagged for the model's current query is applied.
func TestSearchBleveResultAppliedWhenQueryStillCurrent(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "abc")
	m.searchBleveLoading = true

	m = step(t, m, searchBleveResultMsg{query: "abc", ids: []string{"tk-1"}})

	if m.searchBleveFor != "abc" {
		t.Fatalf("searchBleveFor = %q, want \"abc\"", m.searchBleveFor)
	}
	if !m.searchBleveIDs["tk-1"] {
		t.Fatal("searchBleveIDs must contain tk-1 from the applied result")
	}
	if m.searchBleveLoading {
		t.Fatal("applying a (non-stale) result must clear searchBleveLoading")
	}
}
