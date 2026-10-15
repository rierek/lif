package tui

// update_test.go — TDD coverage for the App-Shell Update dispatcher (T8,
// implementation-plan.md »E1 Task 8«): cursor movement/expand/collapse,
// quit-confirm, reload-keeps-cursor-by-ID, and the MANDATORY orphan rule
// (bean bt-7jr8 / T3-review Q01). Also covers the T8 Opus quality-review
// follow-ups (bean bt-7jr8, Runde 2): tree windowing (B01), cycle-bean
// visibility (B02), and tab focus-swap accenting (I05 #1).

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// fixtureBeans is a small milestone -> epic -> 2 tasks hierarchy shared by
// the Update tests below.
func fixtureBeans() []data.Bean {
	return []data.Bean{
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
func fixtureModel(t *testing.T, beans []data.Bean) model {
	t.Helper()
	return step(t, newModel(nil, "/tmp/bt-fixture-repo"), beansLoadedMsg{beans: beans})
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
	without := []data.Bean{beans[0], beans[1], beans[2]} // ms-1, ep-1, tk-1
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
// under the synthetic "(orphaned)" root, never silently dropped, and never
// leak into the real Roots() top level (its Parent is non-empty).
func TestOrphanShownUnderSyntheticRoot(t *testing.T) {
	beans := append(fixtureBeans(), data.Bean{
		ID: "orph-1", Title: "Orphaned Task", Status: "todo", Type: "task",
		Priority: "normal", Parent: "does-not-exist",
	})
	m := fixtureModel(t, beans)

	nodes := m.visibleNodes()
	var orphanRoot *treeNode
	for i := range nodes {
		if nodes[i].orphan {
			orphanRoot = &nodes[i]
		}
	}
	if orphanRoot == nil {
		t.Fatal("orphan root node ('(orphaned)') missing from the tree -- orphan silently dropped")
	}
	if orphanRoot.id != orphanRootID {
		t.Errorf("orphan root id = %q, want the orphanRootID sentinel", orphanRoot.id)
	}

	for _, b := range m.idx.Roots() {
		if b.ID == "orph-1" {
			t.Fatal("orphaned bean leaked into the real Roots() top level")
		}
	}

	// Expand the orphan root -> the orphaned bean appears nested under it.
	m.expanded[orphanRootID] = true
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
// order Index.Children/Roots() already use), NOT the tree package's own
// title-only sortByTitleThenID -- the fixture titles are picked so
// alphabetical-title order and canonical-status order DISAGREE, making the
// assertion a real discriminator.
func TestOrphanBucketUsesCanonicalStatusPriorityTypeTitleOrder(t *testing.T) {
	beans := []data.Bean{
		{ID: "root", Title: "Root", Type: "milestone", Status: "todo"},
		{ID: "orph-z", Title: "Z orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing"},
		{ID: "orph-a", Title: "A orphan", Status: "in-progress", Type: "task", Priority: "critical", Parent: "missing"},
	}
	m := fixtureModel(t, beans)
	m.expanded[orphanRootID] = true
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
	beans := []data.Bean{
		{ID: "orph-alpha", Title: "Alpha orphan", Status: "scrapped", Type: "task", Priority: "normal", Parent: "missing"},
		{ID: "orph-zulu", Title: "Zulu orphan", Status: "in-progress", Type: "task", Priority: "normal", Parent: "missing"},
	}
	m := fixtureModel(t, beans)
	m.showArchived = true
	m.expanded[orphanRootID] = true
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

	m = step(t, m, keyMsg(tea.KeyTab))
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
// parent cycle (A -> B -> A) are unreachable from any root, but their Parent
// DOES resolve (to each other) -- so collectOrphans alone would never catch
// them either. Both must still surface under the synthetic "(orphaned)"
// root, and flattening a pure cycle must return promptly (no hang).
func TestCycleBeansShowUnderOrphanRoot(t *testing.T) {
	beans := []data.Bean{
		{ID: "cyc-a", Title: "Cycle A", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-b"},
		{ID: "cyc-b", Title: "Cycle B", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-a"},
	}
	m := fixtureModel(t, beans) // must return promptly -- no hang on the cycle

	nodes := m.visibleNodes()
	foundOrphanRoot := false
	for _, n := range nodes {
		if n.orphan {
			foundOrphanRoot = true
		}
	}
	if !foundOrphanRoot {
		t.Fatal("orphan root missing even though both beans are unreachable (pure cycle)")
	}
	for _, b := range m.idx.Roots() {
		if b.ID == "cyc-a" || b.ID == "cyc-b" {
			t.Fatal("cycle bean leaked into the real Roots() top level")
		}
	}

	m.expanded[orphanRootID] = true
	nodes = m.visibleNodes()
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n.id] = true
	}
	if !seen["cyc-a"] || !seen["cyc-b"] {
		t.Fatalf("cycle beans not both visible under (orphaned): nodes=%v", nodeIDs(nodes))
	}
}

// fiftyFlatBeans returns 50 flat (unparented) tasks wnd-00..wnd-49 -- a tree
// taller than any reasonable pane, used to exercise windowing (B01).
func fiftyFlatBeans() []data.Bean {
	beans := make([]data.Bean, 50)
	for i := range beans {
		beans[i] = data.Bean{
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
// the cursor is. With 50 flat root nodes and a pane far shorter than that,
// the cursored row must still be part of the rendered window (and a row far
// from the cursor must have scrolled out, or the "window" isn't limiting
// anything).
func TestTreeWindowingKeepsCursorVisible(t *testing.T) {
	beans := fiftyFlatBeans()
	m := fixtureModel(t, beans)
	m.width, m.height = 100, 21 // bodyH-2 == 10 visible rows, far less than 50 nodes
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
func fixtureBeansWithBlocking() []data.Bean {
	return []data.Bean{
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

// TestDetailFocusDigitJumpOpensMatchingSection guards the ziffer-jump 1..4:
// each digit jumps straight to its matching section (0-based secCursor,
// 1-based accOpen per renderAccordion's digit header). Table-driven (I02,
// Review-Runde 2, bean bt-2jve) -- previously only '3' had coverage; this
// folds it into the same table alongside 1/2/4.
func TestDetailFocusDigitJumpOpensMatchingSection(t *testing.T) {
	cases := []struct {
		digit         rune
		wantSecCursor int
		wantAccOpen   int
	}{
		{'1', 0, 1},
		{'2', 1, 2},
		{'3', 2, 3},
		{'4', 3, 4},
	}
	for _, tc := range cases {
		t.Run(string(tc.digit), func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m.cursorID = "ms-1"
			m = step(t, m, keyMsg(tea.KeyTab))
			m = step(t, m, runeMsg(tc.digit))
			if m.secCursor != tc.wantSecCursor || m.accOpen != tc.wantAccOpen {
				t.Fatalf("digit jump '%c': secCursor=%d accOpen=%d, want %d/%d",
					tc.digit, m.secCursor, m.accOpen, tc.wantSecCursor, tc.wantAccOpen)
			}
		})
	}
}

// TestDetailFocusUpDownMovesSectionCursorClampedAtEnds guards section-level
// i/k navigation: 4 fixed sections, down/up both clamp at the ends instead
// of running past them.
func TestDetailFocusUpDownMovesSectionCursorClampedAtEnds(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	m = step(t, m, keyMsg(tea.KeyTab))
	for i := 0; i < 5; i++ { // 5 downs on 4 sections must clamp at index 3
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if m.secCursor != 3 {
		t.Fatalf("secCursor after 5x down = %d, want 3 (clamped)", m.secCursor)
	}
	for i := 0; i < 5; i++ {
		m = step(t, m, keyMsg(tea.KeyUp))
	}
	if m.secCursor != 0 {
		t.Fatalf("secCursor after 5x up = %d, want 0 (clamped)", m.secCursor)
	}
}

// TestDetailFocusRightEntersFieldLevelOnlyForSectionsWithFields guards that
// right/l enters field level exactly on sections that carry fields. E7 T4
// (PF-4, bean bt-kyj5) turns Meta into a 6-entry navigable field list
// (previously fieldless) -- this invalidates the OLD test's "Meta has no
// fields" premise, so Meta now joins Relations as a fields-bearing section;
// Body remains the fieldless negative control. Renamed from
// ...OnlyForBeziehungenSection (PF-7 also renamed the section RELATIONS).
func TestDetailFocusRightEntersFieldLevelOnlyForSectionsWithFields(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab)) // secCursor=0 (Meta)
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 1 {
		t.Fatal("right on Meta (PF-4: 6 navigable fields) must enter field level")
	}

	m = step(t, m, runeMsg('2')) // Body -- digit jump resets detailLevel to 0
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 0 {
		t.Fatal("right on Body (no fields) must stay at section level")
	}

	m = step(t, m, runeMsg('3')) // Relations
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 1 {
		t.Fatal("right on Relations (has fields) must enter field level")
	}
}

// TestDetailFocusEnterOnRelationJumpsCursorAndExitsToTree guards the
// Beziehungs-Sprung: enter on a resolved relationField moves the tree cursor
// to the target bean, exits detail focus, AND expands the target's ancestor
// chain so it is actually visible in the next visibleNodes() call.
func TestDetailFocusEnterOnRelationJumpsCursorAndExitsToTree(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.width, m.height = 100, 30
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3')) // Beziehungen
	if m.secCursor != 2 || m.accOpen != 3 {
		t.Fatalf("setup: secCursor=%d accOpen=%d, want 2/3", m.secCursor, m.accOpen)
	}
	m = step(t, m, keyMsg(tea.KeyRight)) // field level: fields = [Parent ep-1, Blocking bean-b]
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}
	m = step(t, m, keyMsg(tea.KeyDown)) // fieldCursor 0 (ep-1) -> 1 (bean-b)
	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.cursorID != "bean-b" {
		t.Fatalf("cursorID after enter-jump = %q, want bean-b", m.cursorID)
	}
	if m.detailFocus {
		t.Fatal("enter-jump must exit detail focus")
	}
	if !m.expanded["ep-2"] {
		t.Fatal("bean-b's parent (ep-2) must be expanded after the jump so it is visible")
	}
	found := false
	for _, n := range m.visibleNodes() {
		if n.id == "bean-b" {
			found = true
		}
	}
	if !found {
		t.Fatal("bean-b not visible in the tree after the relation-jump (ancestor expand missing)")
	}
}

// TestDetailFocusEnterOnUnresolvedRelationIsNoOp guards the jump-guard: a
// relationField with beanID == "" (dangling reference) must not move the
// cursor or exit detail focus.
func TestDetailFocusEnterOnUnresolvedRelationIsNoOp(t *testing.T) {
	beans := []data.Bean{
		{ID: "dangling-1", Title: "Dangling Bean", Status: "todo", Type: "task", Priority: "normal",
			BlockedBy: []string{"does-not-exist"}},
	}
	m := fixtureModel(t, beans)
	m.width, m.height = 100, 30
	m.cursorID = "dangling-1"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3')) // Beziehungen
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}
	m = step(t, m, keyMsg(tea.KeyEnter))

	if !m.detailFocus {
		t.Fatal("enter on an unresolved relation must NOT exit detail focus")
	}
	if m.detailLevel != 1 {
		t.Fatal("enter on an unresolved relation must stay in field level")
	}
	if m.cursorID != "dangling-1" {
		t.Fatal("enter on an unresolved relation must not move the tree cursor")
	}
}

// TestDetailFocusLeftAtFieldLevelReturnsToSectionLevel guards left/j at
// field level: it steps back to section level without exiting detail focus.
func TestDetailFocusLeftAtFieldLevelReturnsToSectionLevel(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3'))
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}
	m = step(t, m, keyMsg(tea.KeyLeft))
	if m.detailLevel != 0 {
		t.Fatal("left at field level must return to section level")
	}
	if !m.detailFocus {
		t.Fatal("left at field level must NOT exit detail focus")
	}
}

// TestDetailFocusLeftAtSectionLevelIsNoOp guards B01 (design-spec.md §15
// PF-16, bean bt-ntoz, PF-13-Pfeil-Revision): left/j at section level is now
// a no-op -- it must NOT exit detail focus anymore. Renamed+inverted from
// the former TestDetailFocusLeftAtSectionLevelExitsDetailFocus, which pinned
// the OLD (asymmetric) behavior B01 explicitly revokes: arrow keys are pure
// navigation, never a focus-exit -- exclusively tab/shift+tab (PF-13) and
// now esc's cascade (D03, below) change m.detailFocus.
func TestDetailFocusLeftAtSectionLevelIsNoOp(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	m = step(t, m, keyMsg(tea.KeyTab))
	if !m.detailFocus {
		t.Fatal("setup: expected detail focus on")
	}
	m = step(t, m, keyMsg(tea.KeyLeft))
	if !m.detailFocus {
		t.Fatal("left at section level must NOT exit detail focus anymore (B01)")
	}
	if m.detailLevel != 0 {
		t.Fatalf("left at section level must not change detailLevel either: got %d, want 0", m.detailLevel)
	}
}

// --- D03 (design-spec.md §15 PF-16, bean bt-ntoz): esc as the Detail-Focus
// cascade's ONLY multi-step back path now that B01 removed it from left/j.
// ---

// TestKeyDetailFocusEscAtFieldLevelGoesToSectionLevel guards D03's first
// cascade rung: esc at field level (detailLevel==1) steps back to section
// level, mirroring left's EXISTING (unchanged) field->section behavior --
// detail focus itself stays on.
func TestKeyDetailFocusEscAtFieldLevelGoesToSectionLevel(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3'))
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.detailLevel != 0 {
		t.Fatal("esc at field level must return to section level")
	}
	if !m.detailFocus {
		t.Fatal("esc at field level must NOT exit detail focus (one rung at a time, D03)")
	}
}

// TestKeyDetailFocusEscAtSectionLevelExitsDetailFocus guards D03's second
// cascade rung: esc at section level (detailLevel==0) exits detail focus
// entirely -- the exact two-stage cascade B01 removed from left/j (Feld ->
// Sektion -> Fokus verlassen), now living exclusively behind esc.
func TestKeyDetailFocusEscAtSectionLevelExitsDetailFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	m = step(t, m, keyMsg(tea.KeyTab))
	if !m.detailFocus {
		t.Fatal("setup: expected detail focus on")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.detailFocus {
		t.Fatal("esc at section level must exit detail focus")
	}
}

// TestKeyDetailFocusEscNoopOutsideDetailFocus is a regression guard: esc
// pressed while the Tree already has focus (m.detailFocus already false)
// must NOT accidentally enter detail focus or otherwise reach
// keyDetailFocus's new esc-handler -- handleKey only routes to
// keyDetailFocus while m.detailFocus is true (update.go), so esc outside
// detail focus is handled entirely by keyTree's OWN (pre-existing,
// unchanged by this task) esc-cascade rungs (search_test.go
// TestSearchEscWhileTypingCancelsAndClearsQuery/
// TestSearchEscAfterCommitClearsQuery cover those already).
func TestKeyDetailFocusEscNoopOutsideDetailFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	if m.detailFocus {
		t.Fatal("setup: expected detail focus off")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.detailFocus {
		t.Fatal("esc outside detail focus must not turn detail focus on")
	}
}

// --- F01 (design-spec.md §15, E9 Task 7, bean bt-13l7): fullscreenDetail
// integration with the EXISTING Detail-Fokus-Maschine (focusedBean's new
// fullscreenDetail case, activateDetailField's new Relations-Sprung case,
// keyDetailFocus's Back-case esc-exit rung). keyFullscreen itself + the
// fullscreenList side are covered in view_fullscreen_test.go instead. ---

// TestFocusedBeanResolvesFullscreenBeanID guards focusedBean()'s NEW,
// vorrangig geprüften fullscreenDetail case: m.fullscreenBeanID wins over
// whatever the Tree cursor currently points at (the two are deliberately
// INDEPENDENT, design-spec.md §15).
func TestFocusedBeanResolvesFullscreenBeanID(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1" // deliberately a DIFFERENT bean than fullscreenBeanID
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "tk-2"

	b := m.focusedBean()
	if b == nil || b.ID != "tk-2" {
		t.Fatalf("focusedBean() = %v, want tk-2 (fullscreenBeanID takes precedence over the Tree cursor)", b)
	}
}

// TestFocusedBeanFullscreenDetailUnknownBeanIDReturnsNil is a defensive
// nil-safety guard: an unresolved fullscreenBeanID (e.g. a bean deleted
// externally while the Vollbild was open) must not panic.
func TestFocusedBeanFullscreenDetailUnknownBeanIDReturnsNil(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "does-not-exist"

	if b := m.focusedBean(); b != nil {
		t.Fatalf("focusedBean() = %v, want nil for an unresolved fullscreenBeanID", b)
	}
}

// TestActivateDetailFieldJumpStaysInFullscreenDetail guards the ONE new case
// activateDetailField's Relations-jump default gets: inside fullscreenDetail
// a jump must NOT exit to the Split-Tree/Backlog (m.detailFocus stays
// whatever it was) -- it re-targets fullscreenBeanID and reinitializes the
// Detail-Fokus-Maschine, same reset shape as keyFullscreen's own
// Listen-Vollbild-entry (view_fullscreen_test.go).
func TestActivateDetailFieldJumpStaysInFullscreenDetail(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.detailFocus = true
	m.cursorID = "unrelated"
	m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 2, 3, 1, 1
	b := m.focusedBean()

	tm, cmd := m.activateDetailField(b, relationField{kind: "", beanID: "bean-b"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}
	if nm.fullscreen != fullscreenDetail {
		t.Fatalf("fullscreen = %v, want unchanged fullscreenDetail (Relations-Sprung must NOT exit Vollbild)", nm.fullscreen)
	}
	if nm.fullscreenBeanID != "bean-b" {
		t.Fatalf("fullscreenBeanID = %q, want bean-b (jump target)", nm.fullscreenBeanID)
	}
	if nm.cursorID != "unrelated" {
		t.Fatalf("cursorID = %q, want unchanged (fullscreenDetail jump must not touch the Tree/Backlog cursor)", nm.cursorID)
	}
	if !nm.detailFocus {
		t.Fatal("detailFocus must stay unchanged (true) -- the fullscreenDetail jump branch never touches it")
	}
	if nm.secCursor != 0 || nm.accOpen != 1 || nm.detailLevel != 0 || nm.fieldCursor != 0 {
		t.Fatalf("Detail-Fokus-Maschine not reset: secCursor=%d accOpen=%d detailLevel=%d fieldCursor=%d, want 0/1/0/0",
			nm.secCursor, nm.accOpen, nm.detailLevel, nm.fieldCursor)
	}
	if cmd != nil {
		t.Fatal("activateDetailField(fullscreenDetail jump) must not return a Cmd")
	}
}

// TestActivateDetailFieldJumpPushesHistoryInFullscreen is the bean's own
// explicitly-named TDD step (F01 History-Stack, E9 Task 8, bean bt-1vbp):
// a Relations-Sprung inside fullscreenDetail pushes the bean being LEFT
// onto navBack (Notes for T8, bean bt-13l7: the push happens BEFORE
// m.fullscreenBeanID is overwritten).
func TestActivateDetailFieldJumpPushesHistoryInFullscreen(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	b := m.focusedBean()

	tm, _ := m.activateDetailField(b, relationField{kind: "", beanID: "bean-b"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}

	if len(nm.navBack) != 1 || nm.navBack[0] != "bean-a" {
		t.Fatalf("navBack = %v, want [bean-a] (the bean being left)", nm.navBack)
	}
	if nm.fullscreenBeanID != "bean-b" {
		t.Fatalf("fullscreenBeanID = %q, want bean-b", nm.fullscreenBeanID)
	}
}

// TestActivateDetailFieldJumpClearsForwardHistoryOnNewJump guards the
// Standard-Browser-Semantik design-spec.md §15 calls for: a FRESH jump
// kappt navForward, even if it was non-empty (a previous Back left stale
// forward-history that a new jump must discard, not silently keep around).
func TestActivateDetailFieldJumpClearsForwardHistoryOnNewJump(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.navForward = []string{"stale-forward-entry"}
	b := m.focusedBean()

	tm, _ := m.activateDetailField(b, relationField{kind: "", beanID: "bean-b"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}

	if nm.navForward != nil {
		t.Fatalf("navForward = %v, want nil (a fresh jump must clear stale forward-history)", nm.navForward)
	}
}

// TestActivateDetailFieldJumpDoesNotMutateSharedNavBackAcrossModelCopies
// pins the I01 Copy-on-Write convention at this History-Push's own call
// site -- mirrors TestSetExpandedDoesNotMutateSharedMapAcrossModelCopies'
// own rationale (below), applied to navBack's []string instead of
// m.expanded's map[string]bool.
func TestActivateDetailFieldJumpDoesNotMutateSharedNavBackAcrossModelCopies(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.navBack = []string{"pre-existing"}
	copy1 := m
	b := copy1.focusedBean()

	tm, _ := copy1.activateDetailField(b, relationField{kind: "", beanID: "bean-b"})
	copy2, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}

	if len(m.navBack) != 1 {
		t.Errorf("original m.navBack was mutated by copy2's History-Push -- navBack is a shared slice, not copy-on-write (I01)")
	}
	if len(copy2.navBack) != 2 || copy2.navBack[0] != "pre-existing" || copy2.navBack[1] != "bean-a" {
		t.Fatalf("copy2.navBack = %v, want [pre-existing bean-a]", copy2.navBack)
	}
}

// TestKeyDetailFocusRoutesToDetailFocusMachineViaFullscreenAlone is a
// regression guard for handleKey's own routing line (update.go): it must
// fire on m.fullscreen == fullscreenDetail even when m.detailFocus is FALSE
// -- inside the Vollbild-Detail, m.detailFocuss truth value is irrelevant to
// the dispatch decision (design-spec.md §15), the Vollbild state itself is
// the signal-giver.
func TestKeyDetailFocusRoutesToDetailFocusMachineViaFullscreenAlone(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	if m.detailFocus {
		t.Fatal("setup: expected detailFocus false")
	}

	m = step(t, m, runeMsg('3')) // digit-jump -- only keyDetailFocus understands this

	if m.secCursor != relationsSectionIdx || m.accOpen != relationsSectionIdx+1 {
		t.Fatalf("digit-jump did not reach keyDetailFocus: secCursor=%d accOpen=%d, want %d/%d",
			m.secCursor, m.accOpen, relationsSectionIdx, relationsSectionIdx+1)
	}
}

// TestKeyDetailFocusEscFieldLevelStepsToSectionLevelInFullscreen is
// TestKeyDetailFocusEscAtFieldLevelGoesToSectionLevel's regression pin under
// fullscreenDetail (D03's first cascade rung, unchanged): field level steps
// back to section level, staying inside the Vollbild.
func TestKeyDetailFocusEscFieldLevelStepsToSectionLevelInFullscreen(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"

	m = step(t, m, runeMsg('3'))         // Relations section
	m = step(t, m, keyMsg(tea.KeyRight)) // field level
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.detailLevel != 0 {
		t.Fatal("esc at field level must return to section level (D03, unchanged rung)")
	}
	if m.fullscreen != fullscreenDetail {
		t.Fatal("esc at field level must NOT leave the Vollbild (one rung at a time, D03)")
	}
}

// TestKeyDetailFocusEscSectionLevelExitsFullscreenToTree guards the NEW D03
// rung (F01): esc at section level leaves fullscreenDetail entirely, syncs
// the Tree cursor onto the last-shown Vollbild bean (PO: "mit dem AKTUELLEN
// Bean selektiert") and expands its ancestors so it is actually visible.
func TestKeyDetailFocusEscSectionLevelExitsFullscreenToTree(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-b" // Parent ep-2, under ms-1 -- both start collapsed
	m.cursorID = "irrelevant"

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if !m.detailFocus {
		t.Fatal("esc-exit must leave the Split-Detail focused (PO: 'mit dem AKTUELLEN Bean selektiert')")
	}
	if m.cursorID != "bean-b" {
		t.Fatalf("cursorID = %q, want bean-b (synced to the last-shown Vollbild bean)", m.cursorID)
	}
	if !m.expanded["ep-2"] {
		t.Fatal("esc-exit must expand bean-b's ancestor (ep-2) so it is visible in the Split-Tree")
	}
}

// TestKeyDetailFocusEscOnVanishedFullscreenBeanExitsFullscreen guards
// Review-Finding F01 (Fix-Runde 1, bean bt-13l7, Schwere hoch): when
// fullscreenBeanID vanishes externally (live watch-reload, repo switch,
// parallel-agent delete -- real in the dogfooding repo), focusedBean()
// returns nil and keyDetailFocus's b==nil guard fires BEFORE the Back-case
// is ever reached. Pre-fix it only cleared m.detailFocus, never
// m.fullscreen -- the PO was trapped in a dead fullscreenDetail with no
// keyboard exit (esc and every other detail key re-hit the same guard,
// only Quit escaped). The guard must ALSO leave the Vollbild.
func TestKeyDetailFocusEscOnVanishedFullscreenBeanExitsFullscreen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "does-not-exist" // vanished externally

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone (b==nil guard must exit the Vollbild, not leave the PO trapped)", m.fullscreen)
	}
	if m.detailFocus {
		t.Fatal("detailFocus must be cleared too (existing guard behavior, unchanged)")
	}
}

// TestKeyDetailFocusEscExitsFullscreenToTreeWhenTargetNotBacklogVisible
// guards Review-Finding F02 (Fix-Runde 1, bean bt-13l7, SUPERVISOR-
// ENTSCHEID "Fallback = View-Wechsel"): after Relations-Sprünge inside a
// Backlog-origin fullscreenDetail, the exit target is USUALLY not
// backlogVisible() anymore (has a Parent / is Epic/Milestone / wrong
// status) -- the pre-fix sync loop then silently found no hit and left
// backlogList.cursor stale, showing a DIFFERENT bean selected (violates
// the PO criterion "mit dem aktuellen Bean selektiert"). Fix: esc switches
// to Browse/Tree instead (the Tree can show EVERY bean), cursor on the
// target + ancestors expanded. Only a still-backlogVisible target keeps
// the existing Backlog sync (TestKeyDetailFocusEscSectionLevelExits
// FullscreenToBacklogWithCursorSynced below, unchanged).
func TestKeyDetailFocusEscExitsFullscreenToTreeWhenTargetNotBacklogVisible(t *testing.T) {
	beans := []data.Bean{
		{ID: "fb-root", Title: "Root Epic", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "fb-a", Title: "Backlog A", Status: "todo", Type: "task", Priority: "normal", Blocking: []string{"fb-child"}},
		{ID: "fb-child", Title: "Child Task", Status: "todo", Type: "task", Priority: "normal", Parent: "fb-root"},
	}
	m := fixtureModel(t, beans)
	m.view = viewBacklog
	// Post-jump state (Reviewer-Repro): Backlog-origin Vollbild now shows
	// fb-child -- parented, therefore NOT in backlogVisible().
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "fb-child"
	m.backlogList.setLen(len(m.backlogVisible()))
	m.backlogList.cursor = 0

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.view != viewBrowseRepo {
		t.Fatalf("view = %v, want viewBrowseRepo (F02 fallback: Tree can show every bean, Backlog cannot)", m.view)
	}
	if m.cursorID != "fb-child" {
		t.Fatalf("cursorID = %q, want fb-child (PO: 'mit dem aktuellen Bean selektiert')", m.cursorID)
	}
	if !m.expanded["fb-root"] {
		t.Fatal("fb-child's ancestor (fb-root) must be expanded so the target is actually visible")
	}
	if !m.detailFocus {
		t.Fatal("esc-exit must leave the Split-Detail focused (unchanged exit contract)")
	}
}

// TestKeyDetailFocusEscSectionLevelExitsFullscreenToBacklogWithCursorSynced
// is the Backlog mirror: backlogList.cursor syncs onto the last-shown
// Vollbild bean's index in backlogVisible() instead of the Tree cursor/
// expand map.
func TestKeyDetailFocusEscSectionLevelExitsFullscreenToBacklogWithCursorSynced(t *testing.T) {
	m := fixtureModel(t, fullscreenBacklogBeans())
	m.view = viewBacklog
	vis := m.backlogVisible()
	if len(vis) < 2 {
		t.Fatalf("setup: need >=2 backlog-visible beans, got %d", len(vis))
	}
	target := vis[1]
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = target.ID
	m.backlogList.setLen(len(vis))
	m.backlogList.cursor = 0 // stale, must be synced onto target

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if !m.detailFocus {
		t.Fatal("esc-exit must leave the Split-Detail focused")
	}
	if m.backlogList.cursor != 1 {
		t.Fatalf("backlogList.cursor = %d, want 1 (synced onto %s)", m.backlogList.cursor, target.ID)
	}
}

// TestKeyDetailFocusEscSectionLevelExitClearsHistoryStacks guards a
// SUPERVISOR-ENTSCHEID deviation from design-spec.md §15's original text
// ("navBack/navForward werden beim Verlassen NICHT geleert" -- bt-13l7's own
// "Notes for T8" quoted the same line verbatim as something T8 need not
// touch). Task 8's Supervisor explicitly OVERRODE that decision: every
// Vollbild-Exit now clears BOTH stacks, so a stale History-Path from one
// fullscreenDetail session can never leak into the next (ERRATA -- see this
// task's Deviations/commit body).
func TestKeyDetailFocusEscSectionLevelExitClearsHistoryStacks(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-b"
	m.navBack = []string{"bean-a"}
	m.navForward = []string{"ep-1"}

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.navBack != nil {
		t.Fatalf("navBack = %v, want nil (Vollbild-Exit must clear it, Supervisor-Entscheid)", m.navBack)
	}
	if m.navForward != nil {
		t.Fatalf("navForward = %v, want nil (Vollbild-Exit must clear it, Supervisor-Entscheid)", m.navForward)
	}
}

// TestKeyDetailFocusEscOnVanishedFullscreenBeanClearsHistoryStacks mirrors
// the above for the OTHER Vollbild-Exit choke point: the F01 b==nil dead-end
// guard (Fix-Runde 1) also leaves fullscreenDetail entirely and must clear
// the same two stacks for the same reason.
func TestKeyDetailFocusEscOnVanishedFullscreenBeanClearsHistoryStacks(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "does-not-exist"
	m.navBack = []string{"ms-1"}
	m.navForward = []string{"tk-1"}

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.navBack != nil || m.navForward != nil {
		t.Fatalf("navBack=%v navForward=%v, want both nil (b==nil guard is a Vollbild-Exit too)", m.navBack, m.navForward)
	}
}

// --- E7 T6 (PF-2/PF-5/PF-13, bean bt-t1uy): shift+tab exit + enter-cascade. ---

// TestKeyShiftTabExitsDetailFocus guards PF-13's new deterministic exit:
// shift+tab flips m.detailFocus to false while leaving every other Detail-
// Focus cursor field (secCursor/accOpen/detailLevel/fieldCursor) untouched --
// unlike tab-IN (which resets all four), shift+tab is a pure focus-out, no
// state reset (PO-Nachtrag 7: "vorhersagbare Paare", not a second reset path).
func TestKeyShiftTabExitsDetailFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3')) // Relations
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.secCursor != 2 || m.accOpen != 3 || m.detailLevel != 1 {
		t.Fatalf("setup: secCursor=%d accOpen=%d detailLevel=%d, want 2/3/1", m.secCursor, m.accOpen, m.detailLevel)
	}

	m = step(t, m, keyMsg(tea.KeyShiftTab))
	if m.detailFocus {
		t.Fatal("shift+tab must exit detail focus")
	}
	if m.secCursor != 2 || m.accOpen != 3 || m.detailLevel != 1 {
		t.Fatalf("shift+tab must not reset the Detail-Focus cursor state: secCursor=%d accOpen=%d detailLevel=%d, want 2/3/1 (unchanged)",
			m.secCursor, m.accOpen, m.detailLevel)
	}
}

// TestKeyShiftTabNoopWhenNotInDetailFocus guards the no-op case: shift+tab
// while the Tree already has focus must not do anything surprising (no
// panic, cursorID/view untouched).
func TestKeyShiftTabNoopWhenNotInDetailFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	if m.detailFocus {
		t.Fatal("setup: expected detail focus off")
	}
	before := m.cursorID
	m = step(t, m, keyMsg(tea.KeyShiftTab))
	if m.detailFocus {
		t.Fatal("shift+tab must stay a no-op when Tree already has focus")
	}
	if m.cursorID != before {
		t.Fatalf("shift+tab moved cursorID from %q to %q while a no-op", before, m.cursorID)
	}
}

// TestKeyTabStillTogglesBothDirections is a regression guard (PF-13): tab
// keeps its existing bidirectional toggle behavior after the raw
// `msg.String()=="tab"` comparison is replaced by keybind.Matches(msg,
// keys.FocusIn) -- same key, same effect, now routed through the typed
// binding.
func TestKeyTabStillTogglesBothDirections(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, keyMsg(tea.KeyTab))
	if !m.detailFocus {
		t.Fatal("tab (1st press) did not enter detail focus")
	}
	m = step(t, m, keyMsg(tea.KeyTab))
	if m.detailFocus {
		t.Fatal("tab (2nd press) did not exit detail focus")
	}
}

// TestKeyDetailFocusEnterAtSectionLevelEntersFieldLevel guards PF-5's new
// section-level enter alias: on Meta (which has fields, PF-4), enter behaves
// exactly like right/l -- enters field level at fieldCursor 0.
func TestKeyDetailFocusEnterAtSectionLevelEntersFieldLevel(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab)) // secCursor=0 (Meta), detailLevel=0
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.detailLevel != 1 {
		t.Fatal("enter on Meta (has fields) must enter field level")
	}
	if m.fieldCursor != 0 {
		t.Fatalf("fieldCursor = %d, want 0", m.fieldCursor)
	}
	if !m.detailFocus {
		t.Fatal("enter at section level must not exit detail focus")
	}
}

// TestKeyDetailFocusEnterAtSectionLevelNoopWithoutFields guards the same
// alias's guard condition using History (section 4, historySectionIdx) --
// like Body, History carries no .fields, so enter must stay at section
// level (mirrors the existing right/l behavior for fieldless sections). Body
// itself briefly had a special-cased $EDITOR-opening exception here (E8
// Task 6, bean bt-y2iw, B10) -- D01 (design-spec.md §15 PF-17, bean bt-z4b1)
// REVERTED that exception ersatzlos (TestKeyDetailFocusEnterOnBodyIsNoOpAgain
// below), so Body is once again just another fieldless section covered by
// this same generic invariant.
func TestKeyDetailFocusEnterAtSectionLevelNoopWithoutFields(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('4')) // History -- no fields
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.detailLevel != 0 {
		t.Fatal("enter on History (no fields) must stay at section level")
	}
	if m.form != nil || m.overlay != overlayNone {
		t.Fatal("enter on History (no fields) must be a pure no-op -- no form/overlay")
	}
}

