package tui

// view_browse_backlog.go — V3 Backlog (design-spec.md §6, E2 Task 5, bean
// bt-gzu6): parentless+ready beans (idx.Backlog(), E1 Task 3), Master-Detail
// reusing Task 1's Accordion (via focusedBean(), Task 2) and Task 3/4's
// shared search+facet predicate (m.beanMatches) -- NOT a second, parallel
// filter implementation (devd has two parallel Tree-/Backlog-filters,
// beans-tui consolidates into one, box_filter_facets.go). Sort-Toggle `S`
// cycles status/priority/created/updated (design-spec §6 V3) instead of a
// floating sort menu (no V8 overlay for this in the spec, a deliberate
// simplification vs. devd's backlogSortBox).
//
// backlogList staleness note: `/` and `f` route through the SHARED
// keySearchInput/keyFilterMenu handlers (update.go/box_filter_facets.go),
// which only know about the Tree's cursorID -- they never touch backlogList.
// So backlogList.length/.cursor can go stale relative to backlogVisible()'s
// current (shrunk) length while the user is typing/toggling facets.
// keyBacklog resyncs it (setLen) at the top of every key it handles, before
// any up/down move, so staleness never survives past the next backlog
// keypress -- see TestKeyBacklogResyncsStaleCursorBeforeMove
// (view_browse_backlog_test.go) for the regression guard.
//
// CORRECTION (E2-T6 fast-follow, T5-review): the previous version of this
// comment claimed staleness was "purely a keep future move() bounds correct
// concern, not a render bug" -- understated. EVERY render between the shrink
// and the next backlog keypress (i.e. live-typing a search character or
// toggling a facet while m.view == viewBacklog) calls backlogRows/
// renderBacklogDetailPane with the STALE m.backlogList.cursor against the
// ALREADY-shrunk backlogVisible(). backlogRows only highlights row i==pos,
// so a cursor left pointing past the new (shorter) vis highlights NOTHING
// (the `▌` selection bar vanishes); renderBeanDetailBox's own
// `pos < len(vis)` guard (view_browse_repo.go) then falls through to its
// "(no selection)" placeholder even though a real selection exists the
// instant the next keypress resyncs it. Both are cosmetic flicker, never a
// panic or an out-of-bounds index (windowStart/windowAround still clamp the
// window itself) -- but they ARE visible render artifacts a user can
// observe while typing, not merely a latent move()-bound hazard.

