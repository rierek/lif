package tui

// mouse.go — E5 Task 4 (bean bt-mne6, epic bt-5h4d, design decision f): Maus
// (Wheel/Klick/Doppelklick). Port devd update.go's Mausteil (handleMouse,
// ~/dev/DeveloperDashboard/apps/cli-go/internal/tui/update.go:
// 459-524, mouseTreeClick/treeClickIdx:887-950) -- TWO deliberate deviations
// vs. devd's own handleMouse, both consequences of design decision f
// (view_browse_repo.go's own doc comment, "kein m.scroll-Feld existiert"):
//
//  1. Wheel moves the active view's CURSOR (m.cursorID/m.backlogList.cursor
//     via the two *CursorMove helpers, view_browse_repo.go/
//     view_browse_backlog.go) instead of devd's `m.scroll -= 3`/`+= 3`
//     non-Tree branch -- beans-tui has no scroll-offset field to move;
//     windowAround/windowStart already follow the cursor automatically on
//     render. EXCEPTION (bean bt-ze10, epic bt-vy1q F1): while
//     boxFormEnabled(), a wheel tick landing inside the Detail pane's own
//     bounds (boxFormWheelHit, below) now scrolls model.boxFormScroll
//     instead -- the one real scroll-offset field this codebase has, and
//     deliberately scoped to that ONE mode/pane so this doc comment's claim
//     stays true everywhere else (wheelMove's own doc comment has the gate).
//  2. Left-click dispatch is a SINGLE `switch m.view` over two sibling
//     views (viewBrowseRepo/viewBacklog), not devd's Tree-vs-everything-else
//     split (devd's non-Tree views are all Chrome()-scrolled detail screens;
//     beans-tui's two views are both master-detail panes with their own
//     *ClickRow row-mapping, view_*.go).
//
// Toast-Klick-Vorrang (design decision a, overlay_show_toast.go's own doc
// comment): handleMouse's FIRST check is m.toastHit/dismissToast,
// unconditional on any overlay/form guard below it -- Port devd
// update.go:463-468 verbatim, Cross-Feature-Fix precedent (devd DD2-272/
// 273): a Toast is not modal, so its click-dismiss must reach the PO even
// while a form/overlay is open. TestToastClickDismissesEvenWithFormOpen
// (mouse_test.go) is the regression guard for this exact ordering.

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
)

// minTreeWidthFloor/maxTreeWidthFloor mirror config.minTreeWidth/
// config.maxTreeWidth (internal/config/settings.go) -- unexported there, so
// duplicated here as literal values rather than imported (T6b, bean
// bt-pd22, T5-Review I01): the two packages' clamp ranges must agree, but
// there is no shared exported constant to single-source them from without
// widening config's public surface for two int literals.
const (
	minTreeWidthFloor = 24
	maxTreeWidthFloor = 60
)

// treeWidthFloor resolves configured (the caller's m.settings.Layout.
// TreeWidth, clickPaneGeometry's own treeWidth param below) into the
// numeric floor masterDetailWidths (view.go) receives. 0/unset -- every
// existing golden fixture/test model built via newModel/fixtureModel, whose
// m.settings is the config.Settings{} zero value until app.go's Run() (or a
// Settings-Form submit) populates it (types.go's own "settings" field
// doc-stamp) -- falls back to 24, the SAME value this file hardcoded before
// T6b, so the Golden snapshot tests (tree/chrome/backlog) stay
// byte-identical. A configured value is additionally
// clamped into [minTreeWidthFloor,maxTreeWidthFloor] -- consistent with
// config.validateSettings' own [24,60] clamp range: belt-and-braces here,
// since a LIVE model's m.settings already arrives pre-clamped via
// config.LoadSettings (app.go Run()) / the settings-Submit's own re-read
// (config.LoadSettings, box_confirm_create.go submitForm "settings" case).
func treeWidthFloor(configured int) int {
	if configured <= 0 {
		return minTreeWidthFloor
	}
	if configured < minTreeWidthFloor {
		return minTreeWidthFloor
	}
	if configured > maxTreeWidthFloor {
		return maxTreeWidthFloor
	}
	return configured
}

// clickPaneGeometry recomputes the numeric frame geometry every one of the
// two View functions (viewBrowseRepo/viewBacklog, view_browse_repo.go/
// view_browse_backlog.go) build ahead of their own two-pane body -- bodyH/
// lw/rw plus the screen origin (originX/originY) of the pane's FIRST
// windowed content row (the search-or-summary head row already consumed --
// PF-10, bean bt-uyzf, removed the pane's own title+separator lines, so
// nothing else precedes it now). Single source for BOTH halves: the two View functions
// themselves (each now calls this instead of an independently maintained
// avail/bodyH/lw/rw copy) AND treeClickRow/backlogClickRow (Golden-Rule-
// Drift-Schutz, mirrors windowStart's own shared-geometry rationale,
// view_browse_repo.go). head/localKeys are the caller's own EXACT
// breadcrumb()/footer() strings (browseRepoChrome/backlogChrome), so headH
// always matches that view's real head height (narrow-terminal 2-line
// breadcrumb wrap included). treeWidth is the caller's OWN
// m.settings.Layout.TreeWidth (T6b, bean bt-pd22, T5-Review I01 -- BEFORE
// this task, every caller passed a hardcoded "24" straight into
// masterDetailWidths below, a silent no-op for the Settings-Form's own
// tree_width field) -- resolved via treeWidthFloor just above before
// reaching masterDetailWidths, so every one of this function's View-function
// callers AND its *ClickRow callers picks up a configured Baumbreite
// consistently, the same Single-Source guarantee this doc comment's opening
// paragraph already establishes for bodyH/lw/rw/originX/originY themselves.
//
// status is the caller's OWN m.statusLine(innerW) string (view.go, bean
// bt-oqsv): the footer's status row is no longer a permanently reserved
// blank line, it exists only while it carries the watch-unavailable
// indicator. It therefore has to be an INPUT here -- the old hardcoded "+2"
// silently assumed the row was always present, and keeping it after the
// reservation was dropped would offset every mouse click by exactly one row.
func clickPaneGeometry(w, h int, head, localKeys, status string, treeWidth int) (bodyH, lw, rw, originX, originY int) {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	innerH := h - 2
	footH := lipgloss.Height(localKeys) + 1 + statusLineHeight(status) // + divider above footer + optional status line (bt-oqsv)
	avail := innerH - lipgloss.Height(head) - footH - 1                // - divider under the top bar
	if avail < 4 {
		avail = 18 // height unknown (init/tests) -> generous fallback, mirrors Chrome()
	}
	bodyH = avail - 2 // both panes add their own border (+2, Golden Rule #1)
	if bodyH < 1 {
		bodyH = 1
	}
	lw, rw = masterDetailWidths(innerW, treeWidthFloor(treeWidth))
	// originY: outer top border(1) + head line(s) + divider(1) + the pane's
	// OWN top border(1) -- its title line + separator line no longer exist
	// (PF-10, design-spec.md §15, epic-E7-plan.md »Task 5«, bean bt-uyzf:
	// renderPane dropped both, the Breadcrumb alone carries view identity
	// now; the pane's own top border is a DIFFERENT line and stays). The row
	// after this is row 0 of the caller's own windowed-content index space
	// (the search/summary head line).
	originY = 1 + lipgloss.Height(head) + 1 + 1
	originX = 1 // outer border's left column
	return
}