// TestKeyDetailFocusEnterOnBodyIsNoOpAgain is the B10-Revision regression
// test (D01, design-spec.md §15 PF-17, bean bt-z4b1, supersedes E8-B10):
// enter on Section [2] BODY used to open $EDITOR (B10) -- that special case
// is now REMOVED ersatzlos ("PO's neues Mentalmodell reserviert '$EDITOR
// öffnen' ausschließlich für e"). BODY carries no .fields, so enter must
// fall straight through to the generic fields>0 guard and do nothing --
// exactly the pre-E8 state, mirrors
// TestKeyDetailFocusEnterAtSectionLevelNoopWithoutFields's assertions for
// History.
func TestKeyDetailFocusEnterOnBodyIsNoOpAgain(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('2')) // Body -- no fields
	if m.detailLevel != 0 {
		t.Fatal("setup: expected section level")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.detailLevel != 0 {
		t.Fatal("enter on Body (no fields) must stay at section level")
	}
	if nm.form != nil || nm.overlay != overlayNone {
		t.Fatal("enter on Body (no fields) must be a pure no-op -- no form/overlay")
	}
	if nm.editorTarget != "" || nm.editorETag != "" || nm.editorSnapshot != nil {
		t.Fatal("enter on Body must NEVER open $EDITOR anymore (B10-Revision, D01) -- that is e/ctrl+e's job exclusively")
	}
	if cmd != nil {
		t.Fatal("enter on Body (no fields) must return a nil Cmd")
	}
}

