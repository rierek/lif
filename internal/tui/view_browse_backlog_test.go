package tui

// view_browse_backlog_test.go — TDD coverage for Backlog-View V3 `b` (E2 Task
// 5, bean bt-gzu6, design-spec.md §6 V3): idx.Backlog() reuse, the shared
// search+facet predicate (m.beanMatches, Task 3/4), the Sort-Toggle `S`
// cycle, Master-Detail focus reuse (focusedBean(), Task 2's Accordion, Task
// 1), and windowing reuse (E1 windowAround). Reuses fixtureBeans/
// fixtureModel/step/keyMsg/runeMsg/nodeIDs/equalStrings from update_test.go
// (same package).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/data"
)

// backlogBeans is a small parentless fixture set covering both Backlog
// membership (parentless, non-milestone/epic, status todo/draft) and
// non-membership (parented, milestone/epic type, or completed/in-progress
// status) -- mirrors idx.Backlog()'s own filter (data/index.go).
func backlogBeans() []data.Bean {
	return []data.Bean{
		{ID: "bk-mlst", Title: "A Milestone", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "bk-tsk1", Title: "Backlog Task One", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "bk-tsk2", Title: "Backlog Task Two", Status: "draft", Type: "bug", Priority: "high"},
		{ID: "bk-done", Title: "Done Task", Status: "completed", Type: "task", Priority: "normal"},
		{ID: "bk-child", Title: "Parented Task", Status: "todo", Type: "task", Priority: "normal", Parent: "bk-mlst"},
	}
}

// --- backlogVisible / idx.Backlog() reuse ---

// TestBacklogShowsParentlessReadyBeansFromIndex guards that backlogVisible
// reuses idx.Backlog() (E1 Task 3) unchanged: only parentless, non-milestone/
// epic, todo/draft beans appear.
func TestBacklogShowsParentlessReadyBeansFromIndex(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	vis := m.backlogVisible()

	var got []string
	for _, b := range vis {
		got = append(got, b.ID)
	}
	want := []string{"bk-tsk1", "bk-tsk2"} // milestone/completed/parented excluded
	if !equalStrings(got, want) {
		t.Fatalf("backlogVisible() ids = %v, want %v", got, want)
	}
}

// TestBacklogVisibleNilIndexReturnsNil guards the pre-load state (m.idx ==
// nil, e.g. Backlog opened before the first beansLoadedMsg arrives) never
// panics.
func TestBacklogVisibleNilIndexReturnsNil(t *testing.T) {
	m := newModel(nil, "/tmp/bt-fixture-repo")
	if vis := m.backlogVisible(); vis != nil {
		t.Fatalf("backlogVisible() on nil idx = %v, want nil", vis)
	}
}

// --- shared search + facet filters ---

// TestBacklogAppliesSharedSearchAndFacetFilters guards that backlogVisible
// calls the SAME m.beanMatches predicate the Tree uses (Task 3/4) -- no
// second, parallel filter implementation.
func TestBacklogAppliesSharedSearchAndFacetFilters(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m = setSearchQuery(m, "One") // matches only "Backlog Task One"

	vis := m.backlogVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk1" {
		t.Fatalf("backlogVisible() with searchQuery=One = %v, want [bk-tsk1]", vis)
	}

	m = setSearchQuery(m, "")
	m.filterStatus = map[string]bool{"draft": true}
	vis = m.backlogVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk2" {
		t.Fatalf("backlogVisible() with filterStatus=draft = %v, want [bk-tsk2]", vis)
	}
}

// --- backlogList staleness (E2-T6 fast-follow, T5-review) ---

// staleBacklogBeans returns 10 Backlog-eligible beans (8 "task", 2 "bug",
// all todo/parentless) so a facet filter can shrink backlogVisible() from
// 10 down to 2 while a cursor parked at index 9 goes stale.
func staleBacklogBeans() []data.Bean {
	var beans []data.Bean
	for i := 0; i < 8; i++ {
		beans = append(beans, data.Bean{
			ID: fmt.Sprintf("stale-tsk%d", i), Title: fmt.Sprintf("Task %d", i),
			Status: "todo", Type: "task", Priority: "normal",
		})
	}
	beans = append(beans,
		data.Bean{ID: "stale-bug0", Title: "Bug Zero", Status: "todo", Type: "bug", Priority: "high"},
		data.Bean{ID: "stale-bug1", Title: "Bug One", Status: "todo", Type: "bug", Priority: "high"},
	)
	return beans
}