// handleMouse dispatches every tea.MouseMsg (Update()'s own case, update.go
// -- placed ahead of the `if m.form != nil` fallback per that case's own
// placement comment). Toast-Klick-Vorrang first (design decision a, this
// file's own doc comment), THEN the overlay guard (forms/pickers/menus/
// palette/search/filter/help/quit-confirm ignore the mouse entirely, devd
// precedent update.go:470-474), THEN wheel, THEN left-click dispatch.
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress && m.toastHit(msg.X, msg.Y) {
		return m.dismissToast()
	}

	// F01 (design-spec.md §15, E9 Task 7, bean bt-13l7): Maus im Vollbild --
	// explizites Nicht-Ziel (v1, dokumentierter Scope-Cut). A click/wheel
	// against the fullscreen single-pane geometry would otherwise be
	// interpreted against the (WRONG) Split-Geometry clickPaneGeometry/
	// treeClickRow/backlogClickRow/detailClickRow still compute -- this guard
	// turns that into a safe no-op instead of a false hit against the wrong
	// row/section. Placed directly after the Toast-Klick-Vorrang (that stays
	// unconditional, design decision a) and BEFORE the pre-existing overlay
	// guard below (same full-capture precedent).
	if m.fullscreen != fullscreenNone {
		return m, nil
	}

	// Slice D (bt-f0y9, "feld-verankertes Inline-Dropdown", D09 revidiert):
	// the anchored value menu is the ONE overlay mouse-native enough to act
	// on a click (box_menu_value.go's mouseValueMenuClick) -- checked BEFORE
	// the blanket overlay guard below, which would otherwise swallow this
	// click same as every other overlay (devd precedent, update.go:470-474,
	// still correct for every OTHER overlay: pickers/forms/palette/help/
	// confirms stay keyboard-only, unchanged). Gated on boxFormEnabled()
	// here too (mirrors composeOverlays' own identical gate,
	// view_browse_repo.go) so accordion mode's centered value menu (Slice C,
	// out of Slice D's scope) falls straight through to the blanket guard
	// below, UNCHANGED.
	if boxFormEnabled() && m.overlay == overlayValueMenu &&
		msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		return m.mouseValueMenuClick(msg)
	}

	// Modale/Overlays sind tastaturgesteuert -- Maus ignorieren (kein
	// Fehlklick-Fokus), devd precedent update.go:470-474. m.view ==
	// viewLobby closes the gap bt-mne6's own Notes-für-T6 flagged (T4 could
	// not add this guard since viewLobby did not exist yet) -- E5 Task 6
	// (bean bt-zhwl): the Lobby is keyboard-only in V1 (design-spec.md §9
	// scope cut, no click-to-select row mapping), so a click anywhere on it
	// is a no-op, same as every other full-capture state in this list.
	// m.view == viewTagManagement (E10 Task 2, bean bt-r92i, T2-Review
	// Fix-Runde 1 F02): the Tag-Management page is the SAME keyboard-only
	// full-capture class as the Lobby -- currently redundant in effect
	// (both wheelMove's and the left-click dispatch's view switches below
	// have no matching case AND no default, so every path already no-ops),
	// but Defense-in-Depth: this list's own doc claim of completeness must
	// hold, and a future T3-T6 change to either switch must not silently
	// route stray clicks against the tag page's foreign geometry.
	if m.form != nil || m.overlay != overlayNone || m.paletteOpen || m.filterOpen ||
		m.searchActive || m.helpOpen || m.confirmQuit || m.view == viewLobby ||
		m.view == viewTagManagement {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.wheelMove(msg, -1), nil
	case tea.MouseButtonWheelDown:
		return m.wheelMove(msg, 1), nil
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return m, nil
		}
		// bt-wtqs (epic bt-vy1q): the persistent filter-chip strip
		// (box_filter_bar.go) sits ABOVE the Tree/Detail split, spanning
		// the full inner width -- checked BEFORE the m.view switch below
		// (the same "earliest wins" precedent as the Toast/fullscreen/
		// overlay guards above), since a strip hit is never a Tree-or-
		// Detail click. filterBarHit itself is a no-op unless
		// boxFormEnabled() && m.view == viewBrowseRepo, so this is dead
		// weight (never true) for every other view/flag-off combination --
		// the existing switch below is reached completely unchanged
		// whenever the strip is not even rendered.
		if facet, ok := filterBarHit(m, msg); ok {
			return m.openFilterMenuAt(facet)
		}
		switch m.view {
		case viewBrowseRepo:
			return m.mouseTreeClick(msg)
		case viewBacklog:
			return m.mouseBacklogClick(msg)
		}
	}
	return m, nil
}