// TestKeyNodeActionEditorAlwaysOpensBeanEditor is D01's core end-to-end
// regression test (design-spec.md §15 PF-17, bean bt-z4b1, supersedet E8-
// B10): e/ctrl+e must open the SAME whole-bean $EDITOR path from EVERY
// realistic entry point -- Tree (no detail focus), Backlog (no detail
// focus), and Detail focus parked on EVERY section/field level (META at
// section AND field level, BODY, RELATIONS, HISTORY). Testing only one
// entry point was exactly ../docs/LESSONS-LEARNED.md's Forward-Guard #3 gap
// ("neue Kaskaden IMMER end-to-end testen -- JEDE Stufe, aus jedem
// realistischen Ausgangszustand") -- this test exists specifically so D01's
// "egal an welcher Stelle" claim does not repeat that mistake. Never opens
// the Title-Edit-Form (the former section-context-sensitive B10 branching
// is GONE, no exceptions).
func TestKeyNodeActionEditorAlwaysOpensBeanEditor(t *testing.T) {
	setup := func(t *testing.T) (model, *data.Bean) {
		t.Helper()
		beans := fixtureBeans()
		for i := range beans {
			if beans[i].ID == "tk-1" {
				beans[i].ETag = "tk-1-etag"
			}
		}
		m := fixtureModel(t, beans)
		m = focusBean(m, "tk-1")
		return m, m.idx.ByID["tk-1"]
	}

	cases := []struct {
		name  string
		setup func(m model) model
	}{
		{"tree, no detail focus", func(m model) model { return m }},
		{"detail focus, META section level", func(m model) model {
			m.detailFocus, m.secCursor, m.detailLevel = true, metaSectionIdx, 0
			return m
		}},
		{"detail focus, META field level", func(m model) model {
			m.detailFocus, m.secCursor, m.detailLevel, m.fieldCursor = true, metaSectionIdx, 1, 0
			return m
		}},
		{"detail focus, BODY section", func(m model) model {
			m.detailFocus, m.secCursor, m.detailLevel = true, bodySectionIdx, 0
			return m
		}},
		{"detail focus, RELATIONS section", func(m model) model {
			m.detailFocus, m.secCursor, m.detailLevel = true, relationsSectionIdx, 0
			return m
		}},
		{"detail focus, HISTORY section", func(m model) model {
			m.detailFocus, m.secCursor, m.detailLevel = true, historySectionIdx, 0
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
				if nm.editorETag != b.ETag {
					t.Fatalf("editorETag = %q, want %q", nm.editorETag, b.ETag)
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

	// Backlog entry point -- a separate m.view/focusedBean() path
	// (focusedBean's viewBacklog case, update.go).
	for _, key := range []tea.KeyMsg{runeMsg('e'), keyMsg(tea.KeyCtrlE)} {
		t.Run("backlog, no detail focus/"+key.String(), func(t *testing.T) {
			beans := backlogBeans()
			for i := range beans {
				beans[i].ETag = beans[i].ID + "-etag"
			}
			m := fixtureModel(t, beans)
			m.view = viewBacklog
			m.backlogList.setLen(len(m.backlogVisible()))
			m.backlogList.cursor = 0
			b := m.backlogSelected()
			if b == nil {
				t.Fatal("setup: backlogSelected() is nil")
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

// metaFieldCursorTo steps `down` n times from a just-entered Meta field level
// (fieldCursor 0, title) to reach the field at index n -- mirrors metaFields'
// fixed order (title/status/type/priority/tags, PF-15/D01; shrunk from 7 to
// 5 by bt-lg68).
func metaFieldCursorTo(t *testing.T, m model, n int) model {
	t.Helper()
	for i := 0; i < n; i++ {
		m = step(t, m, keyMsg(tea.KeyDown))
	}
	if m.fieldCursor != n {
		t.Fatalf("setup: fieldCursor = %d, want %d", m.fieldCursor, n)
	}
	return m
}

// TestKeyDetailFocusEnterOnStatusFieldOpensValueMenuSeededToStatus guards
// PF-5's Meta field->Overlay dispatch: enter on the status field (index 1)
// opens the combined Value-Menu seeded on the "status" group at the bean's
// CURRENT status -- and, per design decision (epic-E7-plan.md Task 6 Step
// 7), m.detailFocus stays true (the overlay lays on top, no exit).
func TestKeyDetailFocusEnterOnStatusFieldOpensValueMenuSeededToStatus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter)) // section -> field level, fieldCursor=0 (title)
	m = metaFieldCursorTo(t, m, 1)       // status

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	if m.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", m.mutTarget)
	}
	want := valueMenuCursorFor(m.menuItems, "status", "todo")
	if m.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (status/todo)", m.menu.cursor, want)
	}
	if !m.detailFocus {
		t.Fatal("detailFocus must stay true while the seeded overlay is open (design decision, Task 6 Step 7)")
	}
}

// TestKeyDetailFocusEnterOnTypeFieldOpensValueMenuSeededToType is the "type"
// group counterpart (field index 2).
func TestKeyDetailFocusEnterOnTypeFieldOpensValueMenuSeededToType(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // type=task
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter))
	m = metaFieldCursorTo(t, m, 2) // type

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	want := valueMenuCursorFor(m.menuItems, "type", "task")
	if m.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (type/task)", m.menu.cursor, want)
	}
}

// TestKeyDetailFocusEnterOnPriorityFieldOpensValueMenuSeededToPriority is the
// "priority" group counterpart (field index 3).
func TestKeyDetailFocusEnterOnPriorityFieldOpensValueMenuSeededToPriority(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // priority=normal
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter))
	m = metaFieldCursorTo(t, m, 3) // priority

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	want := valueMenuCursorFor(m.menuItems, "priority", "normal")
	if m.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (priority/normal)", m.menu.cursor, want)
	}
}

