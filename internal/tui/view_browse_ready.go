package tui

// view_browse_ready.go — V3 Ready (design-spec.md §6, E2 Task 5, bean
// bt-gzu6): the Ready set (Store.Ready, same rule as `lif list --ready`), Master-Detail
// reusing Task 1's Accordion (via focusedBean(), Task 2) and Task 3/4's
// shared search+facet predicate (m.beanMatches) -- NOT a second, parallel
// filter implementation (devd has two parallel Tree-/Ready-filters,
// beans-tui consolidates into one, box_filter_facets.go). Sort-Toggle `S`
// cycles status/priority/created/updated (design-spec §6 V3) instead of a
// floating sort menu (no V8 overlay for this in the spec, a deliberate
// simplification vs. devd's readySortBox).
//
// readyList staleness note: `/` and `f` route through the SHARED
// keySearchInput/keyFilterMenu handlers (update.go/box_filter_facets.go),
// which only know about the Tree's cursorID -- they never touch readyList.
// So readyList.length/.cursor can go stale relative to readyVisible()'s
// current (shrunk) length while the user is typing/toggling facets.
// keyReady resyncs it (setLen) at the top of every key it handles, before
// any up/down move, so staleness never survives past the next ready
// keypress -- see TestKeyReadyResyncsStaleCursorBeforeMove
// (view_browse_ready_test.go) for the regression guard.
//
// CORRECTION (E2-T6 fast-follow, T5-review): the previous version of this
// comment claimed staleness was "purely a keep future move() bounds correct
// concern, not a render bug" -- understated. EVERY render between the shrink
// and the next ready keypress (i.e. live-typing a search character or
// toggling a facet while m.view == viewReady) calls readyRows/
// renderReadyDetailPane with the STALE m.readyList.cursor against the
// ALREADY-shrunk readyVisible(). readyRows only highlights row i==pos,
// so a cursor left pointing past the new (shorter) vis highlights NOTHING
// (the `▌` selection bar vanishes); renderBeanDetailBox's own
// `pos < len(vis)` guard (view_browse_repo.go) then falls through to its
// "(no selection)" placeholder even though a real selection exists the
// instant the next keypress resyncs it. Both are cosmetic flicker, never a
// panic or an out-of-bounds index (windowStart/windowAround still clamp the
// window itself) -- but they ARE visible render artifacts a user can
// observe while typing, not merely a latent move()-bound hazard.

import (
	"github.com/rierek/lif/pkg/item"
	"sort"
	"strings"
	"time"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/internal/ui"
)

// readySortModes is the canonical 4-stop Sort-Toggle `S` cycle (bean
// bt-gzu6 Akzeptanz: "S zyklisch status->prio->created->updated->status").
//
// ERRATUM vs. the plan's own sketch (epic-E2-plan.md »Task 5«):
// `modes := []string{"", "priority", "created", "updated", "status"}`
// treated "" (the untouched default, meaning "leave idx.Ready()'s own
// canonical order in place") as a FIFTH distinct cycle stop alongside
// "status" -- but "" and "status" render IDENTICALLY (the canonical order is
// already status-tier ordered, sortReady's own "" case is a no-op for
// exactly that reason). A naive "find current, advance, wrap" over that
// 5-element slice would wrap from "status" (index 4) back to "" (index 0)
// instead of to "priority" (index 1) -- a keypress that changes
// m.readySort's STRING value but not the rendered order at all, i.e. a
// dead 5th keystroke once per lap. That contradicts the bean's own "4
// modes" wording. Fixed here: "" is an ALIAS for "status" only when
// LOOKING UP the current position (nextReadySort below); once cycling
// starts, "" is never revisited -- a pure period-4 cycle, always a
// visible change on every press.
var readySortModes = []string{"status", "priority", "created", "updated"}