// filterBarFieldAt maps a click at (row, col) relative to the filter strip's
// OWN top-left corner (row 0 = the strip's first screen row -- its own top
// border, box_filter_bar.go's dropdownBox/gridRow shape; col 0 = the strip's
// own left edge, the SAME zero point filterBar(width) itself renders from)
// into one of its four chips. gridColAt (below) walks the SAME
// gridColWidths(4, width) filterBar's own gridRow call renders from -- the
// strip's real column boundaries and this hit-test can never independently
// drift (Golden-Rule-Drift-Schutz, detailBoxFormClickRow's own gridColAt use,
// above). facets is filterBar's own fixed cell order (box_filter_bar.go's
// `cells` literal: Type/Status/Priority/Tags), mapped to buildFilterItems'
// facet-key vocabulary (box_filter_facets.go) -- "tag", singular, NOT "tags".
// ok=false for a row outside [0,filterBarHeight) or a col past the last chip
// (the 1-cell inter-chip gap, or past the strip's own right edge).
func filterBarFieldAt(row, col, width int) (facet string, ok bool) {
	if row < 0 || row >= filterBarHeight {
		return "", false
	}
	widths := gridColWidths(4, width)
	i := gridColAt(widths, col)
	if i < 0 {
		return "", false
	}
	facets := []string{"type", "status", "priority", "tag"}
	return facets[i], true
}

// filterBarHit resolves msg against the persistent filter-chip strip
// (box_filter_bar.go, bt-wtqs) while boxFormEnabled() -- the strip only ever
// renders in Browse (viewBrowseRepo, view_browse_repo.go's ONE call site;
// Backlog never shows it, the SAME B6 precedent detailBoxFormClickRow/
// treeClickRow's own doc comments already establish), so a non-Browse view
// short-circuits false here without reconstructing any geometry at all.
//
// Reconstructs the SAME clickPaneGeometry geometry treeClickRow/
// detailBoxFormClickRow do (Golden-Rule-Drift-Schutz) -- but, deliberately,
// BEFORE either of those applies their own `originY += filterBarHeight`
// correction: clickPaneGeometry's raw originY assumes screen content begins
// exactly 1 row after a bordered box's own top border sitting directly below
// the header divider (its own doc comment, mouse.go above) -- while
// boxFormEnabled(), that bordered box IS the filter strip itself
// (view_browse_repo.go's content build order: head+div, THEN filterBarRow,
// THEN the Tree/Detail split), so the strip's own top-border row is
// originY-1, and its 3 rows span [originY-1, originY-1+filterBarHeight).
// This is exactly the row treeClickRow/detailBoxFormClickRow skip PAST via
// their own `+= filterBarHeight` (which lands one row AFTER the strip, at
// originY-1+filterBarHeight == originY+filterBarHeight-1... no: originY
// itself, corrected, becomes originY+filterBarHeight, i.e. exactly the
// strip's bottom border's own next row) -- the two hit-tests are two
// windows on the SAME originY-1 anchor, verified against a real View()
// render (mouse_filter_bar_test.go's filterBarClickAt).
func filterBarHit(m model, msg tea.MouseMsg) (facet string, ok bool) {
	if !boxFormEnabled() || m.view != viewBrowseRepo {
		return "", false
	}
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.browseRepoChrome(innerW)
	_, _, _, originX, rawOriginY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth)
	stripTop := rawOriginY - 1
	return filterBarFieldAt(msg.Y-stripTop, msg.X-originX, innerW)
}

// wheelMove dispatches a single wheel tick (delta = ±1) -- design decision f:
// Wheel bewegt den View-eigene CURSOR (nicht einen Scroll-Offset), same ±1
// step as a single i/k keypress (devd's own Tree-wheel precedent,
// update.go:476-487 -- devd's NON-Tree ×3 m.scroll branch has no beans-tui
// equivalent, this file's own doc comment). bt-ze10 (epic bt-vy1q F1) adds
// the ONE exception: while boxFormEnabled() AND the wheel event's own
// coordinates land inside the Detail pane (boxFormWheelHit, below), the tick
// scrolls model.boxFormScroll instead of moving a cursor -- checked first so
// it takes priority over the view switch, mirrors the Toast-Klick-Vorrang/
// fullscreen/overlay guards above it in handleMouse (earliest-wins
// precedent). boxFormEnabled() off (or a wheel outside the Detail pane, e.g.
// over the Tree/Backlog) always falls through to the original switch below,
// UNCHANGED.
func (m model) wheelMove(msg tea.MouseMsg, delta int) model {
	if boxFormEnabled() && boxFormWheelHit(m, msg) {
		return m.adjustBoxFormScroll(m.focusedBean(), delta)
	}
	switch m.view {
	case viewBrowseRepo:
		return m.treeCursorMove(m.visibleNodes(), delta)
	case viewBacklog:
		return m.backlogCursorMove(m.backlogVisible(), delta)
	}
	return m
}

