package tui

// view_fullscreen_test.go — TDD coverage for F01 Kernmechanik: Vollbild-Modus
// `v` (design-spec.md §15 "F01 — Vollbild-Navigation", bean bt-13l7, E9 Task
// 7). OHNE den History-Stack (Task 8, bean bt-1vbp). Reuses fixtureBeans/
// fixtureBeansWithBlocking/fixtureModel/step/keyMsg/runeMsg (update_test.go)
// and backlogBeans (view_browse_backlog_test.go) -- same package.
//
// focusedBean()/activateDetailField()/keyDetailFocus()'s own fullscreenDetail
// integration is covered in update_test.go instead (co-located with those
// functions' existing test coverage); the mouse guard is covered in
// mouse_test.go (co-located with handleMouse's existing tests). This file
// covers keyFullscreen itself + renderFullscreenBody.

import (
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// fullscreenBacklogBeans is a small parentless, Backlog-eligible fixture
// carrying a Blocking relation between its two members -- used to exercise
// F01's Backlog symmetry (a Relations-Sprung inside fullscreenDetail while
// m.view == viewBacklog, and the esc-exit's backlogList cursor sync).
func fullscreenBacklogBeans() []data.Bean {
	return []data.Bean{
		{ID: "fb-a", Title: "Backlog A", Status: "todo", Type: "task", Priority: "normal", Blocking: []string{"fb-b"}},
		{ID: "fb-b", Title: "Backlog B", Status: "todo", Type: "task", Priority: "normal"},
	}
}

// --- Einstieg (v) ---

// TestKeyFullscreenEntersListModeWhenTreeFocused guards the PO-Wortlaut's
// first case: Browse+links-fokussiert (m.detailFocus == false) -> Beans-
// Liste Vollbild.
func TestKeyFullscreenEntersListModeWhenTreeFocused(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	if m.detailFocus {
		t.Fatal("setup: expected Tree focus (detailFocus false)")
	}

	m = step(t, m, runeMsg('v'))

	if m.fullscreen != fullscreenList {
		t.Fatalf("fullscreen = %v, want fullscreenList", m.fullscreen)
	}
}

// TestKeyFullscreenEntersDetailModeWhenDetailFocused guards the PO-Wortlaut's
// second case: Browse+rechts-fokussiert (m.detailFocus == true) -> Detail-
// View Vollbild, targeting the currently focused bean.
func TestKeyFullscreenEntersDetailModeWhenDetailFocused(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	m = step(t, m, keyMsg(tea.KeyTab)) // detailFocus = true

	m = step(t, m, runeMsg('v'))

	if m.fullscreen != fullscreenDetail {
		t.Fatalf("fullscreen = %v, want fullscreenDetail", m.fullscreen)
	}
	if m.fullscreenBeanID != "ms-1" {
		t.Fatalf("fullscreenBeanID = %q, want ms-1", m.fullscreenBeanID)
	}
}

// TestKeyFullscreenNoOpWhenAlreadyFullscreenList guards the Planner
// decision: v is a EINWEG-Einstieg, not a toggle -- a second v while already
// in fullscreenList changes nothing.
func TestKeyFullscreenNoOpWhenAlreadyFullscreenList(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenList

	m = step(t, m, runeMsg('v'))

	if m.fullscreen != fullscreenList {
		t.Fatalf("fullscreen = %v, want unchanged fullscreenList (v is one-way, no toggle)", m.fullscreen)
	}
}

// TestKeyFullscreenNoOpWhenAlreadyFullscreenDetail mirrors the List case for
// fullscreenDetail -- v must not re-target a DIFFERENT bean even if the
// Tree/Backlog cursor has since moved.
func TestKeyFullscreenNoOpWhenAlreadyFullscreenDetail(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "ms-1"
	m.cursorID = "tk-2" // deliberately a DIFFERENT bean

	m = step(t, m, runeMsg('v'))

	if m.fullscreen != fullscreenDetail || m.fullscreenBeanID != "ms-1" {
		t.Fatalf("fullscreen=%v fullscreenBeanID=%q, want unchanged fullscreenDetail/ms-1", m.fullscreen, m.fullscreenBeanID)
	}
}

// TestKeyFullscreenNoOpInLobby guards keyFullscreen's own defensive
// m.view == viewLobby guard directly (handleKey's earlier viewLobby capture
// already makes this unreachable via the full Update() dispatch -- this
// calls keyFullscreen directly to pin the function's own contract).
func TestKeyFullscreenNoOpInLobby(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewLobby

	handled, tm, cmd := m.keyFullscreen(runeMsg('v'))

	if handled {
		t.Fatal("keyFullscreen must not claim to handle v while m.view == viewLobby")
	}
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("keyFullscreen did not return a model, got %T", tm)
	}
	if nm.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone (unchanged)", nm.fullscreen)
	}
	if cmd != nil {
		t.Fatal("keyFullscreen(lobby) must not return a Cmd")
	}
}

