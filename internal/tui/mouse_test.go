package tui

// mouse_test.go — TDD coverage for E5 Task 4 (bean bt-mne6, epic bt-5h4d,
// design decision f): Maus (Wheel/Klick/Doppelklick). Mirrors devd
// mouse_test.go's render-grounded click pattern (~/dev/Developer
// Dashboard/apps/cli-go/internal/tui/mouse_test.go): a click coordinate is
// found by rendering the REAL View() and locating a known substring on
// screen, never hand-computed against the click formula itself (that would
// be circular -- a formula bug would pass its own test). Doppelklick tests
// inject a fixed m.clock (mirrors devd's own TestMouseDoubleClickCollapses
// OpenNode/TestMouseSingleClickOnOpenNodeStaysOpen pattern) for a
// deterministic <500ms window instead of a real sleep.

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// wheelMsg builds a wheel tea.MouseMsg (X/Y are irrelevant to wheel dispatch,
// handleMouse's wheel branch only reads Button).
func wheelMsg(b tea.MouseButton) tea.MouseMsg {
	return tea.MouseMsg{Button: b, Action: tea.MouseActionPress}
}

// screenLines renders m's FULL View() and splits it into ANSI-stripped
// lines -- real screen geometry (outer border, panes, everything), the same
// ground truth clickAt/leftPaneClickAt below search over.
func screenLines(m model) []string {
	return strings.Split(ansi.Strip(m.View()), "\n")
}

// leftPaneClickAt finds substr's first occurrence WITHIN the left (list)
// pane's column span (boundary = originX+lw, from clickPaneGeometry,
// mouse.go) of m's rendered View() and returns a real left-click
// tea.MouseMsg at that screen coordinate. Restricting to the left pane
// matters: the right Detail pane can render the SAME text (e.g. a bean
// title also shown in its own Meta section, or listed under a parent's
// Beziehungen section) -- a plain "first match anywhere" search could
// silently click the wrong pane. head/localKeys must be the SAME strings
// the caller's own view renders with (browseRepoChrome/readyChrome), so
// the boundary matches the real render exactly.
func leftPaneClickAt(t *testing.T, m model, head, localKeys, substr string) tea.MouseMsg {
	t.Helper()
	_, lw, _, originX, _ := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, false)
	boundary := originX + lw
	for y, l := range screenLines(m) {
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		col := ansi.StringWidth(l[:i])
		if col >= boundary {
			continue // leftmost match is already in the right Detail pane
		}
		return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: col, Y: y}
	}
	t.Fatalf("substr %q not found in the left pane of the rendered View()", substr)
	return tea.MouseMsg{}
}

// treeClickAt is leftPaneClickAt scoped to the Browse/Tree view's own
// chrome (browseRepoChrome, view_browse_repo.go).
func treeClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	head, localKeys := m.browseRepoChrome(m.width - 2)
	return leftPaneClickAt(t, m, head, localKeys, substr)
}

// rightPaneClickAt mirrors leftPaneClickAt but scoped to the RIGHT Detail
// pane's own column span (X >= originX+lw) -- B07 (design-spec.md §15
// PF-16, bean bt-duz7)'s render-grounded click helper for the
// mouseDetailClick dispatch path.
func rightPaneClickAt(t *testing.T, m model, head, localKeys, substr string) tea.MouseMsg {
	t.Helper()
	_, lw, _, originX, _ := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(m.width-2), m.settings.Layout.TreeWidth, false)
	boundary := originX + lw
	for y, l := range screenLines(m) {
		i := strings.Index(l, substr)
		if i < 0 {
			continue
		}
		col := ansi.StringWidth(l[:i])
		if col < boundary {
			continue // leftmost match is in the left pane
		}
		return tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: col, Y: y}
	}
	t.Fatalf("substr %q not found in the right Detail pane of the rendered View()", substr)
	return tea.MouseMsg{}
}

// detailClickAt is rightPaneClickAt scoped to the Browse/Tree view's own
// chrome (browseRepoChrome).
func detailClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	head, localKeys := m.browseRepoChrome(m.width - 2)
	return rightPaneClickAt(t, m, head, localKeys, substr)
}

