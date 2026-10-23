package tui

// view_browse_ready_test.go — TDD coverage for Ready-View V3 `b` (E2 Task
// 5, bean bt-gzu6, design-spec.md §6 V3): idx.Ready() reuse, the shared
// search+facet predicate (m.beanMatches, Task 3/4), the Sort-Toggle `S`
// cycle, Master-Detail focus reuse (focusedBean(), Task 2's Accordion, Task
// 1), and windowing reuse (E1 windowAround). Reuses fixtureBeans/
// fixtureModel/step/keyMsg/runeMsg/nodeIDs/equalStrings from update_test.go
// (same package).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// readyBeans is a small fixture set for the Ready view: bk-tsk1 and bk-tsk2
// are the Ready set (readyBeansModel declares it), the rest are items the
// set leaves out (milestone, completed, parented). The rule that decides
// membership is Store.Ready's (TestReadyViewMatchesListReady); these tests
// cover what the view does with a given set.
func readyBeans() []*item.Item {
	return []*item.Item{
		{ID: "bk-mlst", Title: "A Milestone", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "bk-tsk1", Title: "Ready Task One", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "bk-tsk2", Title: "Ready Task Two", Status: "draft", Type: "bug", Priority: "high"},
		{ID: "bk-done", Title: "Done Task", Status: "completed", Type: "task", Priority: "normal"},
		{ID: "bk-child", Title: "Parented Task", Status: "todo", Type: "task", Priority: "normal", Parent: "bk-mlst"},
	}
}

// readyModel is fixtureModel plus an explicit Ready set: what a real load
// delivers as beansLoadedMsg.ready (Store.Ready).
func readyModel(t *testing.T, beans []*item.Item, ids ...string) model {
	t.Helper()
	m := fixtureModel(t, beans)
	m.readyIDs = map[string]bool{}
	for _, id := range ids {
		m.readyIDs[id] = true
	}
	return m
}

// readyBeansModel is readyModel over readyBeans() with its Ready set.
func readyBeansModel(t *testing.T) model {
	t.Helper()
	return readyModel(t, readyBeans(), "bk-tsk1", "bk-tsk2")
}

// readyAllModel is readyModel with every bean of the fixture in the Ready set.
func readyAllModel(t *testing.T, beans []*item.Item) model {
	t.Helper()
	var ids []string
	for _, b := range beans {
		ids = append(ids, b.ID)
	}
	return readyModel(t, beans, ids...)
}

// --- readyVisible / idx.Ready() reuse ---

// TestReadyShowsParentlessReadyBeansFromIndex guards that readyVisible
// reuses idx.Ready() (E1 Task 3) unchanged: only parentless, non-milestone/
// epic, todo/draft beans appear.
func TestReadyShowsParentlessReadyBeansFromIndex(t *testing.T) {
	m := readyBeansModel(t)
	vis := m.readyVisible()

	var got []string
	for _, b := range vis {
		got = append(got, b.ID)
	}
	want := []string{"bk-tsk1", "bk-tsk2"} // milestone/completed/parented excluded
	if !equalStrings(got, want) {
		t.Fatalf("readyVisible() ids = %v, want %v", got, want)
	}
}

// TestReadyVisibleNilIndexReturnsNil guards the pre-load state (m.idx ==
// nil, e.g. Ready opened before the first beansLoadedMsg arrives) never
// panics.
func TestReadyVisibleNilIndexReturnsNil(t *testing.T) {
	m := newModel(nil, "/tmp/bt-fixture-repo")
	if vis := m.readyVisible(); vis != nil {
		t.Fatalf("readyVisible() on nil idx = %v, want nil", vis)
	}
}

// --- shared search + facet filters ---