// --- Listen-Vollbild -> Detail-Vollbild (enter) ---

// TestKeyFullscreenEnterOnListEntersDetailMode guards enter on a Tree bean
// while m.fullscreen == fullscreenList: switches to fullscreenDetail
// targeting the cursored bean AND reinitializes the Detail-Fokus-Maschine
// (Meta/section level, field cursor 0) -- a stale position from a previous
// visit must not leak in.
func TestKeyFullscreenEnterOnListEntersDetailMode(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.cursorID = "ms-1"
	m.fullscreen = fullscreenList
	m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 2, 3, 1, 1 // stale, must be reset

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.fullscreen != fullscreenDetail {
		t.Fatalf("fullscreen = %v, want fullscreenDetail", m.fullscreen)
	}
	if m.fullscreenBeanID != "ms-1" {
		t.Fatalf("fullscreenBeanID = %q, want ms-1", m.fullscreenBeanID)
	}
	if m.secCursor != 0 || m.accOpen != 1 || m.detailLevel != 0 || m.fieldCursor != 0 {
		t.Fatalf("Detail-Fokus-Maschine not reset: secCursor=%d accOpen=%d detailLevel=%d fieldCursor=%d, want 0/1/0/0",
			m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor)
	}
}

// TestKeyFullscreenEnterOnListEntersDetailModeBacklog is
// TestKeyFullscreenEnterOnListEntersDetailMode's Backlog mirror (Akzeptanz:
// "Symmetrisch für Browse (Tree) UND Backlog").
func TestKeyFullscreenEnterOnListEntersDetailModeBacklog(t *testing.T) {
	m := fixtureModel(t, fullscreenBacklogBeans())
	m.view = viewBacklog
	vis := m.backlogVisible()
	if len(vis) == 0 {
		t.Fatal("setup: need at least 1 backlog-visible bean")
	}
	m.backlogList.setLen(len(vis))
	m.backlogList.cursor = 0
	want := vis[0].ID
	m.fullscreen = fullscreenList

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.fullscreen != fullscreenDetail {
		t.Fatalf("fullscreen = %v, want fullscreenDetail", m.fullscreen)
	}
	if m.fullscreenBeanID != want {
		t.Fatalf("fullscreenBeanID = %q, want %q", m.fullscreenBeanID, want)
	}
	if m.view != viewBacklog {
		t.Fatalf("view = %v, want unchanged viewBacklog (F01 never changes m.view)", m.view)
	}
}

// TestKeyFullscreenEnterOnEmptyListIsNoOp guards the orphan-root cursor case
// (focusedBean() returns nil): enter in fullscreenList must not crash or
// enter fullscreenDetail with an empty target. Needs a REAL dangling-parent
// bean in the fixture (mirrors TestKeyDetailFocusOnOrphanRootExitsGracefully,
// above) -- otherwise flattenTree never emits the synthetic "(orphaned)" root
// node at all, and m.cursorID=orphanRootID would silently resolve to
// cursorPos's no-match fallback (index 0, a REAL bean) instead.
func TestKeyFullscreenEnterOnEmptyListIsNoOp(t *testing.T) {
	beans := append(fixtureBeans(), data.Bean{
		ID: "orph-1", Title: "Orphan", Status: "todo", Type: "task", Priority: "normal", Parent: "missing",
	})
	m := fixtureModel(t, beans)
	m.cursorID = orphanRootID
	m.fullscreen = fullscreenList

	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.fullscreen != fullscreenList {
		t.Fatalf("fullscreen = %v, want unchanged fullscreenList (orphan-root cursor has no focusable bean)", m.fullscreen)
	}
	if m.fullscreenBeanID != "" {
		t.Fatalf("fullscreenBeanID = %q, want empty", m.fullscreenBeanID)
	}
}

// --- Ausstieg (esc) aus fullscreenList ---

// TestKeyFullscreenEscExitsListModeToSplitView guards the direct
// fullscreenList exit: no cursor sync needed, the cursor was never decoupled
// while in fullscreenList.
func TestKeyFullscreenEscExitsListModeToSplitView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	m.fullscreen = fullscreenList

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.cursorID != "ms-1" {
		t.Fatalf("cursorID = %q, want unchanged ms-1", m.cursorID)
	}
}

