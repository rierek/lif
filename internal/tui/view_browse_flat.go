package tui

// view_browse_flat.go — jira-style-ui experiment, Slice 5 (S5): the Nested/
// Flat Browse toggle (`G`, keymap.go). Additive to viewBrowseRepo
// (view_browse_repo.go): when m.flatView is true, the Browse view's LEFT
// pane renders a FLAT, sorted list of beans (columns: status glyph / type
// glyph / Key / Title) instead of the Tree, reusing view_browse_ready.go's
// row-rendering primitive (readyRowText) and cursor/window mechanics
// (windowAround) verbatim rather than inventing a second copy. UNLIKE the
// full-screen Ready view (viewReady), this stays Browse: the Detail
// pane (right) and the whole master-detail split are UNCHANGED, only the
// left pane's content source switches.
//
// Selected-bean resolution deliberately does NOT try to reconcile a flat
// index cursor with the Tree's bean-ID cursorID into one shared cursor --
// flatList (types.go) is its own minimal listState, index-based exactly like
// readyList, kept SEPARATE from the Tree's own cursorID/expanded state so
// toggling `G` back to nested resumes the Tree exactly where it was.
// Default m.flatView is false, so viewBrowseRepo's existing (unbranched)
// Tree-render path is untouched -- tree.golden and every other pre-existing
// Browse golden stay byte-identical.

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// flatVisible returns every bean matching the SAME shared search+facet+
// archive predicate the Tree uses (m.beanMatches, box_filter_facets.go --
// identical to readyVisible()'s own predicate half, view_browse_ready.go),
// but sourced from the WHOLE index (m.idx.ByID) rather than the Ready set's
// parentless+ready subset -- a flat mode toggle for Browse must show every
// bean the Tree would, not just the Ready's narrower set. Ordered via
// data.SortBeans (Status -> Priority -> Type -> Title), the SAME canonical
// order the roadmap tree and notOnRoadmap already use for the identical
// "iterate idx.ByID, filter, SortBeans" pattern.
func (m model) flatVisible() []*item.Item {
	if m.idx == nil {
		return nil
	}
	var out []*item.Item
	for _, b := range m.idx.ByID {
		if m.beanMatches(b) {
			out = append(out, b)
		}
	}
	data.SortBeans(out)
	return out
}

// flatSelected returns the bean under flatList's cursor, or nil for an
// empty/out-of-range list -- mirrors readySelected() exactly
// (view_browse_ready.go), including its staleness tolerance: a cursor
// left pointing past a since-narrowed vis (search/filter routes through the
// SHARED keySearchInput/keyFilterMenu handlers, which know nothing about
// flatList, same gap readyList's own doc comment documents) simply
// resolves to nil/no-highlight until the next flat keypress resyncs it via
// keyFlat's own setLen call, below.
func (m model) flatSelected() *item.Item {
	vis := m.flatVisible()
	if m.flatList.cursor < 0 || m.flatList.cursor >= len(vis) {
		return nil
	}
	return vis[m.flatList.cursor]
}

// flatRows renders every visible flat row, mirroring readyRows' own D08
// cursor treatment (view_browse_ready.go) verbatim: a leading `▌` bar plus
// the whole row accent-tinted when focused, muted when the Detail pane has
// focus instead, windowed around the cursor via the EXISTING windowAround
// (E1 Task 8) -- no new fenestration mechanism. Row TEXT itself reuses
// readyRowText unchanged (status glyph + type icon + Key + Title) -- the
// row-rendering half of "REUSE Ready's row-rendering + selection logic",
// this slice's own scope instruction.
func (m model) flatRows(vis []*item.Item, focused bool, bodyH, width int) []string {
	rows, _ := blockWindow(m.flatBlocks(vis, focused, width), bodyH, m.flatList.cursor)
	return rows
}

