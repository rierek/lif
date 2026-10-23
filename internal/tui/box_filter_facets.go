package tui

// box_filter_facets.go — the ONE shared facet filter for Tree, flat AND Ready
// (E2 Task 5, bean bt-gzu6, reuses everything here unchanged). design-spec.md
// §6 US-05: "Filter wirkt auf Tree UND Ready". The facet predicate, the facet
// state helpers, and the value menu a focused filter field opens (the bar
// itself is box_filter_bar.go, the focus jump layout.go). Port pattern: devd
// view_browse_ready.go:89-136 (buildReadyFilterItems shape) --
// consolidated into ONE implementation instead of devd's two parallel ones
// (Tree-filter vs. Ready-filter), since beans-tui's filter state already
// lives model-wide, not per-view. No "Art" facet (devd's Milestone/Sprint/
// Issue split) -- beans' Type enum (milestone/epic/feature/task/bug) already
// covers that distinction, so Status/Type/Priority/Tag is the complete set.

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/pkg/item"
	"sort"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// ffItem is one facet-menu row (port devd view_browse_project.go:48-54,
// struct shape 1:1). facet is one of "status"/"type"/"priority"/"tag".
type ffItem struct {
	facet string
	value string
	label string
}

// facetHead groups the filter menu's rows under a muted sub-header per facet
// (port devd view_browse_project.go:1003's facetHead map).
var facetHead = map[string]string{
	"status":   "Status",
	"type":     "Type",
	"priority": "Priority",
	"tag":      "Tags",
	"archive":  "Archive",
}

// buildFilterItems assembles the menu row list: fixed Status/Type/Priority
// facets (the beans enums, design-spec.md §4 -- 5 values each) plus the
// dynamic Tag facet collected from the currently loaded beans.
//
// Enum single source (design decision b, E3 Task 1, bean bt-dlgk): the
// former 15 hardcoded rows are now three loops over data.StatusValues()/
// TypeValues()/PriorityValues() -- the SAME canonical tier order
// data/index.go's statusOrder/typeOrder/priorityOrder derive from, so the
// menu still reads top-to-bottom in "most urgent first" order, and this menu
// can never drift out of sync with box_menu_value.go's combined value menu
// (E3), which consumes the exact same accessors. Type labels are lowercase
// now (previously "Milestone"/"Epic"/... capitalized) -- a deliberate
// consistency fix with status/priority's already-lowercase labels; no golden
// test covers the filter box (box_filter_facets_test.go only asserts facet
// counts/values, not label casing).
func (m model) buildFilterItems() []ffItem {
	var items []ffItem
	for _, v := range data.StatusValues() {
		items = append(items, ffItem{"status", v, v})
	}
	for _, v := range data.TypeValues() {
		items = append(items, ffItem{"type", v, v})
	}
	for _, v := range data.PriorityValues() {
		items = append(items, ffItem{"priority", v, v})
	}
	for _, tag := range m.tagFilterOptions() {
		items = append(items, ffItem{"tag", tag, tag})
	}
	// E5 Task 7 (bean bt-ggt2, design decision e): ONE standalone row, NOT
	// part of the Status-enum loop above (data.StatusValues() has no
	// "archive" pseudo-status) -- toggles m.showArchived (a program DEFAULT,
	// not a PO-set facet, filterActive()'s own doc-stamp) via the SAME
	// space/x toggle path every other row uses (facetOn/toggleFacet below),
	// no new key. value "show" is a placeholder, never map-looked-up:
	// facetOn/toggleFacet special-case facet=="archive" against
	// m.showArchived directly.
	items = append(items, ffItem{"archive", "show", "Show archived"})
	return items
}