// TestKeyFullscreenEscExitsListModeToSplitViewBacklog is the Backlog mirror.
func TestKeyFullscreenEscExitsListModeToSplitViewBacklog(t *testing.T) {
	m := fixtureModel(t, backlogBeans())
	m.view = viewBacklog
	m.fullscreen = fullscreenList
	m.backlogList.cursor = 1

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.view != viewBacklog {
		t.Fatalf("view = %v, want unchanged viewBacklog", m.view)
	}
	if m.backlogList.cursor != 1 {
		t.Fatalf("backlogList.cursor = %d, want unchanged 1", m.backlogList.cursor)
	}
}

// --- History-Stack (ctrl+left/ctrl+right, [/] Fallback, F01, E9 Task 8, bean bt-1vbp) ---

// TestKeyFullscreenEscExitsListModeClearsHistoryStacks is the fullscreenList
// mirror of update_test.go's two Vollbild-Exit clearing guards -- defensive:
// navBack/navForward can only ever be populated INSIDE fullscreenDetail (the
// Scope-Entscheidung, design-spec.md §15), and fullscreenDetail never exits
// straight into fullscreenList (always via fullscreenNone first), so this
// path should already see empty stacks in practice -- pinned anyway so a
// future change to that invariant cannot silently leak History across a
// fullscreenList session (Supervisor-Entscheid: EVERY Vollbild-Exit clears
// both stacks -- ERRATA vs. design-spec.md §15's original "werden beim
// Verlassen NICHT geleert" text).
func TestKeyFullscreenEscExitsListModeClearsHistoryStacks(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.fullscreen = fullscreenList
	m.navBack = []string{"ms-1"}
	m.navForward = []string{"tk-1"}

	m = step(t, m, keyMsg(tea.KeyEsc))

	if m.fullscreen != fullscreenNone {
		t.Fatalf("fullscreen = %v, want fullscreenNone", m.fullscreen)
	}
	if m.navBack != nil || m.navForward != nil {
		t.Fatalf("navBack=%v navForward=%v, want both nil", m.navBack, m.navForward)
	}
}

// TestHistoryBackPopsAndPushesForward is the bean's own explicitly-named TDD
// step, tested via BOTH key encodings individually (Arbeitsregeln: die
// terminal-Fallback-Pfade je einzeln verifizieren, nicht nur eine Variante)
// -- pops navBack's top entry into fullscreenBeanID, pushes the bean being
// LEFT onto navForward, and reinitializes the Detail-Fokus-Maschine (same
// reset shape as every other Vollbild target-switch).
func TestHistoryBackPopsAndPushesForward(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"ctrl+left", keyMsg(tea.KeyCtrlLeft)},
		{"[ fallback", runeMsg('[')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeansWithBlocking())
			m.fullscreen = fullscreenDetail
			m.fullscreenBeanID = "bean-b"
			m.navBack = []string{"bean-a"}
			m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 2, 3, 1, 1

			m = step(t, m, tc.key)

			if m.fullscreenBeanID != "bean-a" {
				t.Fatalf("fullscreenBeanID = %q, want bean-a (popped from navBack)", m.fullscreenBeanID)
			}
			if len(m.navBack) != 0 {
				t.Fatalf("navBack = %v, want empty (popped)", m.navBack)
			}
			if len(m.navForward) != 1 || m.navForward[0] != "bean-b" {
				t.Fatalf("navForward = %v, want [bean-b] (the bean being left)", m.navForward)
			}
			if m.secCursor != 0 || m.accOpen != 1 || m.detailLevel != 0 || m.fieldCursor != 0 {
				t.Fatalf("Detail-Fokus-Maschine not reset: secCursor=%d accOpen=%d detailLevel=%d fieldCursor=%d, want 0/1/0/0",
					m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor)
			}
		})
	}
}