// mouseTreeClick dispatches a Tree left-click (design decision f): resolves
// the clicked row via treeClickRow (view_browse_repo.go, pure geometry, no
// side effect), sets the cursor, then applies devd-D03's Doppelklick
// semantics (Port devd mouseTreeClick verbatim, update.go:922-950) -- a
// SECOND click on the SAME node within doubleClickInterval collapses an
// OPEN node; a single click on a CLOSED expandable node expands it; a
// single click on an ALREADY-open node only moves the cursor (no toggle,
// devd D03).
func (m model) mouseTreeClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.flatView {
		// S5/S6: the Nested/Flat Browse toggle (`G`, view_browse_flat.go)
		// swaps the LEFT pane's content source -- a click there must resolve
		// against flatVisible()/flatList (flatClickRow), never the Tree's
		// own nodes/cursorID, mirroring the SAME m.flatView branch
		// viewBrowseRepo's own render already takes (view_browse_repo.go).
		return m.mouseFlatClick(msg)
	}
	nodes := m.visibleNodes()
	idx, ok := treeClickRow(m, nodes, msg)
	if !ok {
		// B07 (design-spec.md §15 PF-16, bean bt-duz7): treeClickRow
		// returns ok=false BOTH for a click outside the Tree pane's own
		// column (right Detail pane) AND for one inside it that misses
		// every rendered row (above the search line, or past the last
		// node) -- delegating unconditionally to mouseDetailClick is safe
		// for the latter case too: detailClickRow (mouse.go, below)
		// independently re-checks msg.X against the Detail pane's OWN
		// column span and rejects (ok=false, no-op) a click that is still
		// inside the Tree's column, so no cross-pane leakage occurs.
		// Implementer decision (bean bt-duz7 Architektur-Vorgabe #4,
		// "saubersten Diff-Groesse"): the smallest possible diff over a
		// second, independently-computed X-bounds pre-check here.
		return m.mouseDetailClick(msg)
	}
	n := nodes[idx]
	if n.placeholder {
		// D01 (bean bt-39cl): the archive-hint row is non-selectable -- a
		// click on it is a No-Op (mirrors skipPlaceholder's keyboard-side
		// decision: it is never a legitimate cursor target).
		return m, nil
	}
	m.cursorID = n.id

	// Doppelklick = zweiter Klick auf DENSELBEN Node-Index innerhalb des
	// Zeitfensters. lastClickAt=zero (Zero-Value) ⇒ riesiges Delta ⇒ erster
	// Klick nie Doppelklick. now() reads the INJECTABLE clock (update.go), so
	// none of this depends on wall time in tests -- bean bt-vpvu calls that
	// out explicitly as the flakiness trap to avoid.
	now := m.now()
	isDouble := idx == m.lastClickIdx && now.Sub(m.lastClickAt) < doubleClickInterval
	m.lastClickIdx = idx
	m.lastClickAt = now

	if n.hasKids && isDouble {
		// bean bt-vpvu (PO-Befund #14) CHANGES the ported devd-D03 semantics.
		// devd expanded a CLOSED node on a single click and only collapsed on
		// a double; the PO asked for the plain desktop-list model instead:
		//
		//   single click  -> select only (the bean appears in the Detail pane)
		//   double click  -> toggle this bean's expansion
		//
		// Recorded as a deliberate deviation from a written-down decision (the
		// same care STATE.md's own "Verhaltensaenderung gegen einen
		// niedergeschriebenen Beschluss" note asks for): single-click-expands
		// made every attempt to merely LOOK at an epic restructure the tree
		// under the pointer, which is what made the pane feel unselectable.
		//
		// The click pair is CONSUMED here (lastClickAt reset to zero): without
		// it a third rapid click would pair with the second and undo the
		// toggle immediately, so a triple click would read as "nothing
		// happened".
		m.lastClickAt = time.Time{}
		return m.setExpanded(n, !n.open), nil
	}
	return m, nil
}

// mouseFlatClick dispatches a Flat-list left-click (S6, S5's Nested/Flat
// Browse toggle `G`, view_browse_flat.go) -- the flat-list analog of
// mouseTreeClick: resolves the clicked row via flatClickRow (view_browse_flat.go,
// pure geometry, mirrors treeClickRow's own structure) then sets flatList's
// index cursor. No Doppelklick semantics, same rationale as
// mouseBacklogClick just below (flat list, keyFlat's own doc comment: "kein
// Doppelklick-Bedarf").
func (m model) mouseFlatClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	vis := m.flatVisible()
	idx, ok := flatClickRow(m, vis, msg)
	if !ok {
		// B07 precedent (mouseTreeClick/mouseBacklogClick, above): delegate
		// to the shared Detail-Pane click dispatcher for anything outside
		// the flat list's own column/row span.
		return m.mouseDetailClick(msg)
	}
	m.flatList.setLen(len(vis))
	m.flatList.cursor = idx
	return m, nil
}

// mouseBacklogClick dispatches a Backlog left-click: resolves the clicked
// row via backlogClickRow (view_browse_backlog.go) and sets the cursor --
// no Doppelklick semantics (flat list, plan epic-E5-plan.md »Task 4« Step
// 6: "kein Doppelklick-Bedarf bei Backlog").
func (m model) mouseBacklogClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	vis := m.backlogVisible()
	idx, ok := backlogClickRow(m, vis, msg)
	if !ok {
		// B07: mirrors mouseTreeClick's own delegation above (same
		// rationale -- backlogClickRow's ok=false covers BOTH "wrong pane"
		// and "in-pane but no row hit", detailClickRow independently
		// re-validates the X bounds either way).
		return m.mouseDetailClick(msg)
	}
	m.backlogList.setLen(len(vis))
	m.backlogList.cursor = idx
	return m, nil
}