// readyDetailClickAt is rightPaneClickAt scoped to the Ready view's own
// chrome (readyChrome) -- guards detailClickRow's m.view branch.
func readyDetailClickAt(t *testing.T, m model, substr string) tea.MouseMsg {
	t.Helper()
	head, localKeys := m.readyChrome(m.width - 2)
	return rightPaneClickAt(t, m, head, localKeys, substr)
}

// --- Wheel: moves the active view's cursor (design decision f) ---

func TestWheelUpDownMovesTreeCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true // nodes: ms-1(0) ep-1(1) tk-1(2) tk-2(3)
	m.cursorID = "ms-1"

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel down) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ep-1" {
		t.Fatalf("wheel down: cursorID = %q, want ep-1", m2.cursorID)
	}

	tm2, _ := m2.handleMouse(wheelMsg(tea.MouseButtonWheelUp))
	m3 := tm2.(model)
	if m3.cursorID != "ms-1" {
		t.Fatalf("wheel up: cursorID = %q, want ms-1", m3.cursorID)
	}
}

func TestWheelMovesReadyCursor(t *testing.T) {
	m := readyBeansModel(t)
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.view = viewReady
	vis := m.readyVisible()
	if len(vis) < 2 {
		t.Fatalf("setup: need >=2 ready-visible beans, got %d", len(vis))
	}
	m.readyList.setLen(len(vis))
	m.readyList.cursor = 0

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel down) did not return a model, got %T", tm)
	}
	if m2.readyList.cursor != 1 {
		t.Fatalf("wheel down: readyList.cursor = %d, want 1", m2.readyList.cursor)
	}

	tm2, _ := m2.handleMouse(wheelMsg(tea.MouseButtonWheelUp))
	m3 := tm2.(model)
	if m3.readyList.cursor != 0 {
		t.Fatalf("wheel up: readyList.cursor = %d, want 0", m3.readyList.cursor)
	}
}

// --- Click: sets the Tree cursor, render-grounded ---

func TestClickSetsTreeCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "ms-1"

	msg := treeClickAt(t, m, "Task Two")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.cursorID != "tk-2" {
		t.Fatalf("click on \"Task Two\" row: cursorID = %q, want tk-2", m2.cursorID)
	}
}

// --- Doppelklick: devd D03 semantics (design decision f) ---
//
// NOTE (bean bt-vpvu): devd-D03's "a single click EXPANDS a closed node" half
// was dropped on PO request -- a single click now only selects, and the
// double click toggles in BOTH directions. The two tests below cover the
// halves that survived unchanged (double click collapses an open node; a lone
// single click never toggles); the new semantics live in
// mouse_tree_select_test.go.

// TestDoubleClickTogglesExpand: a SECOND click on the SAME node within
// doubleClickInterval collapses an already-open, expandable node.
func TestDoubleClickTogglesExpand(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true // Epic One open, expandable (2 children)
	m.cursorID = "ms-1"
	fixed := time.Unix(1000, 0)
	m.clock = func() time.Time { return fixed }

	msg := treeClickAt(t, m, "Epic One")
	tm, _ := m.handleMouse(msg) // 1st click: registers, does not collapse
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if !m2.expanded["ep-1"] {
		t.Fatalf("setup: first click on an open node must not collapse it")
	}

	tm2, _ := m2.handleMouse(msg) // 2nd click, same fixed time/node -> double
	m3 := tm2.(model)
	if m3.expanded["ep-1"] {
		t.Fatalf("double click on an open, expandable node must collapse it (devd D03)")
	}
}

// TestSingleClickOnOpenNodeDoesNotCollapse: an ISOLATED single click on an
// already-open node only moves the cursor -- it must NOT toggle (devd D03,
// distinct from the double-click case above: no second click ever arrives).
func TestSingleClickOnOpenNodeDoesNotCollapse(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "ms-1"
	fixed := time.Unix(2000, 0)
	m.clock = func() time.Time { return fixed }

	msg := treeClickAt(t, m, "Epic One")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if !m2.expanded["ep-1"] {
		t.Fatal("a single click on an already-open node must NOT collapse it (devd D03)")
	}
	if m2.cursorID != "ep-1" {
		t.Fatalf("cursorID = %q, want ep-1 (click still moves the cursor)", m2.cursorID)
	}
}