// nextReadySort advances current to the next Sort-Toggle stop, wrapping.
func nextReadySort(current string) string {
	if current == "" {
		current = "status" // alias: the canonical order IS status-tier order
	}
	for i, mode := range readySortModes {
		if mode == current {
			return readySortModes[(i+1)%len(readySortModes)]
		}
	}
	return readySortModes[0]
}

// readySortDisplayLabel maps the active Sort-Toggle mode to its Ready-
// search-line suffix label (D02, design-spec.md §15 PF-16, bean bt-ntoz/
// bt-d8kc): "" aliases to "status" (same alias nextReadySort itself uses
// -- the canonical order IS status-tier order), "priority"
// abbreviates to "prio" (PO example verbatim: "⌕ / search · sort prio"),
// every other mode (status/created/updated) renders as its own word
// unchanged.
func readySortDisplayLabel(mode string) string {
	if mode == "" {
		mode = "status"
	}
	if mode == "priority" {
		return "prio"
	}
	return mode
}

// readyVisible returns the items of m.readyIDs (the `lif list --ready` set,
// Store.Ready) in canonical order (data.SortBeans, ID as the stable
// tie-break), narrowed by the SAME shared search+facet predicate the Tree
// uses (m.beanMatches, Task 3/4), then ordered per the active Sort-Toggle
// mode (sortReady).
func (m model) readyVisible() []*item.Item {
	if m.idx == nil {
		return nil
	}
	var out []*item.Item
	for id := range m.readyIDs {
		if b, ok := m.idx.ByID[id]; ok && m.beanMatches(b) {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	data.SortBeans(out)
	sortReady(out, m.readySort)
	return out
}

// sortReady re-orders beans in place per mode -- "" leaves the canonical
// (Status -> Priority -> Type -> Title) order readyVisible built untouched.
// data.StatusRank/PriorityRank (E2 Task 1) back the status/priority modes;
// created/updated compare CreatedAt/UpdatedAt via afterOrLast (nil-safe).
func sortReady(beans []*item.Item, mode string) {
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
	} // "" -- leave the canonical order in place
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

// readySelected returns the bean under readyList's cursor, or nil for an
// empty/out-of-range list (pre-load, or every ready row filtered out).
func (m model) readySelected() *item.Item {
	vis := m.readyVisible()
	if m.readyList.cursor < 0 || m.readyList.cursor >= len(vis) {
		return nil
	}
	return vis[m.readyList.cursor]
}

// readyRowText renders one Ready row's plain content: status glyph +
// type icon + ID (Sapphire) + title -- mirrors treeRowText's glyph order
// (view_browse_repo.go) minus the indent/expand-marker (Ready is flat, no
// hierarchy; devd's variable-height block windowing is deliberately NOT
// ported here, see plan Port-Referenzen -- a beans title is short enough for
// the existing single-line windowAround, `truncate` handles the rare overlong
// case exactly like every other row in the app).
// slug is the caller's m.beanIDPrefix() (view_browse_repo.go) -- the ID is
// shortened by shortBeanID exactly like treeRowText does it (bean bt-pl5p):
// Tree and Ready/Flat are TWO separate render paths, and only changing one
// would let them drift apart visually.
func readyRowText(b *item.Item, slug string) string {
	head, title := readyRowParts(b, slug)
	return head + title
}

// readyRowParts splits a Ready/flat row into its fixed HEAD and its
// variable TITLE -- treeRowParts' counterpart (view_browse_repo.go), for the
// same hanging-indent wrapping reason (bean bt-f68z, PO-Befund #13). The
// doc-comment claim above that "a beans title is short enough for the existing
// single-line windowAround" is what the PO's live test disproved.
func readyRowParts(b *item.Item, slug string) (head, title string) {
	return theme.StatusIcon(b.Status) + " " + theme.TypeIcon(b.Type) + " " + theme.Key.Render(shortBeanID(b.ID, slug)) + " ", b.Title
}

// readyRowLines renders one Ready/flat row as one or more terminal lines,
// hanging-indented to the title column (treeRowLines' counterpart).
func readyRowLines(b *item.Item, slug string, width int) []string {
	head, title := readyRowParts(b, slug)
	return hangingWrap(head, lipgloss.Width(head), title, width)
}

// readyRows renders every visible Ready row, applying the same D08
// cursor treatment as treeRows (view_browse_repo.go): a leading `▌` bar plus
// the whole row accent-tinted when focused, muted when the Detail pane has
// focus instead. Windowed around the cursor via the EXISTING windowAround
// (E1 Task 8) -- no new fenestration mechanism (plan Port-Referenzen).
func (m model) readyRows(vis []*item.Item, focused bool, bodyH, width int) []string {
	rows, _ := blockWindow(m.readyBlocks(vis, focused, width), bodyH, m.readyList.cursor)
	return rows
}

// readyBlocks is the Ready's treeBlocks (view_browse_repo.go): one
// variable-height line block per bean, shared by readyRows above and
// readyClickRow below (bean bt-f68z -- rows are no longer 1:1 with beans).
func (m model) readyBlocks(vis []*item.Item, focused bool, width int) [][]string {
	pos := m.readyList.cursor
	blocks := m.flatListBlocks(vis, width-1)
	for i := range blocks {
		blocks[i] = decorateRowBlock(blocks[i], i == pos, focused)
	}
	return blocks
}

// flatListBlocks renders the beans of a flat list (Ready, Browse-Flat) as one
// line block each, width being the budget WITHOUT the cursor column. Beside
// the detail pane the rows stay compact (glyphs, readyRowLines); without it
// they go through the roadmap's row renderer (ui.TreeBlocks) minus the tree
// lines, so status word, type word and priority show as in lif roadmap.
func (m model) flatListBlocks(vis []*item.Item, width int) [][]string {
	if m.layout.hasDetail() {
		slug := m.beanIDPrefix()
		blocks := make([][]string, len(vis))
		for i, b := range vis {
			blocks[i] = readyRowLines(b, slug, width)
		}
		return blocks
	}
	flat := make([]ui.FlatItem, len(vis))
	for i, b := range vis {
		flat[i] = ui.FlatItem{Item: b, IsLast: true}
	}
	return ui.TreeBlocks(ui.RowsFromFlatItems(flat), width, false, data.ActiveConfig())
}

// renderReadyDetailPane renders the Detail box-form for the Ready's
// selected bean -- keys off readyList's index cursor into vis instead of a
// treeNode slice, then hands off to the shared renderBeanDetailBox
// (view_browse_repo.go), so Tree and Ready can never drift out of sync on it.
func (m model) renderReadyDetailPane(vis []*item.Item, w, h int, focused bool) string {
	pos := m.readyList.cursor
	var b *item.Item
	if pos >= 0 && pos < len(vis) {
		b = vis[pos]
	}
	return m.renderBeanDetailBox(b, w, h, focused)
}

// readyChrome builds viewReady's own head/localKeys strings -- factored
// out (E5 Task 4, bean bt-mne6) so readyClickRow (mouse.go) can
// reconstruct this view's IDENTICAL geometry via clickPaneGeometry without a
// second, independently maintained copy of this breadcrumb/footer
// construction (Golden-Rule-Drift-Schutz, mirrors browseRepoChrome's own
// rationale, view_browse_repo.go).
func (m model) readyChrome(innerW int) (head, localKeys string) {
	head = breadcrumb(m.repoLabel(), "Ready", renderBindings(globalBindings()), innerW)
	localKeys = footer(m.contextualLocalHint(m.footerBindings()), innerW)
	return
}

// viewReady renders the two-pane master-detail Ready view -- mirrors
// viewBrowseRepo's algebra exactly (view_browse_repo.go) so the frame always
// fills width x height (Golden Rule #1: no Height() on a bordered style),
// just with readyVisible() rows on the left instead of the Tree's
// treeNode flattening.
func (m model) viewReady() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	head, localKeys := m.readyChrome(innerW)

	div := theme.Dim.Render(strings.Repeat("─", innerW))
	// bt-81f0 (Notifications vereinheitlichen, Q1-Annahme): m.err lost the
	// status line's rendering slot -- Toast is the ONE visible channel now.
	status := m.statusLine(innerW)

	// E5 Task 4 (bean bt-mne6): bodyH/lw/rw now come from the SAME
	// clickPaneGeometry helper readyClickRow (below) uses -- single source
	// for the numeric pane geometry (Golden-Rule-Drift-Schutz).
	bodyH, lw, rw, _, _ := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.layout)
	vis := m.readyVisible()

	// Same filter-bar reclaim as viewBrowseRepo: the Ready list shares the
	// Browse facet filter (readyVisible -> beanMatches), so the bar shows in
	// the filter layouts.
	var filterBarRow string
	if m.layout.hasFilter() {
		filterBarRow = m.filterBar(innerW)
		bodyH -= lipgloss.Height(filterBarRow)
		if bodyH < 1 {
			bodyH = 1
		}
	}

	var body string
	if m.layout == layoutDetail {
		body = m.renderReadyDetailPane(vis, rw, bodyH, true) // Detail alone at full width
	} else {
		// Same 1-header-row budget trade as the Tree's search head (Task 3):
		// the search/filter summary line costs 1 line of the list pane's bodyH
		// content budget, so the actual rows window to bodyH-1 (PF-10, bean
		// bt-uyzf, widened from bodyH-3 now that renderPane no longer reserves
		// its own title+separator lines).
		// D02 (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): the Ready-Sort-
		// Indicator -- a dezent (theme.Muted) suffix on the search line showing
		// the active Sort-Toggle mode.
		searchLine := m.treeSearchLine(lw-2, "sort "+readySortDisplayLabel(m.readySort))
		rows := append([]string{searchLine}, m.readyRows(vis, m.listFocus(), bodyH-1, lw-2)...)
		listBox := renderPane(pane{rows: rows}, lw, bodyH, m.listFocus())
		body = listBox
		if rw > 0 {
			detailBox := m.renderReadyDetailPane(vis, rw, bodyH, m.detailFocus)
			body = lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
		}
	}

	content := head + "\n" + div
	if filterBarRow != "" {
		content += "\n" + filterBarRow
	}
	content = appendStatusLine(content+"\n"+body+"\n"+div+"\n"+localKeys, status)
	out := outerBorder(content, innerW, true)

	return m.composeOverlays(out, w, h)
}