// detailClickRow maps a Detail-Pane click to either a Section-Header hit
// (fieldIdx == -1) or a Meta field-row hit (secIdx == metaSectionIdx,
// fieldIdx >= 0) -- B07 (design-spec.md §15 PF-16, bean bt-duz7), ANALOG
// treeClickRow/backlogClickRow above: reconstructs the pane's geometry via
// the SAME clickPaneGeometry + browseRepoChrome/backlogChrome helpers those
// two use (Golden-Rule-Drift-Schutz) -- picks the caller's OWN chrome via
// m.view, since the Detail pane renders identically from BOTH viewBrowseRepo
// and viewBacklog (renderBeanAccordionPane, view_browse_repo.go), just
// alongside a different left pane/footer.
//
// The row->section/field walk below is GROUNDED against the real render
// pipeline, not a second, independently-maintained height formula: it reuses
// the EXACT SAME state (m.detailFocus/m.secCursor/m.accOpen/m.fieldCursor/
// m.detailLevel) and the EXACT SAME beanSections() call renderAccordionPane
// itself makes (view_browse_repo.go), then walks secs with the IDENTICAL
// isOpen/activeSec conditionals renderAccordion (accordion.go) uses to
// build its own line-by-line output (B04, design-spec.md §15 PF-17, bean
// bt-b0w0: the former fieldStrip-shown conditional this walk used to mirror
// is GONE on both sides -- removed here in lockstep with accordion.go's own
// removal, see the row++ block's own comment below). Section-body line
// counts are measured via lipgloss.Height on the SAME s.body string
// renderAccordion hands to boxStyle.Render -- PaddingLeft alone never
// changes a line count, so this is byte-equivalent without reconstructing
// boxStyle's own Width/Padding here. A change to either function's
// algorithm is caught by the render-grounded tests in mouse_test.go, which
// locate click coordinates by searching the REAL m.View() output (mirrors
// treeClickAt/leftPaneClickAt's own pattern) -- never by hand-deriving a row
// number (the "selbst-referenzielle Geometrie-Test" trap E7 already hit
// once, this task's own Architektur-Vorgabe #2 warns against repeating it).
//
// v1 simplification (bean bt-duz7 Architektur-Vorgabe #2, no PO-Wortlaut
// requires more): ONLY Meta (metaSectionIdx) has a fixed, direct
// Zeile->Feldindex mapping (Zeile 0 = title ... Zeile 4 = tags, per
// metaFields' fixed order). bt-lg68 shrunk this from 7 to 5 rows
// (created_at/updated_at removed). A click landing inside an OPEN
// Body/Relations/History section's body resolves to that section's OWN
// header hit (fieldIdx == -1), never a field index. Relations rows carry their own
// ▷/▶ cursor marker now too (B04, relationsSectionBody) but are NOT yet
// individually mouse-clickable -- that stays v1 scope, unchanged by this
// task (no PO-Wortlaut requires it; B04's own scope is keyboard navigation
// + render, mirrors "Pfeiltasten-Navigation ändert KEINEN Code").
func detailClickRow(m model, b *data.Bean, msg tea.MouseMsg) (secIdx, fieldIdx int, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	var head, localKeys string
	if m.view == viewBacklog {
		head, localKeys = m.backlogChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	bodyH, lw, rw, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth)

	if msg.X < originX+lw || msg.X >= originX+lw+rw {
		return 0, 0, false // left pane, or off-screen -- no Detail target
	}
	clickRow := msg.Y - originY
	if clickRow < 0 || clickRow >= bodyH {
		return 0, 0, false // above the pane, or past renderPane's own Golden-Rule-#1 line cap
	}
	if b == nil {
		return 0, 0, false // "(no selection)" placeholder -- nothing to hit
	}

	// detailHeaderBlock (view_detail_bean.go) ALWAYS renders 5 fixed rows
	// (ID/Title/blank/type-status-prio/blank) ahead of the Accordion --
	// bt-duz7's own PO-Wortlaut ("Kopfblock-Offset 5 Zeilen beachten!").
	const headerBlockLines = 5
	if clickRow < headerBlockLines {
		return 0, 0, false // Kopfblock -- never a Section-/Feld-Treffer
	}
	accordionRow := clickRow - headerBlockLines

	bodyW := rw - 4
	if bodyW < 1 {
		bodyW = 1
	}
	secs := beanSections(m.idx, b, bodyW, m.detailFocus, m.secCursor, m.fieldCursor, m.detailLevel)

	row := 0
	for i, s := range secs {
		n := i + 1
		isOpen := n == m.accOpen // PF-18: exclusive-open incl. Meta, mirrors renderAccordion (revises PF-1)
		if accordionRow == row {
			return i, -1, true // this section's own header row
		}
		row++
		if !isOpen {
			continue
		}
		// B04 (design-spec.md §15 PF-17, bean bt-b0w0): the former
		// fieldStrip-row skip (an extra row this walk used to add for an
		// active section carrying fields, mirroring renderAccordion's own
		// removed fieldStrip branch) is GONE -- RELATIONS was its only
		// remaining caller, and RELATIONS' rows now carry their own ▷/▶
		// markers INLINE in s.body (relationsSectionBody), not in a
		// separate strip. Keeping this skip after B04 would silently
		// shift every row-count past an active+open RELATIONS section by
		// one (TestDetailClickRowNoOffByOneWhenRelationsSectionActiveWithFields,
		// mouse_test.go, catches the regression).
		bodyLines := lipgloss.Height(s.body)
		if accordionRow >= row && accordionRow < row+bodyLines {
			if i == metaSectionIdx {
				if fi := accordionRow - row; fi >= 0 && fi < len(s.fields) {
					return i, fi, true
				}
			}
			return i, -1, true // Body/Relations/History body, or an out-of-range Meta row -- section-level hit
		}
		row += bodyLines
	}
	return 0, 0, false // past the last rendered section (renderPane's own line cap already excluded above, defensive)
}

// detailClickKeyBase offsets every Detail-Pane clickKey into a number space
// GUARANTEED disjoint from any realistic Tree/Backlog row index (B01,
// Fix-Runde 1, bean bt-duz7): all three click dispatchers share the ONE
// m.lastClickIdx int (Architektur-Vorgabe #3, "KEIN zweites Zeitfenster-
// Paar"), and without the offset a Tree click on row index N followed
// <500ms by a FIRST click on the Detail field whose unoffset key was also N
// (e.g. Tree row 1 -> title: field, 0*10+0+1 == 1) aliased into a false
// double click that opened the field's overlay/form without any prior
// selection (reviewer-reproduced).
const detailClickKeyBase = 1000

// detailClickKey folds a detailClickRow hit (secIdx, fieldIdx) into the one
// shared m.lastClickIdx int (I01, Fix-Runde 1: named+testable instead of an
// inline expression): section-header hits (fieldIdx == -1) land on
// base+0/10/20/30, Meta field hits on base+1..base+5 (bt-lg68: shrunk from
// 7 to 5 Meta fields) -- disjoint from each other AND (via
// detailClickKeyBase) from every Tree/Backlog row index.
func detailClickKey(secIdx, fieldIdx int) int {
	return detailClickKeyBase + secIdx*10 + fieldIdx + 1
}