// TestHistoryForwardPopsAndPushesBack is HistoryBack's symmetric mirror.
func TestHistoryForwardPopsAndPushesBack(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"ctrl+right", keyMsg(tea.KeyCtrlRight)},
		{"] fallback", runeMsg(']')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeansWithBlocking())
			m.fullscreen = fullscreenDetail
			m.fullscreenBeanID = "bean-a"
			m.navForward = []string{"bean-b"}
			m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 2, 3, 1, 1

			m = step(t, m, tc.key)

			if m.fullscreenBeanID != "bean-b" {
				t.Fatalf("fullscreenBeanID = %q, want bean-b (popped from navForward)", m.fullscreenBeanID)
			}
			if len(m.navForward) != 0 {
				t.Fatalf("navForward = %v, want empty (popped)", m.navForward)
			}
			if len(m.navBack) != 1 || m.navBack[0] != "bean-a" {
				t.Fatalf("navBack = %v, want [bean-a] (the bean being left)", m.navBack)
			}
			if m.secCursor != 0 || m.accOpen != 1 || m.detailLevel != 0 || m.fieldCursor != 0 {
				t.Fatalf("Detail-Fokus-Maschine not reset: secCursor=%d accOpen=%d detailLevel=%d fieldCursor=%d, want 0/1/0/0",
					m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor)
			}
		})
	}
}

// TestHistoryBackNoOpWhenStackEmpty is the bean's own explicitly-named TDD
// step.
func TestHistoryBackNoOpWhenStackEmpty(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyCtrlLeft))

	if m.fullscreenBeanID != "bean-a" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-a (No-Op, empty navBack)", m.fullscreenBeanID)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("navForward = %v, want unchanged empty", m.navForward)
	}
}

// TestHistoryForwardNoOpWhenStackEmpty mirrors the above for HistoryForward.
func TestHistoryForwardNoOpWhenStackEmpty(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"

	m = step(t, m, keyMsg(tea.KeyCtrlRight))

	if m.fullscreenBeanID != "bean-a" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-a (No-Op, empty navForward)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("navBack = %v, want unchanged empty", m.navBack)
	}
}

// TestHistoryBackNoOpOutsideFullscreenDetail is the bean's own
// explicitly-named TDD step, table-driven over every OTHER state the
// Akzeptanz-Checkliste/Arbeitsregeln name explicitly: Split-Detail
// (fullscreenNone + detailFocus), Listen-Vollbild (fullscreenList), and the
// Tree/Backlog itself (fullscreenNone, no detailFocus).
func TestHistoryBackNoOpOutsideFullscreenDetail(t *testing.T) {
	cases := []struct {
		name       string
		fullscreen fullscreenMode
		detailFoc  bool
	}{
		{"Split-Detail", fullscreenNone, true},
		{"Listen-Vollbild", fullscreenList, false},
		{"Tree/Backlog", fullscreenNone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeansWithBlocking())
			m.cursorID = "bean-a"
			m.fullscreen = c.fullscreen
			m.detailFocus = c.detailFoc
			m.navBack = []string{"ep-1"}

			m = step(t, m, keyMsg(tea.KeyCtrlLeft))

			if len(m.navBack) != 1 || m.navBack[0] != "ep-1" {
				t.Fatalf("navBack = %v, want unchanged [ep-1] (HistoryBack must be a No-Op outside fullscreenDetail)", m.navBack)
			}
		})
	}
}

// TestHistoryForwardNoOpOutsideFullscreenDetail mirrors the above for
// HistoryForward.
func TestHistoryForwardNoOpOutsideFullscreenDetail(t *testing.T) {
	cases := []struct {
		name       string
		fullscreen fullscreenMode
		detailFoc  bool
	}{
		{"Split-Detail", fullscreenNone, true},
		{"Listen-Vollbild", fullscreenList, false},
		{"Tree/Backlog", fullscreenNone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeansWithBlocking())
			m.cursorID = "bean-a"
			m.fullscreen = c.fullscreen
			m.detailFocus = c.detailFoc
			m.navForward = []string{"ep-1"}

			m = step(t, m, keyMsg(tea.KeyCtrlRight))

			if len(m.navForward) != 1 || m.navForward[0] != "ep-1" {
				t.Fatalf("navForward = %v, want unchanged [ep-1] (HistoryForward must be a No-Op outside fullscreenDetail)", m.navForward)
			}
		})
	}
}

// TestHistoryKeysNoOpWhileOverlayOpen mirrors
// TestKeyFullscreenVNoOpWhileOverlayOrFormOpen for the History keys:
// handleKey's overlay full-capture check (update.go) runs BEFORE
// keyFullscreen's own checkpoint, so ctrl+left/[ typed while a node-action
// overlay is open inside fullscreenDetail must not navigate History --
// architecturally guaranteed by dispatch order, pinned here since it is the
// same LESSONS-LEARNED Eintrag-3 class ("neue Kaskaden IMMER end-to-end
// testen, aus jedem realistischen Ausgangszustand").
func TestHistoryKeysNoOpWhileOverlayOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.navBack = []string{"ep-1"}
	m = m.openValueMenu("status")
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: openValueMenu did not set the overlay")
	}

	m = step(t, m, keyMsg(tea.KeyCtrlLeft))

	if m.fullscreenBeanID != "bean-a" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-a (History must not leak through an open overlay)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 1 {
		t.Fatalf("navBack = %v, want unchanged [ep-1]", m.navBack)
	}
}