// TestKeyDetailFocusEnterOnTitleFieldOpensEditTitleForm guards the "title"
// kind dispatch: enter on the title field (index 0, the default fieldCursor
// right after entering field level) opens the SAME Title-Edit-Form the `e`
// key opens (openEditTitleForm).
func TestKeyDetailFocusEnterOnTitleFieldOpensEditTitleForm(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter)) // field level, fieldCursor=0 (title)

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.form == nil {
		t.Fatal("enter on the title field must open a form")
	}
	if m.formKind != "editTitle" {
		t.Fatalf("formKind = %q, want editTitle", m.formKind)
	}
	if m.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", m.mutTarget)
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

// TestKeyDetailFocusEnterOnTagsFieldOpensTagPicker guards D01's Enter-Kaskade
// dispatch (design-spec.md §15 PF-15, bean bt-e6q9): enter on the tags field
// (index 4, the last Meta field since bt-lg68 removed created_at/updated_at)
// opens the SAME Tag-Picker the
// `t` key opens (m.openTagPicker()) -- and, mirroring status/type/priority,
// m.detailFocus stays true (the overlay lays on top as its own capture
// state, D02 "schnell/einfach").
func TestKeyDetailFocusEnterOnTagsFieldOpensTagPicker(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter)) // section -> field level, fieldCursor=0 (title)
	m = metaFieldCursorTo(t, m, 4)       // tags

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.overlay != overlayTagPicker {
		t.Fatalf("overlay = %v, want overlayTagPicker", m.overlay)
	}
	if m.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", m.mutTarget)
	}
	if !m.detailFocus {
		t.Fatal("detailFocus must stay true while the seeded overlay is open (mirrors status/type/priority, Task 6 Step 7 design decision)")
	}
}