// mouseDetailClick dispatches a Detail-Pane left-click (B07, design-spec.md
// §15 PF-16, bean bt-duz7): resolves the clicked row via detailClickRow
// (pure geometry, no side effect, above) then applies the SAME
// lastClickIdx/lastClickAt Doppelklick machinery mouseTreeClick already uses
// (Architektur-Vorgabe #3, "gleiche Felder auf model wiederverwenden, KEIN
// zweites Zeitfenster-Paar") -- keys are folded via detailClickKey (above)
// so they can alias neither each other nor a Tree/Backlog row index (B01).
//
// A single click on a NOT-yet-selected target ALWAYS selects (Fall a/b:
// activates+expands the section, additionally enters field level for a Meta
// field) and NEVER opens an overlay. Fall c has TWO triggers (PO wording
// verbatim, "Doppelklick (oder Zweitklick auf ein bereits selektiertes
// Feld)" -- B02, Fix-Runde 1): (1) a double click within
// doubleClickInterval (mirrors mouseTreeClick's isDouble), and (2) a second
// click on an ALREADY-selected field, which fires INDEPENDENT of the time
// window -- the Enter-Kaskade it mirrors (activateDetailField, update.go)
// is windowless, so "click the selected field again" must be too.
// "Selected" is the same state the ▶ marker renders from (metaSectionBody,
// view_detail_bean.go): m.detailFocus && detailLevel==1 && the cursor pair
// matches -- a stale fieldCursor without detailFocus does NOT count.
//
// BODY's section header (fieldIdx==-1) used to have its OWN double-click
// exception here, opening $EDITOR via the (since-removed) openBodyEditor
// helper -- mirroring keyDetailFocus's former enter-on-BODY branch (bt-y2iw
// "Notes for bt-duz7", E8 Task 6/B10). D01 (design-spec.md §15 PF-17, bean
// bt-z4b1) REVERTED that keyDetailFocus branch ersatzlos ("PO's neues
// Mentalmodell reserviert '$EDITOR öffnen' ausschließlich für e") -- this
// mirrored mouse exception is reverted the SAME way, for the same reason: a
// double click on ANY section header (BODY included) is now a plain no-op,
// exactly like every other section header.
func (m model) mouseDetailClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	b := m.focusedBean()
	if b == nil {
		return m, nil
	}
	if boxFormEnabled() {
		// S6 ("Box-form detail click-to-edit"): while the experiment flag is
		// on, renderAccordionPane (view_browse_repo.go) swaps the Detail
		// pane's body for detailBoxForm's jira-style boxes (box_detail_form.go)
		// -- detailClickRow's accordion Section/Field walk below is the WRONG
		// geometry in that mode (no beanSections/headerBlockLines exist),
		// so this branches to the box-form-specific hit test instead, BEFORE
		// ever reaching the accordion path. Default off leaves the branch
		// dead, existing accordion click tests/goldens byte-identical.
		return m.mouseBoxFormDetailClick(b, msg)
	}
	secIdx, fieldIdx, ok := detailClickRow(m, b, msg)
	if !ok {
		return m, nil
	}

	clickKey := detailClickKey(secIdx, fieldIdx)
	now := m.now()
	isDouble := clickKey == m.lastClickIdx && now.Sub(m.lastClickAt) < doubleClickInterval
	// B02 (Fix-Runde 1): capture BEFORE the state update below whether the
	// clicked field was ALREADY the selected one -- the PO's second Fall-c
	// trigger, windowless (see doc comment).
	wasSelected := fieldIdx >= 0 && m.detailFocus && m.detailLevel == 1 &&
		m.secCursor == secIdx && m.fieldCursor == fieldIdx
	m.lastClickIdx = clickKey
	m.lastClickAt = now

	m.detailFocus = true
	m.secCursor = secIdx
	m.accOpen = secIdx + 1

	if fieldIdx < 0 {
		m.detailLevel = 0
		m.fieldCursor = 0
		return m, nil
	}

	// detailClickRow only ever returns fieldIdx >= 0 nested under
	// i == metaSectionIdx (its own doc comment above) -- metaFields(b) is
	// therefore the SAME fixed 7-entry slice s.fields already was.
	m.detailLevel = 1
	m.fieldCursor = fieldIdx

	if isDouble || wasSelected {
		fields := metaFields(b)
		if fieldIdx < len(fields) {
			return m.activateDetailField(b, fields[fieldIdx])
		}
	}
	return m, nil
}

// boxFormTarget* — detailBoxFormClickRow's hit-test result kinds (S6, "Box-
// form detail click-to-edit"): named strings mirroring update.go's own
// keyNodeAction group vocabulary (openValueMenu's "status"/"type"/
// "priority" argument, box_menu_value.go) plus "parent"/"tags"/"editor" for
// the picker/whole-bean-editor targets -- ONE vocabulary, so
// detailBoxFormClickRow and mouseBoxFormDetailClick's dispatch switch can
// never drift on the set of valid targets.
const (
	boxFormTargetStatus   = "status"
	boxFormTargetType     = "type"
	boxFormTargetPriority = "priority"
	boxFormTargetParent   = "parent"
	boxFormTargetTags     = "tags"
	boxFormTargetEditor   = "editor"
)

// gridColAt buckets a column offset (relative to a gridRow's OWN left edge --
// detailBoxFormClickRow's clickCol := msg.X - (originX+lw), the same zero
// point the row's boxes themselves render from, box_detail_form.go's
// gridRow) into the gridColWidths (box_detail_form.go) column index it falls
// under -- the ONE place that walks widths+gap, so gridRow's real rendered
// column boundaries and this hit-test can never independently drift
// (Golden-Rule-Drift-Schutz, detailClickRow's own doc comment precedent,
// above). Returns -1 for a click past the last column (e.g. inside the
// 1-col inter-box gap, or past the row's own right edge).
func gridColAt(widths []int, col int) int {
	x := 0
	for i, w := range widths {
		if col >= x && col < x+w {
			return i
		}
		x += w + detailBoxFormGap
	}
	return -1
}

// boxFormEffectiveScroll returns m.boxFormScroll if it still targets b, or 0
// if the selection has moved to a DIFFERENT bean since boxFormScroll was
// last written (bean bt-ze10 Akzeptanz "Offset resets when the selected bean
// changes") -- types.go's own boxFormScrollBean doc comment has the full
// "derived reset, not N explicit reset call sites" rationale. Pure/read-
// only: renderBeanAccordionPane (view_browse_repo.go) calls this to get the
// value it hands to renderAccordionPane's own boxScroll param; it never
// mutates model.boxFormScroll itself (adjustBoxFormScroll, below, is the
// only writer).
func boxFormEffectiveScroll(m model, b *data.Bean) int {
	if b == nil || b.ID != m.boxFormScrollBean {
		return 0
	}
	return m.boxFormScroll
}

