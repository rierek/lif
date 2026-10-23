package tui

// update_test.go — TDD coverage for the App-Shell Update dispatcher (T8,
// implementation-plan.md »E1 Task 8«): cursor movement/expand/collapse,
// quit-confirm, reload-keeps-cursor-by-ID, and the MANDATORY orphan rule
// (bean bt-7jr8 / T3-review Q01). Also covers the T8 Opus quality-review
// follow-ups (bean bt-7jr8, Runde 2): tree windowing (B01), cycle-bean
// visibility (B02), and tab focus-swap accenting (I05 #1).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/theme"
)

// fixtureBeans is a small milestone -> epic -> 2 tasks hierarchy shared by
// the Update tests below.
func fixtureBeans() []*item.Item {
	return []*item.Item{
		{ID: "ms-1", Title: "Milestone One", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		// tk-1 sorts before tk-2 (Index.sortBeans: Status -> Priority -> Type ->
		// Title; in-progress ranks before todo) -- deliberately picked so
		// "tk-1 first, tk-2 last" holds without fighting the real sort order.
		{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high", Parent: "ep-1"},
		{ID: "tk-2", Title: "Task Two", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-1"},
	}
}

// fixtureModel builds a model already past its initial load (mirrors what
// Init()+the real beansLoadedMsg round-trip produces, without a live client).
// The Detail pane is shown (a real model starts with it hidden, see
// TestStartLayoutShowsNoDetail) so the split-layout tests keep their
// geometry. The synthetic "Unscheduled" and "Not on roadmap" roots are
// expanded (a real model starts collapsed, see
// TestRoadmapTreeStartsCollapsedAtMilestones) so fixture items outside a
// milestone are reachable without every test expanding them.
func fixtureModel(t *testing.T, beans []*item.Item) model {
	t.Helper()
	m := step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: beans})
	m.layout = layoutFilterDetail
	m.expanded = map[string]bool{unscheduledRootID: true, notOnRoadmapRootID: true}
	return m
}

// step sends msg through m.Update and type-asserts the resulting tea.Model
// back to model, failing the test if that assertion ever breaks (it never
// should -- Update always returns the same concrete type it received).
func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	tm, _ := m.Update(msg)
	mm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(%T) did not return a model, got %T", msg, tm)
	}
	return mm
}