// --- Overlay guard: mouse ignored while a form/overlay/palette/search/
// filter/help/quit-confirm fully captures input (devd precedent) ---

func TestMouseIgnoredWhileFormOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.cursorID = "ms-1"

	nm, _ := m.openCreateForm()
	m, ok := nm.(model)
	if !ok || m.form == nil {
		t.Fatal("setup: openCreateForm did not set m.form")
	}

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ms-1" {
		t.Fatalf("wheel while a form is open moved the cursor: cursorID = %q, want unchanged (ms-1)", m2.cursorID)
	}
}

func TestMouseIgnoredWhileOverlayOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.cursorID = "ms-1"
	m.overlay = overlayValueMenu

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ms-1" {
		t.Fatalf("wheel while an overlay is open moved the cursor: cursorID = %q, want unchanged (ms-1)", m2.cursorID)
	}
}

// TestHandleMouseIgnoredWhenFullscreenActive guards F01's own documented
// Scope-Cut (design-spec.md §15, E9 Task 7, bean bt-13l7): a click/wheel
// against the fullscreen single-pane geometry would otherwise be
// misinterpreted against the (wrong) Split-Geometry clickPaneGeometry/
// treeClickRow/etc. still compute -- the guard turns it into a safe no-op,
// mirroring TestMouseIgnoredWhileOverlayOpen's own pattern.
func TestHandleMouseIgnoredWhenFullscreenActive(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.cursorID = "ms-1"
	m.fullscreen = fullscreenList

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ms-1" {
		t.Fatalf("wheel while fullscreen is active moved the cursor: cursorID = %q, want unchanged (ms-1)", m2.cursorID)
	}
}

// TestHandleMouseIgnoredOnTagManagementPage guards F02 (E10 Task 2, bean
// bt-r92i, T2-Review Fix-Runde 1): the Tag-Management page is a
// keyboard-only full-capture state -- wheel and click must be no-ops there,
// same contract as viewLobby (mirrors TestHandleMouseIgnoredWhenFullscreen
// Active's pattern). Honest scope note: this test pins the CONTRACT, not
// the specific guard line -- today the guard is redundant in effect
// (wheelMove's and the left-click dispatch's view switches have no
// viewTagManagement case and no default, so removal of the F02 guard alone
// does not turn this red); the test exists so any FUTURE change to either
// switch that starts routing mouse events against this page's foreign
// geometry fails loudly here.
func TestHandleMouseIgnoredOnTagManagementPage(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.cursorID = "ms-1"
	nm, _ := m.openTagManagementPage()
	m, ok := nm.(model)
	if !ok || m.view != viewTagManagement {
		t.Fatal("setup: openTagManagementPage did not switch to viewTagManagement")
	}

	tm, _ := m.handleMouse(wheelMsg(tea.MouseButtonWheelDown))
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(wheel) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ms-1" {
		t.Fatalf("wheel on the tag page moved the TREE cursor: cursorID = %q, want unchanged (ms-1)", m2.cursorID)
	}
	if m2.tagMgmtCursor.cursor != m.tagMgmtCursor.cursor {
		t.Fatalf("wheel on the tag page moved tagMgmtCursor: %d, want unchanged (%d)", m2.tagMgmtCursor.cursor, m.tagMgmtCursor.cursor)
	}

	click := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: 5, Y: 5}
	tm2, _ := m.handleMouse(click)
	m3, ok := tm2.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm2)
	}
	if m3.cursorID != "ms-1" || m3.view != viewTagManagement || m3.overlay != overlayNone {
		t.Fatalf("click on the tag page had a side effect: cursorID=%q view=%v overlay=%v", m3.cursorID, m3.view, m3.overlay)
	}
}