import (
	"sort"
	"strings"
	"time"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// backlogSortModes is the canonical 4-stop Sort-Toggle `S` cycle (bean
// bt-gzu6 Akzeptanz: "S zyklisch status->prio->created->updated->status").
//
// ERRATUM vs. the plan's own sketch (epic-E2-plan.md »Task 5«):
// `modes := []string{"", "priority", "created", "updated", "status"}`
// treated "" (the untouched default, meaning "leave idx.Backlog()'s own
// canonical order in place") as a FIFTH distinct cycle stop alongside
// "status" -- but "" and "status" render IDENTICALLY (idx.Backlog() is
// already status-tier ordered, sortBacklog's own "" case is a no-op for
// exactly that reason). A naive "find current, advance, wrap" over that
// 5-element slice would wrap from "status" (index 4) back to "" (index 0)
// instead of to "priority" (index 1) -- a keypress that changes
// m.backlogSort's STRING value but not the rendered order at all, i.e. a
// dead 5th keystroke once per lap. That contradicts the bean's own "4
// modes" wording. Fixed here: "" is an ALIAS for "status" only when
// LOOKING UP the current position (nextBacklogSort below); once cycling
// starts, "" is never revisited -- a pure period-4 cycle, always a
// visible change on every press.
var backlogSortModes = []string{"status", "priority", "created", "updated"}

// nextBacklogSort advances current to the next Sort-Toggle stop, wrapping.
func nextBacklogSort(current string) string {
	if current == "" {
		current = "status" // alias: idx.Backlog()'s own order IS status-tier order
	}
	for i, mode := range backlogSortModes {
		if mode == current {
			return backlogSortModes[(i+1)%len(backlogSortModes)]
		}
	}
	return backlogSortModes[0]
}

// backlogSortDisplayLabel maps the active Sort-Toggle mode to its Backlog-
// search-line suffix label (D02, design-spec.md §15 PF-16, bean bt-ntoz/
// bt-d8kc): "" aliases to "status" (same alias nextBacklogSort itself uses
// -- idx.Backlog()'s own canonical order IS status-tier order), "priority"
// abbreviates to "prio" (PO example verbatim: "⌕ / search · sort prio"),
// every other mode (status/created/updated) renders as its own word
// unchanged.
func backlogSortDisplayLabel(mode string) string {
	if mode == "" {
		mode = "status"
	}
	if mode == "priority" {
		return "prio"
	}
	return mode
}

// backlogVisible returns idx.Backlog() (E1 Task 3, unchanged) narrowed by
// the SAME shared search+facet predicate the Tree uses (m.beanMatches, Task
// 3/4), then ordered per the active Sort-Toggle mode (sortBacklog).
func (m model) backlogVisible() []*data.Bean {
	if m.idx == nil {
		return nil
	}
	var out []*data.Bean
	for _, b := range m.idx.Backlog() {
		if m.beanMatches(b) {
			out = append(out, b)
		}
	}
	sortBacklog(out, m.backlogSort)
	return out
}

// sortBacklog re-orders beans in place per mode -- "" leaves idx.Backlog()'s
// own canonical (Status -> Priority -> Type -> Title) order untouched.
// data.StatusRank/PriorityRank (E2 Task 1) back the status/priority modes;
// created/updated compare CreatedAt/UpdatedAt via afterOrLast (nil-safe).
func sortBacklog(beans []*data.Bean, mode string) {
	switch mode {
	case "priority":
		sort.SliceStable(beans, func(i, j int) bool {
			return data.PriorityRank(beans[i].Priority) < data.PriorityRank(beans[j].Priority)
		})
	case "created":
		sort.SliceStable(beans, func(i, j int) bool { return afterOrLast(beans[i].CreatedAt, beans[j].CreatedAt) })
	case "updated":
		sort.SliceStable(beans, func(i, j int) bool { return afterOrLast(beans[i].UpdatedAt, beans[j].UpdatedAt) })
	case "status":
		sort.SliceStable(beans, func(i, j int) bool {
			return data.StatusRank(beans[i].Status) < data.StatusRank(beans[j].Status)
		})
	} // "" -- leave idx.Backlog()'s own canonical order in place
}

// afterOrLast orders newest-first; nil timestamps sort last, never panics.
func afterOrLast(a, b *time.Time) bool {
	if a == nil {
		return false
	}
	if b == nil {
		return true
	}
	return a.After(*b)
}

// backlogSelected returns the bean under backlogList's cursor, or nil for an
// empty/out-of-range list (pre-load, or every backlog row filtered out).
func (m model) backlogSelected() *data.Bean {
	vis := m.backlogVisible()
	if m.backlogList.cursor < 0 || m.backlogList.cursor >= len(vis) {
		return nil
	}
	return vis[m.backlogList.cursor]
}

// backlogRowText renders one Backlog row's plain content: status glyph +
// type icon + ID (Sapphire) + title -- mirrors treeRowText's glyph order
// (view_browse_repo.go) minus the indent/expand-marker (Backlog is flat, no
// hierarchy; devd's variable-height block windowing is deliberately NOT
// ported here, see plan Port-Referenzen -- a beans title is short enough for
// the existing single-line windowAround, `truncate` handles the rare overlong
// case exactly like every other row in the app).
// slug is the caller's m.beanIDPrefix() (view_browse_repo.go) -- the ID is
// shortened by shortBeanID exactly like treeRowText does it (bean bt-pl5p):
// Tree and Backlog/Flat are TWO separate render paths, and only changing one
// would let them drift apart visually.
func backlogRowText(b *data.Bean, slug string) string {
	head, title := backlogRowParts(b, slug)
	return head + title
}

// backlogRowParts splits a Backlog/flat row into its fixed HEAD and its
// variable TITLE -- treeRowParts' counterpart (view_browse_repo.go), for the
// same hanging-indent wrapping reason (bean bt-f68z, PO-Befund #13). The
// doc-comment claim above that "a beans title is short enough for the existing
// single-line windowAround" is what the PO's live test disproved.
func backlogRowParts(b *data.Bean, slug string) (head, title string) {
	return theme.StatusIcon(b.Status) + " " + theme.TypeIcon(b.Type) + " " + theme.Key.Render(shortBeanID(b.ID, slug)) + " ", b.Title
}

// backlogRowLines renders one Backlog/flat row as one or more terminal lines,
// hanging-indented to the title column (treeRowLines' counterpart).
func backlogRowLines(b *data.Bean, slug string, width int) []string {
	head, title := backlogRowParts(b, slug)
	return hangingWrap(head, lipgloss.Width(head), title, width)
}

// backlogRows renders every visible Backlog row, applying the same D08
// cursor treatment as treeRows (view_browse_repo.go): a leading `▌` bar plus
// the whole row accent-tinted when focused, muted when the Detail pane has
// focus instead. Windowed around the cursor via the EXISTING windowAround
// (E1 Task 8) -- no new fenestration mechanism (plan Port-Referenzen).
func (m model) backlogRows(vis []*data.Bean, focused bool, bodyH, width int) []string {
	rows, _ := blockWindow(m.backlogBlocks(vis, focused, width), bodyH, m.backlogList.cursor)
	return rows
}

// backlogBlocks is the Backlog's treeBlocks (view_browse_repo.go): one
// variable-height line block per bean, shared by backlogRows above and
// backlogClickRow below (bean bt-f68z -- rows are no longer 1:1 with beans).
func (m model) backlogBlocks(vis []*data.Bean, focused bool, width int) [][]string {
	pos := m.backlogList.cursor
	slug := m.beanIDPrefix()
	blocks := make([][]string, len(vis))
	for i, b := range vis {
		blocks[i] = decorateRowBlock(backlogRowLines(b, slug, width-1), i == pos, focused)
	}
	return blocks
}

// renderBacklogDetailPane renders the Detail box-form for the Backlog's
// selected bean -- keys off backlogList's index cursor into vis instead of a
// treeNode slice, then hands off to the shared renderBeanDetailBox
// (view_browse_repo.go), so Tree and Backlog can never drift out of sync on it.
func (m model) renderBacklogDetailPane(vis []*data.Bean, w, h int, focused bool) string {
	pos := m.backlogList.cursor
	var b *data.Bean
	if pos >= 0 && pos < len(vis) {
		b = vis[pos]
	}
	return m.renderBeanDetailBox(b, w, h, focused)
}

// backlogChrome builds viewBacklog's own head/localKeys strings -- factored
// out (E5 Task 4, bean bt-mne6) so backlogClickRow (mouse.go) can
// reconstruct this view's IDENTICAL geometry via clickPaneGeometry without a
// second, independently maintained copy of this breadcrumb/footer
// construction (Golden-Rule-Drift-Schutz, mirrors browseRepoChrome's own
// rationale, view_browse_repo.go).
func (m model) backlogChrome(innerW int) (head, localKeys string) {
	head = breadcrumb(m.repoLabel(), "Backlog", renderBindings(globalBindings()), innerW)
	localKeys = footer(m.contextualLocalHint(backlogLocalBindings()), innerW)
	return
}

// backlogLocalBindings is the Backlog view's own Footer Zone 3 local set --
// D06+Q06 (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc) REBUILD this from
// scratch, superseding PF-11's list, mirroring browseRepoLocalBindings'
// exact Q06 order (view_browse_repo.go) verbatim.
//
// D02 (design-spec.md §15 PF-17, bean bt-tct9/bt-1e0t, PO-bestätigt Option b
// + Präzisierung, 2026-07-16) SUPERSEDES the former ERRATUM below: the
// Backlog footer previously appended `keys.Sort` as a documented Backlog-
// exclusive addition (bean bt-d8kc) so the feature stayed discoverable
// without a PO instruction to remove it. The PO has now given that
// instruction explicitly: "'S Sort' fliegt aus dem Backlog-Footer; die
// S-Taste bleibt funktional, wird aber NUR im Help-Overlay ('?')
// dokumentiert." `S` stays fully wired (keyBacklog's Sort case, unchanged,
// below) and stays documented -- just Help-overlay-only now (helpGroups(),
// keymap.go, already lists it under "Actions", untouched by this change).
// The runtime state indicator (`· sort <modus>` search-line suffix,
// treeSearchLine, view_browse_repo.go, E8/D02) is unaffected -- it was
// always the primary visible surface for the active sort mode, the footer
// entry was a secondary discoverability aid the PO judged unnecessary.
// Trigger: at <82 columns the old 14-entry list (13 + Sort) wrapped to
// THREE footer lines where the Tree's 13-entry list stayed at two (D06
// permits at most two) -- removing Sort restores parity with
// browseRepoLocalBindings, both lists are now IDENTICAL.
//
// Former ERRATUM text (historical, superseded by D02 above, kept for
// audit trail): "Q06's PO-verbatim list is phrased once for BOTH Browse and
// Backlog and never mentions Sort (`S`) -- Sort has no Tree analog
// (sortBacklog/nextBacklogSort are Backlog-only), so the shared list simply
// never had occasion to name it. Sort stays a Backlog-EXCLUSIVE addition
// here rather than being silently dropped: it is a pre-existing,
// frequently-used, currently-visible feature, and no PO instruction asked
// for its removal ('kein Entzug ohne PO-Anweisung')."
func backlogLocalBindings() []keybind.Binding {
	return browseRepoLocalBindings()
}

// viewBacklog renders the two-pane master-detail Backlog view -- mirrors
// viewBrowseRepo's algebra exactly (view_browse_repo.go) so the frame always
// fills width x height (Golden Rule #1: no Height() on a bordered style),
// just with backlogVisible() rows on the left instead of the Tree's
// treeNode flattening.
func (m model) viewBacklog() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	head, localKeys := m.backlogChrome(innerW)

	div := theme.Dim.Render(strings.Repeat("─", innerW))
	// bt-81f0 (Notifications vereinheitlichen, Q1-Annahme): m.err lost the
	// status line's rendering slot -- Toast is the ONE visible channel now.
	status := m.statusLine(innerW)

	// E5 Task 4 (bean bt-mne6): bodyH/lw/rw now come from the SAME
	// clickPaneGeometry helper backlogClickRow (below) uses -- single source
	// for the numeric pane geometry (Golden-Rule-Drift-Schutz).
	bodyH, lw, rw, _, _ := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	vis := m.backlogVisible()

	var body string
	if m.fullscreen != fullscreenNone {
		// F01 (design-spec.md §15, E9 Task 7, bean bt-13l7): Vollbild -- mirrors
		// viewBrowseRepo's own new branch (view_browse_repo.go), symmetric for
		// Backlog (m.view stays viewBacklog throughout, PO-Wortlaut: Vollbild
		// is orthogonal to which view is active). paneW: see viewBrowseRepo's
		// own doc comment for the full border-accounting rationale (ONE pane's
		// content width is innerW-2, not innerW -- caught live via tmux smoke).
		paneW := innerW - 2
		var listRows []string
		if m.fullscreen == fullscreenList {
			searchLine := m.treeSearchLine(paneW-2, "sort "+backlogSortDisplayLabel(m.backlogSort))
			listRows = append([]string{searchLine}, m.backlogRows(vis, true, bodyH-1, paneW-2)...)
		}
		var detailBean *data.Bean
		if m.fullscreen == fullscreenDetail {
			detailBean = m.focusedBean()
		}
		body = renderFullscreenBody(m.fullscreen, paneW, bodyH, listRows, true, m.idx, detailBean, boxFormEffectiveScroll(m, detailBean))
	} else {
		// Same 1-header-row budget trade as the Tree's search head (Task 3):
		// the search/filter summary line costs 1 line of the list pane's bodyH
		// content budget, so the actual rows window to bodyH-1 (PF-10, bean
		// bt-uyzf, widened from bodyH-3 now that renderPane no longer reserves
		// its own title+separator lines).
		// D02 (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): the Backlog-Sort-
		// Indicator -- a dezent (theme.Muted) suffix on the search line showing
		// the active Sort-Toggle mode.
		searchLine := m.treeSearchLine(lw-2, "sort "+backlogSortDisplayLabel(m.backlogSort))
		rows := append([]string{searchLine}, m.backlogRows(vis, !m.detailFocus, bodyH-1, lw-2)...)
		listBox := renderPane(pane{rows: rows}, lw, bodyH, !m.detailFocus)
		body = listBox
		if rw > 0 {
			detailBox := m.renderBacklogDetailPane(vis, rw, bodyH, m.detailFocus)
			body = lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
		}
	}

	content := appendStatusLine(head+"\n"+div+"\n"+body+"\n"+div+"\n"+localKeys, status)
	out := outerBorder(content, innerW, true)

	return m.composeOverlays(out, w, h)
}