// boxFormScrollBounds returns detailBoxForm's total rendered line count and
// the Detail pane's own visible line budget for b -- what adjustBoxFormScroll
// (below) clamps a new offset against. Reconstructs the SAME
// clickPaneGeometry geometry detailBoxFormClickRow (below) and
// boxFormWheelHit (below) do (Golden-Rule-Drift-Schutz), with ONE correction
// neither of those needs for their own purpose (a click/wheel Y bounds
// check, not a content-height BUDGET): the persistent filter-chip row's
// height is reclaimed from bodyH here, exactly as viewBrowseRepo's own
// View() does before the split ever renders (view_browse_repo.go's "S3"
// comment, boxFormEnabled()) -- that SAME post-reclaim bodyH is what
// ultimately reaches renderAccordionPane as its `h` param at render time, so
// this must match it or the stored offset and the actual render disagree.
//
// bt-s90e (epic bt-vy1q) added the fullscreenDetail WIDTH branch below. F1
// (bean bt-ze10) had guarded the Vollbild off entirely rather than clamp
// against the wrong geometry; the fix turned out to be width-ONLY. Height
// needs no branch: bodyH is already read off the CURRENT model's own chrome
// (browseRepoChrome/backlogChrome are footer-context-aware, so the Vollbild
// yields its own correct value here for free) via the SAME clickPaneGeometry
// call viewBrowseRepo makes BEFORE its fullscreen branch, filter-bar reclaim
// included. The Vollbild renders ONE full-width pane instead of the split's
// right half, so accW comes from paneW (innerW-2, view_browse_repo.go's own
// border-accounting comment) minus renderAccordionPane's own w-2, i.e.
// innerW-4 -- NOT rw-2.
func boxFormScrollBounds(m model, b *data.Bean) (total, height int) {
	if b == nil {
		return 0, 0
	}
	accW, height := boxFormPaneMetrics(m, b)
	form := detailBoxForm(m.idx, b, accW, -1) // cursor never changes line counts
	return len(strings.Split(form, "\n")), height
}

// boxFormPaneMetrics is boxFormScrollBounds' geometry half, split out for
// bean bt-1o4g's field cursor (boxFormNav, box_nav_field.go), which needs the
// SAME accW/height pair but renders the form as BLOCKS (boxFormBlocks) rather
// than one joined string. Sharing this keeps the cursor's scroll-into-view
// arithmetic and the wheel/keyboard clamp arithmetic on one geometry -- a
// cursor computed against a different height than the one the offset is
// clamped to would let a focused field land just outside the viewport.
func boxFormPaneMetrics(m model, b *data.Bean) (accW, height int) {
	if b == nil {
		return 1, 0
	}
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	var head, localKeys string
	if m.view == viewBacklog {
		head, localKeys = m.backlogChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	bodyH, _, rw, _, _ := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth)
	if m.view != viewBacklog {
		bodyH -= filterBarHeight // B6 precedent: only viewBrowseRepo shows the filter bar
		if bodyH < 1 {
			bodyH = 1
		}
	}

	accW = rw - 2
	if m.fullscreen == fullscreenDetail {
		// bt-s90e: Vollbild-Detail is ONE full-width pane, not the lw/rw split --
		// paneW is innerW-2, minus renderAccordionPane's own w-2.
		accW = innerW - 4
	}
	if accW < 1 {
		accW = 1
	}
	return accW, bodyH
}

// adjustBoxFormScroll applies delta to b's box-form scroll offset (relative
// to boxFormEffectiveScroll's own stale-selection baseline, so a bean change
// since the last adjustment starts back at 0 rather than compounding a
// stale offset), clamps it via boxFormScrollBounds/clampBoxFormScroll
// (box_detail_form.go), and records b's ID as the offset's new owner. Shared
// by keyDetailFocus's up/down (update.go, keyboard) and wheelMove (above,
// mouse wheel) -- ONE mutation point, so the two input paths can never
// independently drift on the reset-on-bean-change or clamp rules. No-op
// (returns m unchanged) when b is nil (defensive -- callers already guard
// this in practice: keyDetailFocus's own b==nil early-return, wheelMove's
// boxFormWheelHit implying a resolved Detail pane).
func (m model) adjustBoxFormScroll(b *data.Bean, delta int) model {
	if b == nil {
		return m
	}
	base := boxFormEffectiveScroll(m, b)
	total, height := boxFormScrollBounds(m, b)
	m.boxFormScroll = clampBoxFormScroll(base+delta, total, height)
	m.boxFormScrollBean = b.ID
	return m
}

// boxFormWheelHit reports whether msg lands inside the Detail pane's own
// column/row bounds -- wheelMove (above) uses this to decide whether a wheel
// tick scrolls the box-form Detail pane (bean bt-ze10) instead of moving the
// Tree/Backlog cursor. Mirrors detailBoxFormClickRow's own X/Y bounds check
// (below) minus the row->target mapping a click needs -- a wheel tick has no
// "which box did I land on" question, only "am I over the pane at all".
func boxFormWheelHit(m model, msg tea.MouseMsg) bool {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	var head, localKeys string
	if m.view == viewBacklog {
		head, localKeys = m.backlogChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	bodyH, lw, rw, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth)
	if m.view != viewBacklog {
		originY += filterBarHeight // B6 precedent, detailBoxFormClickRow below
	}

	if msg.X < originX+lw || msg.X >= originX+lw+rw {
		return false
	}
	row := msg.Y - originY
	return row >= 0 && row < bodyH
}

