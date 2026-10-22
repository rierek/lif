package tui

// box_filter_facets.go — the ONE shared facet-filter menu for Tree (this
// task) AND Backlog (E2 Task 5, bean bt-gzu6, reuses everything here
// unchanged). design-spec.md §6 US-05: "Filter wirkt auf Tree UND Backlog".
// Port pattern: devd view_browse_backlog.go:89-136 (buildBacklogFilterItems
// shape) + view_browse_project.go:936-1033 (keyTreeFilter/treeFilterBox) --
// consolidated into ONE implementation instead of devd's two parallel ones
// (Tree-filter vs. Backlog-filter), since beans-tui's filter state already
// lives model-wide, not per-view. No "Art" facet (devd's Milestone/Sprint/
// Issue split) -- beans' Type enum (milestone/epic/feature/task/bug) already
// covers that distinction, so Status/Type/Priority/Tag is the complete set.

import (
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
	return b.Status != "completed" && b.Status != "scrapped"
}

// beanMatches is the ONE combined predicate every filtered view (Tree here,
// Backlog in E2 Task 5) calls -- AND of search (Task 3's beanMatchesSearch),
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
// the single lookup treeFilterBox's checkbox rendering and the menu-nav
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

// filterFacetOrder returns the distinct facet keys present in items, in
// order of FIRST appearance -- buildFilterItems always emits one contiguous
// run per facet (status/type/priority/tag/archive, in that fixed loop
// order), so this doubles as "the tab order" without hardcoding facetHead's
// own key set: a facet with zero rows (e.g. no tags loaded yet) is simply
// never reported as a tab, no phantom empty category.
func filterFacetOrder(items []ffItem) []string {
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it.facet] {
			seen[it.facet] = true
			out = append(out, it.facet)
		}
	}
	return out
}

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

// filterMenuActiveFacet resolves m.filterTab against the CURRENT
// filterItems (defensively clamped -- filterItems can shrink, e.g. a tag
// facet losing its last row, between one menu-open and the next) and
// returns the active facet's key plus its [start,end) row range.
func (m model) filterMenuActiveFacet() (facet string, start, end int, ok bool) {
	facets := filterFacetOrder(m.filterItems)
	if len(facets) == 0 {
		return "", 0, 0, false
	}
	tab := m.filterTab
	if tab < 0 {
		tab = 0
	} else if tab >= len(facets) {
		tab = len(facets) - 1
	}
	facet = facets[tab]
	start, end = filterFacetRange(m.filterItems, facet)
	return facet, start, end, true
}

// filterMenuMoveCursor moves the row cursor by d, CLAMPED to the ACTIVE
// tab's own [start,end) range (PO-Klickpfad step 3, epic-E12-plan.md Item
// 7: "im fokussierten Tab ... kann ich mit Pfeil-rauf/runter direkt die
// Auswahl navigieren") -- up/down must never cross into a neighboring
// category's rows. Pointer receiver mutating the caller's local model copy
// in place, same convention as the pre-existing m.filterMenu.move(d) call
// this replaces (keyFilterMenu holds m by value; Go lets a pointer-receiver
// method run against an addressable local variable).
func (m *model) filterMenuMoveCursor(d int) {
	_, start, end, ok := m.filterMenuActiveFacet()
	if !ok || end <= start {
		return
	}
	c := m.filterMenu.cursor + d
	if c < start {
		c = start
	}
	if c >= end {
		c = end - 1
	}
	m.filterMenu.cursor = c
}

// filterMenuSwitchTab moves the active tab by d (wrapping around both
// ends), then jumps the row cursor to the NEW tab's first element
// (PO-Klickpfad steps 2+3: "tab/shift-tab andere Filter wählen" ->
// "im fokussierten Tab ist das erste Element immer aktiv").
func (m model) filterMenuSwitchTab(d int) model {
	facets := filterFacetOrder(m.filterItems)
	if len(facets) == 0 {
		return m
	}
	n := len(facets)
	m.filterTab = ((m.filterTab+d)%n + n) % n
	start, _ := filterFacetRange(m.filterItems, facets[m.filterTab])
	m.filterMenu.cursor = start
	return m
}