func keyMsg(k tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: k} }
func runeMsg(r rune) tea.KeyMsg       { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// setSearchQuery is the test-side counterpart of keySearchInput's per-
// keystroke prefix parse (bt-2kfl, D03): tests across this package set
// m.searchQuery directly (no real tea.Update keystroke round-trip), so this
// keeps m.searchPrefixFacets/m.searchPrefixRest in sync the same way a real
// keystroke would via applySearchPrefixes -- beanMatchesSearch/filterSummary
// read the derived fields, not m.searchQuery itself, so a test that skips
// this call would see an empty rest/facets regardless of what it typed.
func setSearchQuery(m model, q string) model {
	m.searchQuery = q
	return m.applySearchPrefixes()
}

// stripHint is renderBindings' own test-side counterpart (view.go): strips
// ANSI (ansi.Strip) AND normalizes the nbsp (U+00A0) renderBindings glues a
// binding's key+desc together with (D06 regression fix, view.go's own doc
// comment on the `nbsp` const -- prevents wordwrap from orphaning a key
// alone on one line) back to a plain ASCII space, so Header/Footer-hint
// assertions can keep comparing against ordinary "key desc" literals without
// caring about that internal wrap-atomicity implementation detail.
func stripHint(s string) string {
	return strings.ReplaceAll(ansi.Strip(s), "\u00a0", " ")
}

func nodeIDs(nodes []treeNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.id
	}
	return ids
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCursorMovesAndExpands drives a KeyMsg sequence (down/expand/collapse)
// against a fixture Index and asserts both the visible node set and the
// cursor position at each step.
func TestCursorMovesAndExpands(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())

	// Initial state: only the milestone is a root (epic/tasks are parented),
	// and the cursor defaults onto it.
	nodes := m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1"}) {
		t.Fatalf("initial roots = %v, want [ms-1]", got)
	}
	if m.cursorID != "ms-1" {
		t.Fatalf("initial cursorID = %q, want ms-1", m.cursorID)
	}

	// down at the bottom of a 1-row list is a no-op (clamped).
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "ms-1" {
		t.Fatalf("cursorID after down (no-op) = %q, want ms-1", m.cursorID)
	}

	// right: expand the milestone -> its epic becomes visible.
	m = step(t, m, keyMsg(tea.KeyRight))
	nodes = m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1", "ep-1"}) {
		t.Fatalf("after expand ms-1: nodes = %v, want [ms-1 ep-1]", got)
	}

	// down: cursor moves onto the epic.
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "ep-1" {
		t.Fatalf("cursorID after down = %q, want ep-1", m.cursorID)
	}

	// right: expand the epic -> both tasks become visible.
	m = step(t, m, keyMsg(tea.KeyRight))
	nodes = m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1", "ep-1", "tk-1", "tk-2"}) {
		t.Fatalf("after expand ep-1: nodes = %v, want [ms-1 ep-1 tk-1 tk-2]", got)
	}

	// down, down: cursor lands on the last task; a further down clamps there.
	m = step(t, m, keyMsg(tea.KeyDown))
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "tk-2" {
		t.Fatalf("cursorID after 2x down = %q, want tk-2", m.cursorID)
	}
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.cursorID != "tk-2" {
		t.Fatalf("cursor moved past the last row: cursorID = %q, want tk-2 (clamped)", m.cursorID)
	}

	// left on a leaf is a no-op (leaves carry no expand state).
	m = step(t, m, keyMsg(tea.KeyLeft))
	if len(m.visibleNodes()) != 4 {
		t.Fatal("left on a leaf changed the visible node count")
	}

	// enter on a leaf is also a no-op (task scope: leaves don't open detail yet).
	m = step(t, m, keyMsg(tea.KeyEnter))
	if len(m.visibleNodes()) != 4 || m.cursorID != "tk-2" {
		t.Fatal("enter on a leaf must be a no-op")
	}

	// up, up: cursor back on the epic; left collapses it -> tasks disappear.
	m = step(t, m, keyMsg(tea.KeyUp))
	m = step(t, m, keyMsg(tea.KeyUp))
	if m.cursorID != "ep-1" {
		t.Fatalf("cursorID after 2x up = %q, want ep-1", m.cursorID)
	}
	m = step(t, m, keyMsg(tea.KeyLeft))
	nodes = m.visibleNodes()
	if got := nodeIDs(nodes); !equalStrings(got, []string{"ms-1", "ep-1"}) {
		t.Fatalf("after collapse ep-1: nodes = %v, want [ms-1 ep-1]", got)
	}
}