// TestHistoryBackForwardRoundTripReturnsToOriginalBean is the bean's own
// explicitly-named TDD step: N Relations-Sprünge, then N x Back, then N x
// Forward must land EXACTLY back at the original bean (identical stack
// depths at each end of the round trip, not just the same fullscreenBeanID).
func TestHistoryBackForwardRoundTripReturnsToOriginalBean(t *testing.T) {
	beans := []data.Bean{
		{ID: "n1", Title: "N1", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "n2", Title: "N2", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "n3", Title: "N3", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "n4", Title: "N4", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "n1"

	// 3 Relations-Sprünge: n1 -> n2 -> n3 -> n4 (activateDetailField's
	// jump case directly, same path every other Push test in
	// update_test.go exercises -- the UI path to reach it (Beziehungen
	// field enter) is covered separately, this isolates the Stack algebra).
	for _, target := range []string{"n2", "n3", "n4"} {
		b := m.focusedBean()
		tm, _ := m.activateDetailField(b, relationField{kind: "", beanID: target})
		m = tm.(model)
	}
	if m.fullscreenBeanID != "n4" {
		t.Fatalf("setup: fullscreenBeanID = %q, want n4 after 3 jumps", m.fullscreenBeanID)
	}
	if len(m.navBack) != 3 {
		t.Fatalf("setup: navBack = %v, want 3 entries after 3 jumps", m.navBack)
	}
	// T8-Review PRELUDE finding T8-F02: pin the EXACT navBack contents (push
	// order), not just its length -- a bug that pushed the right length but
	// wrong order/values would otherwise slip through undetected.
	wantNavBackAfterSetup := []string{"n1", "n2", "n3"}
	if !reflect.DeepEqual(m.navBack, wantNavBackAfterSetup) {
		t.Fatalf("setup: navBack = %v, want %v (exact push order)", m.navBack, wantNavBackAfterSetup)
	}

	for i := 0; i < 3; i++ {
		m = step(t, m, keyMsg(tea.KeyCtrlLeft))
	}
	if m.fullscreenBeanID != "n1" {
		t.Fatalf("after 3x Back: fullscreenBeanID = %q, want n1 (original)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("after 3x Back: navBack = %v, want empty", m.navBack)
	}
	if len(m.navForward) != 3 {
		t.Fatalf("after 3x Back: navForward = %v, want 3 entries", m.navForward)
	}
	// T8-F02: exact navForward contents, not just length -- Back pushes the
	// bean being LEFT in LIFO order (n4 first, then n3, then n2).
	wantNavForwardAfterBack := []string{"n4", "n3", "n2"}
	if !reflect.DeepEqual(m.navForward, wantNavForwardAfterBack) {
		t.Fatalf("after 3x Back: navForward = %v, want %v (exact LIFO push order)", m.navForward, wantNavForwardAfterBack)
	}

	for i := 0; i < 3; i++ {
		m = step(t, m, keyMsg(tea.KeyCtrlRight))
	}
	if m.fullscreenBeanID != "n4" {
		t.Fatalf("after 3x Forward: fullscreenBeanID = %q, want n4 (back to pre-Back state)", m.fullscreenBeanID)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("after 3x Forward: navForward = %v, want empty", m.navForward)
	}
	if len(m.navBack) != 3 {
		t.Fatalf("after 3x Forward: navBack = %v, want 3 entries (identical to pre-Back state)", m.navBack)
	}
	// T8-F02: the round trip's whole point is landing EXACTLY back where it
	// started -- assert the FINAL navBack/navForward contents via
	// reflect.DeepEqual against the exact pre-Back snapshots above, not just
	// matching lengths (a length-only check would miss e.g. a silent
	// element-order swap that still nets out to the same count).
	if !reflect.DeepEqual(m.navBack, wantNavBackAfterSetup) {
		t.Fatalf("after 3x Forward: navBack = %v, want %v (identical contents to pre-Back state)", m.navBack, wantNavBackAfterSetup)
	}
	if !reflect.DeepEqual(m.navForward, []string{}) {
		t.Fatalf("after 3x Forward: navForward = %v, want [] (identical contents to pre-jump state)", m.navForward)
	}
}

// TestHistoryBackSkipsVanishedEntry guards the Supervisor-Entscheid's
// F01-Analogie: a bean referenced by navBack can vanish externally (repo
// watch-reload, parallel-agent delete/rename -- same real-world trigger
// class as the F01 b==nil-Guard fix, update_test.go) between the jump and
// the Back press. HistoryBack must not land on/get stuck on the vanished
// entry -- it skips past it and lands on the next STILL-VALID entry further
// back instead (not-trap, same spirit as the guard fix).
func TestHistoryBackSkipsVanishedEntry(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-b"
	m.navBack = []string{"bean-a", "does-not-exist"} // top entry vanished

	m = step(t, m, keyMsg(tea.KeyCtrlLeft))

	if m.fullscreenBeanID != "bean-a" {
		t.Fatalf("fullscreenBeanID = %q, want bean-a (skipped past the vanished top entry)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("navBack = %v, want empty (both the vanished AND the landed-on entry consumed)", m.navBack)
	}
	if len(m.navForward) != 1 || m.navForward[0] != "bean-b" {
		t.Fatalf("navForward = %v, want [bean-b]", m.navForward)
	}
}

// TestHistoryBackAllEntriesVanishedIsCleanNoOp is
// TestHistoryBackSkipsVanishedEntry's exhaustion case: every navBack entry
// is vanished -- HistoryBack must land on nothing (fullscreenBeanID
// unchanged) and leave a CLEAN, drained stack rather than an infinite loop
// or a permanently-stuck dead entry (Supervisor-Entscheid: "No-Op mit
// sauberem Zustand").
func TestHistoryBackAllEntriesVanishedIsCleanNoOp(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-b"
	m.navBack = []string{"does-not-exist-1", "does-not-exist-2"}

	m = step(t, m, keyMsg(tea.KeyCtrlLeft))

	if m.fullscreenBeanID != "bean-b" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-b (no valid entry anywhere in navBack)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("navBack = %v, want drained empty (clean state, both vanished entries discarded)", m.navBack)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("navForward = %v, want unchanged empty (nothing was actually navigated)", m.navForward)
	}
}

// TestHistoryForwardSkipsVanishedEntry mirrors
// TestHistoryBackSkipsVanishedEntry for HistoryForward.
func TestHistoryForwardSkipsVanishedEntry(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.navForward = []string{"bean-b", "does-not-exist"}

	m = step(t, m, keyMsg(tea.KeyCtrlRight))

	if m.fullscreenBeanID != "bean-b" {
		t.Fatalf("fullscreenBeanID = %q, want bean-b (skipped past the vanished top entry)", m.fullscreenBeanID)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("navForward = %v, want empty", m.navForward)
	}
	if len(m.navBack) != 1 || m.navBack[0] != "bean-a" {
		t.Fatalf("navBack = %v, want [bean-a]", m.navBack)
	}
}

// TestHistoryBackNoOpWhenIndexNil pins T8-Review PRELUDE finding T8-F01
// (view_fullscreen.go:94): `if m.idx == nil { break }` inside HistoryBack's
// pop loop had no dedicated coverage -- every other History test builds its
// model via fixtureModel, which always leaves m.idx non-nil after the
// beansLoadedMsg round-trip. A nil m.idx IS reachable in practice (a
// History key pressed before the first beansLoadedMsg lands, or mid a
// repo-switch that clears the Index) -- the same defensive trigger class as
// the b==nil-Guard fix (update.go), just one layer earlier (no Index at all
// to resolve a bean against). Pins the IMPLEMENTED behavior as-is, not a
// fix: no panic, and no NAVIGATION happens (fullscreenBeanID/navForward
// stay untouched) -- but the popped top entry IS silently dropped from
// navBack (consumed by the loop body's reslice before the nil check fires),
// a known, accepted trade-off of this defensive guard's current shape, not
// a bug to change here.
func TestHistoryBackNoOpWhenIndexNil(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-b"
	m.navBack = []string{"bean-a"}
	m.idx = nil

	m = step(t, m, keyMsg(tea.KeyCtrlLeft))

	if m.fullscreenBeanID != "bean-b" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-b (nil m.idx: no navigation, no panic)", m.fullscreenBeanID)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("navForward = %v, want unchanged empty (no navigation happened)", m.navForward)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("navBack = %v, want drained empty (documented: the nil-idx guard consumes the popped entry)", m.navBack)
	}
}

// TestHistoryForwardNoOpWhenIndexNil mirrors TestHistoryBackNoOpWhenIndexNil
// for HistoryForward's own nil-idx guard (view_fullscreen.go:112).
func TestHistoryForwardNoOpWhenIndexNil(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "bean-a"
	m.navForward = []string{"bean-b"}
	m.idx = nil

	m = step(t, m, keyMsg(tea.KeyCtrlRight))

	if m.fullscreenBeanID != "bean-a" {
		t.Fatalf("fullscreenBeanID = %q, want unchanged bean-a (nil m.idx: no navigation, no panic)", m.fullscreenBeanID)
	}
	if len(m.navBack) != 0 {
		t.Fatalf("navBack = %v, want unchanged empty (no navigation happened)", m.navBack)
	}
	if len(m.navForward) != 0 {
		t.Fatalf("navForward = %v, want drained empty (documented: the nil-idx guard consumes the popped entry)", m.navForward)
	}
}

// --- Kein neuer viewID-Wert (Akzeptanz-Checkliste) ---

// TestFullscreenNeverChangesViewID guards the state-model's central
// invariant: fullscreen is orthogonal to m.view, never mutates it.
func TestFullscreenNeverChangesViewID(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.cursorID = "ms-1"
	before := m.view

	m = step(t, m, runeMsg('v'))

	if m.view != before {
		t.Fatalf("view changed from %v to %v -- F01 must be orthogonal to m.view", before, m.view)
	}
}

// --- Rendering ---

// TestRenderFullscreenBodyListUsesFullPaneWidth is a direct, render-grounded
// width assert (no -update Golden needed): renderPane's Width(w) sets the
// CONTENT width, the RoundedBorder adds 2 more columns on top -- rendered
// total width must equal innerW+2, proving the fullscreenList branch uses
// the FULL innerW (not a Split lw/rw).
func TestRenderFullscreenBodyListUsesFullPaneWidth(t *testing.T) {
	rows := []string{"row one", "row two"}
	out := renderFullscreenBody(fullscreenList, 40, 10, rows, true, nil, nil, 0, 1, 0, 0, 0)
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("renderFullscreenBody returned no lines")
	}
	if w := lipgloss.Width(lines[0]); w != 42 {
		t.Fatalf("renderFullscreenBody(fullscreenList) rendered width = %d, want 42 (innerW 40 + 2 border columns)", w)
	}
}

// TestRenderFullscreenBodyDetailUsesFullPaneWidth mirrors the List case for
// fullscreenDetail (renderAccordionPane's own Width(w) + border contract).
func TestRenderFullscreenBodyDetailUsesFullPaneWidth(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	b := m.idx.ByID["ms-1"]

	out := renderFullscreenBody(fullscreenDetail, 50, 12, nil, true, m.idx, b, 0, 1, 0, 0, 0)
	lines := strings.Split(out, "\n")
	if len(lines) == 0 {
		t.Fatal("renderFullscreenBody returned no lines")
	}
	if w := lipgloss.Width(lines[0]); w != 52 {
		t.Fatalf("renderFullscreenBody(fullscreenDetail) rendered width = %d, want 52 (innerW 50 + 2 border columns)", w)
	}
}

// TestViewBrowseRepoFullscreenFitsOuterFrame is a regression guard for a REAL
// bug the tmux smoke caught live (not visible to
// TestRenderFullscreenBodyListUsesFullPaneWidth/...DetailUsesFullPaneWidth
// above, which only exercise renderFullscreenBody in ISOLATION): the call
// site originally passed innerW straight through as the single pane's
// content width, double-counting the pane's OWN RoundedBorder against the
// outer frame's border and overflowing it by 2 columns/rows -- visibly
// broke the outer frame in a real terminal (the bottom border wrapped onto
// its own line). Render-grounded at the FULL m.View() level, both
// fullscreen flavors, so a future regression at either call site (Tree OR
// Backlog) is caught the same way this one was.
func TestViewBrowseRepoFullscreenFitsOuterFrame(t *testing.T) {
	for _, fs := range []fullscreenMode{fullscreenList, fullscreenDetail} {
		m := fixtureModel(t, fixtureBeans())
		m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
		m.cursorID = "ms-1"
		m.fullscreen = fs
		m.fullscreenBeanID = "ms-1"

		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) > 30 {
			t.Fatalf("fullscreen=%v: View() produced %d lines, want <= 30 (outer frame overflow)", fs, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 100 {
				t.Fatalf("fullscreen=%v: line %d width = %d, want <= 100 (outer frame overflow): %q", fs, i, w, l)
			}
		}
	}
}

// TestOverlayRendersOverFullscreen pins Review-Finding I01's first half
// (Fix-Runde 1, bean bt-13l7): a node-action overlay opened FROM inside the
// Vollbild-Detail (every s/t/a/r/d key works there verbatim via
// focusedBean()'s fullscreenDetail case) must actually render ON TOP of the
// fullscreen body -- composeOverlays runs after the new fullscreen branch,
// structurally correct but previously unpinned.
func TestOverlayRendersOverFullscreen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = "ms-1"

	m = m.openValueMenu("status")
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: openValueMenu did not set the overlay (focusedBean's fullscreenDetail case broken?)")
	}

	out := ansi.Strip(m.View())
	if !strings.Contains(out, "enter:apply") {
		t.Fatalf("value menu's own hint line must be visible over the fullscreen body, got:\n%s", out)
	}
}