// TestKeyBacklogResyncsStaleCursorBeforeMove is the MANDATORY T5-review
// fast-follow regression test (bean bt-enrd): park the cursor deep in the
// Backlog (index 9 of 10), shrink backlogVisible() via the SHARED facet
// filter (Task 4) while m.view == viewBacklog -- direct field assignment
// mirrors exactly what `f`+space would produce via keyFilterMenu, and (like
// `/`'s keySearchInput) never touches backlogList, so m.backlogList is now
// stale relative to the shrunk backlogVisible() (file doc comment above).
// The next backlog key press must have keyBacklog's setLen resync recover
// the cursor into the new bounds BEFORE any move -- no out-of-range cursor,
// no vanished selection-highlight render artifact.
func TestKeyBacklogResyncsStaleCursorBeforeMove(t *testing.T) {
	m := fixtureModel(t, staleBacklogBeans())
	m.view = viewBacklog
	m.backlogList.setLen(len(m.backlogVisible())) // 10 items, fresh
	m.backlogList.cursor = 9                      // park deep -- last row

	m.filterType = map[string]bool{"bug": true} // shared facet, shrinks to 2

	vis := m.backlogVisible()
	if len(vis) != 2 {
		t.Fatalf("fixture setup: backlogVisible() after filterType=bug = %d, want 2", len(vis))
	}
	if m.backlogList.cursor < len(vis) {
		t.Fatal("fixture setup: cursor must still be OUT of the new bounds before the backlog key -- test proves nothing otherwise")
	}

	m2 := step(t, m, keyMsg(tea.KeyDown)) // any backlog key resyncs first

	if m2.backlogList.cursor < 0 || m2.backlogList.cursor >= len(vis) {
		t.Fatalf("cursor not recovered: cursor=%d, len(vis)=%d (want in [0,%d))", m2.backlogList.cursor, len(vis), len(vis))
	}
	sel := m2.backlogSelected()
	if sel == nil {
		t.Fatal("backlogSelected() == nil after recovery -- cursor should point at a valid bean")
	}
	if sel.Type != "bug" {
		t.Fatalf("recovered selection = %s (%s), want one of the filterType=bug beans", sel.ID, sel.Type)
	}

	// No render artifact: the recovered selection must actually be
	// highlighted in the rendered frame (backlogRows' i==pos check), not
	// silently dropped the way a still-stale cursor would render.
	out := m2.viewBacklog()
	if !strings.Contains(out, sel.ID) {
		t.Fatalf("recovered selection %s not present in rendered Backlog view:\n%s", sel.ID, out)
	}
}

// --- Sort-Toggle S ---

// TestBacklogSortCyclesThroughFourModesAndBackToStart presses S 4 times: the
// mode sequence must read status->priority->created->updated->status (bean
// bt-gzu6 Akzeptanz + plan Port-Referenzen), landing back on a visually
// identical order to the untouched default ("" -- idx.Backlog()'s own
// canonical order IS already status-tier order).
//
// ERRATUM vs. the plan's own sketch: the plan's `modes := []string{"",
// "priority", "created", "updated", "status"}` cycled through FIVE distinct
// string states ("" and "status" both members) -- wrapping from "status"
// (index 4) back to "" (index 0) instead of back to "priority" (index 1).
// Since "" and "status" render IDENTICALLY (idx.Backlog() canonical order
// already IS status-tier order), that 5-state cycle produces a DEAD 5th
// keypress every lap (state changes "status"->"", but the rendered order
// does not) -- contradicting the bean's own "4 modes" Akzeptanz wording.
// Fixed here with a 4-element canonical mode list (nextBacklogSort,
// view_browse_backlog.go) that treats "" as an ALIAS for "status" when
// finding the current position, never re-entering "" once cycling starts.
func TestBacklogSortCyclesThroughFourModesAndBackToStart(t *testing.T) {
	beans := []data.Bean{
		{ID: "bk-a", Title: "Zeta", Status: "todo", Type: "task", Priority: "low"},
		{ID: "bk-b", Title: "Alpha", Status: "in-progress", Type: "task", Priority: "critical"},
	}
	m := fixtureModel(t, beans)
	m.view = viewBacklog

	wantSeq := []string{"priority", "created", "updated", "status"}
	for i, want := range wantSeq {
		m = step(t, m, runeMsg('S'))
		if m.backlogSort != want {
			t.Fatalf("press %d: backlogSort = %q, want %q", i+1, m.backlogSort, want)
		}
	}

	// A 5th press must cycle back to "priority" -- NOT re-visit "" (proves
	// the fix: a pure 4-state cycle, no dead keypress).
	m = step(t, m, runeMsg('S'))
	if m.backlogSort != "priority" {
		t.Fatalf("press 5: backlogSort = %q, want %q (period-4 cycle, no dead state)", m.backlogSort, "priority")
	}
}