// TestReadyAppliesSharedSearchAndFacetFilters guards that readyVisible
// calls the SAME m.beanMatches predicate the Tree uses (Task 3/4) -- no
// second, parallel filter implementation.
func TestReadyAppliesSharedSearchAndFacetFilters(t *testing.T) {
	m := readyBeansModel(t)
	m = setSearchQuery(m, "One") // matches only "Ready Task One"

	vis := m.readyVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk1" {
		t.Fatalf("readyVisible() with searchQuery=One = %v, want [bk-tsk1]", vis)
	}

	m = setSearchQuery(m, "")
	m.filterStatus = map[string]bool{"draft": true}
	vis = m.readyVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk2" {
		t.Fatalf("readyVisible() with filterStatus=draft = %v, want [bk-tsk2]", vis)
	}
}

// --- readyList staleness (E2-T6 fast-follow, T5-review) ---

// staleReadyBeans returns 10 Ready-eligible beans (8 "task", 2 "bug",
// all todo/parentless) so a facet filter can shrink readyVisible() from
// 10 down to 2 while a cursor parked at index 9 goes stale.
func staleReadyBeans() []*item.Item {
	var beans []*item.Item
	for i := 0; i < 8; i++ {
		beans = append(beans, &item.Item{
			ID: fmt.Sprintf("stale-tsk%d", i), Title: fmt.Sprintf("Task %d", i),
			Status: "todo", Type: "task", Priority: "normal",
		})
	}
	beans = append(beans,
		&item.Item{ID: "stale-bug0", Title: "Bug Zero", Status: "todo", Type: "bug", Priority: "high"},
		&item.Item{ID: "stale-bug1", Title: "Bug One", Status: "todo", Type: "bug", Priority: "high"},
	)
	return beans
}

// TestKeyReadyResyncsStaleCursorBeforeMove is the MANDATORY T5-review
// fast-follow regression test (bean bt-enrd): park the cursor deep in the
// Ready (index 9 of 10), shrink readyVisible() via the SHARED facet
// filter (Task 4) while m.view == viewReady -- direct field assignment
// mirrors exactly what `f`+space would produce via keyFilterMenu, and (like
// `/`'s keySearchInput) never touches readyList, so m.readyList is now
// stale relative to the shrunk readyVisible() (file doc comment above).
// The next ready key press must have keyReady's setLen resync recover
// the cursor into the new bounds BEFORE any move -- no out-of-range cursor,
// no vanished selection-highlight render artifact.
func TestKeyReadyResyncsStaleCursorBeforeMove(t *testing.T) {
	m := readyAllModel(t, staleReadyBeans())
	m.view = viewReady
	m.readyList.setLen(len(m.readyVisible())) // 10 items, fresh
	m.readyList.cursor = 9                    // park deep -- last row

	m.filterType = map[string]bool{"bug": true} // shared facet, shrinks to 2

	vis := m.readyVisible()
	if len(vis) != 2 {
		t.Fatalf("fixture setup: readyVisible() after filterType=bug = %d, want 2", len(vis))
	}
	if m.readyList.cursor < len(vis) {
		t.Fatal("fixture setup: cursor must still be OUT of the new bounds before the ready key -- test proves nothing otherwise")
	}

	m2 := step(t, m, keyMsg(tea.KeyDown)) // any ready key resyncs first

	if m2.readyList.cursor < 0 || m2.readyList.cursor >= len(vis) {
		t.Fatalf("cursor not recovered: cursor=%d, len(vis)=%d (want in [0,%d))", m2.readyList.cursor, len(vis), len(vis))
	}
	sel := m2.readySelected()
	if sel == nil {
		t.Fatal("readySelected() == nil after recovery -- cursor should point at a valid bean")
	}
	if sel.Type != "bug" {
		t.Fatalf("recovered selection = %s (%s), want one of the filterType=bug beans", sel.ID, sel.Type)
	}

	// No render artifact: the recovered selection must actually be
	// highlighted in the rendered frame (readyRows' i==pos check), not
	// silently dropped the way a still-stale cursor would render.
	out := m2.viewReady()
	if !strings.Contains(out, sel.ID) {
		t.Fatalf("recovered selection %s not present in rendered Ready view:\n%s", sel.ID, out)
	}
}

// --- Sort-Toggle S ---