// keyBacklog drives the Backlog view: up/down move backlogList, enter is a
// handled no-op (T6-Review B01: TAB is the ONLY detail-focus entry,
// PO-Nachtrag 3 / D01 revidiert -- the flat list has no expand concept, so
// enter is the analog of keyTree's leaf no-op), S cycles the sort mode,
// `/`/`f` open the SAME shared search input/filter menu the Tree uses,
// `b`/esc return to Tree. Detail focus (once entered via tab) delegates to
// keyDetailFocus verbatim (Task 2) -- focusedBean()'s viewBacklog case
// (update.go) is what makes that reuse possible without a second
// Accordion-navigation implementation.
func (m model) keyBacklog(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detailFocus { // defensive: handleKey already routes detailFocus to keyDetailFocus first
		return m.keyDetailFocus(msg)
	}

	// Resync backlogList's length against the CURRENT filtered/sorted view
	// before touching it -- search/filter toggles routed through the shared
	// keySearchInput/keyFilterMenu handlers never update backlogList (see
	// file doc comment), so this is the one place that closes the gap.
	vis := m.backlogVisible()
	m.backlogList.setLen(len(vis))

	switch {
	case keybind.Matches(msg, keys.Sort):
		m.backlogSort = nextBacklogSort(m.backlogSort)
		return m, nil
	case keybind.Matches(msg, keys.Search):
		return m.openSearchInput()
	case keybind.Matches(msg, keys.Filter):
		return m.openFilterMenu()
	case keybind.Matches(msg, keys.FilterClear):
		// B01 (E2-Abschluss-Übernahme, Epic-Body bt-gzcu): X-Direct-Clear wirkte
		// nur in keyTree (update.go) -- gleicher clearFacets()-Helper, danach
		// Längen-Resync gegen die JETZT breitere Sicht.
		m = m.clearFacets()
		m.backlogList.setLen(len(m.backlogVisible()))
		return m, nil
	case keybind.Matches(msg, keys.Backlog), keybind.Matches(msg, keys.Back):
		m.view = viewBrowseRepo
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		// T6-Review B01 (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert): enter
		// is a handled no-op -- TAB is the ONLY detail-focus entry (handleKey,
		// view-agnostic). Previously this case still carried the pre-revision
		// entry behavior (detailFocus=true + cursor reset); the Backlog is a
		// flat list with no expand concept, so its enter is the analog of
		// keyTree's leaf no-op. Kept as an explicit handled case (not removed)
		// so enter can't fall through to the nav-key switch below.
		return m, nil
	}

	switch navKey(msg.String()) {
	case "up":
		// E5 Task 4 (bean bt-mne6): factored into backlogCursorMove (below)
		// so the keyboard AND the wheel dispatch (mouse.go handleMouse)
		// share the exact same clamp logic instead of two independent
		// copies.
		m = m.backlogCursorMove(vis, -1)
	case "down":
		m = m.backlogCursorMove(vis, 1)
	}
	return m, nil
}