// TestQuitConfirmNoReposConfigured guards the quit-confirm modal's
// open/cancel round trip: q opens it (visible in View()), esc cancels it
// (gone from View()). Its final enter->tea.Quit assertion is the B08/A2
// Randfall specifically (bean bt-1u0t, epic-E8-plan.md Task 5) --
// fixtureModel/newModel never load config.yaml, so m.settings.Repos is nil
// here, which is exactly the "no repos configured" case that still quits
// directly. RENAMED from the pre-B08 "TestQuitConfirm" (which read as if
// q+enter from Browse always quit outright -- true before A2, now only true
// in this Randfall) to avoid pinning stale behavior by name; the cascade's
// other two stages (stage 1: repos configured -> Lobby; stage 2: already in
// the Lobby -> Quit) are covered by box_confirm_quit_test.go's
// TestKeyConfirmQuitEnter* tests.
func TestQuitConfirmNoReposConfigured(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24

	m = step(t, m, runeMsg('q'))
	if !m.confirmQuit {
		t.Fatal("q did not open the quit-confirm")
	}
	if !strings.Contains(m.View(), "Quit?") {
		t.Fatal("View() does not show the quit-confirm once open")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.confirmQuit {
		t.Fatal("esc did not cancel the quit-confirm")
	}
	if strings.Contains(m.View(), "Quit?") {
		t.Fatal("View() still shows the quit-confirm after esc")
	}

	m = step(t, m, runeMsg('q'))
	_, cmd := m.Update(keyMsg(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter on the quit-confirm must return a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("enter's Cmd did not resolve to tea.QuitMsg")
	}
}

// TestCtrlCQuitsImmediately guards the task-scoped distinction from `q`:
// ctrl+c bypasses the confirm entirely.
func TestCtrlCQuitsImmediately(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	_, cmd := m.Update(keyMsg(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("ctrl+c must return a Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c's Cmd did not resolve to tea.QuitMsg")
	}
}

// TestReloadKeepsCursorOnID guards US-10: reloading with the same beans keeps
// the cursor on the same bean ID; reloading with the cursored bean removed
// clamps to the nearest surviving row instead of resetting to the top.
func TestReloadKeepsCursorOnID(t *testing.T) {
	beans := fixtureBeans()
	m := fixtureModel(t, beans)
	m = step(t, m, keyMsg(tea.KeyRight)) // expand ms-1
	m = step(t, m, keyMsg(tea.KeyDown))  // -> ep-1
	m = step(t, m, keyMsg(tea.KeyRight)) // expand ep-1
	m = step(t, m, keyMsg(tea.KeyDown))  // -> tk-1
	m = step(t, m, keyMsg(tea.KeyDown))  // -> tk-2
	if m.cursorID != "tk-2" {
		t.Fatalf("setup: cursorID = %q, want tk-2", m.cursorID)
	}

	// Reload with the same beans (unchanged): cursor stays on tk-2.
	m = step(t, m, beansLoadedMsg{beans: beans})
	if m.cursorID != "tk-2" {
		t.Fatalf("cursor after no-op reload = %q, want tk-2 (unchanged)", m.cursorID)
	}

	// Reload with tk-2 removed: cursor must not still point at it, and must
	// clamp to a bean that is actually still visible in the new tree.
	without := []*item.Item{beans[0], beans[1], beans[2]} // ms-1, ep-1, tk-1
	m = step(t, m, beansLoadedMsg{beans: without})
	if m.cursorID == "tk-2" {
		t.Fatal("cursor still points at the removed bean")
	}
	nodes := m.visibleNodes()
	found := false
	for _, n := range nodes {
		if n.id == m.cursorID {
			found = true
		}
	}
	if !found {
		t.Fatalf("cursorID %q does not match any visible node after reload", m.cursorID)
	}
	// tk-2 sat at the last position (index 3 of 4); with only 3 nodes left
	// (ms-1, ep-1, tk-1) the clamp lands on the new last row, tk-1.
	if m.cursorID != "tk-1" {
		t.Fatalf("cursorID after removal = %q, want tk-1 (clamped to nearest surviving row)", m.cursorID)
	}
}

// errBoom is a minimal error fixture for TestBeansLoadedErrorSurfacesInStatusLine.
type errBoom struct{}

func (errBoom) Error() string { return "boom" }

// TestBeansLoadedErrorSurfacesInStatusLine guards that a failed (re)load
// renders into the status line instead of crashing/blanking the tree.
func TestBeansLoadedErrorSurfacesInStatusLine(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24

	m = step(t, m, beansLoadedMsg{err: errBoom{}})
	if m.err == "" {
		t.Fatal("m.err empty after a failed reload")
	}
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("View() does not surface the load error %q", m.err)
	}
	// The tree itself must survive untouched (idx not clobbered by a failed reload).
	if len(m.visibleNodes()) == 0 {
		t.Error("tree emptied out after a failed reload -- previous Index must be kept")
	}
}

// TestOrphanShownUnderSyntheticRoot guards the MANDATORY orphan rule (bean
// bt-7jr8 / T3-review Q01): a bean with an unresolvable parent must appear
// under a synthetic root ("Not on roadmap"), never
// silently dropped, and never at depth 0 among the milestones.
func TestOrphanShownUnderSyntheticRoot(t *testing.T) {
	beans := append(fixtureBeans(), &item.Item{
		ID: "orph-1", Title: "Orphaned Task", Status: "todo", Type: "task",
		Priority: "normal", Parent: "does-not-exist",
	})
	m := fixtureModel(t, beans)

	nodes := m.visibleNodes()
	var orphanRoot *treeNode
	for i := range nodes {
		if nodes[i].section != "" {
			orphanRoot = &nodes[i]
		}
	}
	if orphanRoot == nil {
		t.Fatal("orphan root node ('(orphaned)') missing from the tree -- orphan silently dropped")
	}
	if orphanRoot.id != notOnRoadmapRootID {
		t.Errorf("orphan root id = %q, want the notOnRoadmapRootID sentinel", orphanRoot.id)
	}

	// Expand the orphan root -> the orphaned bean appears nested under it.
	m.expanded[notOnRoadmapRootID] = true
	nodes = m.visibleNodes()
	found := false
	for _, n := range nodes {
		if n.id == "orph-1" {
			found = true
			if n.depth == 0 {
				t.Error("orphaned bean rendered at depth 0 -- must be nested under the orphan root")
			}
		}
	}
	if !found {
		t.Fatal("orphaned bean not visible even with the orphan root expanded")
	}
}

// TestOrphanBucketUsesCanonicalStatusPriorityTypeTitleOrder guards I03
// closure (bean bt-9ldr, E2 Task 4): the orphan bucket must sort via
// data.SortBeans (Status -> Priority -> Type -> Title, the single-source
// order Index.Children already uses), NOT the tree package's own
// title-only sortByTitleThenID -- the fixture titles are picked so
// alphabetical-title order and canonical-status order DISAGREE, making the
// assertion a real discriminator.
func TestOrphanBucketUsesCanonicalStatusPriorityTypeTitleOrder(t *testing.T) {
	beans := []*item.Item{
		{ID: "root", Title: "Root", Type: "milestone", Status: "todo"},
		{ID: "orph-z", Title: "Z orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing"},
		{ID: "orph-a", Title: "A orphan", Status: "in-progress", Type: "task", Priority: "critical", Parent: "missing"},
	}
	m := fixtureModel(t, beans)
	m.expanded[notOnRoadmapRootID] = true
	nodes := m.visibleNodes()
	// canonical order ranks in-progress before todo -> orph-a (in-progress)
	// must render before orph-z (todo); orph-a also happens to win
	// alphabetically here, but TestOrphanBucketUsesCanonicalOrderOverTitle
	// below picks titles where the two orders disagree, so this pair alone
	// is a sanity check, not the sole discriminator.
	var order []string
	for _, n := range nodes {
		if n.bean != nil && n.bean.Parent == "missing" {
			order = append(order, n.id)
		}
	}
	if len(order) != 2 || order[0] != "orph-a" {
		t.Fatalf("orphan order = %v, want [orph-a orph-z] (canonical status tier, not title)", order)
	}
}

// TestOrphanBucketUsesCanonicalOrderOverTitle is the real discriminator for
// I03: titles are picked so alphabetical order and canonical status order
// DISAGREE ("Alpha" alphabetically first but "scrapped" sorts last) -- this
// fails under the old sortByTitleThenID (title-only) and passes only under
// data.SortBeans.
//
// showArchived is set true here (E5 Task 7, bean bt-ggt2): one of the two
// fixture beans is deliberately "scrapped" to create the sort-order
// discriminator above -- since T7, that status is hidden by DEFAULT
// (beanMatchesArchive, box_filter_facets.go), which would collapse this
// test's own two-orphan setup down to one and defeat its actual point (pure
// canonical-order proof, unrelated to archive visibility). Toggling
// showArchived on keeps both orphans visible so the ordering assertion below
// still exercises what it was written to exercise.
func TestOrphanBucketUsesCanonicalOrderOverTitle(t *testing.T) {
	beans := []*item.Item{
		{ID: "orph-alpha", Title: "Alpha orphan", Status: "scrapped", Type: "task", Priority: "normal", Parent: "missing"},
		{ID: "orph-zulu", Title: "Zulu orphan", Status: "in-progress", Type: "task", Priority: "normal", Parent: "missing"},
	}
	m := fixtureModel(t, beans)
	m.showArchived = true
	m.expanded[notOnRoadmapRootID] = true
	nodes := m.visibleNodes()
	var order []string
	for _, n := range nodes {
		if n.bean != nil && n.bean.Parent == "missing" {
			order = append(order, n.id)
		}
	}
	if len(order) != 2 || order[0] != "orph-zulu" {
		t.Fatalf("orphan order = %v, want [orph-zulu orph-alpha] (in-progress before scrapped, contradicting alphabetical title order)", order)
	}
}

// --- T8 Opus quality review, Runde 2 (bean bt-7jr8) ---

// TestTabTogglesDetailFocusAndPaneAccent guards Q01 end-to-end: tab flips
// m.detailFocus, AND the rendered pane border actually swaps which side
// carries the Mauve (focused) accent -- not just the model flag. Renders
// through the exact same renderPane/renderDetailPane calls viewBrowseRepo
// itself uses (I05 #1).
func TestTabTogglesDetailFocusAndPaneAccent(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	mauve := fgEscape(theme.Mauve)
	// Configured type colours (milestone = mauve) would put Mauve into the
	// detail pane too; only the border accent may carry it here.
	theme.SetConfiguredColors(nil, nil, nil)
	t.Cleanup(func() { applyRepoConfig(nil) })

	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 24
	if m.detailFocus {
		t.Fatal("detailFocus must start false (Tree focused by default)")
	}

	nodes := m.visibleNodes()
	treePane := renderPane(pane{rows: m.treeRows(nodes, !m.detailFocus, 10, 28)}, 30, 10, !m.detailFocus)
	detailPane := m.renderDetailPane(nodes, 30, 10, m.detailFocus)
	if !strings.Contains(treePane, mauve) {
		t.Error("Tree pane not accented while Tree has focus")
	}
	if strings.Contains(detailPane, mauve) {
		t.Error("Detail pane accented while Tree still has focus")
	}

	m = tabIntoDetail(t, m)
	if !m.detailFocus {
		t.Fatal("tab did not flip detailFocus to true")
	}

	treePane = renderPane(pane{rows: m.treeRows(nodes, !m.detailFocus, 10, 28)}, 30, 10, !m.detailFocus)
	detailPane = m.renderDetailPane(nodes, 30, 10, m.detailFocus)
	if strings.Contains(treePane, mauve) {
		t.Error("Tree pane still accented after tab moved focus to Detail")
	}
	if !strings.Contains(detailPane, mauve) {
		t.Error("Detail pane not accented after tab moved focus there")
	}
}

// TestCycleBeansShowUnderOrphanRoot guards B02: beans trapped in a pure
// parent cycle (A -> B -> A) must still surface under a synthetic root
// ("Not on roadmap": the roadmap places them nowhere), and flattening a pure
// cycle must return promptly (no hang).
func TestCycleBeansShowUnderOrphanRoot(t *testing.T) {
	beans := []*item.Item{
		{ID: "cyc-a", Title: "Cycle A", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-b"},
		{ID: "cyc-b", Title: "Cycle B", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-a"},
	}
	m := realModel(t, beans) // must return promptly -- no hang on the cycle

	nodes := m.visibleNodes()
	if len(nodes) != 1 || nodes[0].id != notOnRoadmapRootID {
		t.Fatalf("want exactly the Not-on-roadmap root, got %v", nodeIDs(nodes))
	}

	m.expanded[notOnRoadmapRootID] = true
	nodes = m.visibleNodes()
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n.id] = true
		if n.id != notOnRoadmapRootID && (n.depth != 1 || n.hasKids) {
			t.Errorf("cycle bean %s must be a flat depth-1 row, got depth %d hasKids %v", n.id, n.depth, n.hasKids)
		}
	}
	if !seen["cyc-a"] || !seen["cyc-b"] {
		t.Fatalf("cycle beans not both visible under Not on roadmap: nodes=%v", nodeIDs(nodes))
	}
}

// fiftyFlatBeans returns 50 parentless tasks wnd-00..wnd-49 -- a tree
// taller than any reasonable pane, used to exercise windowing (B01).
func fiftyFlatBeans() []*item.Item {
	beans := make([]*item.Item, 50)
	for i := range beans {
		beans[i] = &item.Item{
			ID:       fmt.Sprintf("wnd-%02d", i),
			Title:    fmt.Sprintf("Window Task %02d", i),
			Status:   "todo",
			Type:     "task",
			Priority: "normal",
		}
	}
	return beans
}

// TestTreeWindowingKeepsCursorVisible guards B01: renderPane alone only
// clips rows to the pane height starting from row 0 -- it has no idea where
// the cursor is. With 50 rows (the Unscheduled root's tasks) and a pane far
// shorter than that, the cursored row must still be part of the rendered
// window (and a row far from the cursor must have scrolled out, or the
// "window" isn't limiting anything).
func TestTreeWindowingKeepsCursorVisible(t *testing.T) {
	beans := fiftyFlatBeans()
	m := fixtureModel(t, beans)
	m.expanded[unscheduledRootID] = true // parentless tasks sit under the Unscheduled root
	m.width, m.height = 100, 21          // bodyH-2 == 10 visible rows, far less than 50 nodes
	m.cursorID = "wnd-40"

	out := m.View()
	if !strings.Contains(out, "wnd-40") {
		t.Fatalf("cursored row (wnd-40) not visible in View() with 50 nodes / ~10 visible rows -- windowing missing/broken:\n%s", out)
	}
	if strings.Contains(out, "wnd-00") {
		t.Error("row far from the cursor (wnd-00) still visible -- window did not scroll")
	}
}

// --- E2 Task 2 (bean bt-2jve): I01 (expanded-map copy-on-write), Q01 (Init()
// nil-guard covered in app_test.go), Fokus-Maschine (keyDetailFocus). ---

// TestSetExpandedDoesNotMutateSharedMapAcrossModelCopies guards I01 (bean
// bt-7jr8, T8-review): setExpanded must clone m.expanded before writing, not
// mutate the shared backing map in place -- a struct copy (Go map values are
// reference types) must never see the other copy's expand-state change.
func TestSetExpandedDoesNotMutateSharedMapAcrossModelCopies(t *testing.T) {
	base := model{expanded: map[string]bool{}}
	copy1 := base // struct copy -- map HEADER copied, backing array still shared pre-fix
	node := treeNode{id: "x", hasKids: true}
	copy2 := copy1.setExpanded(node, true)

	if copy1.expanded["x"] {
		t.Error("copy1.expanded was mutated by copy2's setExpanded call -- " +
			"expanded is a shared map, not copy-on-write (I01, bean bt-7jr8 T8-review)")
	}
	if !copy2.expanded["x"] {
		t.Error("copy2.expanded should carry the new expand state")
	}
}

// fixtureBeansWithBlocking is a milestone with two epics, bean-a (under
// ep-1, Blocking bean-b) and bean-b (under ep-2) -- used to exercise the
// Beziehungen-field jump + ancestor-expand (bean-b's ancestor ep-2 starts
// collapsed, so the jump must expand it).
func fixtureBeansWithBlocking() []*item.Item {
	return []*item.Item{
		{ID: "ms-1", Title: "Milestone", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		{ID: "ep-2", Title: "Epic Two", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		{ID: "bean-a", Title: "Bean A", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-1", Blocking: []string{"bean-b"}},
		{ID: "bean-b", Title: "Bean B", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-2"},
	}
}

// TestFocusedBeanDispatchesOnTreeCursorInBrowseView guards focusedBean()'s
// Browse-view dispatch (devd port focusedIssue, view_detail_issue.go:20-35):
// in viewBrowseRepo it must return the currently tree-cursored bean.
func TestFocusedBeanDispatchesOnTreeCursorInBrowseView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	b := m.focusedBean()
	if b == nil || b.ID != "ms-1" {
		t.Fatalf("focusedBean() = %v, want ms-1", b)
	}
}

// --- D03 (design-spec.md §15 PF-16, bean bt-ntoz): esc as the Detail-Focus
// cascade's ONLY multi-step back path now that B01 removed it from left/j.
// ---

// --- E7 T6 (PF-2/PF-5/PF-13, bean bt-t1uy): shift+tab exit + enter-cascade. ---

// TestKeyNodeActionEditorAlwaysOpensBeanEditor is D01's core end-to-end
// regression test (design-spec.md §15 PF-17, bean bt-z4b1, supersedet E8-
// B10): e/ctrl+e must open the SAME whole-bean $EDITOR path from EVERY
// realistic entry point -- Tree (no detail focus), Ready (no detail
// focus), and Detail focus parked on EVERY section/field level (META at
// section AND field level, BODY, RELATIONS, HISTORY). Testing only one
// entry point was exactly ../docs/LESSONS-LEARNED.md's Forward-Guard #3 gap
// ("neue Kaskaden IMMER end-to-end testen -- JEDE Stufe, aus jedem
// realistischen Ausgangszustand") -- this test exists specifically so D01's
// "egal an welcher Stelle" claim does not repeat that mistake. Never opens
// the Title-Edit-Form (the former section-context-sensitive B10 branching
// is GONE, no exceptions).
func TestKeyNodeActionEditorAlwaysOpensBeanEditor(t *testing.T) {
	setup := func(t *testing.T) (model, *item.Item) {
		t.Helper()
		m := fixtureModel(t, fixtureBeans())
		m = focusBean(m, "tk-1")
		return m, m.idx.ByID["tk-1"]
	}

	cases := []struct {
		name  string
		setup func(m model) model
	}{
		{"tree, no detail focus", func(m model) model { return m }},
		{"detail focus, first field", func(m model) model {
			m.layout, m.detailFocus = layoutFilterDetail, true
			return m
		}},
		{"detail focus, field cursor on status", func(m model) model {
			m.layout, m.detailFocus = layoutFilterDetail, true
			m.boxFormCursor, m.boxFormCursorBean = 1, "tk-1"
			return m
		}},
	}

	for _, tc := range cases {
		for _, key := range []tea.KeyMsg{runeMsg('e'), keyMsg(tea.KeyCtrlE)} {
			t.Run(fmt.Sprintf("%s/%s", tc.name, key.String()), func(t *testing.T) {
				m, b := setup(t)
				m = tc.setup(m)

				tm, cmd := m.Update(key)
				nm, ok := tm.(model)
				if !ok {
					t.Fatalf("Update did not return a model, got %T", tm)
				}
				if nm.editorTarget != b.ID {
					t.Fatalf("editorTarget = %q, want %q", nm.editorTarget, b.ID)
				}
				if nm.editorETag != b.ETag() {
					t.Fatalf("editorETag = %q, want %q", nm.editorETag, b.ETag())
				}
				if nm.editorSnapshot == nil || nm.editorSnapshot.ID != b.ID {
					t.Fatalf("editorSnapshot = %v, want a snapshot of %q", nm.editorSnapshot, b.ID)
				}
				if nm.form != nil {
					t.Fatal("e/ctrl+e must NEVER open the Title-Edit-Form (D01)")
				}
				if cmd == nil {
					t.Fatal("e/ctrl+e must return a Cmd (the ShowRaw read, first Cmd-hop)")
				}
			})
		}
	}

	// Ready entry point -- a separate m.view/focusedBean() path
	// (focusedBean's viewReady case, update.go).
	for _, key := range []tea.KeyMsg{runeMsg('e'), keyMsg(tea.KeyCtrlE)} {
		t.Run("ready, no detail focus/"+key.String(), func(t *testing.T) {
			m := readyBeansModel(t)
			m.view = viewReady
			m.readyList.setLen(len(m.readyVisible()))
			m.readyList.cursor = 0
			b := m.readySelected()
			if b == nil {
				t.Fatal("setup: readySelected() is nil")
			}

			tm, cmd := m.Update(key)
			nm, ok := tm.(model)
			if !ok {
				t.Fatalf("Update did not return a model, got %T", tm)
			}
			if nm.editorTarget != b.ID {
				t.Fatalf("editorTarget = %q, want %q", nm.editorTarget, b.ID)
			}
			if nm.editorSnapshot == nil || nm.editorSnapshot.ID != b.ID {
				t.Fatalf("editorSnapshot = %v, want a snapshot of %q", nm.editorSnapshot, b.ID)
			}
			if nm.form != nil {
				t.Fatal("e/ctrl+e must NEVER open the Title-Edit-Form (D01)")
			}
			if cmd == nil {
				t.Fatal("e/ctrl+e must return a Cmd")
			}
		})
	}
}

// TestKeyDetailFocusEnterOnCreatedAtFieldNoop / ...UpdatedAtField... (the
// "readonly" kind's Noop guards, indices 5/6) were REMOVED by bt-lg68 (PO-
// Nebenbefund, US-Review Runde 3): created_at/updated_at no longer exist as
// Meta fields (doubly rendered vs. HISTORY) -- metaFields shrinks to 5
// entries (title/status/type/priority/tags), so these two indices are gone
// and the "readonly" kind can no longer be reached via metaFields/enter-
// dispatch. activateDetailField's own "readonly" case (update.go) becomes
// dead code by the same removal (see its doc comment).

// --- B07 (design-spec.md §15 PF-16, bean bt-duz7, E8 Task 4):
// activateDetailField extraction -- direct unit coverage for the extracted
// helper itself (mirrors the 7 keyDetailFocus enter-cascade cases above,
// now against the shared helper both keyDetailFocus AND mouse.go's
// mouseDetailClick call, Architektur-Vorgabe #1). ---

// TestActivateDetailFieldReadonlyIsNoop (the "readonly" kind guard) was
// REMOVED by bt-lg68: metaFields no longer produces "readonly" entries
// (created_at/updated_at removed, HISTORY is the sole source), and
// activateDetailField's own "readonly" switch case was removed in lockstep
// as dead code (see its doc comment, update.go) -- there is no longer a
// production path that constructs relationField{kind: "readonly"}.

// markerOnLineContaining is a small render-grounded assertion helper: true
// if some line of s contains BOTH substr and marker.
func markerOnLineContaining(s, substr, marker string) bool {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, substr) && strings.Contains(l, marker) {
			return true
		}
	}
	return false
}

// TestKeyDetailFocusOnOrphanRootExitsGracefully is a defensive nil-safety
// test for focusedBean()'s orphan-guard: cursoring the synthetic
// "(orphaned)" root itself (not a real bean) must never panic and must exit
// detail focus rather than getting stuck.
func TestKeyDetailFocusOnOrphanRootExitsGracefully(t *testing.T) {
	beans := append(fixtureBeans(), &item.Item{
		ID: "orph-1", Title: "Orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing",
	})
	m := fixtureModel(t, beans)
	m.cursorID = notOnRoadmapRootID

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyRight)) // must not panic
	if m.detailFocus {
		t.Fatal("keyDetailFocus on an orphan-root cursor (no focusable bean) must exit detail focus")
	}
}

// TestWindowAroundStableAtEdges guards windowStart/windowAround directly
// (devd port): fits-entirely is a no-op, the window never runs past either
// edge, and the cursor stays inside the returned window at every position --
// no jitter, no out-of-range slice.
func TestWindowAroundStableAtEdges(t *testing.T) {
	rows := make([]string, 20)
	for i := range rows {
		rows[i] = fmt.Sprintf("row-%02d", i)
	}

	if got := windowAround(rows, 30, 5); len(got) != 20 {
		t.Fatalf("windowAround with height >= len(rows) must be a no-op, got %d rows", len(got))
	}

	for _, cursor := range []int{0, 1, 5, 10, 14, 15, 19} {
		win := windowAround(rows, 8, cursor)
		if len(win) != 8 {
			t.Fatalf("cursor=%d: window has %d rows, want 8", cursor, len(win))
		}
		want := fmt.Sprintf("row-%02d", cursor)
		found := false
		for _, r := range win {
			if r == want {
				found = true
			}
		}
		if !found {
			t.Errorf("cursor=%d: cursored row %q not in window %v", cursor, want, win)
		}
	}
}

// --- E2-T2 Quality-Review, Runde 2 (bean bt-2jve) ---

// TestExpandAncestorsOfHandlesParentCycle guards B01 (Critical, Review-Runde
// 2): expandAncestorsOf's ancestor walk had no visited-set, so a cyclic
// parent map hung it forever on any relation-jump into the cycle, freezing
// the whole TUI. Run with a bounded `go test -timeout` so a regression fails
// loudly (timeout) instead of hanging the suite.
func TestExpandAncestorsOfHandlesParentCycle(t *testing.T) {
	treeParent := map[string]string{"cyc-a": "cyc-b", "cyc-b": "cyc-a"}

	out := expandAncestorsOf(treeParent, map[string]bool{}, "cyc-a")
	if !out["cyc-b"] {
		t.Error("expandAncestorsOf did not mark cyc-a's immediate parent (cyc-b) expanded")
	}
}

// TestRepoSwitchClearsToast is the T6-Review Prelude I01 regression guard
// (T6b, bean bt-pd22): a Toast surfaced against the OLD repo -- sticky or
// not -- must not linger visible after a successful repo switch.
// applyRepoSwitched clears it UNCONDITIONALLY (update.go), unlike
// applyLoaded's clearToastUnlessSticky (which protects a sticky toast
// across a SAME-repo background reload) -- a repo switch is a bigger
// session discontinuity, so even the one sticky ErrConflict toast kind must
// not survive it. sticky=true here is the worst case (the ONE toast kind
// that otherwise survives every OTHER reload path, showToast's own doc
// comment).
func TestRepoSwitchClearsToast(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // applyRepoSwitched's best-effort config.SetLastRepo must not touch the real $HOME

	m := fixtureModel(t, fixtureBeans())
	m, _ = m.showToast(toastError, "Conflict: bean changed externally", "", nil, true)
	if m.toast == nil {
		t.Fatal("setup: showToast did not set m.toast")
	}

	newRepo := t.TempDir()
	tm, _ := m.applyRepoSwitched(repoSwitchedMsg{
		store:   newTestStore(t),
		repoDir: newRepo,
		beans:   []*item.Item{},
	})
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("applyRepoSwitched did not return a model, got %T", tm)
	}
	if m2.toast != nil {
		t.Fatalf("m.toast = %+v after a successful repo switch, want nil (even a sticky toast from the OLD repo must clear)", m2.toast)
	}
}