// --- B07 (design-spec.md §15 PF-16, bean bt-duz7, E8 Task 4):
// activateDetailField extraction -- direct unit coverage for the extracted
// helper itself (mirrors the 7 keyDetailFocus enter-cascade cases above,
// now against the shared helper both keyDetailFocus AND mouse.go's
// mouseDetailClick call, Architektur-Vorgabe #1). ---

// TestActivateDetailFieldStatusOpensValueMenu guards the status/type/
// priority group: activateDetailField opens the seeded Value-Menu, m.
// detailFocus is left untouched by the helper itself (callers set it).
func TestActivateDetailFieldStatusOpensValueMenu(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo
	b := m.focusedBean()

	tm, cmd := m.activateDetailField(b, relationField{kind: "status"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}
	if nm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", nm.overlay)
	}
	if nm.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", nm.mutTarget)
	}
	want := valueMenuCursorFor(nm.menuItems, "status", "todo")
	if nm.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (status/todo)", nm.menu.cursor, want)
	}
	if cmd != nil {
		t.Fatal("activateDetailField(status) must not return a Cmd")
	}
}

// TestActivateDetailFieldTypeOpensValueMenu is the "type" group counterpart.
func TestActivateDetailFieldTypeOpensValueMenu(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // type=task
	b := m.focusedBean()

	tm, _ := m.activateDetailField(b, relationField{kind: "type"})
	nm := tm.(model)
	if nm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", nm.overlay)
	}
	want := valueMenuCursorFor(nm.menuItems, "type", "task")
	if nm.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (type/task)", nm.menu.cursor, want)
	}
}