// TestKeyFullscreenVNoOpWhileOverlayOrFormOpen pins Review-Finding I01's
// second half: `v` typed while a full-capture state is active (node-action
// overlay, huh form) must NOT enter the Vollbild -- handleKey's capture
// order routes those keys to keyOverlay/keyForm long before keyFullscreen's
// checkpoint is reached (structurally correct, previously unpinned).
func TestKeyFullscreenVNoOpWhileOverlayOrFormOpen(t *testing.T) {
	t.Run("overlay", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m.cursorID = "ms-1"
		m = m.openValueMenu("status")

		m = step(t, m, runeMsg('v'))

		if m.fullscreen != fullscreenNone {
			t.Fatalf("fullscreen = %v, want fullscreenNone (v must not leak through an open overlay)", m.fullscreen)
		}
	})
	t.Run("form", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m.cursorID = "ms-1"
		tm, _ := m.openCreateForm()
		fm, ok := tm.(model)
		if !ok || fm.form == nil {
			t.Fatal("setup: openCreateForm did not set m.form")
		}

		fm = step(t, fm, runeMsg('v'))

		if fm.fullscreen != fullscreenNone {
			t.Fatalf("fullscreen = %v, want fullscreenNone (v must not leak through an open form)", fm.fullscreen)
		}
	})
}