// detailBoxFormClickRow maps a Detail-Pane click to a box-form field target
// (S6) while boxFormEnabled() -- the box-form analog of detailClickRow
// (accordion mode, above): reconstructs the SAME clickPaneGeometry geometry
// (Golden-Rule-Drift-Schutz), corrected by the B6 filter-bar offset (this
// file's treeClickRow doc comment, view_browse_repo.go) -- ONLY viewBrowseRepo
// ever shows the filter bar (box_filter_bar.go's one call site), so the
// offset is guarded on `m.view != viewBacklog` here, the SAME branch
// detailClickRow's own head/localKeys selection already makes.
//
// detailBoxForm's OWN fixed row order (box_detail_form.go doc comment:
// Title(3 lines) -> rowA Status|Type|Priority(3 lines) -> rowB Parent|Tags
// (3 lines) -> Body/Relations/History panels, unbounded) is walked directly
// here rather than re-deriving it from beanSections/accordion state (box-
// form has none yet -- design-spec.md D12, no per-field cursor). Rows past
// rowB (clickRow >= 9, Body/Relations/History) collapse into ONE "editor"
// target: keys.Editor already opens the SAME whole-bean $EDITOR
// unconditionally regardless of section context (D01, update.go's own
// keyNodeAction doc comment, "egal an welcher Stelle") -- so this is not a
// "wrong action" the way mis-mapping Status/Type/Priority/Parent/Tags would
// be, it is the exact same action a real `e` press already fires from
// anywhere in the pane today.
//
// NO fullscreenDetail branch here, deliberately (bt-hd42, checked rather than
// assumed): boxFormScrollBounds/boxFormPaneMetrics need their own accW branch
// because the KEYBOARD reaches the Vollbild (bt-s90e), but handleMouse (above)
// returns a flat no-op for `m.fullscreen != fullscreenNone` BEFORE the wheel
// and left-click dispatch -- F01's documented v1 scope cut, "Maus im Vollbild".
// Every caller of this function is downstream of that guard, so a Vollbild
// click can never arrive here. A width branch would be code for a dead path;
// if F01 is ever lifted, the accW below must gain boxFormPaneMetrics' own
// innerW-4 branch in the same change.
func detailBoxFormClickRow(m model, msg tea.MouseMsg) (target string, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	var head, localKeys string
	if m.view == viewBacklog {
		head, localKeys = m.backlogChrome(innerW)
	} else {
		head, localKeys = m.browseRepoChrome(innerW)
	}

	bodyH, lw, rw, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth)
	if m.view != viewBacklog {
		originY += filterBarHeight // B6: viewBacklog never shows the filter bar
	}

	if msg.X < originX+lw || msg.X >= originX+lw+rw {
		return "", false // left pane, or off-screen -- no Detail target
	}
	clickRow := msg.Y - originY
	if clickRow < 0 || clickRow >= bodyH {
		return "", false // above the pane, or past renderPane's own line cap
	}

	accW := rw - 2
	if accW < 1 {
		accW = 1
	}
	clickCol := msg.X - (originX + lw)

	// bt-hd42: clickRow is a SCREEN row; boxFormFieldAt's table is indexed by
	// CONTENT row. Those two coincide only while the Detail pane sits at scroll
	// offset 0 -- which is precisely why every pre-existing box-form click test
	// passed and the bug still shipped. Adding the offset back is the whole
	// fix: the pane renders content rows [off, off+bodyH) at screen rows
	// [0, bodyH), so the inverse is contentRow = clickRow + off.
	//
	// The offset is read through boxFormEffectiveScroll -- NOT m.boxFormScroll
	// directly -- for the same reason the render does (view_browse_repo.go's
	// renderBeanAccordionPane): that helper applies bt-ze10's DERIVED per-bean
	// reset (a stale offset belonging to a previously selected bean reads as 0
	// rather than being nulled at N call sites). Reading the raw field here
	// would resolve clicks against an offset the visible render is not using,
	// re-opening this very bug in the bean-change direction.
	//
	// Both scroll INPUT paths (wheelMove/adjustBoxFormScroll above, keyboard
	// boxFormNav in box_nav_field.go) write that one field pair, so neither can
	// drift away from this correction.
	field := boxFormFieldAt(clickRow+boxFormEffectiveScroll(m, m.focusedBean()), clickCol, accW)
	if field < 0 {
		return "", false // inside a row but not over a box (inter-box gap)
	}
	return boxFormFieldOrder[field].target, true
}

// mouseBoxFormDetailClick dispatches a Detail-Pane left-click while
// boxFormEnabled() (S6, "Box-form detail click-to-edit"): resolves the
// clicked box via detailBoxFormClickRow (above, pure geometry) then fires
// the EXACT SAME action keyNodeAction's own s/o/u/a/t/e branch fires for
// each key (update.go:760-823) -- box-form fields are direct dropdownBox/
// panelBox hotkeys (box_detail_form.go), not an accordion Section/Field
// two-level selection, so a single click ALWAYS opens the target's editor/
// menu/picker immediately -- no Doppelklick semantics here, mirrors
// mouseBacklogClick's own "flat list, kein Doppelklick-Bedarf" precedent
// (this file). Matches the S6 task spec verbatim ("Trigger the SAME action
// those keys fire").
func (m model) mouseBoxFormDetailClick(b *data.Bean, msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	target, ok := detailBoxFormClickRow(m, msg)
	if !ok {
		return m, nil
	}
	return m.activateBoxFormTarget(b, target)
}

// activateBoxFormTarget dispatches a boxFormTarget* onto its overlay/picker/
// $EDITOR -- the SHARED helper between mouseBoxFormDetailClick (Maus, above)
// and boxFormActivateCursor (enter on the field cursor, box_nav_field.go,
// bean bt-1o4g). Extracted verbatim from the click dispatch's former inline
// switch, mirroring activateDetailField's own "exactly ONE place that
// dispatches a field onto its overlay" precedent (update.go) -- keyboard and
// mouse open the same thing for the same box, by construction.
func (m model) activateBoxFormTarget(b *data.Bean, target string) (tea.Model, tea.Cmd) {
	switch target {
	case boxFormTargetStatus:
		return m.openValueMenu("status"), nil
	case boxFormTargetType:
		return m.openValueMenu("type"), nil
	case boxFormTargetPriority:
		return m.openValueMenu("priority"), nil
	case boxFormTargetParent:
		return m.openParentPicker()
	case boxFormTargetTags:
		return m.openTagPicker()
	case boxFormTargetEditor:
		return m.openBeanEditor(b)
	}
	return m, nil
}