// TestActivateDetailFieldPriorityOpensValueMenu is the "priority" group
// counterpart.
func TestActivateDetailFieldPriorityOpensValueMenu(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // priority=normal
	b := m.focusedBean()

	tm, _ := m.activateDetailField(b, relationField{kind: "priority"})
	nm := tm.(model)
	if nm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", nm.overlay)
	}
	want := valueMenuCursorFor(nm.menuItems, "priority", "normal")
	if nm.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (priority/normal)", nm.menu.cursor, want)
	}
}

// TestActivateDetailFieldTagsOpensTagPicker guards the "tags" kind.
func TestActivateDetailFieldTagsOpensTagPicker(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	b := m.focusedBean()

	tm, _ := m.activateDetailField(b, relationField{kind: "tags"})
	nm := tm.(model)
	if nm.overlay != overlayTagPicker {
		t.Fatalf("overlay = %v, want overlayTagPicker", nm.overlay)
	}
	if nm.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", nm.mutTarget)
	}
}

// TestActivateDetailFieldTitleOpensEditTitleForm guards the "title" kind.
func TestActivateDetailFieldTitleOpensEditTitleForm(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	b := m.focusedBean()

	tm, cmd := m.activateDetailField(b, relationField{kind: "title"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}
	if nm.form == nil || nm.formKind != "editTitle" {
		t.Fatalf("form/formKind = %v/%q, want a non-nil form with formKind editTitle", nm.form, nm.formKind)
	}
	if nm.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", nm.mutTarget)
	}
	if cmd == nil {
		t.Fatal("activateDetailField(title) must return the form's Init Cmd")
	}
}