// tagFilterOptions collects distinct tag names across every loaded bean,
// deduped, in a DETERMINISTIC order.
//
// ERRATUM vs. the plan's own buildFilterItems sketch ("tag options DYNAMIC
// from idx.ByID, stable insertion order"): idx.ByID is a Go map, which has
// NO defined iteration order at all -- ranging over it directly (as devd's
// tagFilterOptions does over its own api.Issue SLICE, a genuinely
// insertion-ordered source) would make the tag facet's row order flicker
// nondeterministically between calls, breaking any golden test that ever
// renders the filter box.
//
// Fix, two passes: (1) sort the collected beans by ID first -- a
// deterministic BASELINE order that does not depend on the map walk: (2)
// THEN run data.SortBeans (the same single-source canonical order I03
// already established for the orphan bucket above) on top of that baseline.
// data.SortBeans uses sort.SliceStable, so beans that tie completely under
// its own criteria (same Status/Priority/Type/Title -- a real, observed case
// in this package's own tests, see TestTagFilterOptionsDeterministicAcross
// Calls) preserve the ID-sorted baseline instead of silently inheriting
// whatever order that call's map walk happened to produce. A single
// data.SortBeans pass alone is NOT sufficient for full determinism on ties.
func (m model) tagFilterOptions() []string {
	if m.idx == nil {
		return nil
	}
	all := make([]*item.Item, 0, len(m.idx.ByID))
	for _, b := range m.idx.ByID {
		all = append(all, b)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	data.SortBeans(all)

	seen := map[string]bool{}
	var out []string
	for _, b := range all {
		for _, t := range b.Tags {
			if t != "" && !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// filterActive reports whether any facet is currently narrowing the view.
func (m model) filterActive() bool {
	return len(m.filterStatus)+len(m.filterType)+len(m.filterPriority)+len(m.filterTag) > 0
}

// beanMatchesFacets is the facet half of the combined filter predicate
// (beanMatches AND-combines this with beanMatchesSearch, view_browse_repo.go
// / search_test.go, E2 Task 3). An empty facet map imposes no constraint
// (matches everything); a non-empty one requires membership.
func (m model) beanMatchesFacets(b *item.Item) bool {
	if len(m.filterStatus) > 0 && !m.filterStatus[b.Status] {
		return false
	}
	if len(m.filterType) > 0 && !m.filterType[b.Type] {
		return false
	}
	if len(m.filterPriority) > 0 && !m.filterPriority[b.Priority] {
		return false
	}
	if len(m.filterTag) > 0 {
		hit := false
		for _, t := range b.Tags {
			if m.filterTag[t] {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

// beanMatchesArchive is the archive-visibility half of the combined filter
// predicate (E5 Task 7, bean bt-ggt2, design decision e): completed/scrapped
// beans -- archived (Bean.IsArchived(), data/bean.go) or merely
// completed-but-still-in-.beans/, the distinction is irrelevant to
// visibility policy, epic-E5-plan.md »Task 7« -- are hidden unless
// m.showArchived is toggled on. A PROGRAM DEFAULT, not a PO-set facet:
// filterActive() deliberately does NOT consult m.showArchived (doc-stamp
// there), so this predicate lives entirely outside the four
// filterStatus/filterType/filterPriority/filterTag maps beanMatchesFacets
// checks.
func (m model) beanMatchesArchive(b *item.Item) bool {
	if m.showArchived {
		return true
	}
	return data.IsOpenStatus(b.Status)
}

// beanMatches is the ONE combined predicate every filtered view (Tree here,
// Ready in E2 Task 5) calls -- AND of search (Task 3's beanMatchesSearch),
// facets (beanMatchesFacets), and archive-visibility (beanMatchesArchive,
// E5 Task 7). Search is NOT special-cased against the archive default: an
// async Bleve hit for an archived bean is excluded here exactly like any
// facet-excluded hit -- the same AND-combination status/type/priority
// already impose on a Bleve result set (beanMatchesSearch's own doc
// comment applies unchanged), no separate wiring needed in
// applyBleveResult (documented Task-7 decision, "Suche respektiert den
// Archiv-Default").
func (m model) beanMatches(b *item.Item) bool {
	return m.beanMatchesSearch(b) && m.beanMatchesFacets(b) && m.beanMatchesArchive(b)
}

// treeActive generalizes Task 3's treeSearchActive() to "search OR facets are
// narrowing the tree" -- visibleNodes() (view_browse_repo.go) switches to
// the filtered flattening whenever this is true.
func (m model) treeActive() bool {
	return m.treeSearchActive() || m.filterActive()
}

// facetOn reports whether item's value is currently set in its facet map --
// the single lookup the value menu's checkbox rendering and the menu-nav
// tests both need.
func (m model) facetOn(it ffItem) bool {
	switch it.facet {
	case "status":
		return m.filterStatus[it.value]
	case "type":
		return m.filterType[it.value]
	case "priority":
		return m.filterPriority[it.value]
	case "tag":
		return m.filterTag[it.value]
	case "archive":
		return m.showArchived
	}
	return false
}

// toggleFacet flips it's membership in its facet map. I01 (bean bt-7jr8
// T8-review, types.go doc-stamp): clones the TARGET map via cloneBoolMap
// before writing -- copy-on-write, same convention as setExpanded
// (update.go) and expandAncestorsOf.
func (m model) toggleFacet(it ffItem) model {
	toggle := func(mp map[string]bool) map[string]bool {
		out := cloneBoolMap(mp)
		if out[it.value] {
			delete(out, it.value)
		} else {
			out[it.value] = true
		}
		return out
	}
	switch it.facet {
	case "status":
		m.filterStatus = toggle(m.filterStatus)
	case "type":
		m.filterType = toggle(m.filterType)
	case "priority":
		m.filterPriority = toggle(m.filterPriority)
	case "tag":
		m.filterTag = toggle(m.filterTag)
	case "archive":
		// E5 Task 7: a plain bool, not a facet map -- no cloneBoolMap
		// aliasing hazard (model is a value receiver, the bool is copied by
		// value like every other scalar field).
		m.showArchived = !m.showArchived
	}
	return m
}

// clearFacets resets all four facet maps to FRESH EMPTY maps, not nil --
// keeps filterActive()/len() checks cheap and the map identity consistently
// non-nil after any clear, mirroring devd's own X-clear (view_browse_project
// .go:955-960, `map[treeKind]bool{}` literals, not `nil`).
func (m model) clearFacets() model {
	m.filterStatus = map[string]bool{}
	m.filterType = map[string]bool{}
	m.filterPriority = map[string]bool{}
	m.filterTag = map[string]bool{}
	return m
}

// filterFields lists the filter bar's fields left to right. They are the
// focus targets of the bar: m.filterField indexes this slice, and the bar's
// cell order, its mouse hit test and the value menu all read this one list.
var filterFields = []string{"type", "status", "priority", "tag", "archive"}

// filterFacetRange returns the [start,end) index range items occupies for
// facet -- a single linear scan suffices since buildFilterItems groups each
// facet's rows contiguously (one loop per facet). Returns (0, 0) if facet
// is absent.
func filterFacetRange(items []ffItem, facet string) (start, end int) {
	start, end = -1, -1
	for i, it := range items {
		if it.facet == facet {
			if start == -1 {
				start = i
			}
			end = i + 1
		} else if start != -1 {
			break
		}
	}
	if start == -1 {
		return 0, 0
	}
	return start, end
}

// filterMenuMaxRows caps the rows the value menu shows at once, so the popup
// fits below the bar on the terminal it renders on; a long facet (Tags)
// scrolls inside the popup.
func (m model) filterMenuMaxRows() int {
	h := m.height
	if h <= 0 {
		h = 24
	}
	return max(3, h-14)
}

// openFilterMenu opens the value menu of the focused filter field with the
// cursor on its first value. A field without values (Tags in a store without
// tags) opens nothing.
func (m model) openFilterMenu() model {
	m.filterItems = m.buildFilterItems()
	start, end := filterFacetRange(m.filterItems, filterFields[m.filterField])
	if end <= start {
		return m
	}
	m.filterMenu = listState{length: len(m.filterItems), cursor: start}
	m.overlay = overlayFilterMenu
	return m
}

// filterMenuRows returns the [start,end) rows of filterItems that belong to
// the focused field.
func (m model) filterMenuRows() (start, end int) {
	return filterFacetRange(m.filterItems, filterFields[m.filterField])
}

// filterMenuWindow returns the [first,last) rows the popup shows: the cursor
// row is always inside, so a long facet scrolls with the cursor.
func (m model) filterMenuWindow() (first, last int) {
	start, end := m.filterMenuRows()
	vis := min(end-start, m.filterMenuMaxRows())
	first = start + max(0, m.filterMenu.cursor-start-vis+1)
	return first, first + vis
}

// keyFilterMenu drives the open value menu: up/down move within the field's
// values, space/x and enter toggle the cursored value (the filter applies at
// once and the menu stays open), X clears all facets, esc closes. The menu
// captures every other key (handleKey checks m.overlay before the global
// keys).
func (m model) keyFilterMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	start, end := m.filterMenuRows()
	switch navKey(msg.String()) {
	case "up":
		m.filterMenu.cursor = max(start, m.filterMenu.cursor-1)
		return m, nil
	case "down":
		m.filterMenu.cursor = min(end-1, m.filterMenu.cursor+1)
		return m, nil
	}
	switch {
	case keybind.Matches(msg, keys.Back):
		m.overlay = overlayNone
	case keybind.Matches(msg, keys.FilterClear):
		m = m.clearFacets().resetCursorToFirstVisible()
	case keybind.Matches(msg, keys.Toggle), keybind.Matches(msg, filterMenuApplyHint):
		m = m.toggleFacet(m.filterItems[m.filterMenu.cursor]).resetCursorToFirstVisible()
	}
	return m, nil
}

// filterMenuBox renders the value menu popup of the focused field: a hint
// line, then the visible window of its values with a [x]/[ ] box each. The
// hint stays one short line: modalBox's wrap hard-splits long hint lines
// under a real TrueColor terminal.
func (m model) filterMenuBox() string {
	start, end := m.filterMenuRows()
	first, last := m.filterMenuWindow()
	title := facetHead[filterFields[m.filterField]]
	if last-first < end-start {
		title += fmt.Sprintf(" %d/%d", m.filterMenu.cursor-start+1, end-start)
	}
	var b strings.Builder
	b.WriteString(theme.Muted.Render("space/x:toggle  X:clear  esc:close") + "\n")
	for i := first; i < last; i++ {
		it := m.filterItems[i]
		box := theme.Dim.Render("[ ]")
		if m.facetOn(it) {
			box = theme.Accent.Render("[x]")
		}
		cursor := "  "
		label := it.label
		if i == m.filterMenu.cursor {
			cursor = theme.Accent.Render("▸ ")
			label = theme.Header.Render(label)
		}
		b.WriteString(cursor + box + " " + label + "\n")
	}
	body := strings.TrimSuffix(b.String(), "\n")
	return modalPanel(title, body, "", clampModalWidth(valueMenuContentWidth(title, body), m.width), theme.Mauve)
}

// filterMenuFirstRow is the popup row (0 = its top border) of the first
// value: border, title, hint line.
const filterMenuFirstRow = 3

// filterMenuRect is the popup's screen rect: left-aligned under the focused
// field of the filter bar, shifted left when it would pass the right edge.
func (m model) filterMenuRect() (x, y, w, h int) {
	lines := strings.Split(m.filterMenuBox(), "\n")
	for _, l := range lines {
		w = max(w, ansi.StringWidth(l))
	}
	h = len(lines)
	bx, by, innerW := filterBarOrigin(m)
	x = bx + m.filterField*detailBoxFormGap
	for _, cw := range gridColWidths(len(filterFields), innerW)[:m.filterField] {
		x += cw
	}
	if tw := m.width; tw > 0 && x+w > tw {
		x = max(0, tw-w)
	}
	return x, by + filterBarHeight, w, h
}

// mouseFilterMenuClick handles a left click while the value menu is open: a
// click on a value row toggles it (the menu stays open), a click outside the
// popup closes it, a click on the popup's chrome does nothing.
func (m model) mouseFilterMenuClick(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	x, y, w, h := m.filterMenuRect()
	if msg.X < x || msg.X >= x+w || msg.Y < y || msg.Y >= y+h {
		m.overlay = overlayNone
		return m, nil
	}
	first, last := m.filterMenuWindow()
	if i := first + msg.Y - y - filterMenuFirstRow; msg.Y-y >= filterMenuFirstRow && i < last {
		m.filterMenu.cursor = i
		m = m.toggleFacet(m.filterItems[i]).resetCursorToFirstVisible()
	}
	return m, nil
}

// filterSummary summarizes the active facets for the Tree head row
// (treeSearchLine, view_browse_repo.go) -- port devd filterSummary
// (view_browse_project.go:1035-1060), generalized to beans-tui's 4-facet
// shared state (no "Art" facet here, see the file doc comment above).
//
// bt-2kfl (D02, "eine Wahrheit mit Facetten-State" as a UNION display, NOT a
// write-merge): each facet row shows the UNION of the f-menu's own map
// (m.filterStatus et al.) AND whatever the user has typed as a search
// prefix (m.searchPrefixFacets, search_prefix.go) -- READ-only here, this
// function itself never writes to m.filterStatus/etc. (D02's own
// invariant). `union` is a local closure (not a new top-level func) so this
// stays the ONLY box_filter_facets.go edit for bt-2kfl (file discipline,
// parallel worktree wave); it reuses the existing joinFilterKeys unchanged.
// With no typed prefixes (m.searchPrefixFacets empty/nil, the pre-bt-2kfl
// default), union(mp, nil) degrades to exactly mp's own keys -- this
// function's output is byte-identical to the pre-bt-2kfl version whenever
// no prefix is typed (golden-safe).
func (m model) filterSummary() string {
	union := func(mp map[string]bool, extra []string) map[string]bool {
		if len(mp) == 0 && len(extra) == 0 {
			return nil
		}
		out := make(map[string]bool, len(mp)+len(extra))
		for k, v := range mp {
			if v {
				out[k] = true
			}
		}
		for _, v := range extra {
			out[v] = true
		}
		return out
	}
	var parts []string
	if st := union(m.filterStatus, m.searchPrefixFacets["status"]); len(st) > 0 {
		parts = append(parts, "St:"+joinFilterKeys(st))
	}
	if ty := union(m.filterType, m.searchPrefixFacets["type"]); len(ty) > 0 {
		parts = append(parts, "Ty:"+joinFilterKeys(ty))
	}
	if pr := union(m.filterPriority, m.searchPrefixFacets["priority"]); len(pr) > 0 {
		parts = append(parts, "Pr:"+joinFilterKeys(pr))
	}
	if tg := union(m.filterTag, m.searchPrefixFacets["tag"]); len(tg) > 0 {
		parts = append(parts, "Tags:"+joinFilterKeys(tg))
	}
	return strings.Join(parts, " ")
}

// joinFilterKeys returns a facet map's set keys, comma-joined.
//
// ERRATUM vs. devd's own joinFilterKeys (view_browse_project.go:1062-1071):
// that port source ranges over the map directly with no sort step -- Go map
// iteration order is randomized per-process, so the SAME filter state would
// render its summary keys in a different order from one View() call to the
// next (well, actually stable within one run since Go re-randomizes only
// across process starts, but still nondeterministic across test runs / a
// real hazard for any future golden test covering the filter-active head
// row). Fixed here with sort.Strings -- same display contract, deterministic
// order.
func joinFilterKeys(mp map[string]bool) string {
	var out []string
	for k, v := range mp {
		if v {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// clampModalWidth clamps a floating box's preferred width to the terminal
// width (port devd forms_shared.go clampModalWidth, DD2-55): shrinks on
// narrow terminals (termW-4 leaves 2 cols of margin/border on each side),
// floor 24. First shared-modal-width consumer in beans-tui -- the existing
// quit-confirm box (box_confirm_quit.go) still hardcodes a bare 40; not
// retrofitted here, out of this task's scope.
func clampModalWidth(pref, termW int) int {
	w := pref
	if termW > 4 && termW-4 < w {
		w = termW - 4
	}
	if w < 24 {
		w = 24
	}
	return w
}

// wideModalWidth sizes a floating box relative to the terminal, unlike
// clampModalWidth (which only ever SHRINKS a fixed preference) -- B06,
// design-spec.md §15 PF-17: the Blocking-/Parent-Picker need to GROW on
// wide terminals (PO: "die Breite muss viel weiter werden"), not stay
// pinned to a fixed 48. ~85% of termW, floor 60 (never narrower than the
// old fixed value), ceiling termW-4 (same 2-column margin convention as
// clampModalWidth).
func wideModalWidth(termW int) int {
	w := termW * 85 / 100
	if w < 60 {
		w = 60
	}
	if termW > 4 && w > termW-4 {
		w = termW - 4
	}
	if w < 24 {
		w = 24 // absolute floor, mirrors clampModalWidth's own floor
	}
	return w
}