// flatBlocks is the flat list's treeBlocks (view_browse_repo.go): one
// variable-height line block per bean, shared by flatRows above and
// flatClickRow below so the click resolves against the SAME window the
// renderer produced (bean bt-f68z -- rows stopped being 1:1 with beans once
// titles wrap).
func (m model) flatBlocks(vis []*item.Item, focused bool, width int) [][]string {
	pos := m.flatList.cursor
	slug := m.beanIDPrefix()
	blocks := make([][]string, len(vis))
	for i, b := range vis {
		blocks[i] = decorateRowBlock(readyRowLines(b, slug, width-1), i == pos, focused)
	}
	return blocks
}

// renderFlatDetailPane renders the Detail box-form for the flat list's
// selected bean -- mirrors renderReadyDetailPane (view_browse_ready.go)
// exactly, keying off flatList's index cursor into vis instead of a treeNode
// slice, then handing off to the shared renderBeanDetailBox
// (view_browse_repo.go) -- Browse's flat mode and the Ready can never
// drift out of sync on the Detail body, same rationale as the Tree/Ready
// split that shared function already serves.
func (m model) renderFlatDetailPane(vis []*item.Item, w, h int, focused bool) string {
	pos := m.flatList.cursor
	var b *item.Item
	if pos >= 0 && pos < len(vis) {
		b = vis[pos]
	}
	return m.renderBeanDetailBox(b, w, h, focused)
}

// keyFlat drives Browse's flat mode: up/down move flatList's cursor over
// flatVisible() (the selection half of "REUSE Ready's row-rendering +
// selection logic" -- mirrors readyCursorMove's own resync-before-move
// pattern, view_browse_ready.go). left/right/enter are handled no-ops --
// a flat list has no expand concept, same rationale as keyReady's own
// enter no-op (view_browse_ready.go doc comment). Called from keyTree
// (update.go) once m.flatView is true, AFTER the shared search/filter/
// Ready/FilterClear/Back handling that switch already does -- so `/`, `f`,
// `X`, `b`, esc all keep working identically in flat mode (same shared
// state, filtered set narrows/widens exactly like the Tree's own).
// flatClickRow maps a mouse click to a flat-list row index (S6, mouse.go's
// mouseFlatClick) -- the flat-list analog of treeClickRow (view_browse_repo.go):
// IDENTICAL geometry reconstruction (browseRepoChrome + clickPaneGeometry,
// Golden-Rule-Drift-Schutz), windowed via windowStart(len(vis), bodyH-1,
// m.flatList.cursor) the SAME way flatRows itself windows (above) -- row 0 is
// the search head line (treeSearchLine), never a row target, same PF-10
// convention treeClickRow's own doc comment documents.
//
// B6 (S6): the persistent filter bar (box_filter_bar.go) sits between the
// header divider and this split -- flat mode is
// only ever reached from viewBrowseRepo (the filter bar's one call site), so
// the correction below mirrors treeClickRow's own exactly.
func flatClickRow(m model, vis []*item.Item, msg tea.MouseMsg) (idx int, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.browseRepoChrome(innerW)

	bodyH, lw, _, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.layout)
	// bt-vpvu: the bodyH half of the filter-bar correction was missing
	// here exactly as it was in treeClickRow -- same root cause, same fix
	// (that function's own comment carries the full analysis).
	originY += m.filterRows()
	bodyH -= m.filterRows()
	if bodyH < 1 {
		bodyH = 1
	}

	if msg.X < originX || msg.X >= originX+lw {
		return 0, false // right Detail pane, or off-screen -- no flat-row target
	}
	headRows := m.treePaneHeadRows(lw - 2)
	clickRow := msg.Y - originY - len(headRows)
	if clickRow < 0 {
		return 0, false // above the pane, or on the search head / column header
	}
	_, rowElem := blockWindow(m.flatBlocks(vis, !m.detailFocus, lw-2), bodyH-len(headRows), m.flatList.cursor)
	return blockWindowElemAt(rowElem, clickRow)
}

func (m model) keyFlat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	vis := m.flatVisible()
	m.flatList.setLen(len(vis))
	switch navKey(msg.String()) {
	case "up":
		m.flatList.move(-1)
	case "down":
		m.flatList.move(1)
	}
	return m, nil
}