// TestActivateDetailFieldReadonlyIsNoop (the "readonly" kind guard) was
// REMOVED by bt-lg68: metaFields no longer produces "readonly" entries
// (created_at/updated_at removed, HISTORY is the sole source), and
// activateDetailField's own "readonly" switch case was removed in lockstep
// as dead code (see its doc comment, update.go) -- there is no longer a
// production path that constructs relationField{kind: "readonly"}.

// TestActivateDetailFieldJumpMovesCursorAndExitsDetailFocus guards the
// default ("") Relations-jump kind: unchanged E2 behavior, now reached via
// the shared helper.
func TestActivateDetailFieldJumpMovesCursorAndExitsDetailFocus(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.width, m.height = 100, 30
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"
	m.detailFocus = true
	b := m.focusedBean()

	tm, cmd := m.activateDetailField(b, relationField{kind: "", beanID: "bean-b"})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}
	if nm.cursorID != "bean-b" {
		t.Fatalf("cursorID = %q, want bean-b", nm.cursorID)
	}
	if nm.detailFocus {
		t.Fatal("jump must exit detail focus")
	}
	if cmd != nil {
		t.Fatal("activateDetailField(jump) must not return a Cmd")
	}
}

// TestActivateDetailFieldJumpUnresolvedIsNoop guards the jump kind's
// dangling-reference guard: an empty beanID (unresolved reference) must not
// move the cursor or exit detail focus.
func TestActivateDetailFieldJumpUnresolvedIsNoop(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m.detailFocus = true
	b := m.focusedBean()

	tm, cmd := m.activateDetailField(b, relationField{kind: "", beanID: ""})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("activateDetailField did not return a model, got %T", tm)
	}
	if nm.cursorID != "tk-2" {
		t.Fatalf("cursorID = %q, want unchanged tk-2", nm.cursorID)
	}
	if !nm.detailFocus {
		t.Fatal("an unresolved jump target must not exit detail focus")
	}
	if cmd != nil {
		t.Fatal("activateDetailField(unresolved jump) must not return a Cmd")
	}
}

// TestKeyDetailFocusEnterOnRelationFieldStillJumps is a regression guard: the
// Relations section's field kind ("", the default relationField zero value)
// must still hit the unchanged jump branch after keyDetailFocus's enter-on-
// field-level block becomes a kind-switch (already covered end-to-end by
// TestDetailFocusEnterOnRelationJumpsCursorAndExitsToTree/
// TestDetailFocusEnterOnUnresolvedRelationIsNoOp above -- this pins the same
// invariant under the T6 test name the plan calls for).
func TestKeyDetailFocusEnterOnRelationFieldStillJumps(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.width, m.height = 100, 30
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3')) // Relations
	m = step(t, m, keyMsg(tea.KeyRight))
	m = step(t, m, keyMsg(tea.KeyDown)) // fieldCursor 0 (ep-1) -> 1 (bean-b)
	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.cursorID != "bean-b" {
		t.Fatalf("cursorID after enter-jump = %q, want bean-b", m.cursorID)
	}
	if m.detailFocus {
		t.Fatal("enter-jump must still exit detail focus (unchanged E2 behavior)")
	}
}

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

// TestKeyDetailFocusArrowNavigatesRelationsRows guards B04.2 (design-spec.md
// §15 PF-17, bean bt-b0w0): keyDetailFocus's EXISTING up/down field-cursor
// logic (secs[m.secCursor].fields -- UNCHANGED by this task) now visibly
// selects the REAL Relations rows (Parent/Blocking/...) instead of the
// removed fieldStrip. Render-grounded (LESSONS-LEARNED #4/#6: state-only
// fieldCursor assertions -- already covered by
// TestKeyDetailFocusEnterOnRelationFieldStillJumps above -- would miss a
// render-side regression, e.g. the marker staying stuck on row 0).
func TestKeyDetailFocusArrowNavigatesRelationsRows(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.width, m.height = 100, 30
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a" // Parent=ep-1, Blocking=[bean-b] -- 2 Relations fields

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3')) // Relations
	m = step(t, m, keyMsg(tea.KeyRight))

	// fieldCursor 0 -> Parent (ep-1) row carries ▶.
	rendered0 := ansi.Strip(m.renderBeanAccordionPane(m.focusedBean(), 60, 30, true))
	if !markerOnLineContaining(rendered0, "ep-1", "▶") {
		t.Fatalf("fieldCursor=0: expected ▶ on the Parent (ep-1) row, got:\n%s", rendered0)
	}

	m = step(t, m, keyMsg(tea.KeyDown)) // fieldCursor 0 -> 1 (Blocking bean-b)
	rendered1 := ansi.Strip(m.renderBeanAccordionPane(m.focusedBean(), 60, 30, true))
	if !markerOnLineContaining(rendered1, "bean-b", "▶") {
		t.Fatalf("fieldCursor=1: expected ▶ on the Blocking (bean-b) row, got:\n%s", rendered1)
	}
	if markerOnLineContaining(rendered1, "ep-1", "▶") {
		t.Fatalf("fieldCursor=1: the Parent (ep-1) row must no longer carry ▶, got:\n%s", rendered1)
	}
}