// TestBacklogSortOrdersChangeVisibly guards that each sort mode actually
// reorders the visible list using data.StatusRank/PriorityRank (Task 1) and
// CreatedAt/UpdatedAt, nil timestamps sorting last without panicking.
func TestBacklogSortOrdersChangeVisibly(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	beans := []data.Bean{
		{ID: "bk-a", Title: "A", Status: "todo", Type: "task", Priority: "low", CreatedAt: &older, UpdatedAt: nil},
		{ID: "bk-b", Title: "B", Status: "todo", Type: "task", Priority: "critical", CreatedAt: &newer, UpdatedAt: &newer},
	}
	m := fixtureModel(t, beans)

	m.backlogSort = "priority"
	vis := m.backlogVisible()
	if vis[0].ID != "bk-b" { // critical ranks before low
		t.Fatalf("priority sort first = %s, want bk-b (critical)", vis[0].ID)
	}

	m.backlogSort = "created"
	vis = m.backlogVisible()
	if vis[0].ID != "bk-b" { // newer CreatedAt first
		t.Fatalf("created sort first = %s, want bk-b (newer)", vis[0].ID)
	}

	m.backlogSort = "updated"
	vis = m.backlogVisible() // bk-a has nil UpdatedAt -- must sort last, not panic
	if vis[0].ID != "bk-b" {
		t.Fatalf("updated sort first = %s, want bk-b (nil UpdatedAt sorts last)", vis[0].ID)
	}
}