// keyReady drives the Ready view: up/down move readyList, enter is a
// handled no-op (T6-Review B01: TAB is the ONLY detail-focus entry,
// PO-Nachtrag 3 / D01 revidiert -- the flat list has no expand concept, so
// enter is the analog of keyTree's leaf no-op), S cycles the sort mode,
// `/` opens the SAME shared search input the Tree uses (`f` is global),
// esc clears an active search/filter, then returns to the Roadmap. Detail focus (once entered via tab) delegates to
// keyDetailFocus verbatim (Task 2) -- focusedBean()'s viewReady case
// (update.go) is what makes that reuse possible without a second
// Accordion-navigation implementation.
func (m model) keyReady(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detailFocus { // defensive: handleKey already routes detailFocus to keyDetailFocus first
		return m.keyDetailFocus(msg)
	}

	// Resync readyList's length against the CURRENT filtered/sorted view
	// before touching it -- search/filter toggles routed through the shared
	// keySearchInput/keyFilterMenu handlers never update readyList (see
	// file doc comment), so this is the one place that closes the gap.
	vis := m.readyVisible()
	m.readyList.setLen(len(vis))

	switch {
	case keybind.Matches(msg, keys.Sort):
		m.readySort = nextReadySort(m.readySort)
		return m, nil
	case keybind.Matches(msg, keys.Search):
		return m.openSearchInput()
	case keybind.Matches(msg, keys.FilterClear):
		// B01 (E2-Abschluss-Übernahme, Epic-Body bt-gzcu): X-Direct-Clear wirkte
		// nur in keyTree (update.go) -- gleicher clearFacets()-Helper, danach
		// Längen-Resync gegen die JETZT breitere Sicht.
		m = m.clearFacets()
		m.readyList.setLen(len(m.readyVisible()))
		return m, nil
	case keybind.Matches(msg, keys.Back):
		return m.escBack(), nil
	case keybind.Matches(msg, keys.Enter):
		// T6-Review B01 (bean bt-t1uy, PO-Nachtrag 3 / D01 revidiert): enter
		// is a handled no-op -- TAB is the ONLY detail-focus entry (handleKey,
		// view-agnostic). Previously this case still carried the pre-revision
		// entry behavior (detailFocus=true + cursor reset); the Ready is a
		// flat list with no expand concept, so its enter is the analog of
		// keyTree's leaf no-op. Kept as an explicit handled case (not removed)
		// so enter can't fall through to the nav-key switch below.
		return m, nil
	}

	switch navKey(msg.String()) {
	case "up":
		// E5 Task 4 (bean bt-mne6): factored into readyCursorMove (below)
		// so the keyboard AND the wheel dispatch (mouse.go handleMouse)
		// share the exact same clamp logic instead of two independent
		// copies.
		m = m.readyCursorMove(vis, -1)
	case "down":
		m = m.readyCursorMove(vis, 1)
	}
	return m, nil
}