// TestKeyDetailFocusDigitJumpUsesBeanSectionCount guards PF-2's robustness
// fix: the digit-jump range check now compares against beanSectionCount (4)
// instead of a hardcoded '4' literal -- digit 5 (out of range) must stay a
// no-op, exactly as it already was before the refactor (regression pin, not
// a behavior change).
func TestKeyDetailFocusDigitJumpUsesBeanSectionCount(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, keyMsg(tea.KeyTab)) // secCursor=0, accOpen=1 (tab-in reset)

	m = step(t, m, runeMsg('5'))

	if m.secCursor != 0 || m.accOpen != 1 {
		t.Fatalf("digit 5 (beyond beanSectionCount=%d) must stay a no-op: secCursor=%d accOpen=%d, want 0/1",
			beanSectionCount, m.secCursor, m.accOpen)
	}
}

// TestKeyDetailFocusOnOrphanRootExitsGracefully is a defensive nil-safety
// test for focusedBean()'s orphan-guard: cursoring the synthetic
// "(orphaned)" root itself (not a real bean) must never panic and must exit
// detail focus rather than getting stuck.
func TestKeyDetailFocusOnOrphanRootExitsGracefully(t *testing.T) {
	beans := append(fixtureBeans(), data.Bean{
		ID: "orph-1", Title: "Orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing",
	})
	m := fixtureModel(t, beans)
	m.cursorID = orphanRootID

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
// 2): expandAncestorsOf's ancestor walk had no visited-set, so a Parent
// cycle in the on-disk data (A -> B -> A -- beans-legal, frontmatter is
// hand-editable) hung it forever on any relation-jump into the cycle,
// freezing the whole TUI. Run with a bounded `go test -timeout` so a
// regression fails loudly (timeout) instead of hanging the suite.
func TestExpandAncestorsOfHandlesParentCycle(t *testing.T) {
	beans := []data.Bean{
		{ID: "cyc-a", Title: "Cycle A", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-b"},
		{ID: "cyc-b", Title: "Cycle B", Status: "todo", Type: "task", Priority: "normal", Parent: "cyc-a"},
	}
	idx := data.NewIndex(beans)

	out := expandAncestorsOf(idx, map[string]bool{}, "cyc-a")
	if !out["cyc-b"] {
		t.Error("expandAncestorsOf did not mark cyc-a's immediate parent (cyc-b) expanded")
	}
}

// TestFieldCursorClampsAfterReloadShrinksRelations guards B02 (Critical,
// Review-Runde 2): m.secCursor/m.fieldCursor are model state that survives a
// beansLoadedMsg reload untouched -- if a watch-reload shrinks the focused
// bean's Beziehungen fields while the user is parked at field level,
// keyDetailFocus's `secs[m.secCursor].fields[m.fieldCursor]` indexed out of
// range and panicked. Repro: drill into field 1 of 2, reload down to 1
// relation, press enter -- must not panic, must clamp to a sane state.
func TestFieldCursorClampsAfterReloadShrinksRelations(t *testing.T) {
	beans := []data.Bean{
		{ID: "root-1", Title: "Root", Status: "todo", Type: "task", Priority: "normal",
			Blocking: []string{"blk-1", "blk-2"}},
		{ID: "blk-1", Title: "Blocker One", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "blk-2", Title: "Blocker Two", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.width, m.height = 100, 30
	m.cursorID = "root-1"

	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, runeMsg('3'))         // Beziehungen
	m = step(t, m, keyMsg(tea.KeyRight)) // field level, fieldCursor 0 (blk-1)
	if m.detailLevel != 1 {
		t.Fatal("setup: expected field level")
	}
	m = step(t, m, keyMsg(tea.KeyDown)) // fieldCursor 1 (blk-2) -- field 2 of 2
	if m.fieldCursor != 1 {
		t.Fatalf("setup: fieldCursor = %d, want 1", m.fieldCursor)
	}

	// A watch-reload shrinks root-1's Blocking down to 1 entry while the
	// user is still parked at field index 1 -- fieldCursor is now stale
	// relative to the freshly computed secs.
	shrunk := []data.Bean{
		{ID: "root-1", Title: "Root", Status: "todo", Type: "task", Priority: "normal",
			Blocking: []string{"blk-1"}},
		{ID: "blk-1", Title: "Blocker One", Status: "todo", Type: "task", Priority: "normal"},
	}
	m = step(t, m, beansLoadedMsg{beans: shrunk})

	// A key press right after the reload (no jump side-effect: "up" only
	// decrements if fieldCursor > 0) must clamp the stale fieldCursor into
	// range for the shrunk section without panicking -- direct clamp
	// evidence, still focused on root-1.
	m = step(t, m, keyMsg(tea.KeyUp))
	if m.fieldCursor != 0 {
		t.Fatalf("fieldCursor after reload+clamp = %d, want 0 (only 1 field left)", m.fieldCursor)
	}

	// Enter must not panic either, even though fieldCursor was stale
	// relative to the pre-reload (2-field) section shape: clamped to the
	// sole remaining (resolved) relation, it now behaves like a normal
	// relation-jump -- itself sane B02 behavior, not a special case.
	m = step(t, m, keyMsg(tea.KeyEnter))
	if m.fieldCursor < 0 {
		t.Fatalf("fieldCursor went negative: %d", m.fieldCursor)
	}
}

// TestTabReentryResetsStaleDetailFocusState guards I01 (Important,
// Review-Runde 2): re-entering detail focus after having drilled into a deep
// section/field state must never leak that state into the new visit -- tab
// always re-enters the accordion at Meta/section level/field 0 (Q01,
// handleKey's tab case, update.go). Previously untested end-to-end.
func TestTabReentryResetsStaleDetailFocusState(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyTab)) // tab in
	m = step(t, m, runeMsg('3'))       // section 3 (Beziehungen)
	m = step(t, m, keyMsg(tea.KeyRight))
	if m.secCursor != 2 || m.accOpen != 3 || m.detailLevel != 1 {
		t.Fatalf("setup: secCursor=%d accOpen=%d detailLevel=%d, want 2/3/1",
			m.secCursor, m.accOpen, m.detailLevel)
	}

	m = step(t, m, keyMsg(tea.KeyTab)) // tab out
	if m.detailFocus {
		t.Fatal("setup: tab did not exit detail focus")
	}
	m = step(t, m, keyMsg(tea.KeyTab)) // tab back in
	if !m.detailFocus {
		t.Fatal("setup: tab did not re-enter detail focus")
	}
	if m.secCursor != 0 || m.accOpen != 1 || m.detailLevel != 0 || m.fieldCursor != 0 {
		t.Fatalf("stale detail-focus state leaked across tab re-entry: secCursor=%d accOpen=%d detailLevel=%d fieldCursor=%d, want 0/1/0/0",
			m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor)
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
		client:  &data.Client{RepoDir: newRepo},
		repoDir: newRepo,
		beans:   []data.Bean{},
	})
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("applyRepoSwitched did not return a model, got %T", tm)
	}
	if m2.toast != nil {
		t.Fatalf("m.toast = %+v after a successful repo switch, want nil (even a sticky toast from the OLD repo must clear)", m2.toast)
	}
}