// backlogCursorMove resyncs backlogList's length against vis (this file's
// own staleness guard, doc comment above) then shifts the cursor by delta
// (listState.move, list.go, already clamped) -- factored out of keyBacklog's
// own up/down case (E5 Task 4, bean bt-mne6) so the keyboard AND the wheel
// dispatch (mouse.go handleMouse) share one clamp. The setLen call is
// idempotent when the caller already resynced (keyBacklog does, at its own
// top) -- it is NOT redundant for the wheel dispatch, which has no prior
// resync point of its own.
func (m model) backlogCursorMove(vis []*data.Bean, delta int) model {
	m.backlogList.setLen(len(vis))
	m.backlogList.move(delta)
	return m
}

// backlogClickRow maps a mouse click to a row index into vis (E5 Task 4,
// bean bt-mne6, design decision f; caller: mouse.go's mouseBacklogClick) --
// mirrors treeClickRow's own reconstruction (view_browse_repo.go) against
// viewBacklog's OWN render formula (above, via backlogChrome +
// clickPaneGeometry): same bodyH/lw algebra, just this view's own head/
// localKeys. Row 0 is the search/facet head line (m.treeSearchLine, shared
// with the Tree) -- never a row target; row 1+ maps via
// windowStart(len(vis), bodyH-1, m.backlogList.cursor) + (clickRow-1) -- PF-10
// (bean bt-uyzf) widened this from bodyH-3 now that renderPane no longer
// reserves title+separator lines -- flat list, no Doppelklick concept (plan
// epic-E5-plan.md »Task 4« Step 6).
func backlogClickRow(m model, vis []*data.Bean, msg tea.MouseMsg) (idx int, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.backlogChrome(innerW)

	bodyH, lw, _, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)

	if msg.X < originX || msg.X >= originX+lw {
		return 0, false
	}
	clickRow := msg.Y - originY - 1 // row 0 is the search/facet head line, never a target
	if clickRow < 0 {
		return 0, false
	}

	// bean bt-f68z: Backlog titles wrap too, so the clicked LINE is resolved
	// through the same blockWindow the renderer used instead of by dividing
	// screen rows by one.
	_, rowElem := blockWindow(m.backlogBlocks(vis, !m.detailFocus, lw-2), bodyH-1, m.backlogList.cursor)
	return blockWindowElemAt(rowElem, clickRow)
}
