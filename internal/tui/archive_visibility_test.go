package tui

// archive_visibility_test.go — E5 Task 7 (bean bt-ggt2, epic bt-5h4d): TDD
// coverage for the Archiv-Sicht (completed/scrapped hidden by default,
// togglable via the f-menu's "Show archived" row, design decision
// e). Reuses fixtureModel/step/keyMsg/runeMsg/nodeIDs/equalStrings
// (update_test.go, same package).

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

// archiveFixtureBeans is a small tree with one completed leaf so tests below
// can assert it is hidden/shown without disturbing fixtureBeans()'s own
// (unaffected) shape.
func archiveFixtureBeans() []data.Bean {
	return []data.Bean{
		{ID: "ms-1", Title: "Milestone One", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high", Parent: "ep-1"},
		{ID: "tk-done", Title: "Task Done", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-1"},
	}
}

// --- Default-hide ---

// TestArchivedBeanHiddenFromTreeByDefault guards the default-hide contract:
// showArchived's zero value is false, so a completed bean must not appear in
// visibleNodes() even with its parent fully expanded.
func TestArchivedBeanHiddenFromTreeByDefault(t *testing.T) {
	m := fixtureModel(t, archiveFixtureBeans())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true

	nodes := m.visibleNodes()
	for _, n := range nodes {
		if n.id == "tk-done" {
			t.Fatal("tk-done (status completed) visible by default -- want hidden (showArchived defaults false)")
		}
	}
}

// TestArchivedBeanShownWhenShowArchivedToggled drives the REAL f-menu key
// path (space on the "Show archived" row) end-to-end, not a direct
// field write -- proves the facet wiring (buildFilterItems/facetOn/
// toggleFacet) actually reaches m.showArchived, no new key involved.
func TestArchivedBeanShownWhenShowArchivedToggled(t *testing.T) {
	m := fixtureModel(t, archiveFixtureBeans())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true

	m = step(t, m, runeMsg('f'))
	if !m.filterOpen {
		t.Fatal("setup: f did not open the filter menu")
	}
	idx := -1
	for i, it := range m.filterItems {
		if it.facet == "archive" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("setup: no \"archive\" facet row in filterItems, got %+v", m.filterItems)
	}
	m.filterMenu.cursor = idx

	m = step(t, m, keyMsg(tea.KeySpace))
	if !m.showArchived {
		t.Fatal("space on the archive row did not set m.showArchived = true")
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.filterOpen {
		t.Fatal("esc did not close the filter menu")
	}

	found := false
	for _, n := range m.visibleNodes() {
		if n.id == "tk-done" {
			found = true
		}
	}
	if !found {
		t.Fatal("tk-done not visible after toggling showArchived on via the real f-menu key path")
	}

	// Toggle back off (same row, still cursored) -- must hide it again.
	m = step(t, m, runeMsg('f'))
	m.filterMenu.cursor = idx
	m = step(t, m, keyMsg(tea.KeySpace))
	if m.showArchived {
		t.Fatal("second space toggle did not flip m.showArchived back to false")
	}
}

// TestBuildFilterItemsIncludesArchiveRow guards the NEW standalone facet row
// (design decision e: "NICHT Teil der Status-Enum-Schleife"): buildFilterItems
// appends exactly one facet=="archive" row, labeled verbatim per the bean's
// own acceptance text.
func TestBuildFilterItemsIncludesArchiveRow(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	items := m.buildFilterItems()
	n := 0
	for _, it := range items {
		if it.facet == "archive" {
			n++
			if it.label != "Show archived" {
				t.Errorf("archive row label = %q, want %q", it.label, "Show archived")
			}
		}
	}
	if n != 1 {
		t.Fatalf("archive facet rows = %d, want exactly 1", n)
	}
}

// --- Ancestor visibility (NO code change expected -- structural proof) ---

// TestArchivedOnlyMilestoneDisappearsWithNoOpenDescendant guards the
// Ancestor-Sichtbarkeit contract (bean bt-ggt2 acceptance box, explicitly NO
// code change -- flattenTreeFiltered's existing subtreeHasMatch generically
// covers this once beanMatches carries the archive check): a milestone whose
// ONLY child is also completed must vanish from the tree entirely, root and
// all, when showArchived is false.
func TestArchivedOnlyMilestoneDisappearsWithNoOpenDescendant(t *testing.T) {
	beans := []data.Bean{
		{ID: "ms-done", Title: "Done Milestone", Status: "completed", Type: "milestone", Priority: "normal"},
		{ID: "tk-done", Title: "Done Task", Status: "completed", Type: "task", Priority: "normal", Parent: "ms-done"},
		{ID: "ms-open", Title: "Open Milestone", Status: "todo", Type: "milestone", Priority: "normal"},
	}
	m := fixtureModel(t, beans)

	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-open"}) {
		t.Fatalf("visibleNodes() = %v, want [ms-open] (ms-done + its only, also-completed child fully hidden)", got)
	}
}

// --- filterActive()/treeActive() stay unaffected (design decision e: "kein PO-Facet") ---

// TestArchiveToggleDoesNotCountAsFilterActive guards design decision e's
// explicit clause: toggling showArchived must NOT flip filterActive() or
// treeActive() -- those stay reserved for PO-set Status/Type/Priority/Tag
// facets and the search query, so the Tree's "Filter aktiv" red status line
// is unaffected by the archive default.
func TestArchiveToggleDoesNotCountAsFilterActive(t *testing.T) {
	m := model{showArchived: true}
	if m.filterActive() {
		t.Fatal("filterActive() true with only showArchived set -- archive default must not count as an active facet")
	}
	if m.treeActive() {
		t.Fatal("treeActive() true with only showArchived set -- must stay reserved for search/facets")
	}
}

// TestFilterClearXDoesNotResetArchiveToggle guards that X (clearFacets, the
// top-level facet reset) leaves m.showArchived untouched -- it is not one of
// the four facet maps X clears.
func TestFilterClearXDoesNotResetArchiveToggle(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.showArchived = true

	m = step(t, m, runeMsg('X'))

	if !m.showArchived {
		t.Fatal("X reset m.showArchived to false -- the archive toggle is not a facet map X clears")
	}
}

// --- Search respects the archive default (documented Task-7 decision) ---

// TestArchivedBeanExcludedFromBleveHitByDefault guards the documented Task-7
// decision (epic-E5-plan.md »Task 7«: "Suche respektiert den Archiv-
// Default"): an async Bleve hit for an archived bean is excluded by the SAME
// AND-combination beanMatches already imposes for facets -- no special-
// casing was needed in applyBleveResult, this is purely beanMatches picking
// up the new beanMatchesArchive term.
func TestArchivedBeanExcludedFromBleveHitByDefault(t *testing.T) {
	beans := []data.Bean{
		{ID: "tk-done", Title: "Refactor the done task", Status: "completed", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m = setSearchQuery(m, "refactor")
	m.searchBleveFor = "refactor"
	m.searchBleveIDs = map[string]bool{"tk-done": true}

	b := m.idx.ByID["tk-done"]
	if b == nil {
		t.Fatal("setup: tk-done not in idx.ByID")
	}
	if m.beanMatches(b) {
		t.Fatal("beanMatches true for an archived Bleve hit with showArchived=false, want false")
	}
	m.showArchived = true
	if !m.beanMatches(b) {
		t.Fatal("beanMatches false for an archived Bleve hit with showArchived=true, want true")
	}
}

// --- Backlog is structurally unaffected (index.go Backlog(): no change) ---

// TestBacklogUnaffectedByArchiveToggle guards the "KEINE Änderung nötig"
// acceptance box for index.go Backlog(): idx.Backlog() already excludes
// completed/scrapped structurally (todo/draft only) -- toggling showArchived
// must not add them back, since backlogVisible() only narrows the
// PRE-filtered idx.Backlog() set, never widens it.
func TestBacklogUnaffectedByArchiveToggle(t *testing.T) {
	beans := []data.Bean{
		{ID: "tk-todo", Title: "Open Task", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "tk-done", Title: "Done Task", Status: "completed", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)

	before := m.backlogVisible()
	if len(before) != 1 || before[0].ID != "tk-todo" {
		t.Fatalf("backlogVisible() (showArchived=false) = %v, want just tk-todo", before)
	}

	m.showArchived = true
	after := m.backlogVisible()
	if len(after) != 1 || after[0].ID != "tk-todo" {
		t.Fatalf("backlogVisible() (showArchived=true) = %v, want unchanged (tk-todo only) -- Backlog() already excludes completed structurally", after)
	}
}

// --- Repo-switch reset (T6-Note bug class) ---

// TestRepoSwitchResetsShowArchived guards the T6-Note bug class explicitly
// called out for T7 (bt-zhwl's "Notes for T7"): a showArchived=true toggle
// from the OLD repo must not leak into the newly loaded repo.
func TestRepoSwitchResetsShowArchived(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // applyRepoSwitched's best-effort config.SetLastRepo must not touch the real $HOME

	m := fixtureModel(t, archiveFixtureBeans())
	m.showArchived = true

	newRepo := t.TempDir()
	tm, _ := m.applyRepoSwitched(repoSwitchedMsg{
		client:  &data.Client{RepoDir: newRepo},
		repoDir: newRepo,
		beans:   []data.Bean{},
	})
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("applyRepoSwitched did not return a model, got %T", tm)
	}
	if m2.showArchived {
		t.Fatal("showArchived leaked across a repo switch, want reset to false")
	}
}