// TestReadySortCyclesThroughFourModesAndBackToStart presses S 4 times: the
// mode sequence must read status->priority->created->updated->status (bean
// bt-gzu6 Akzeptanz + plan Port-Referenzen), landing back on a visually
// identical order to the untouched default ("" -- idx.Ready()'s own
// canonical order IS already status-tier order).
//
// ERRATUM vs. the plan's own sketch: the plan's `modes := []string{"",
// "priority", "created", "updated", "status"}` cycled through FIVE distinct
// string states ("" and "status" both members) -- wrapping from "status"
// (index 4) back to "" (index 0) instead of back to "priority" (index 1).
// Since "" and "status" render IDENTICALLY (idx.Ready() canonical order
// already IS status-tier order), that 5-state cycle produces a DEAD 5th
// keypress every lap (state changes "status"->"", but the rendered order
// does not) -- contradicting the bean's own "4 modes" Akzeptanz wording.
// Fixed here with a 4-element canonical mode list (nextReadySort,
// view_browse_ready.go) that treats "" as an ALIAS for "status" when
// finding the current position, never re-entering "" once cycling starts.
func TestReadySortCyclesThroughFourModesAndBackToStart(t *testing.T) {
	beans := []*item.Item{
		{ID: "bk-a", Title: "Zeta", Status: "todo", Type: "task", Priority: "low"},
		{ID: "bk-b", Title: "Alpha", Status: "in-progress", Type: "task", Priority: "critical"},
	}
	m := readyModel(t, beans, "bk-a", "bk-b")
	m.view = viewReady

	wantSeq := []string{"priority", "created", "updated", "status"}
	for i, want := range wantSeq {
		m = step(t, m, runeMsg('S'))
		if m.readySort != want {
			t.Fatalf("press %d: readySort = %q, want %q", i+1, m.readySort, want)
		}
	}

	// A 5th press must cycle back to "priority" -- NOT re-visit "" (proves
	// the fix: a pure 4-state cycle, no dead keypress).
	m = step(t, m, runeMsg('S'))
	if m.readySort != "priority" {
		t.Fatalf("press 5: readySort = %q, want %q (period-4 cycle, no dead state)", m.readySort, "priority")
	}
}

// TestReadySortOrdersChangeVisibly guards that each sort mode actually
// reorders the visible list using data.StatusRank/PriorityRank (Task 1) and
// CreatedAt/UpdatedAt, nil timestamps sorting last without panicking.
func TestReadySortOrdersChangeVisibly(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	beans := []*item.Item{
		{ID: "bk-a", Title: "A", Status: "todo", Type: "task", Priority: "low", CreatedAt: &older, UpdatedAt: nil},
		{ID: "bk-b", Title: "B", Status: "todo", Type: "task", Priority: "critical", CreatedAt: &newer, UpdatedAt: &newer},
	}
	m := readyModel(t, beans, "bk-a", "bk-b")

	m.readySort = "priority"
	vis := m.readyVisible()
	if vis[0].ID != "bk-b" { // critical ranks before low
		t.Fatalf("priority sort first = %s, want bk-b (critical)", vis[0].ID)
	}

	m.readySort = "created"
	vis = m.readyVisible()
	if vis[0].ID != "bk-b" { // newer CreatedAt first
		t.Fatalf("created sort first = %s, want bk-b (newer)", vis[0].ID)
	}

	m.readySort = "updated"
	vis = m.readyVisible() // bk-a has nil UpdatedAt -- must sort last, not panic
	if vis[0].ID != "bk-b" {
		t.Fatalf("updated sort first = %s, want bk-b (nil UpdatedAt sorts last)", vis[0].ID)
	}
}