// TestToastClickDismissesEvenWithFormOpen is the Cross-Feature-Fix
// regression guard (design decision a, Port devd DD2-272/273): a Toast
// click-dismiss must reach the PO even while a form is open. Goes through
// the FULL m.Update() dispatcher (not handleMouse directly) -- this is the
// test update.go's own placement comment calls out as the actual
// verification that the tea.MouseMsg case runs ahead of the `if m.form !=
// nil` fallback, not just documentation of the assumption.
func TestToastClickDismissesEvenWithFormOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})

	nm, _ := m.openCreateForm()
	m, ok := nm.(model)
	if !ok || m.form == nil {
		t.Fatal("setup: openCreateForm did not set m.form")
	}

	m, _ = m.showToast(toastInfo, "x", "", nil, false)
	x, y, _, _ := m.toastGeometry()

	tm, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, X: x, Y: y})
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(mouse) did not return a model, got %T", tm)
	}
	if m2.toast != nil {
		t.Fatal("a click on the toast must dismiss it even while a form is open (Cross-Feature-Fix)")
	}
	if m2.form == nil {
		t.Error("the form must stay open -- only the toast is dismissed by this click")
	}
}

// --- TreeWidth wiring (T6b, bean bt-pd22, T5-Review I01): clickPaneGeometry
// resolves treeWidth (the caller's m.settings.Layout.TreeWidth) into
// masterDetailWidths' floor via treeWidthFloor (mouse.go) instead of the
// hardcoded "24" every caller passed before this task. w=72/h=30 is chosen
// so innerW=70's own w/3=23 floor-1fr sits BELOW both 24 (the fallback) and
// 28 (the configured value below) while w*2/5=28's cap does NOT clip 28 --
// i.e. a width where the floor's own value is what determines lw, not the
// unrelated 1fr/cap arithmetic masterDetailWidths (view.go) also applies.

// TestTreeWidthZeroFallsBackToDefault guards the 0/unset fallback: every
// existing golden fixture/test model's m.settings is the config.Settings{}
// zero value (TreeWidth 0) until app.go's Run() or a Settings-Form submit
// populates it -- clickPaneGeometry(treeWidth=0) must still resolve to 24,
// the SAME value this file hardcoded before T6b, so the seven Golden
// snapshot tests stay byte-identical.
func TestTreeWidthZeroFallsBackToDefault(t *testing.T) {
	_, lw, _, _, _ := clickPaneGeometry(72, 30, "head", "footer", "", 0, false)
	if lw != 24 {
		t.Fatalf("clickPaneGeometry(treeWidth=0): lw = %d, want 24 (fallback, byte-identical to the pre-T6b hardcoded floor)", lw)
	}
}

// TestTreeWidthFromSettingsAffectsGeometry guards the actual wiring: a
// configured m.settings.Layout.TreeWidth must widen the left pane beyond
// the 0/unset fallback -- the whole point of T6b (T5-Review I01: before
// this task, the Settings-Form's tree_width field was persisted/validated
// but had NO effect on the render, a silent no-op for the PO).
func TestTreeWidthFromSettingsAffectsGeometry(t *testing.T) {
	_, lwDefault, _, _, _ := clickPaneGeometry(72, 30, "head", "footer", "", 0, false)
	_, lwConfigured, _, _, _ := clickPaneGeometry(72, 30, "head", "footer", "", 28, false)
	if lwConfigured != 28 {
		t.Fatalf("clickPaneGeometry(treeWidth=28): lw = %d, want 28", lwConfigured)
	}
	if lwConfigured <= lwDefault {
		t.Fatalf("configured treeWidth (28) did not widen the left pane vs. the default fallback (lw=%d): got lw=%d", lwDefault, lwConfigured)
	}
}

// TestClickPaneGeometryOriginYExcludesTitleAndSeparator guards PF-10
// (design-spec.md §15, epic-E7-plan.md »Task 5«, bean bt-uyzf): renderPane no
// longer draws a title line + underline-separator ahead of a pane's rows, so
// clickPaneGeometry's originY loses the two `+1` terms that used to account
// for them -- originY is now outer-top-border(1) + head height + divider(1)
// + the pane's OWN top border(1) (a DIFFERENT line, still there), with row 0
// of the caller's own windowed-content index space starting immediately
// after.
func TestClickPaneGeometryOriginYExcludesTitleAndSeparator(t *testing.T) {
	head := "head"
	_, _, _, _, originY := clickPaneGeometry(80, 24, head, "footer", "", 0, false)
	want := 1 + lipgloss.Height(head) + 1 + 1
	if originY != want {
		t.Fatalf("originY = %d, want %d (outer border(1) + head(%d) + divider(1) + pane's own top border(1) -- PF-10 drops only the pane's title+separator lines)", originY, want, lipgloss.Height(head))
	}
}