// TestBacklogSortDisplayLabel guards D02's own tiny mapping helper
// (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): "" aliases to "status"
// (same alias nextBacklogSort itself already uses), "priority" abbreviates
// to "prio" (PO example verbatim: "⌕ / search · sort prio"), every other
// mode renders unchanged.
func TestBacklogSortDisplayLabel(t *testing.T) {
	cases := []struct{ mode, want string }{
		{"", "status"},
		{"status", "status"},
		{"priority", "prio"},
		{"created", "created"},
		{"updated", "updated"},
	}
	for _, c := range cases {
		if got := backlogSortDisplayLabel(c.mode); got != c.want {
			t.Errorf("backlogSortDisplayLabel(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}

// --- Master-Detail focus reuse ---

// TestBacklogMasterDetailFocusSwapMatchesD03BorderColors guards that tab
// while m.view == viewBacklog toggles m.detailFocus exactly like the Tree
// view (existing renderPane focus-border machinery, no new border logic).
func TestBacklogMasterDetailFocusSwapMatchesD03BorderColors(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.view = viewBacklog

	if m.detailFocus {
		t.Fatal("detailFocus should start false")
	}
	m = step(t, m, keyMsg(tea.KeyTab))
	if !m.detailFocus {
		t.Fatal("tab should have set detailFocus = true")
	}
	m = step(t, m, keyMsg(tea.KeyTab))
	if m.detailFocus {
		t.Fatal("tab should have set detailFocus = false")
	}
}

// TestBacklogAccordionReusesTask1SectionsViaFocusedBean guards that entering
// detail focus from the Backlog list drives m.focusedBean() to the SELECTED
// backlog bean, not the (irrelevant, possibly stale) tree cursor -- proves
// focusedBean()'s Task-2 dispatcher is genuinely reused, not reimplemented.
// T6-Review B01 (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert): entry is via
// TAB now (the ONLY detail-focus entry, view-agnostic in handleKey) -- this
// test previously pinned the pre-revision enter-entry behavior; enter's own
// no-op is pinned separately by TestBacklogEnterDoesNotEnterDetailFocus.
func TestBacklogAccordionReusesTask1SectionsViaFocusedBean(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.view = viewBacklog
	m.backlogList.setLen(len(m.backlogVisible()))
	m.backlogList.move(1) // move onto the 2nd backlog row (bk-tsk2)

	want := m.backlogVisible()[m.backlogList.cursor]
	got := m.focusedBean()
	if got == nil || got.ID != want.ID {
		t.Fatalf("focusedBean() = %v, want %s (the Backlog list's selected bean)", got, want.ID)
	}

	m = step(t, m, keyMsg(tea.KeyTab))
	if !m.detailFocus {
		t.Fatal("tab on a focusable backlog row must enter detail focus")
	}
	if got := m.focusedBean(); got == nil || got.ID != want.ID {
		t.Fatalf("focusedBean() inside detail focus = %v, want %s (still the Backlog selection)", got, want.ID)
	}
}

// TestBacklogEnterDoesNotEnterDetailFocus is the T6-Review B01 regression
// guard (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert: "KEIN enter als
// Detail-Fokus-Einstieg -- der bestehende tab-Mechanismus ... BLEIBT der
// Einstieg"): enter on a Backlog row is a handled no-op (the Backlog is a
// flat list, no expand concept -- the analog of keyTree's leaf no-op), it
// must NOT flip detailFocus or reset the accordion cursor state. Before this
// fix keyBacklog's enter case still carried the pre-revision entry behavior
// (detailFocus=true + full cursor reset) -- Tree and Backlog had drifted
// apart on the ONE rule PO-Nachtrag 3 made view-agnostic.
func TestBacklogEnterDoesNotEnterDetailFocus(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.view = viewBacklog
	m.backlogList.setLen(len(m.backlogVisible()))
	m.backlogList.move(1) // a real, focusable row (bk-tsk2) -- the strongest case

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.detailFocus {
		t.Fatal("enter on a backlog row must NOT enter detail focus (tab is the only entry, PO-Nachtrag 3)")
	}
	if m.view != viewBacklog {
		t.Fatalf("enter must stay in the Backlog view, view = %v", m.view)
	}
	if m.backlogList.cursor != 1 {
		t.Fatalf("enter must not move the backlog cursor, cursor = %d, want 1", m.backlogList.cursor)
	}
}

// --- b open/close, shared filter state preserved ---

// TestBacklogOpenFromTreeAndBackPreservesSharedFilterState guards that `b`
// from Tree with an active facet filter shows the SAME filtered view in
// Backlog, and esc/b back to Tree leaves the filter active there too --
// ONE shared filter state (Task 4), never reset by a view switch.
func TestBacklogOpenFromTreeAndBackPreservesSharedFilterState(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.filterStatus = map[string]bool{"draft": true}

	m = step(t, m, runeMsg('b'))
	if m.view != viewBacklog {
		t.Fatalf("b should open Backlog, view = %v", m.view)
	}
	vis := m.backlogVisible()
	if len(vis) != 1 || vis[0].ID != "bk-tsk2" {
		t.Fatalf("Backlog with filterStatus=draft carried over = %v, want [bk-tsk2]", vis)
	}

	m = step(t, m, runeMsg('b'))
	if m.view != viewBrowseRepo {
		t.Fatalf("b again should return to Tree, view = %v", m.view)
	}
	if !m.filterStatus["draft"] {
		t.Fatal("filterStatus must still be active back in Tree -- shared state, not reset by the view switch")
	}
}

// TestBacklogEscReturnsToTree guards the esc half of the "b open, esc/b
// back" contract (design-spec.md, bean bt-gzu6 Akzeptanz).
func TestBacklogEscReturnsToTree(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m = step(t, m, runeMsg('b'))
	if m.view != viewBacklog {
		t.Fatalf("b should open Backlog, view = %v", m.view)
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.view != viewBrowseRepo {
		t.Fatalf("esc should return to Tree, view = %v", m.view)
	}
}

// --- B01 (E3 Task 1, bean bt-dlgk): X/FilterClear must also work in keyBacklog ---

// TestKeyBacklogFilterClearResetsFacets guards B01 (E2-Abschluss carryover,
// epic-bean bt-gzcu PFLICHT-item): X was wired as a direct top-level facet
// reset in keyTree (update.go's keyTree FilterClear case) but the parallel
// case was missing from keyBacklog -- X silently fell through to the nav-key
// switch at the bottom of keyBacklog and did nothing, leaving active facets
// in place while the Backlog view kept rendering the narrowed list. keyTree
// parity: same clearFacets() helper both keyFilterMenu's own X-case and
// keyTree's X-case already share (box_filter_facets.go).
func TestKeyBacklogFilterClearResetsFacets(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewBacklog
	m.filterStatus = map[string]bool{"todo": true}
	nm := step(t, m, runeMsg('X'))
	if nm.filterActive() {
		t.Fatal("X in Backlog view must clear all facets (B01, keyTree parity)")
	}
}

// --- windowing reuse ---

// TestBacklogWindowingReusesExistingWindowAround guards that a Backlog
// longer than the pane's row budget keeps the cursor visible via the
// existing windowAround/windowStart (E1 Task 8) -- no new fenestration
// mechanism.
func TestBacklogWindowingReusesExistingWindowAround(t *testing.T) {
	var beans []data.Bean
	for i := 0; i < 40; i++ {
		beans = append(beans, data.Bean{
			ID: "bk-w" + string(rune('a'+i)), Title: "Row", Status: "todo", Type: "task", Priority: "normal",
		})
	}
	m := fixtureModel(t, beans)
	m.view = viewBacklog
	m.width, m.height = 100, 20 // small enough that 40 rows won't all fit
	m.backlogList.setLen(len(m.backlogVisible()))
	m.backlogList.cursor = 35 // near the end

	out := m.viewBacklog()
	if !strings.Contains(out, "bk-w"+string(rune('a'+35))) {
		t.Fatalf("cursor row (index 35) must stay visible via windowAround, output:\n%s", out)
	}
}

// --- Golden ---

// goldenBacklogModel builds the deterministic fixture rendered by
// TestBacklogGolden: two Backlog-eligible beans (todo task, draft bug -- both
// row kinds on screen at once) plus a milestone AND a parented task (both
// excluded from idx.Backlog(), proving the view really is the backlog, not
// the tree), cursor parked on the second row, Backlog view active.
func goldenBacklogModel(t *testing.T) model {
	t.Helper()
	beans := []data.Bean{
		{ID: "gbk-mlst", Title: "Golden Milestone", Status: "in-progress", Type: "milestone", Priority: "high"},
		{ID: "gbk-tsk1", Title: "First backlog task", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "gbk-tsk2", Title: "Second backlog task", Status: "draft", Type: "bug", Priority: "critical", Tags: []string{"backend"}},
		{ID: "gbk-child", Title: "Parented, excluded", Status: "todo", Type: "task", Priority: "normal", Parent: "gbk-mlst"},
	}

	m := newModel(nil, "/tmp/bt-golden-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewBacklog
	m.backlogList.setLen(len(m.backlogVisible()))
	m.backlogList.cursor = 1 // gbk-tsk2
	return m
}

// TestBacklogGolden renders a full 100x30 frame of the Backlog view and
// compares it against testdata/backlog.golden.
// Regenerate with: go test ./internal/tui/ -run TestBacklogGolden -update
func TestBacklogGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenBacklogModel(t)
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("backlog view height=%d, want 30 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Errorf("line %d overflows width (%d > 100): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "backlog.golden")
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
		t.Errorf("backlog view output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// TestBacklogGoldenDeterministic guards that View() is a pure function of
// the model for the Backlog view too: repeated calls on identical state must
// byte-for-byte agree.
func TestBacklogGoldenDeterministic(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenBacklogModel(t)
	a := m.View()
	b := m.View()
	if a != b {
		t.Error("View() is not deterministic across repeated calls with identical Backlog model state")
	}
}

// --- backlogChrome: PF-11 Header/Footer-Keybinding-Split (design-spec.md
// §15, epic-E7-plan.md Task 7, bean bt-m6at). Mirrors
// view_browse_repo_test.go's browseRepoChrome coverage exactly -- same
// Header 7-Globals/Footer-context contract, just this view's own Chrome
// function.

// TestBacklogChromeHeaderShowsExactlyFourGlobals guards D04 (design-spec.md
// §15 PF-16, bean bt-ntoz/bt-d8kc, SUPERSEDES the previous 7-Globals
// header): mirrors view_browse_repo_test.go's own coverage, this view's own
// Chrome function.
func TestBacklogChromeHeaderShowsExactlyFourGlobals(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	head, _ := m.backlogChrome(200) // wide enough to never trigger breadcrumb's narrow-stack fallback
	plain := stripHint(head)
	for _, want := range []string{"K commands", "p repos", "? help", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("backlogChrome header = %q, want it to contain %q", plain, want)
		}
	}
	for _, absent := range []string{"ctrl+r", "esc back", "enter open"} {
		if strings.Contains(plain, absent) {
			t.Errorf("backlogChrome header = %q, must NOT contain %q (D04 degrades it out of the header)", plain, absent)
		}
	}
}

func TestBacklogChromeFooterOmitsEnter(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	_, localKeys := m.backlogChrome(200)
	plain := stripHint(localKeys)
	if strings.Contains(plain, "enter open") {
		t.Errorf("backlogChrome footer = %q, must NOT contain enter open (now header-only, D04)", plain)
	}
}

func TestBacklogChromeFooterShowsFocusInFocusOut(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	_, localKeys := m.backlogChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "tab focus in") {
		t.Errorf("backlogChrome footer = %q, want FocusIn's real hint (not a hand-typed suffix)", plain)
	}
	if !strings.Contains(plain, "shift+tab focus out") {
		t.Errorf("backlogChrome footer = %q, want shift+tab visible for the first time (PF-13)", plain)
	}
}

// TestBacklogChromeFooterMatchesQ06List guards Q06's finale, PO-verbatim
// Footer-Liste (mirrors TestBrowseRepoChromeFooterMatchesQ06List,
// view_browse_repo_test.go, EXACTLY -- D02, bean bt-tct9/bt-1e0t,
// PO-bestätigt Option b + Präzisierung, 2026-07-16, SUPERSEDES the former
// TestBacklogChromeFooterMatchesQ06ListPlusSort/bean bt-d8kc ERRATUM: Sort
// no longer appends to the Backlog footer, S stays functional but moves to
// Help-overlay-only documentation). backlogChrome's footer is now BYTE-
// IDENTICAL to browseRepoChrome's -- both render browseRepoLocalBindings()
// verbatim, no per-view divergence left.
func TestBacklogChromeFooterMatchesQ06List(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	_, localKeys := m.backlogChrome(500) // wide enough for the whole list on one line
	plain := stripHint(localKeys)
	want := "tab focus in · shift+tab focus out · / search · f Filter · c Create · d Delete · b Backlog · y Yank · r Relations · v fullscreen · D toggle detail"
	if plain != want {
		t.Errorf("backlogChrome footer = %q, want exactly %q", plain, want)
	}
}

// TestBacklogLocalBindingsOmitsSort is the direct binding-list guard (D02,
// bean bt-1e0t TDD-Schritte, named explicitly) -- SUPERSEDES the former
// TestBacklogLocalBindingsIncludesSort (pre-D02, bean bt-d8kc), which
// asserted the exact OPPOSITE (Sort as a deliberate Backlog-exclusive
// footer addition). PO-bestätigt D02 (bean bt-tct9, Option b + Präzisierung)
// reverses that call: `S Sort` flies out of the Backlog footer entirely --
// backlogLocalBindings() now returns browseRepoLocalBindings() UNCHANGED,
// no appended Sort. The key stays fully functional (keyBacklog's Sort case,
// below, untouched) and stays documented -- just Help-overlay-only now
// (helpGroups(), keymap.go, already lists it under "Actions", no change
// needed there). Neither view-local footer list carries it any more, so a
// single loop suffices (no split browseRepoLocalBindings() assertion left
// to make, both lists are now identical on this point).
func TestBacklogLocalBindingsOmitsSort(t *testing.T) {
	cases := []struct {
		name string
		list []keybind.Binding
	}{
		{"backlogLocalBindings", backlogLocalBindings()},
		{"browseRepoLocalBindings", browseRepoLocalBindings()},
	}
	for _, c := range cases {
		for _, b := range c.list {
			if strings.Join(b.Keys(), ",") == strings.Join(keys.Sort.Keys(), ",") {
				t.Errorf("%s() unexpectedly includes keys.Sort -- D02 moved Sort to Help-overlay-only (bean bt-1e0t)", c.name)
			}
		}
	}
}

// TestBacklogLocalBindingsOmitsNavigation mirrors
// TestBrowseRepoLocalBindingsOmitsNavigation (view_browse_repo_test.go):
// Q06 removes Up/Down from the Backlog footer too.
func TestBacklogLocalBindingsOmitsNavigation(t *testing.T) {
	for _, nav := range []keybind.Binding{keys.Up, keys.Down} {
		for _, b := range backlogLocalBindings() {
			if strings.Join(b.Keys(), ",") == strings.Join(nav.Keys(), ",") {
				t.Errorf("backlogLocalBindings() still contains navigation binding %v, want it removed (Q06)", nav.Keys())
			}
		}
	}
}

func TestBacklogChromeFooterIsContextSensitiveOnOverlayOpen(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.overlay = overlayValueMenu
	_, localKeys := m.backlogChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "s Status") {
		t.Errorf("backlogChrome footer while overlayValueMenu = %q, want the Value-Menu's own context set", plain)
	}
	if strings.Contains(plain, "c Create") {
		t.Errorf("backlogChrome footer while overlayValueMenu = %q, must not leak the (now irrelevant) view-local Create hint", plain)
	}
}