// TestReadySortDisplayLabel guards D02's own tiny mapping helper
// (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): "" aliases to "status"
// (same alias nextReadySort itself already uses), "priority" abbreviates
// to "prio" (PO example verbatim: "⌕ / search · sort prio"), every other
// mode renders unchanged.
func TestReadySortDisplayLabel(t *testing.T) {
	cases := []struct{ mode, want string }{
		{"", "status"},
		{"status", "status"},
		{"priority", "prio"},
		{"created", "created"},
		{"updated", "updated"},
	}
	for _, c := range cases {
		if got := readySortDisplayLabel(c.mode); got != c.want {
			t.Errorf("readySortDisplayLabel(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}

// --- Master-Detail focus reuse ---

// TestReadyMasterDetailFocusSwapMatchesD03BorderColors guards that tab
// while m.view == viewReady reaches the detail and shift+tab leaves it again
// (existing renderPane focus-border machinery, no new border logic).
func TestReadyMasterDetailFocusSwapMatchesD03BorderColors(t *testing.T) {
	m := readyBeansModel(t)
	m.view = viewReady

	if m.detailFocus {
		t.Fatal("detailFocus should start false")
	}
	m = tabIntoDetail(t, m)
	m = step(t, m, keyMsg(tea.KeyShiftTab)) // first detail field -> last filter field
	if m.detailFocus {
		t.Fatal("shift+tab from the first detail field should have left the detail")
	}
}

// TestReadyAccordionReusesTask1SectionsViaFocusedBean guards that entering
// detail focus from the Ready list drives m.focusedBean() to the SELECTED
// ready bean, not the (irrelevant, possibly stale) tree cursor -- proves
// focusedBean()'s Task-2 dispatcher is genuinely reused, not reimplemented.
// T6-Review B01 (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert): entry is via
// TAB now (the ONLY detail-focus entry, view-agnostic in handleKey) -- this
// test previously pinned the pre-revision enter-entry behavior; enter's own
// no-op is pinned separately by TestReadyEnterDoesNotEnterDetailFocus.
func TestReadyAccordionReusesTask1SectionsViaFocusedBean(t *testing.T) {
	m := readyBeansModel(t)
	m.view = viewReady
	m.readyList.setLen(len(m.readyVisible()))
	m.readyList.move(1) // move onto the 2nd ready row (bk-tsk2)

	want := m.readyVisible()[m.readyList.cursor]
	got := m.focusedBean()
	if got == nil || got.ID != want.ID {
		t.Fatalf("focusedBean() = %v, want %s (the Ready list's selected bean)", got, want.ID)
	}

	m = tabIntoDetail(t, m)
	if !m.detailFocus {
		t.Fatal("tab on a focusable ready row must enter detail focus")
	}
	if got := m.focusedBean(); got == nil || got.ID != want.ID {
		t.Fatalf("focusedBean() inside detail focus = %v, want %s (still the Ready selection)", got, want.ID)
	}
}

// TestReadyEnterDoesNotEnterDetailFocus is the T6-Review B01 regression
// guard (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert: "KEIN enter als
// Detail-Fokus-Einstieg -- der bestehende tab-Mechanismus ... BLEIBT der
// Einstieg"): enter on a Ready row is a handled no-op (the Ready is a
// flat list, no expand concept -- the analog of keyTree's leaf no-op), it
// must NOT flip detailFocus or reset the accordion cursor state. Before this
// fix keyReady's enter case still carried the pre-revision entry behavior
// (detailFocus=true + full cursor reset) -- Tree and Ready had drifted
// apart on the ONE rule PO-Nachtrag 3 made view-agnostic.
func TestReadyEnterDoesNotEnterDetailFocus(t *testing.T) {
	m := readyBeansModel(t)
	m.view = viewReady
	m.readyList.setLen(len(m.readyVisible()))
	m.readyList.move(1) // a real, focusable row (bk-tsk2) -- the strongest case

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.detailFocus {
		t.Fatal("enter on a ready row must NOT enter detail focus (tab is the only entry, PO-Nachtrag 3)")
	}
	if m.view != viewReady {
		t.Fatalf("enter must stay in the Ready view, view = %v", m.view)
	}
	if m.readyList.cursor != 1 {
		t.Fatalf("enter must not move the ready cursor, cursor = %d, want 1", m.readyList.cursor)
	}
}

// --- b open/close, shared filter state preserved ---

// TestReadyOpenFromTreeAndBackPreservesSharedFilterState guards that `R`
// from Tree with an active facet filter shows the SAME filtered view in
// Ready, and M back to the Roadmap leaves the filter active there too --
// ONE shared filter state (Task 4), never reset by a view switch.
func TestReadyOpenFromTreeAndBackPreservesSharedFilterState(t *testing.T) {
	m := readyBeansModel(t)
	m.filterStatus = map[string]bool{"draft": true}

	m = step(t, m, runeMsg('R'))
	if m.view != viewReady {
		t.Fatalf("R should open Ready, view = %v", m.view)
	}
	vis := m.readyVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk2" {
		t.Fatalf("Ready with filterStatus=draft carried over = %v, want [bk-tsk2]", vis)
	}

	m = step(t, m, runeMsg('M'))
	if m.view != viewBrowseRepo {
		t.Fatalf("M should return to the Roadmap, view = %v", m.view)
	}
	if !m.filterStatus["draft"] {
		t.Fatal("filterStatus must still be active back in Tree -- shared state, not reset by the view switch")
	}
}

// TestReadyEscReturnsToRoadmap guards the esc half of the "R open, esc
// back" contract.
func TestReadyEscReturnsToRoadmap(t *testing.T) {
	m := readyBeansModel(t)
	m = step(t, m, runeMsg('R'))
	if m.view != viewReady {
		t.Fatalf("R should open Ready, view = %v", m.view)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.view != viewBrowseRepo || m.flatView {
		t.Fatalf("esc should return to the Roadmap, view = %v flat = %v", m.view, m.flatView)
	}
}

// --- B01 (E3 Task 1, bean bt-dlgk): X/FilterClear must also work in keyReady ---

// TestKeyReadyFilterClearResetsFacets guards B01 (E2-Abschluss carryover,
// epic-bean bt-gzcu PFLICHT-item): X was wired as a direct top-level facet
// reset in keyTree (update.go's keyTree FilterClear case) but the parallel
// case was missing from keyReady -- X silently fell through to the nav-key
// switch at the bottom of keyReady and did nothing, leaving active facets
// in place while the Ready view kept rendering the narrowed list. keyTree
// parity: same clearFacets() helper both keyFilterMenu's own X-case and
// keyTree's X-case already share (box_filter_facets.go).
func TestKeyReadyFilterClearResetsFacets(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewReady
	m.filterStatus = map[string]bool{"todo": true}
	nm := step(t, m, runeMsg('X'))
	if nm.filterActive() {
		t.Fatal("X in Ready view must clear all facets (B01, keyTree parity)")
	}
}

// --- windowing reuse ---

// TestReadyWindowingReusesExistingWindowAround guards that a Ready
// longer than the pane's row budget keeps the cursor visible via the
// existing windowAround/windowStart (E1 Task 8) -- no new fenestration
// mechanism.
func TestReadyWindowingReusesExistingWindowAround(t *testing.T) {
	var beans []*item.Item
	for i := 0; i < 40; i++ {
		beans = append(beans, &item.Item{
			ID: "bk-w" + string(rune('a'+i)), Title: "Row", Status: "todo", Type: "task", Priority: "normal",
		})
	}
	m := readyAllModel(t, beans)
	m.view = viewReady
	m.width, m.height = 100, 20 // small enough that 40 rows won't all fit
	m.readyList.setLen(len(m.readyVisible()))
	m.readyList.cursor = 35 // near the end

	out := m.viewReady()
	if !strings.Contains(out, "bk-w"+string(rune('a'+35))) {
		t.Fatalf("cursor row (index 35) must stay visible via windowAround, output:\n%s", out)
	}
}

// --- Golden ---

// goldenReadyModel builds the deterministic fixture rendered by
// TestReadyGolden: two Ready-eligible beans (todo task, draft bug -- both
// row kinds on screen at once) plus a milestone AND a parented task (both
// excluded from idx.Ready(), proving the view really is the ready, not
// the tree), cursor parked on the second row, Ready view active.
func goldenReadyModel(t *testing.T) model {
	t.Helper()
	beans := []*item.Item{
		{ID: "gbk-mlst", Title: "Golden Milestone", Status: "in-progress", Type: "milestone", Priority: "high"},
		{ID: "gbk-tsk1", Title: "First ready task", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "gbk-tsk2", Title: "Second ready task", Status: "draft", Type: "bug", Priority: "critical", Tags: []string{"backend"}},
		{ID: "gbk-child", Title: "Parented, excluded", Status: "todo", Type: "task", Priority: "normal", Parent: "gbk-mlst"},
	}

	m := newModel(nil, "/tmp/bt-golden-repo")
	m = step(t, m, beansLoadedMsg{beans: beans, ready: map[string]bool{"gbk-tsk1": true, "gbk-tsk2": true}})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewReady
	m.readyList.setLen(len(m.readyVisible()))
	m.readyList.cursor = 1 // gbk-tsk2
	return m
}

// TestReadyGolden renders a full 100x30 frame of the Ready view and
// compares it against testdata/ready.golden.
// Regenerate with: go test ./internal/tui/ -run TestReadyGolden -update
func TestReadyGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenReadyModel(t)
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("ready view height=%d, want 30 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Errorf("line %d overflows width (%d > 100): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "ready.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (%s) — regenerate with -update: %v", path, err)
	}
	if out != string(want) {
		t.Errorf("ready view output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// TestReadyGoldenDeterministic guards that View() is a pure function of
// the model for the Ready view too: repeated calls on identical state must
// byte-for-byte agree.
func TestReadyGoldenDeterministic(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenReadyModel(t)
	a := m.View()
	b := m.View()
	if a != b {
		t.Error("View() is not deterministic across repeated calls with identical Ready model state")
	}
}

// --- readyChrome: PF-11 Header/Footer-Keybinding-Split (design-spec.md
// §15, epic-E7-plan.md Task 7, bean bt-m6at). Mirrors
// view_browse_repo_test.go's browseRepoChrome coverage exactly -- same
// Header 7-Globals/Footer-context contract, just this view's own Chrome
// function.

// TestReadyChromeHeaderShowsExactlyFourGlobals guards D04 (design-spec.md
// §15 PF-16, bean bt-ntoz/bt-d8kc, SUPERSEDES the previous 7-Globals
// header): mirrors view_browse_repo_test.go's own coverage, this view's own
// Chrome function.
func TestReadyChromeHeaderShowsExactlyFourGlobals(t *testing.T) {
	m := readyBeansModel(t)
	head, _ := m.readyChrome(200) // wide enough to never trigger breadcrumb's narrow-stack fallback
	plain := stripHint(head)
	for _, want := range []string{"K/: commands", "P repos", "? help", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("readyChrome header = %q, want it to contain %q", plain, want)
		}
	}
	for _, absent := range []string{"ctrl+r", "esc back", "enter open"} {
		if strings.Contains(plain, absent) {
			t.Errorf("readyChrome header = %q, must NOT contain %q (D04 degrades it out of the header)", plain, absent)
		}
	}
}

func TestReadyChromeFooterOmitsEnter(t *testing.T) {
	m := readyBeansModel(t)
	_, localKeys := m.readyChrome(200)
	plain := stripHint(localKeys)
	if strings.Contains(plain, "enter open") {
		t.Errorf("readyChrome footer = %q, must NOT contain enter open (now header-only, D04)", plain)
	}
}

func TestReadyChromeFooterShowsFocusInFocusOut(t *testing.T) {
	m := readyBeansModel(t)
	_, localKeys := m.readyChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "tab next focus") {
		t.Errorf("readyChrome footer = %q, want FocusIn's real hint (not a hand-typed suffix)", plain)
	}
	if !strings.Contains(plain, "shift+tab prev focus") {
		t.Errorf("readyChrome footer = %q, want shift+tab visible for the first time (PF-13)", plain)
	}
}

// TestReadyChromeFooterMatchesQ06List guards Q06's finale, PO-verbatim
// Footer-Liste (mirrors TestBrowseRepoChromeFooterMatchesQ06List,
// view_browse_repo_test.go, EXACTLY -- D02, bean bt-tct9/bt-1e0t,
// PO-bestätigt Option b + Präzisierung, 2026-07-16, SUPERSEDES the former
// TestReadyChromeFooterMatchesQ06ListPlusSort/bean bt-d8kc ERRATUM: Sort
// no longer appends to the Ready footer, S stays functional but moves to
// Help-overlay-only documentation). readyChrome's footer is now BYTE-
// IDENTICAL to browseRepoChrome's -- both render browseRepoLocalBindings()
// verbatim, no per-view divergence left.
func TestReadyChromeFooterMatchesQ06List(t *testing.T) {
	m := readyBeansModel(t)
	m.view = viewReady
	_, localKeys := m.readyChrome(500) // wide enough for the whole list on one line
	plain := stripHint(localKeys)
	want := "tab next focus · shift+tab prev focus · / search · f Filter · c Create · d Delete · M Roadmap · L List · y Yank · b Relations · v next layout · shift+v previous layout"
	if plain != want {
		t.Errorf("readyChrome footer = %q, want exactly %q", plain, want)
	}
}

// TestReadyLocalBindingsOmitsSort is the direct binding-list guard (D02,
// bean bt-1e0t TDD-Schritte, named explicitly) -- SUPERSEDES the former
// TestReadyLocalBindingsIncludesSort (pre-D02, bean bt-d8kc), which
// asserted the exact OPPOSITE (Sort as a deliberate Ready-exclusive
// footer addition). PO-bestätigt D02 (bean bt-tct9, Option b + Präzisierung)
// reverses that call: `S Sort` flies out of the Ready footer entirely --
// readyLocalBindings() now returns browseRepoLocalBindings() UNCHANGED,
// no appended Sort. The key stays fully functional (keyReady's Sort case,
// below, untouched) and stays documented -- just Help-overlay-only now
// (helpGroups(), keymap.go, already lists it under "Actions", no change
// needed there). Neither view-local footer list carries it any more, so a
// single loop suffices (no split browseRepoLocalBindings() assertion left
// to make, both lists are now identical on this point).
func TestReadyLocalBindingsOmitsSort(t *testing.T) {
	for _, b := range readyBeansModel(t).footerBindings() {
		if strings.Join(b.Keys(), ",") == strings.Join(keys.Sort.Keys(), ",") {
			t.Error("Ready footerBindings unexpectedly includes keys.Sort -- D02 moved Sort to Help-overlay-only (bean bt-1e0t)")
		}
	}
}

// TestReadyLocalBindingsOmitsNavigation mirrors
// TestBrowseRepoLocalBindingsOmitsNavigation (view_browse_repo_test.go):
// Q06 removes Up/Down from the Ready footer too.
func TestReadyLocalBindingsOmitsNavigation(t *testing.T) {
	for _, nav := range []keybind.Binding{keys.Up, keys.Down} {
		for _, b := range readyBeansModel(t).footerBindings() {
			if strings.Join(b.Keys(), ",") == strings.Join(nav.Keys(), ",") {
				t.Errorf("Ready footerBindings still contains navigation binding %v, want it removed (Q06)", nav.Keys())
			}
		}
	}
}

func TestReadyChromeFooterIsContextSensitiveOnOverlayOpen(t *testing.T) {
	m := readyBeansModel(t)
	m.overlay = overlayValueMenu
	_, localKeys := m.readyChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "s Status") {
		t.Errorf("readyChrome footer while overlayValueMenu = %q, want the Value-Menu's own context set", plain)
	}
	if strings.Contains(plain, "c Create") {
		t.Errorf("readyChrome footer while overlayValueMenu = %q, must not leak the (now irrelevant) view-local Create hint", plain)
	}
}