// TestFullscreenRecomputesGeometryOnResize pins Review-Finding I02
// (Fix-Runde 1, bean bt-13l7): a WindowSizeMsg arriving AFTER the Vollbild
// was entered must re-derive the single pane's width/height from the NEW
// terminal size -- architecturally guaranteed (the fullscreen branch
// recomputes from m.width/m.height on every render, no cached geometry),
// but previously unpinned. Same frame-fits assertion as
// TestViewBrowseRepoFullscreenFitsOuterFrame, just with the resize
// happening while the Vollbild is already active.
func TestFullscreenRecomputesGeometryOnResize(t *testing.T) {
	for _, fs := range []fullscreenMode{fullscreenList, fullscreenDetail} {
		m := fixtureModel(t, fixtureBeans())
		m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
		m.cursorID = "ms-1"
		m.fullscreen = fs
		m.fullscreenBeanID = "ms-1"

		// Resize AFTER fullscreen entry -- the render must follow.
		m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})

		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) > 24 {
			t.Fatalf("fullscreen=%v: View() after resize produced %d lines, want <= 24", fs, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 80 {
				t.Fatalf("fullscreen=%v: line %d width = %d after resize, want <= 80: %q", fs, i, w, l)
			}
		}
	}
}

// TestViewBacklogFullscreenFitsOuterFrame is the Backlog mirror.
func TestViewBacklogFullscreenFitsOuterFrame(t *testing.T) {
	for _, fs := range []fullscreenMode{fullscreenList, fullscreenDetail} {
		m := fixtureModel(t, backlogBeans())
		m.view = viewBacklog
		m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
		vis := m.backlogVisible()
		if len(vis) == 0 {
			t.Fatal("setup: need at least 1 backlog-visible bean")
		}
		m.backlogList.setLen(len(vis))
		m.backlogList.cursor = 0
		m.fullscreen = fs
		m.fullscreenBeanID = vis[0].ID

		out := m.View()
		lines := strings.Split(out, "\n")
		if len(lines) > 24 {
			t.Fatalf("fullscreen=%v: View() produced %d lines, want <= 24 (outer frame overflow)", fs, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 80 {
				t.Fatalf("fullscreen=%v: line %d width = %d, want <= 80 (outer frame overflow): %q", fs, i, w, l)
			}
		}
	}
}