// TestClickPaneGeometryMultiLineFooterHeightPin (PRELUDE, T6-Review F01,
// bean bt-13l7, 2026-07-16): D02 collapsed the Ready/Browse footer
// divergence that used to be the ONLY real render exercising clickPaneGeometry
// with footH != 2 (TestDetailClickReadyFooterAt80Cols's own doc comment,
// above) -- every OTHER clickPaneGeometry test in this file (and every real
// Chrome at >=80 columns, post-D02) now renders a 2-line localKeys, so the
// bodyH/footerY boundary math has not been exercised at any OTHER footH value
// since D02 landed. This is a SYNTHETIC (hand-built, not render-grounded)
// regression: a fabricated >=3-line localKeys string proves the dynamic
// footH := lipgloss.Height(localKeys)+1+statusLineHeight(status) mechanism
// (mouse.go) generalizes to any footer height, independent of whether a real
// view ever happens to diverge from 2 lines again. head="H" (1 line) + a
// 3-line localKeys + w=80/h=40 is a small enough fixture that bodyH/originY/
// footerY can be verified against literal numbers, not a second call into the
// function under test (LESSONS-LEARNED #5: a pin test must pin LITERALS,
// never the constant/formula it's supposed to be checking).
//
// bt-oqsv UPDATE: the footer's status row is no longer a permanently
// reserved line, so footH's old constant "+2" became "+1 (divider) +
// statusLineHeight(status)". Both states are pinned below -- the empty
// status (the normal case, one row MORE for the panes) and a present one
// (the pre-bt-oqsv numbers, unchanged).
//
//	innerH = h-2                                          = 38
//	footH  = 3 (localKeys lines) + 1 (divider) + 0|1 (status) = 4 | 5
//	avail  = innerH - head(1) - footH - 1(divider)        = 32 | 31
//	bodyH  = avail - 2 (both panes' own border)           = 30 | 29
//	originY = 1 (outer border) + head(1) + 1 (divider) + 1 (pane's own border) = 4
//	footerY = originY + bodyH + 2 (pane bottom border + divider, D02-era
//	          TestDetailClickReadyFooterAt80Cols precedent)   = 36 | 35
func TestClickPaneGeometryMultiLineFooterHeightPin(t *testing.T) {
	head := "H"
	localKeys := "line one\nline two\nline three" // 3 lines -- the bean's own ">=3-Zeilen" requirement

	bodyH, _, _, _, originY := clickPaneGeometry(80, 40, head, localKeys, "", 0, false)
	if bodyH != 30 {
		t.Fatalf("bodyH (no status line) = %d, want 30 (innerH 38 - head 1 - footH 4 - divider 1 = avail 32, bodyH = avail-2)", bodyH)
	}
	if originY != 4 {
		t.Fatalf("originY = %d, want 4 (outer border 1 + head 1 + divider 1 + pane's own border 1) -- must stay independent of footer height", originY)
	}
	if footerY := originY + bodyH + 2; footerY != 36 {
		t.Fatalf("footerY (no status line) = %d, want 36 (originY 4 + bodyH 30 + pane-bottom-border/divider 2)", footerY)
	}

	bodyHStatus, _, _, _, originYStatus := clickPaneGeometry(80, 40, head, localKeys, "watch unavailable", 0, false)
	if bodyHStatus != 29 {
		t.Fatalf("bodyH (status line present) = %d, want 29 (footH 5: 3 localKeys + 1 divider + 1 status)", bodyHStatus)
	}
	if originYStatus != 4 {
		t.Fatalf("originY (status line present) = %d, want 4 -- the status row sits BELOW the panes, it can never move their origin", originYStatus)
	}
	if footerY := originYStatus + bodyHStatus + 2; footerY != 35 {
		t.Fatalf("footerY (status line present) = %d, want 35 (originY 4 + bodyH 29 + pane-bottom-border/divider 2)", footerY)
	}
}

// --- B07 (design-spec.md §15 PF-16, bean bt-duz7, E8 Task 4): Maus im
// Detail-Pane -- Sektionen + Meta-Felder klickbar ---