// keyFilterMenu drives the floating facet-filter menu (port devd
// keyTreeFilter, view_browse_project.go:936-996, extended bt-2p9m for the
// Querformat Tab-Kategorien): tab/shift+tab switch the active facet
// category (jumping the cursor to its first row), up/down move the cursor
// WITHIN the active category only, space/x toggles the cursored row's
// facet (copy-on-write, unchanged), X clears all four facet maps WITHOUT
// closing the menu (devd parity, unchanged), esc/f close the menu without
// touching filter state (unchanged).
//
// enter APPLIES the cursored value and holds the focus in the region
// instead of closing the menu -- bean bt-8d35's Fokus-Modell, see the case's
// own comment below.
//
// tab/shift+tab safety (epic-E12-plan.md Item 7, "Kein Tastenkonflikt",
// verified by TestFilterMenuTabDoesNotLeakToGlobalFocusToggle): keys.FocusIn
// /keys.FocusOut are ALSO the global Tree<->Detail focus-swap bindings
// (update.go's handleKey, tab/shift+tab case, checked further down that
// function), but handleKey checks m.filterOpen FIRST (Full-Capture,
// update.go, same precedent m.searchActive/m.overlay already use) and
// routes every key straight here instead -- the global case below never
// even runs while the filter menu is open, so binding tab/shift+tab a
// SECOND, different meaning here is safe by construction, not by luck.
func (m model) keyFilterMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.FocusIn):
		return m.filterMenuSwitchTab(1), nil
	case keybind.Matches(msg, keys.FocusOut):
		return m.filterMenuSwitchTab(-1), nil
	}
	switch navKey(msg.String()) {
	case "up":
		m.filterMenuMoveCursor(-1)
		return m, nil
	case "down":
		m.filterMenuMoveCursor(1)
		return m, nil
	}
	switch {
	case keybind.Matches(msg, filterStripApplyHint):
		// bean bt-8d35 Flow #3 ("Anwenden: enter -> Filter wirkt, Fokus bleibt
		// im Strip"): under the Fokus-Modell the Strip is a REGION, and only
		// esc/f leave a region -- enter must therefore APPLY without closing.
		// Applying == toggling the cursored value (the facet maps are
		// copy-on-write and already live), i.e. the same effect keys.Toggle has;
		// PO-Entscheidung 1 gives space and enter that same job inside an open
		// multi-select, with enter carrying the "open/apply" reading.
		if m.filterMenu.cursor < 0 || m.filterMenu.cursor >= len(m.filterItems) {
			return m, nil
		}
		m = m.toggleFacet(m.filterItems[m.filterMenu.cursor])
		return m.resetCursorToFirstVisible(), nil
	case keybind.Matches(msg, keys.Back), keybind.Matches(msg, keys.Filter):
		m.filterOpen = false
		return m.resetCursorToFirstVisible(), nil
	case keybind.Matches(msg, keys.FilterClear):
		m = m.clearFacets()
		return m.resetCursorToFirstVisible(), nil
	case keybind.Matches(msg, keys.Toggle):
		if m.filterMenu.cursor < 0 || m.filterMenu.cursor >= len(m.filterItems) {
			return m, nil
		}
		m = m.toggleFacet(m.filterItems[m.filterMenu.cursor])
		return m.resetCursorToFirstVisible(), nil
	}
	return m, nil
}

// filterTabBar renders the Querformat tab bar: one label per facet
// (facetHead order as it actually appears in filterItems, filterFacetOrder),
// the active tab bracketed+accented, the rest dim. Every label is a single
// word (Status/Type/Priority/Tags/Archive) with no internal space, and the
// brackets around the active one add no internal space either -- each tab
// is one wrap-atomic unit by construction (mirrors renderBindings' own
// nbsp-atomic-unit rationale, view.go, minus the nbsp since there is simply
// no space to protect here), so a narrow-terminal wrap can only ever break
// BETWEEN tabs, never inside one.
func (m model) filterTabBar() string {
	facets := filterFacetOrder(m.filterItems)
	if len(facets) == 0 {
		return ""
	}
	active := m.filterTab
	if active < 0 {
		active = 0
	} else if active >= len(facets) {
		active = len(facets) - 1
	}
	labels := make([]string, len(facets))
	for i, f := range facets {
		name := facetHead[f]
		if name == "" {
			name = f
		}
		if i == active {
			labels[i] = theme.Accent.Render("[" + name + "]")
		} else {
			labels[i] = theme.Dim.Render(name)
		}
	}
	return strings.Join(labels, "  ")
}

// treeFilterBox renders the floating facet-filter menu (port devd
// treeFilterBox, view_browse_project.go:998-1033; bt-2p9m rebuilt as
// Querformat: a horizontal tab bar over the ACTIVE category's own rows only,
// instead of one long vertical list over all five facets -- PO-Review E11
// Runde 2 verbatim, "Wenn ich als Nutzer zu einem konkreten Filter gehen
// möchte, dann muss ich sehr lange mit navigieren"). Width bumped from the
// old fixed 40 to 46 (clampModalWidth still shrinks on narrow terminals,
// floor 24 unchanged) -- the widest tab-bar row (any one of the five tabs
// bracketed active) is 39 cells, +4 for modalBox's border+padding overhead
// leaves headroom without wrapping the tab bar on any terminal wide enough
// to hold the modal at all (verified tmux-Smoke at 80 cols).
//
// Hint line is TWO explicitly hand-broken lines ("\n" inserted here, not one
// long auto-wrapped string) -- a REAL regression found live in this task's
// own tmux-Smoke (NOT caught by any unit test, same class of bug as
// ../docs/LESSONS-LEARNED.md Eintrag 4's NBSP-Wordwrap-Falle, but a DIFFERENT
// wrap path this time): modalBox's lipgloss Width-wrap word-wraps correctly
// with plain ASCII (go test, no TTY -> NoColor profile, zero ANSI bytes),
// but under a REAL TrueColor terminal the rebaseBg-rewritten ANSI stream
// made the same wrap logic hard-split "X:clear" mid-word ("X:cl"/"ear")
// even though that token has no internal space at all -- NBSP cannot fix
// this (there is no space inside the token to protect). The only reliable
// fix: never let this line reach modalBox's auto-wrap in the first place.
// Each hand-placed line (23 / 40 cells) stays safely under the 44-cell
// content budget (width 46 minus modalBox's 2-cell padding) at every width
// this modal ever renders at, so the wrap function never has to make a
// decision on this text at all.
func (m model) treeFilterBox() string {
	var b strings.Builder
	b.WriteString(theme.Muted.Render("space/x:toggle  X:clear") + "\n")
	b.WriteString(theme.Muted.Render("tab/shift+tab:category  enter/esc/f:done") + "\n\n")
	b.WriteString(m.filterTabBar() + "\n\n")

	_, start, end, ok := m.filterMenuActiveFacet()
	if ok {
		for i := start; i < end; i++ {
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
	}
	return modalPanel("Filter", b.String(), "", clampModalWidth(46, m.width), theme.Mauve)
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