// readyCursorMove resyncs readyList's length against vis (this file's
// own staleness guard, doc comment above) then shifts the cursor by delta
// (listState.move, list.go, already clamped) -- factored out of keyReady's
// own up/down case (E5 Task 4, bean bt-mne6) so the keyboard AND the wheel
// dispatch (mouse.go handleMouse) share one clamp. The setLen call is
// idempotent when the caller already resynced (keyReady does, at its own
// top) -- it is NOT redundant for the wheel dispatch, which has no prior
// resync point of its own.
func (m model) readyCursorMove(vis []*item.Item, delta int) model {
	m.readyList.setLen(len(vis))
	m.readyList.move(delta)
	return m
}

// readyClickRow maps a mouse click to a row index into vis (E5 Task 4,
// bean bt-mne6, design decision f; caller: mouse.go's mouseReadyClick) --
// mirrors treeClickRow's own reconstruction (view_browse_repo.go) against
// viewReady's OWN render formula (above, via readyChrome +
// clickPaneGeometry): same bodyH/lw algebra, just this view's own head/
// localKeys. Row 0 is the search/facet head line (m.treeSearchLine, shared
// with the Tree) -- never a row target; row 1+ maps via
// windowStart(len(vis), bodyH-1, m.readyList.cursor) + (clickRow-1) -- PF-10
// (bean bt-uyzf) widened this from bodyH-3 now that renderPane no longer
// reserves title+separator lines -- flat list, no Doppelklick concept (plan
// epic-E5-plan.md »Task 4« Step 6).
func readyClickRow(m model, vis []*item.Item, msg tea.MouseMsg) (idx int, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.readyChrome(innerW)

	bodyH, lw, _, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.layout)
	originY += m.filterRows() // same B6 correction as treeClickRow
	bodyH -= m.filterRows()
	if bodyH < 1 {
		bodyH = 1
	}

	if msg.X < originX || msg.X >= originX+lw {
		return 0, false
	}
	clickRow := msg.Y - originY - 1 // row 0 is the search/facet head line, never a target
	if clickRow < 0 {
		return 0, false
	}

	// bean bt-f68z: Ready titles wrap too, so the clicked LINE is resolved
	// through the same blockWindow the renderer used instead of by dividing
	// screen rows by one.
	_, rowElem := blockWindow(m.readyBlocks(vis, m.listFocus(), lw-2), bodyH-1, m.readyList.cursor)
	return blockWindowElemAt(rowElem, clickRow)
}
