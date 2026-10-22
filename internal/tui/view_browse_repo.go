package tui

// view_browse_repo.go — the Browse Primat-View (design-spec.md §6 V2 basis):
// a two-pane master-detail layout, left = expandable Tree (Milestones as
// roots -> Epics/Features -> leaves, plus a synthetic "(orphaned)" root for
// orphans, MANDATORY per bean bt-7jr8/T3-review Q01), right = a placeholder
// detail preview (full accordion lands in E2). Port-pattern reference: devd
// view_browse_project.go (treeNode flattening + D08 cursor-bar rendering,
// ~/dev/DeveloperDashboard/apps/cli-go/internal/tui/
// view_browse_project.go:382-398).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"path/filepath"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// treeNode is a single visible (already-flattened) row of the tree. Index-free
// (holds a *item.Item pointer straight into the Index, mirroring data.Index's
// own contract: consumers must not mutate it) -- depth-first order matches
// render order 1:1, so cursor position == slice index.
type treeNode struct {
	id      string // bean ID, or orphanRootID for the synthetic "(orphaned)" root
	bean    *item.Item
	depth   int
	hasKids bool
	open    bool
	orphan  bool // true only for the synthetic orphan-root node itself

	// placeholder + hiddenCount (D01, bean bt-39cl): a synthetic, non-
	// selectable hint row rendered as the ONLY child of an expanded node
	// whose Direct-Children are ALL filtered out by the archive default
	// (filteredBeanNode's own archiveOnly branch, below) -- "the marker
	// lies otherwise" (D01 PO-Wortlaut): the expand marker stays ▾ (the
	// epic DOES have children), this row explains WHY none of them render.
	// id is deliberately left "" -- never a legitimate cursor target
	// (treeCursorMove/skipPlaceholder skip it structurally instead of
	// relying on a sentinel ID nobody could accidentally collide with).
	placeholder bool
	hiddenCount int // N = count of direct children hidden by the archive default (D01 "Zählwert N")
}

// flattenTree walks idx depth-first (Roots() -> Children, expand-state from
// expanded) into the visible row list, appending the synthetic orphan root
// (+ its subtree, when expanded) last. Returns nil for a nil idx (pre-load).
func flattenTree(idx *data.Index, expanded map[string]bool) []treeNode {
	if idx == nil {
		return nil
	}

	var nodes []treeNode
	ancestors := map[string]bool{}
	for _, b := range idx.Roots() {
		nodes = appendBeanNode(nodes, idx, b, 0, expanded, ancestors)
	}

	orphans := collectOrphans(idx)
	cycles := collectCycleOrphans(idx, orphans)
	if len(orphans) > 0 || len(cycles) > 0 {
		open := expanded[orphanRootID]
		nodes = append(nodes, treeNode{id: orphanRootID, depth: 0, hasKids: true, open: open, orphan: true})
		if open {
			for _, b := range orphans {
				nodes = appendBeanNode(nodes, idx, b, 1, expanded, ancestors)
			}
			// B02 (T8 Opus review, bean bt-7jr8): cycle-trapped beans render as
			// FLAT depth-1 rows -- never recursed into via appendBeanNode. Every
			// bean whose whole Parent-chain loops back on itself (A -> B -> A)
			// is, by construction, only reachable downward from another member
			// of the very same cycle (see collectCycleOrphans), so recursing
			// would either re-hit the cycle guard for nothing or -- for a bean
			// merely hanging off a cycle member -- render it a second time here
			// AND nested under its (also-unreachable) parent. A flat row keeps
			// the fix simple and duplicate-free: every such bean gets exactly
			// one visible row, just without its undefined/circular nesting
			// reconstructed.
			for _, b := range cycles {
				nodes = append(nodes, treeNode{id: b.ID, bean: b, depth: 1, hasKids: false})
			}
		}
	}
	return nodes
}

// appendBeanNode appends b (and, if expanded, its children) depth-first.
// ancestors is the current DFS path -- a bean already on its own ancestor
// path is skipped (defensive cycle guard; beans' frontmatter is hand-
// editable, a parent cycle is a data error but must never hang/crash bt).
func appendBeanNode(nodes []treeNode, idx *data.Index, b *item.Item, depth int, expanded map[string]bool, ancestors map[string]bool) []treeNode {
	if ancestors[b.ID] {
		return nodes
	}
	children := idx.Children[b.ID]
	open := expanded[b.ID]
	nodes = append(nodes, treeNode{id: b.ID, bean: b, depth: depth, hasKids: len(children) > 0, open: open})
	if open && len(children) > 0 {
		ancestors[b.ID] = true
		for _, c := range children {
			nodes = appendBeanNode(nodes, idx, c, depth+1, expanded, ancestors)
		}
		delete(ancestors, b.ID)
	}
	return nodes
}

// collectOrphans returns every bean whose Parent is set but does not resolve
// to a known bean (dangling parent -- beans-legal, `beans check` only
// reports it as broken_links). Sorted via data.SortBeans (Status -> Priority
// -> Type -> Title) for determinism: idx.ByID iteration order is a Go map,
// so this must never be rendered unsorted (would break golden-test
// determinism and PO expectations alike). I03 (bean bt-7jr8 T8-review,
// closed E2 Task 4/bean bt-9ldr): this used to run its own ad-hoc
// title-only tie-break (sortByTitleThenID) because data.sortBeans was
// unexported at the time -- now that data.SortBeans is exported (E2 Task 1),
// the orphan bucket uses the SAME single-source order as every other bean
// list in the app instead of a second, parallel sort definition.
func collectOrphans(idx *data.Index) []*item.Item {
	var out []*item.Item
	for _, b := range idx.ByID {
		if b.Parent == "" {
			continue
		}
		if _, ok := idx.ByID[b.Parent]; ok {
			continue
		}
		out = append(out, b)
	}
	data.SortBeans(out)
	return out
}

// reachableIDs returns every bean ID reachable via idx.Children, descending
// from roots. Structural, INDEPENDENT of the current expand/collapse UI
// state on purpose: collapsing a node in the tree must never make its
// subtree "disappear" from cycle detection -- only from the currently
// rendered rows (flattenTree's own expanded lookups handle that separately).
// A bean already visited stops the walk -- guards a cycle attached under a
// legitimately-reachable bean the same way appendBeanNode's per-path
// ancestors guard does, just as a global set here since only the full
// reachable SET is needed, not render order.
func reachableIDs(idx *data.Index, roots []*item.Item) map[string]bool {
	seen := map[string]bool{}
	var walk func(b *item.Item)
	walk = func(b *item.Item) {
		if seen[b.ID] {
			return
		}
		seen[b.ID] = true
		for _, c := range idx.Children[b.ID] {
			walk(c)
		}
	}
	for _, b := range roots {
		walk(b)
	}
	return seen
}

// collectCycleOrphans returns every bean reachable from NEITHER a real root
// NOR a dangling-parent orphan's subtree -- beans trapped in a pure parent
// cycle (A -> B -> A), or hanging off one. Their own Parent DOES resolve (so
// collectOrphans misses them) and they can never be a Root() (Parent is
// non-empty), so without this sweep they are silently invisible. B02 (T8
// Opus quality review, bean bt-7jr8): beans-legal state -- frontmatter is
// hand-editable -- must never be dropped; swept into the same synthetic
// "(orphaned)" root as dangling-parent orphans instead.
func collectCycleOrphans(idx *data.Index, orphans []*item.Item) []*item.Item {
	anchors := append(append([]*item.Item{}, idx.Roots()...), orphans...)
	reachable := reachableIDs(idx, anchors)

	var out []*item.Item
	for id, b := range idx.ByID {
		if !reachable[id] {
			out = append(out, b)
		}
	}
	data.SortBeans(out)
	return out
}

// visibleNodes flattens the model's current idx+expanded state -- switches
// to the filtered flattening (flattenTreeFiltered) whenever search OR facets
// are narrowing the tree (m.treeActive(), box_filter_facets.go, E2 Task 4)
// OR the archive default is hiding completed/scrapped beans (E5 Task 7,
// bean bt-ggt2, design decision e: !m.showArchived, the DEFAULT steady
// state, not just an opt-in narrowing). m.beanMatches AND-combines Task 3's
// beanMatchesSearch, Task 4's beanMatchesFacets, and Task 7's
// beanMatchesArchive; all three stay in place as the individual halves that
// combined predicate calls into (Backlog, E2 Task 5, reuses beanMatches
// unchanged).
//
// The `|| !m.showArchived` term is deliberately NOT folded into
// treeActive() itself: treeActive() also drives treeSearchLine's "Filter
// aktiv" red-line rendering (view_browse_repo.go), and a program default
// hiding archived beans must NOT flip that line red -- filterActive()/
// treeActive() stay byte-for-byte unchanged, exactly the "kein PO-Facet"
// acceptance requirement (box_filter_facets.go doc-stamps). This routing
// condition is the ONLY place the archive default plugs in; without it, the
// default-hide would silently never apply whenever the tree has no active
// search/facet (the common case) -- flattenTree ignores beanMatches
// entirely, so a bean's completed/scrapped status would keep rendering
// unfiltered even with m.showArchived false (the exact bug class T6's own
// Notes-für-T7 warned about, generalized from "Settings-Reload" to "any
// silently-bypassed predicate").
func (m model) visibleNodes() []treeNode {
	if m.treeActive() || !m.showArchived {
		// archiveOnly (D01, bean bt-39cl): true iff the filtered flattening
		// is reached ONLY because of the archive default (no search/facet
		// narrowing) -- exactly the `!m.treeActive()` half of this very
		// condition. Gates filteredBeanNode's Voll-Verdeckungs-Platzhalter
		// so it renders ONLY in the pure archive-default view -- "bei
		// aktiver Suche/Facette gilt der bestehende Pfad" (D01 Implementer-
		// Vorgabe, scope-guard against Scope-Creep into the search/facet
		// case, which already has its own, unrelated "0 sichtbare
		// Kinder"-story: the collapsed-context-row devd-DD2-178 parity).
		return flattenTreeFiltered(m.idx, m.expanded, m.beanMatches, !m.treeActive())
	}
	return flattenTree(m.idx, m.expanded)
}

// treeSearchActive reports whether a local/Bleve search query is currently
// narrowing the tree (E2 Task 3; Task 4 extends this with facet state via a
// combined treeActive()).
func (m model) treeSearchActive() bool {
	return strings.TrimSpace(m.searchQuery) != ""
}

// beanMatchesSearch is the search half of the combined filter predicate
// (Task 4 AND-combines this with facet criteria as beanMatches,
// box_filter_facets.go). Below the Bleve threshold (<3 chars), or while a
// Bleve response for THIS exact query hasn't arrived yet, it falls back to
// an immediate local title+ID substring match (case-insensitive) -- once
// searchBleveFor catches up to the current query, the Bleve result set
// becomes authoritative (design-spec.md §6 V2: "Bleve-Modus ab 3 Zeichen").
//
// I01 (E2-T3-Review finding, PFLICHT carried into bean bt-9ldr/Task 4): the
// authoritative-Bleve branch UNIONs the local ID-substring match back in
// rather than replacing it outright. Rationale: data.Store.Search runs a
// title+body full-text query (client.go doc comment) -- it does not
// necessarily index an arbitrary ID substring as a token, so a bean that
// only matched the query by ID would otherwise flicker OUT of the tree the
// instant the async Bleve response for the same query lands, even though
// the user's query still substring-matches its ID exactly as it did before.
//
// bt-2kfl (D02/D03, search_prefix.go): the text half runs against
// m.searchPrefixRest, NOT the raw m.searchQuery -- typed `st:`/`ty:`/`pr:`/
// `tag:` tokens are stripped out by applySearchPrefixes (keySearchInput,
// update.go) before this ever sees the query, so they never enter the
// substring/Bleve match below. The AND-combined prefix-facet half
// (beanMatchesSearchPrefixFacets) is checked FIRST -- a facet miss short-
// circuits before touching the text path at all.
func (m model) beanMatchesSearch(b *item.Item) bool {
	if !m.beanMatchesSearchPrefixFacets(b) {
		return false
	}
	q := strings.ToLower(strings.TrimSpace(m.searchPrefixRest))
	if q == "" {
		return true
	}
	if len(q) >= 3 && m.searchBleveFor == m.searchPrefixRest {
		return m.searchBleveIDs[b.ID] || strings.Contains(strings.ToLower(b.ID), q)
	}
	return strings.Contains(strings.ToLower(b.Title), q) || strings.Contains(strings.ToLower(b.ID), q)
}

// flattenTreeFiltered mirrors flattenTree's depth-first walk but only shows
// a subtree the caller has actually expanded (devd DD2-178 parity,
// view_browse_project.go:215-238, wörtlich portiert per plan Port-Referenzen:
// a manually collapsed ancestor stays collapsed even if a descendant
// matches -- it renders as ONE collapsed context row, never force-expanded).
// Every real root, the orphan bucket, AND the cycle-bean sweep go through
// the SAME predicate: an orphan/cycle bucket with zero matches is omitted
// entirely, matching entries render exactly like the unfiltered tree.
func flattenTreeFiltered(idx *data.Index, expanded map[string]bool, match func(*item.Item) bool, archiveOnly bool) []treeNode {
	if idx == nil {
		return nil
	}

	var nodes []treeNode
	ancestors := map[string]bool{}
	for _, b := range idx.Roots() {
		if ns, hit := filteredBeanNode(idx, b, 0, expanded, ancestors, match, archiveOnly); hit {
			nodes = append(nodes, ns...)
		}
	}

	orphans := collectOrphans(idx)
	cycles := collectCycleOrphans(idx, orphans)
	var orphanNodes []treeNode
	for _, b := range orphans {
		if ns, hit := filteredBeanNode(idx, b, 1, expanded, ancestors, match, archiveOnly); hit {
			orphanNodes = append(orphanNodes, ns...)
		}
	}
	var cycleHits []*item.Item
	for _, b := range cycles {
		if match(b) {
			cycleHits = append(cycleHits, b)
		}
	}

	if len(orphanNodes) > 0 || len(cycleHits) > 0 {
		open := expanded[orphanRootID]
		nodes = append(nodes, treeNode{id: orphanRootID, depth: 0, hasKids: true, open: open, orphan: true})
		if open {
			nodes = append(nodes, orphanNodes...)
			for _, b := range cycleHits { // cycle beans render flat (depth 1), same as flattenTree
				nodes = append(nodes, treeNode{id: b.ID, bean: b, depth: 1, hasKids: false})
			}
		}
	}
	return nodes
}

// filteredBeanNode is flattenTreeFiltered's recursive per-bean step. hit
// reports whether b or anything reachable below it matches: match(b) itself,
// OR (if b is expanded) any expanded-visible child hit, OR (if b is
// collapsed) a structural, expand-state-INDEPENDENT scan of the whole
// subtree below (subtreeHasMatch) -- so a collapsed ancestor still renders
// as ONE collapsed context row when a match exists somewhere beneath it,
// without ever being force-expanded (devd DD2-178 parity, see
// flattenTreeFiltered's own doc comment). ancestors guards a Parent-cycle
// exactly like appendBeanNode's own defensive guard (only threaded through
// the EXPANDED/recursing branch, mirroring appendBeanNode).
func filteredBeanNode(idx *data.Index, b *item.Item, depth int, expanded map[string]bool, ancestors map[string]bool, match func(*item.Item) bool, archiveOnly bool) ([]treeNode, bool) {
	if ancestors[b.ID] {
		return nil, false
	}
	children := idx.Children[b.ID]
	open := expanded[b.ID]
	self := match(b)

	if !open {
		if !self && !subtreeHasMatch(idx, children, b.ID, match) {
			return nil, false
		}
		return []treeNode{{id: b.ID, bean: b, depth: depth, hasKids: len(children) > 0, open: false}}, true
	}

	ancestors[b.ID] = true
	var childNodes []treeNode
	anyChildHit := false
	for _, c := range children {
		if ns, hit := filteredBeanNode(idx, c, depth+1, expanded, ancestors, match, archiveOnly); hit {
			anyChildHit = true
			childNodes = append(childNodes, ns...)
		}
	}
	delete(ancestors, b.ID)

	if !self && !anyChildHit {
		return nil, false
	}
	nodes := []treeNode{{id: b.ID, bean: b, depth: depth, hasKids: len(children) > 0, open: true}}

	// D01 (bean bt-39cl) Voll-Verdeckungs-Fall: node is open, HAS children
	// structurally, but every single one of them was filtered out (anyChildHit
	// false) -- WITHOUT this branch the marker would render ▾ (hasKids=true,
	// self alone kept the parent visible) yet zero rows would follow it, the
	// exact "Marker lügt" bug the Investigation traced to filteredBeanNode.
	// archiveOnly scopes this to the pure archive-default view (visibleNodes'
	// own doc comment) -- when it's false (search/facet narrowing active),
	// the pre-existing behavior (silently zero child rows) is left as-is,
	// deliberately out of scope (D01 Implementer-Vorgabe).
	if archiveOnly && len(children) > 0 && !anyChildHit {
		hidden := 0
		for _, c := range children {
			if !match(c) {
				hidden++
			}
		}
		nodes = append(nodes, treeNode{depth: depth + 1, placeholder: true, hiddenCount: hidden})
	} else {
		nodes = append(nodes, childNodes...)
	}
	return nodes, true
}

// subtreeHasMatch is a structural (expand-state-INDEPENDENT) scan used only
// for a COLLAPSED node (filteredBeanNode): whether match hits anywhere below
// it, so it still renders as a single collapsed context row without being
// force-expanded (devd DD2-178, view_browse_project.go:215-238's documented
// PO decision -- "a manually collapsed node stays collapsed even if it
// contains a match"). skip excludes the collapsed node itself (already
// checked by the caller as `self`); its own local visited set guards a
// Parent-cycle reachable under a collapsed node, independent of the caller's
// DFS-path ancestors set (different concern: this walks unconditionally
// downward, ignoring expand state entirely).
func subtreeHasMatch(idx *data.Index, beans []*item.Item, skip string, match func(*item.Item) bool) bool {
	seen := map[string]bool{skip: true}
	var walk func(b *item.Item) bool
	walk = func(b *item.Item) bool {
		if seen[b.ID] {
			return false
		}
		seen[b.ID] = true
		if match(b) {
			return true
		}
		for _, c := range idx.Children[b.ID] {
			if walk(c) {
				return true
			}
		}
		return false
	}
	for _, b := range beans {
		if walk(b) {
			return true
		}
	}
	return false
}

// cursorPos finds m.cursorID's index in nodes, defaulting to 0 (covers both
// the pre-load state, cursorID == "", and a cursor whose bean disappeared
// without a matching reload clamp having run yet).
func (m model) cursorPos(nodes []treeNode) int {
	for i, n := range nodes {
		if n.id == m.cursorID {
			return i
		}
	}
	return 0
}

// treeCursorMove shifts the Tree cursor by delta (±1), clamped at both ends
// -- factored out of keyTree's own up/down case (update.go, E5 Task 4, bean
// bt-mne6) so the keyboard AND the wheel dispatch (mouse.go handleMouse,
// design decision f: Wheel moves the View-eigene CURSOR) share the exact
// same clamp logic instead of two independent copies.
func (m model) treeCursorMove(nodes []treeNode, delta int) model {
	if len(nodes) == 0 {
		return m
	}
	pos := m.cursorPos(nodes) + delta
	if pos < 0 {
		pos = 0
	}
	if pos > len(nodes)-1 {
		pos = len(nodes) - 1
	}
	pos = skipPlaceholder(nodes, pos, delta)
	m.cursorID = nodes[pos].id
	return m
}

// skipPlaceholder (D01, bean bt-39cl -- Implementer-Entscheidung: NICHT
// selektierbar, Cursor überspringt statt No-Op, siehe bean Notes) advances
// pos in delta's own direction past any archive-hint placeholder row
// (filteredBeanNode, above) -- a pure hint, never a legitimate cursor
// target. Falls back to scanning the OPPOSITE direction once delta's own
// direction runs off the list end (placeholder sitting as the very last
// visible row, its parent epic being the last root) -- guaranteed to
// terminate: index 0 can never be a placeholder (it always immediately
// follows its own parent's row, appendBeanNode/filteredBeanNode's shared
// depth-first order), so a backward scan always finds real ground.
func skipPlaceholder(nodes []treeNode, pos, delta int) int {
	if delta == 0 {
		delta = 1
	}
	step := delta
	for nodes[pos].placeholder {
		next := pos + step
		if next < 0 || next >= len(nodes) {
			step = -step
			next = pos + step
		}
		pos = next
	}
	return pos
}

// treeNodeMarker returns the expand marker (▾ open / ▸ closed / blank leaf).
func treeNodeMarker(n treeNode) string {
	if !n.hasKids {
		return "  "
	}
	if n.open {
		return "▾ "
	}
	return "▸ "
}

// shortBeanID strips the CURRENT repo's own bean-ID prefix from id (bean
// bt-pl5p, Nebenbefund N5, epic bt-vy1q): the left pane showed full IDs
// ("sproutling-btv7") although the header breadcrumb already names the open
// repo -- the slug is pure redundancy there, and the columns it eats are
// exactly the columns the title needs (PO belegte den harten Titel-Clamp in
// beans-tui-boxform-narrow.gif).
//
// Deliberately NOT "cut at the last '-'": bean IDs carry multi-segment
// prefixes (lean-stack-58j0) and the pane also shows FOREIGN/dangling
// references from other repos, which must stay unambiguous. Only the exact
// "<slug>-" prefix of the repo that is open right now is removed; anything
// else (foreign prefix, empty slug, empty remainder) returns id unchanged.
func shortBeanID(id, slug string) string {
	if slug == "" {
		return id
	}
	rest, ok := strings.CutPrefix(id, slug+"-")
	if !ok || rest == "" {
		return id
	}
	return rest
}

// beanIDPrefix is the slug shortBeanID strips from the rows of the currently
// open repo -- data.RepoSlug reads .beans.yml's beans.prefix, the SAME field
// the bean IDs themselves are minted from (repo_slug.go's own doc comment),
// so this can never drift from the real ID shape. Resolved once per render
// by the row builders, not once per row (RepoSlug does file IO).
func (m model) beanIDPrefix() string {
	if m.repoDir == "" {
		return ""
	}
	return data.RepoSlug(m.repoDir)
}

// treeRowText renders one row's plain content: indent + expand marker +
// status glyph + type icon + ID (Sapphire, repo-prefix stripped via
// shortBeanID -- bean bt-pl5p) + title (design-spec.md §8). slug is the
// caller's m.beanIDPrefix(); "" disables shortening entirely.
func treeRowText(n treeNode, slug string) string {
	head, title := treeRowParts(n, slug)
	return head + title
}

// treeRowParts splits one Tree row into its fixed-width HEAD (indent + expand
// marker + status glyph + type icon + ID) and its variable TITLE -- the split
// bean bt-f68z's hanging-indent wrapping needs: the continuation lines are
// padded to the head's own visible width so they start in the title column
// (hangingWrap, line_window.go). treeRowText (above) is the unwrapped
// concatenation, kept as the single-line composer the tests and the
// non-wrapping call sites still use, so head/title can never drift from the
// row they describe.
//
// Placeholder/orphan rows carry no title: their whole content is already a
// hint, and hanging-indenting a hint would be noise -- they return an empty
// title and therefore stay single-line by construction.
func treeRowParts(n treeNode, slug string) (head, title string) {
	indent := strings.Repeat("  ", n.depth)
	marker := treeNodeMarker(n)
	if n.placeholder {
		// D01 (bean bt-39cl): "N archiviert — f→Archive", theme.Muted (the
		// existing Hint-Ton convention, e.g. Overlay-Hints) -- gedimmt, per
		// PO-Wortlaut. hasKids is zero-value false on a placeholder node, so
		// treeNodeMarker already returned the blank "  " above -- same width
		// as every ▾/▸ marker, no layout shift (B03 precedent).
		return indent + marker + theme.Muted.Render(fmt.Sprintf("%d archiviert — f→Archive", n.hiddenCount)), ""
	}
	if n.orphan {
		return indent + marker + theme.Dim.Render("(orphaned)"), ""
	}
	b := n.bean
	return indent + marker + theme.StatusIcon(b.Status) + " " + theme.TypeIcon(b.Type) + " " + theme.Key.Render(shortBeanID(b.ID, slug)) + " ", b.Title
}

// treeRowLines renders one Tree row as one OR MORE terminal lines: the title
// wraps at `width` with a hanging indent aligned to the title column (bean
// bt-f68z / PO-Befund #13, the PO's own sketch). width is the row's total cell
// budget INCLUDING nothing else -- the caller's cursor column is added by
// treeBlocks around this result.
func treeRowLines(n treeNode, slug string, width int) []string {
	head, title := treeRowParts(n, slug)
	if title == "" {
		return []string{truncate(head, width)}
	}
	return hangingWrap(head, lipgloss.Width(head), title, width)
}

// treeRows renders every visible row, applying the D08 cursor treatment to
// the cursor's row only (devd view_browse_project.go:382-398): a leading `▌`
// bar plus the WHOLE row accent-tinted (own per-cell colors stripped first).
// focused=false (Detail pane has focus) freezes the cursor muted instead of
// accent -- only the focused pane's cursor is highlighted (devd D03). bodyH
// is the caller's own row-window height (viewBrowseRepo passes bodyH-1, the
// pane's full content budget minus the 1-line search head -- PF-10, bean
// bt-uyzf, removed renderPane's own title+separator budget entirely, so
// nothing else is subtracted here anymore). Golden Rule #1 still holds:
// windowing trims the ROWS handed to renderPane, it never forces a Height()
// on the bordered style itself. B01 (T8 Opus quality review): rows are windowed around the
// cursor (devd windowAround/windowStart port, view_browse_project.go:
// 647-670) so a tree taller than the pane never hides the cursor below the
// fold.
func (m model) treeRows(nodes []treeNode, focused bool, bodyH, width int) []string {
	rows, _ := blockWindow(m.treeBlocks(nodes, focused, width), bodyH, m.cursorPos(nodes))
	return rows
}

// treeBlocks builds ONE line block per node -- the structure both the renderer
// (treeRows, above) and the mouse hit-test (treeClickRow, below) consume, so
// the click can never land on a different bean than the one the PO sees
// (bean bt-f68z, the "Zeilen sind nicht mehr 1:1 zu beans" follow-up the bean
// itself flags; same Golden-Rule-Drift-Schutz rationale as clickPaneGeometry).
//
// The D08 cursor treatment applies to EVERY line of the cursor's block, not
// just its first: a two-line bean whose second line lost the `▌` bar and the
// accent tint would read as two separate half-selected beans.
//
// width is the row's total cell budget (renderPane's own w-2 truncate budget);
// one cell of it is spent on the cursor column, so the wrap budget handed
// down is width-1.
func (m model) treeBlocks(nodes []treeNode, focused bool, width int) [][]string {
	pos := m.cursorPos(nodes)
	slug := m.beanIDPrefix()
	blocks := make([][]string, len(nodes))
	for i, n := range nodes {
		lines := treeRowLines(n, slug, width-1)
		blocks[i] = decorateRowBlock(lines, i == pos, focused)
	}
	return blocks
}

// decorateRowBlock prepends the 1-cell cursor column to every line of one
// row's block: the D08 `▌` bar plus the whole-line accent tint on the cursor
// row (own per-cell colors stripped first, devd view_browse_project.go:
// 382-398), a plain space otherwise. focused=false (the Detail pane has focus)
// freezes the cursor muted instead of accent -- only the focused pane's cursor
// is highlighted (devd D03). Shared verbatim by the Tree, the Browse flat list
// and the Backlog (view_browse_flat.go / view_browse_backlog.go), which all
// three used to carry their own copy of this five-line block.
func decorateRowBlock(lines []string, isCursor, focused bool) []string {
	out := make([]string, len(lines))
	for j, l := range lines {
		if !isCursor {
			out[j] = " " + l
			continue
		}
		plain := ansi.Strip(l)
		if focused {
			out[j] = theme.Accent.Render("▌" + plain)
		} else {
			out[j] = theme.Dim.Render("▌" + plain)
		}
	}
	return out
}

// windowStart computes the window start index so cursor stays visible --
// centered when there's room, clamped at both edges (devd port:
// view_browse_project.go:649-661, itself shared there by render + the
// mouse-click Y->row mapping). Deterministic in cursor+height alone (no
// hidden state), which is what keeps it jitter-free: the same (n, height,
// cursor) always yields the same window, it never "remembers" a previous
// scroll offset.
func windowStart(n, height, cursor int) int {
	if height <= 0 || n <= height {
		return 0
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > n {
		start = n - height
	}
	return start
}

// windowAround windows rows to height entries so the cursor stays visible
// (devd port: view_browse_project.go:663-670). A no-op when everything
// already fits (including height<=0, an init/edge-case fallback -- renderPane
// itself caps to its own h regardless of how many rows it's handed).
func windowAround(rows []string, height, cursor int) []string {
	if height <= 0 || len(rows) <= height {
		return rows
	}
	start := windowStart(len(rows), height, cursor)
	return rows[start : start+height]
}

// renderDetailPane renders the Detail box form for the cursored bean.
// Resolves the tree cursor's bean (or nil for "no selection") and hands off
// to the shared renderBeanDetailBox (below), which renderBacklogDetailPane
// (view_browse_backlog.go) and the flat list share too, so Tree, Backlog
// and Fullscreen can never drift apart on the Detail body.
func (m model) renderDetailPane(nodes []treeNode, w, h int, focused bool) string {
	pos := m.cursorPos(nodes)
	var b *item.Item
	// I02 (bt-39cl Review R1): placeholder covered explicitly alongside
	// orphan -- its bean is nil anyway (same "(no selection)" outcome), but
	// the guard style stays consistent with focusedBean's own check.
	if pos >= 0 && pos < len(nodes) && !nodes[pos].orphan && !nodes[pos].placeholder {
		b = nodes[pos].bean
	}
	return m.renderBeanDetailBox(b, w, h, focused)
}

// renderBeanDetailBox renders the box-form Detail pane for a single
// already-resolved bean, or the "(no selection)" placeholder for nil -- the
// shared body of renderDetailPane (Tree, above) and renderBacklogDetailPane
// (Backlog), which differ only in HOW they resolve a bean from their own
// cursor shape.
func (m model) renderBeanDetailBox(b *item.Item, w, h int, focused bool) string {
	// The box-form field cursor is only VISIBLE while the Detail pane
	// actually holds focus -- an unfocused pane passes -1 (no Mauve frame
	// anywhere).
	boxCursor := -1
	if focused {
		boxCursor = boxFormEffectiveCursor(m, b)
	}
	return renderDetailBox(m.idx, b, w, h, focused, boxFormEffectiveScroll(m, b), boxCursor)
}

// renderDetailBox is the shared box-form Detail renderer used by the split
// panes and the Vollbild-Detail (renderFullscreenBody).
// boxScroll is the box-form scroll offset into the full form; boxCursor is
// the field cursor index rendered with a Mauve frame, or -1 for none. The
// Vollbild passes -1: there, up/down scroll instead of walking fields.
func renderDetailBox(idx *data.Index, b *item.Item, w, h int, focused bool, boxScroll, boxCursor int) string {
	var rows []string
	if b != nil {
		// accW is the outer box width (border included), what detailBoxForm's
		// dropdownBox/panelBox calls expect.
		accW := w - 2
		if accW < 1 {
			accW = 1
		}
		// detailBoxForm has no windowing of its own, so a tall form (long
		// Body, or Relations/History with many entries) is windowed to `h`
		// (the SAME content-height budget renderPane gets handed below) via
		// scrollView, offset boxScroll.
		// The page indicator rides in the BODY panel's own title row, and
		// that row is pinned to the top of the viewport while the body is
		// paged, so the section title and its indicator stay visible while
		// the body text scrolls beneath. Built from BLOCKS (not the joined
		// detailBoxForm string) so the Body block's boundaries are known:
		// only its first line (the top border) is re-rendered with the dots,
		// and only that line is pinned.
		blocks := boxFormBlocks(idx, b, accW, boxCursor)
		total := 0
		for _, bl := range blocks {
			total += lineCount(bl)
		}
		page, count := boxFormPageIndex(total, h, boxScroll)

		// Re-render the Body top border with the page indicator LEFT-anchored
		// after the label, in the Body block's OWN focus color
		// (boxFormBlocks' same cursor->row rule) so the pinned copy matches the
		// natural one. The dot budget is the header's ACTUAL free width (accW
		// minus corners, "─ Body ─ " label+sep, " (e) " hotkey and the minimum
		// dashes = 21 cells), NOT accW: a long body has more pages than fit as
		// dots, so boxFormPageBadge falls back to the compact "n/N" form when
		// the budget is tight, so it always renders.
		dotsBudget := accW - 21
		if dotsBudget < 1 {
			dotsBudget = 1
		}
		bodyBorder := theme.Overlay
		if boxFormCursorRow(boxCursor) == boxFormRowBody {
			bodyBorder = theme.Mauve
		}
		dots := boxFormPageBadge(page, count, dotsBudget)
		bodyHeader := boxTopBorderBadges("Body", dots, "e", accW, lipgloss.NewStyle().Foreground(bodyBorder))
		if rest := strings.SplitN(blocks[boxFormRowBody], "\n", 2); len(rest) == 2 {
			blocks[boxFormRowBody] = bodyHeader + "\n" + rest[1]
		} else {
			blocks[boxFormRowBody] = bodyHeader
		}

		form := strings.Join(blocks, "\n")
		win, _ := scrollView(form, h, boxScroll)
		winLines := strings.Split(win, "\n")

		// Sticky pin: while the viewport top is scrolled INTO the body (the
		// Body title has passed above the top edge but body content still
		// fills the top), overlay the Body header onto the first content row.
		// The one body line it covers is the topmost -- the least-missed.
		bodyStart := 0
		for i := 0; i < boxFormRowBody; i++ {
			bodyStart += lineCount(blocks[i])
		}
		bodyEnd := bodyStart + lineCount(blocks[boxFormRowBody])
		if boxScroll > bodyStart && boxScroll < bodyEnd && len(winLines) > 0 {
			winLines[0] = bodyHeader
		}
		rows = append(rows, winLines...)
	} else {
		rows = append(rows, theme.Dim.Render("(no selection)"))
	}
	return renderPane(pane{rows: rows}, w, h, focused)
}

// searchShield is the search head row's glyph (U+2315 TELEPHONE RECORDER,
// port devd treeSearchLine view_browse_project.go:1099-1117 -- devd's own
// comment there notes it replaced an ambiguous-width lookalike, DD2-53; kept
// verbatim here for the same EAW-neutral reason).
const searchShield = "⌕"

// treeSearchLine renders the Tree/Backlog pane's persistent search head row
// (design-spec.md §6 V2 "Such-/Filterkopf", port devd treeSearchLine
// view_browse_project.go:1099-1117): the live textinput while typing, the
// committed query AND/OR the active-facet summary in Red once either is
// active (DD2-53 "Filter aktiv" signal, E2 Task 4/bean bt-9ldr: extended to
// cover facets alongside search), or a muted hint when neither is active.
// The idle-hint text is deliberately UNCHANGED from Task 3 (no "f filter"
// addition) so the existing tree.golden fixture (no search/filter state)
// keeps rendering byte-identical -- only the active-state branches grew.
//
// sortSuffix (D02, design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc, the
// Backlog-Sort-Indicator) is appended, ALWAYS theme.Muted ("dezenter
// Suffix", PO-Wortlaut) regardless of the branch's own color, in all THREE
// render branches below when non-empty -- rendered as its OWN span AFTER
// the branch's already-terminated Render() call (never nested inside it):
// ANSI styles do not nest (an inner Reset would clobber an outer wrap, see
// footer()'s own doc comment for the general rule), so concatenating two
// independently-rendered spans is the only way the suffix stays Muted even
// inside the Red-styled treeActive branch. Tree's own call site
// (viewBrowseRepo) always passes "" -- an empty suffix appends nothing, so
// the Tree's search line renders BYTE-IDENTICAL to before D02
// (TestTreeSearchLineEmptySuffixUnchanged, tree.golden's own guarantee).
// Backlog's call site (viewBacklog) passes
// "sort "+backlogSortDisplayLabel(m.backlogSort).
func (m model) treeSearchLine(w int, sortSuffix string) string {
	suffix := ""
	if sortSuffix != "" {
		suffix = theme.Muted.Render(" · " + sortSuffix)
	}
	text, state := m.searchHeadText()
	switch state {
	case searchHeadTyping:
		return truncate(searchShield+" "+text+suffix, w)
	case searchHeadFiltered:
		styled := lipgloss.NewStyle().Foreground(theme.Red).Render(searchShield + " " + text)
		return truncate(styled+suffix, w)
	}
	return truncate(theme.Muted.Render(searchShield+" "+text)+suffix, w)
}

// searchHeadState names the three states the search head can be in. Extracted
// (bean bt-f68z, PO-Befund #11) so the plain single-line head (treeSearchLine,
// above) and the BOXED head (searchHeadBox, below) can differ in FRAMING while
// sharing their CONTENT -- the two must never disagree on what the search
// state is, only on how it is drawn.
type searchHeadState int

const (
	searchHeadIdle searchHeadState = iota
	searchHeadTyping
	searchHeadFiltered
)

// searchHeadText returns the search head's content WITHOUT the ⌕ shield, the
// sort suffix or any styling. The three branches are verbatim the ones
// treeSearchLine carried inline before -- so treeSearchLine's own composition
// (and therefore every pre-existing golden that depends on it) stays
// byte-identical, which is exactly why the shield stayed in the CALLER and did
// not move in here: the boxed variant carries a "Search" frame label instead
// and must not repeat the glyph.
func (m model) searchHeadText() (string, searchHeadState) {
	if m.searchActive {
		text := m.searchInput.View()
		if fs := m.filterSummary(); fs != "" {
			text += "  " + fs
		}
		return text, searchHeadTyping
	}
	if m.treeActive() {
		var parts []string
		if m.treeSearchActive() {
			q := m.searchQuery
			if m.searchBleveLoading {
				q += " …"
			}
			parts = append(parts, q)
		}
		if fs := m.filterSummary(); fs != "" {
			parts = append(parts, fs)
		}
		return strings.Join(parts, "  "), searchHeadFiltered
	}
	return "/ search", searchHeadIdle
}

// searchHeadBoxHeight is the boxed search head's line count (top frame, value
// row, bottom frame) -- the same 3 lines every dropdownBox occupies.
const searchHeadBoxHeight = 3

// searchHeadBox renders the search head as a framed box (bean bt-f68z,
// PO-Befund #11: "⌕ / search verschwindet optisch. Soll ebenfalls boxed
// dargestellt werden, wie die uebrigen Felder"), built from the SAME
// boxTopBorder/boxBottomBorder primitives dropdownBox and the filter chips use
// (box_dropdown.go) -- label in the top frame, hotkey badge in the bottom one.
//
// The frame turns Mauve while the field is focused, mirroring dropdownBox's
// own focus contract; an ACTIVE query/filter additionally tints the value Red,
// the "Filter aktiv" signal (DD2-53) treeSearchLine already carried.
func (m model) searchHeadBox(width int) []string {
	if width < 8 {
		width = 8
	}
	borderColor := theme.Overlay
	if m.searchActive {
		borderColor = theme.Mauve
	}
	frame := lipgloss.NewStyle().Foreground(borderColor)

	text, state := m.searchHeadText()
	inner := width - 4
	switch state {
	case searchHeadFiltered:
		text = lipgloss.NewStyle().Foreground(theme.Red).Render(clampVisible(text, inner))
	case searchHeadIdle:
		text = theme.Muted.Render(clampVisible(text, inner))
	default:
		text = clampVisible(text, inner)
	}
	pad := inner - lipgloss.Width(text)
	if pad < 0 {
		pad = 0
	}
	mid := frame.Render("│") + " " + text + strings.Repeat(" ", pad) + " " + frame.Render("│")
	return []string{boxTopBorder("Search", width, frame), mid, boxBottomBorder("/", width, frame)}
}

// treeHeaderWideMin is the pane width from which the column header spells its
// labels out instead of abbreviating them to the single letters (bean bt-f68z,
// PO-Befund #12: "Master-Detail (schmal): Header auf die Keybindings kuerzen.
// Vollbild (v, breit): Header ausschreiben. Also breitenabhaengig, nicht zwei
// getrennte Implementierungen"). The split's left pane is ~30 cells even on a
// wide terminal (masterDetailWidths' 1fr:2fr, view.go), the Vollbild pane is
// the full inner width -- 48 separates the two cleanly at every terminal size
// the app supports, without a second code path.
const treeHeaderWideMin = 48

// treeColumnHeader renders the Tree/flat pane's column legend (bean bt-f68z,
// PO-Befund #12). One implementation, two label sets chosen by width.
//
// The NARROW form is column-aligned with the rows below it: one leading cell
// for the cursor column, two for the expand marker, then the 1-cell status and
// type glyph columns, the 4-cell short bean ID and the title. The WIDE form
// deliberately drops that alignment for the two glyph columns -- they are ONE
// cell wide by construction, so "Status" and "Type" cannot sit above them; it
// reads as a spelled-out legend instead, which is what "ausschreiben" asks
// for and what the extra Vollbild width makes room for.
func treeColumnHeader(width int) string {
	label := "   S T ID   Title"
	if width >= treeHeaderWideMin {
		label = "   Status Type ID   Title"
	}
	return lipgloss.NewStyle().Foreground(theme.Subtext).Render(truncate(label, width))
}

// treePaneHeadRows builds the rows that precede the bean rows in the Browse
// left pane: the boxed search head (3 lines, bean bt-f68z #11) followed by
// the column header (#12).
//
// This is the ONE place that decides how tall that head is. viewBrowseRepo
// trades its length out of the row budget and treeClickRow/flatClickRow offset
// their hit-test by exactly len(...) -- so a future head row can never desync
// the render from the mouse (the failure bt-vpvu was filed for: the filter bar
// was reclaimed from bodyH by the renderer but not by the hit-test).
func (m model) treePaneHeadRows(width int) []string {
	out := m.searchHeadBox(width)
	return append(out, treeColumnHeader(width))
}

// repoLabel is the breadcrumb `> repo` segment: the repo directory's base
// name (design-spec.md §7's "> repo: Titel" format; port-adaptation vs.
// devd's project slug, view.go's breadcrumb doc comment).
func (m model) repoLabel() string {
	if m.repoDir == "" {
		return "lif"
	}
	return filepath.Base(m.repoDir)
}

// composeOverlays layers every floating overlay onto out in a fixed
// z-priority order (painter's algorithm, placeOverlay) -- E3 Task 1
// extraction (bean bt-dlgk): viewBrowseRepo and viewBacklog used to
// duplicate the filterOpen/confirmQuit block; a new overlay is now wired in
// exactly ONE place instead of two. Order: filter menu first, then the
// node-action overlay (design decision a2 -- EXACTLY one overlayID active
// at a time), then huh forms (T4/T5, m.form != nil is its own separate
// capture state, not part of the overlayID enum), quit-confirm LAST (it can
// interrupt anything, same as its pre-extraction position in both views).
func (m model) composeOverlays(out string, w, h int) string {
	if m.filterOpen {
		out = placeOverlay(out, m.treeFilterBox(), w, h)
	}
	switch m.overlay {
	case overlayValueMenu:
		// Slice C (bt-f0y9, D09 revidiert): field-anchored below the
		// triggering Status/Type/Priority box (placeOverlayAt) --
		// placeValueMenuOverlay (box_menu_value.go) owns that placement, not
		// this switch.
		out = m.placeValueMenuOverlay(out, w, h)
	case overlayTagPicker:
		out = placeOverlay(out, m.tagPickerBox(), w, h)
	case overlayParentPicker:
		out = placeOverlay(out, m.parentPickerBox(), w, h)
	case overlayBlockingPicker:
		out = placeOverlay(out, m.blockingPickerBox(), w, h)
	case overlayCreateConfirm:
		out = placeOverlay(out, m.createConfirmBox(), w, h)
	case overlayDeleteConfirm:
		out = placeOverlay(out, m.deleteBox(), w, h)
	}
	if m.form != nil {
		out = placeOverlay(out, m.formChrome(), w, h)
	}
	// E4 Task 1 (bean bt-jpgn): the Command-Center -- a floating overlay
	// like the ones above, painted BEFORE m.confirmQuit (quit stays the
	// topmost layer, unchanged precedent).
	if m.paletteOpen {
		out = placeOverlay(out, m.paletteBox(), w, h)
	}
	// E5 Task 2 (bean bt-wpn9): the Help-Overlay -- painted AFTER the
	// Command-Center, BEFORE m.confirmQuit (quit stays the topmost layer
	// under the modals, unchanged precedent, Painter's-Algorithmus-
	// Reihenfolge: spät = oben).
	if m.helpOpen {
		out = placeOverlay(out, m.helpBox(), w, h)
	}
	// E10 Task 4 (bean bt-1lsu, D12/D15): the Tag-Management page's own
	// Delete-Confirm -- viewTagManagement is NOT routed through the generic
	// m.overlay enum (D06), so it cannot be just another `case` in the
	// switch above; a page-local bool gets its OWN composeOverlays branch
	// instead, exactly the precedent m.confirmQuit already set (D15's own
	// "exakt wie es bereits m.confirmQuit kennt" wording). Painted BEFORE
	// confirmQuit (quit stays the topmost layer, unchanged precedent) --
	// the two can never actually coincide in practice: `q` never reaches
	// requestQuit while viewTagManagement holds full capture (D06), so this
	// ordering is a defensive mirror of confirmQuit's own position, not a
	// load-bearing z-order decision.
	if m.tagMgmtDeleteConfirm {
		out = placeOverlay(out, m.tagMgmtDeleteConfirmBox(), w, h)
	}
	if m.confirmQuit {
		out = placeOverlay(out, m.quitBox(), w, h)
	}
	return out
}

// View dispatches on viewID (devd port convention: enum + switch in view).
// E2 Task 5 (bean bt-gzu6) adds the first sibling case (viewBacklog); later
// epics grow this switch further, never branches.
//
// E5 Task 1 (bean bt-6dts, design decision a, point 2): every sub-view's
// result is wrapped in m.renderToast(...) HERE, at the top level -- NOT
// inside composeOverlays (called by each of the three sub-views above).
// This is beans-tui's own deviation from devd's View()/viewComposite()
// split (there is no separate viewComposite() layer here, each sub-view
// already ends in its own `return m.composeOverlays(out, w, h)`) -- the
// Toast must float over composeOverlays' ENTIRE stack, including
// confirmQuit (devd's own "über ALLEM" contract, overlay_show_toast.go),
// so it cannot be just another composeOverlays case.
func (m model) View() string {
	var out string
	switch m.view {
	case viewBacklog:
		out = m.viewBacklog()
	case viewLobby:
		// E5 Task 6 (bean bt-zhwl): the Lobby/Repo-Picker -- a full
		// top-level view (view_lobby.go), not an overlay.
		out = m.viewLobby()
	case viewTagManagement:
		// E10 Task 2 (bean bt-r92i): the Tag-Management page -- a full
		// top-level view (view_tag_management.go), not an overlay.
		out = m.viewTagManagement()
	default:
		out = m.viewBrowseRepo()
	}
	return m.renderToast(out)
}

// browseRepoChrome builds viewBrowseRepo's own head/localKeys strings --
// factored out (E5 Task 4, bean bt-mne6) so treeClickRow (mouse.go) can
// reconstruct this view's IDENTICAL geometry via clickPaneGeometry without a
// second, independently maintained copy of this breadcrumb/footer
// construction (Golden-Rule-Drift-Schutz: one source instead of two that
// could drift apart, mirrors windowStart's own shared-geometry rationale).
func (m model) browseRepoChrome(innerW int) (head, localKeys string) {
	head = breadcrumb(m.repoLabel(), "Browse", renderBindings(globalBindings()), innerW)
	localKeys = footer(m.contextualLocalHint(browseRepoLocalBindings()), innerW)
	return
}

// browseRepoLocalBindings is the Tree view's own Footer Zone 3 local set --
// D06+Q06 (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc) REBUILD this from
// scratch, superseding PF-11's list: Navigation (Up/Down/Left/Right) is
// removed entirely ("intuitiv genug", PO-Begruendung), FocusIn/FocusOut come
// FIRST, then the PO-verbatim action order -- `tab focus in · shift+tab
// focus out · / search · f Filter · s Status · c Create · d Delete · e Edit
// · b Backlog · t Tags · y Yank · a Parent · r Blocking`. Backlog (`b`) is
// NEW here (a sensible Tree->Backlog entry point that PF-11's list never
// carried); Filter/Tags/Yank/Parent/Blocking close the pre-existing gap
// PF-11's own doc comment flagged (f/X/b/t/a/B/y were handled by
// keyNodeAction/keyTree but never shown in the footer). X (FilterClear)
// stays deliberately OUT (Q06's list omits it too, same as before).
//
// bt-fy5d (Nebenbefund N2, epic bt-vy1q): the Detail pane renders each
// scalar field as a titled box carrying its own hotkey badge in the bottom
// border ((e) (s) (o) (u) (a) (t), box_detail_form.go) -- the footer repeated
// all of them, which at 80 columns cost a whole third footer line for
// information already on screen. Those keys are therefore dropped from the
// footer DISPLAY. This is display-only: the bindings stay registered in
// keymap.go (single source) and keep their Help-overlay entry, so
// TestHelpGroupsCoverEveryBindingExactlyOnce's drift guard is untouched.
func browseRepoLocalBindings() []keybind.Binding {
	bs := []keybind.Binding{
		keys.FocusIn, keys.FocusOut,
		keys.Search, keys.Filter, keys.Status, keys.Create, keys.Delete, keys.Editor,
		keys.Backlog, keys.TagAssign, keys.Yank, keys.Assign, keys.Blocking,
	}
	out := bs[:0:0]
	for _, b := range bs {
		if boxFormInlineKeys[b.Help().Key] {
			continue
		}
		out = append(out, b)
	}
	// keys.Fullscreen (bean bt-oox1, PO finding #10) and keys.DetailToggle
	// are advertised here: the Help overlay was otherwise the only place
	// they appeared.
	return append(out, keys.Fullscreen, keys.DetailToggle)
}

// boxFormInlineKeys are the footer keys the box-form Detail pane already
// shows itself (bt-fy5d). e/s/a/t are literal (x) badges rendered by
// detailBoxForm's Title/Status/Parent/Tags boxes and panelBox("Body") --
// o (Type) and u (Priority) are badges too but were never in the footer set
// to begin with.
//
// The membership rule is exactly "this key is rendered as an inline (x)
// badge", nothing looser. bean bt-6nuz (PO finding #6) removed `r`
// (Blocking), which bt-fy5d had admitted on the weaker ground that its
// SUBJECT -- the Relations panel -- is on screen. But a panel is not a
// badge: the Relations panel carries no (r), so hiding `r` from the footer
// left the key advertised nowhere at all while it stayed fully live. Since
// a footer entry is the only discovery surface a badge-less key has,
// dropping one is a net loss of information rather than a de-duplication.
// TestBoxFormInlineKeysAllHaveAnInlineBadge (footer_boxform_test.go) now
// holds this rule structurally against the real detailBoxForm render.
var boxFormInlineKeys = map[string]bool{"e": true, "s": true, "a": true, "t": true}

// viewBrowseRepo renders the two-pane master-detail Browse view. Mirrors
// Chrome()'s own algebra exactly (view.go) so the frame always fills
// width x height, just with a two-pane body instead of Chrome's single
// scroll body -- Golden Rule #1 still applies (no Height() on a bordered
// style; renderPane pads/caps to its h param instead).
func (m model) viewBrowseRepo() string {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2

	head, localKeys := m.browseRepoChrome(innerW)

	div := theme.Dim.Render(strings.Repeat("─", innerW))
	// I04 (T8 Opus quality review): a failed data.Watch start (no live
	// reload) must never degrade silently -- it goes into the same
	// indicator slot Chrome() uses for the scroll position (Accent, non-
	// critical). bt-81f0 (Notifications vereinheitlichen, Q1-Annahme):
	// m.err lost this slot entirely -- Toast is the ONE visible channel for
	// real load failures now, the status line carries ONLY this indicator.
	status := m.statusLine(innerW)

	// E5 Task 4 (bean bt-mne6): bodyH/lw/rw now come from the SAME
	// clickPaneGeometry helper treeClickRow (below) uses to map a click back
	// to a row -- single source for the numeric pane geometry, not just the
	// head/localKeys strings above (Golden-Rule-Drift-Schutz).
	bodyH, lw, rw, _, _ := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	nodes := m.visibleNodes()

	// S3 (jira-style-experiment): the persistent filter chip row
	// (box_filter_bar.go, design-spec.md D02/D08) sits between the header
	// divider and the Tree/Detail split. filterBarRow's height is reclaimed
	// from bodyH BEFORE the split renders, so the outer frame's total line
	// count still matches clickPaneGeometry's own avail budget (Golden Rule
	// #1: no forced Height() -- the frame grows exactly to its content, so
	// adding a row without reclaiming its height would grow the frame past
	// the terminal).
	filterBarRow := m.filterBar(innerW)
	bodyH -= lipgloss.Height(filterBarRow)
	if bodyH < 1 {
		bodyH = 1
	}

	var body string
	if m.fullscreen != fullscreenNone {
		// F01 (design-spec.md §15, E9 Task 7, bean bt-13l7): Vollbild --
		// checked BEFORE the Split's JoinHorizontal build below, a NEW
		// render branch entirely (the Split path stays byte-for-byte
		// unchanged for the golden fixtures, Basis-Goldens-Gegenbeleg).
		// paneW is the SINGLE pane's own content-width param passed to
		// renderPane/renderDetailBox (mirrors masterDetailWidths' lw/rw
		// contract, view.go: EVERY renderPane/renderDetailBox caller's
		// `w` is a CONTENT width, its OWN RoundedBorder adds +2 more --
		// masterDetailWidths' lw+rw+4==innerW is exactly this same
		// accounting for the TWO Split panes; here there is only ONE pane,
		// so its content width is innerW-2, not innerW itself (bean
		// bt-13l7's "innerW direkt als Pane-Breite" reads as the pane's
		// TOTAL occupied width, i.e. what masterDetailWidths' RESULT already
		// represents, not renderPane's raw `w` argument -- passing innerW
		// unadjusted double-counts the border and overflows the outer frame
		// by 2 columns, caught live via tmux smoke). bodyH is still the
		// correct height source (unverändert wiederverwendet).
		paneW := innerW - 2
		var listRows []string
		if m.fullscreen == fullscreenList {
			headRows := m.treePaneHeadRows(paneW - 2) // search head + column header (bean bt-f68z #11/#12)
			// B8 (bean bt-s90e): the Vollbild-Liste sources its rows from the
			// SAME m.flatView switch the split's own left pane does (the
			// `else if m.flatView` branch below) -- this branch used to call
			// treeRows() unconditionally, so `v` after `G` silently put the
			// PO back in the Tree they had just toggled away from. Only the
			// row SOURCE differs; the search head line, the bodyH-1 budget
			// trade and the single-pane geometry are shared verbatim.
			// Dispatch already handled flat mode here (keyTree routes to
			// keyFlat, update.go; focusedBean resolves via flatSelected) --
			// the render was the only half missing. m.flatView's default
			// false leaves the Tree path untouched.
			if m.flatView {
				listRows = append(headRows, m.flatRows(m.flatVisible(), true, bodyH-len(headRows), paneW-2)...)
			} else {
				listRows = append(headRows, m.treeRows(nodes, true, bodyH-len(headRows), paneW-2)...)
			}
		}
		var detailBean *item.Item
		if m.fullscreen == fullscreenDetail {
			detailBean = m.focusedBean() // resolves via focusedBean's own fullscreenDetail case (update.go)
		}
		body = renderFullscreenBody(m.fullscreen, paneW, bodyH, listRows, true, m.idx, detailBean, boxFormEffectiveScroll(m, detailBean))
	} else if m.flatView {
		// S5 (jira-style-ui experiment, Nested/Flat Browse toggle `G`,
		// view_browse_flat.go): the LEFT pane renders the flat, sorted bean
		// list instead of the Tree -- everything else (search head row
		// budget trade, Detail pane via the SAME shared renderBeanDetailBox,
		// master-detail Split) mirrors the Tree branch below verbatim, just
		// sourced from flatVisible()/flatRows()/renderFlatDetailPane instead
		// of nodes/treeRows()/renderDetailPane. Checked BEFORE the Tree
		// branch so m.flatView's default false leaves that branch, and every
		// pre-existing golden depending on it, byte-for-byte untouched.
		vis := m.flatVisible()
		headRows := m.treePaneHeadRows(lw - 2) // D02 precedent: flat mode has no Sort-Toggle (unlike Backlog), no suffix
		flatRowsWithHead := append(headRows, m.flatRows(vis, !m.detailFocus, bodyH-len(headRows), lw-2)...)
		flatBox := renderPane(pane{rows: flatRowsWithHead}, lw, bodyH, !m.detailFocus)
		body = flatBox
		if rw > 0 {
			detailBox := m.renderFlatDetailPane(vis, rw, bodyH, m.detailFocus)
			body = lipgloss.JoinHorizontal(lipgloss.Top, flatBox, detailBox)
		}
	} else {
		// E2 Task 3 (bean bt-4ep2): the search head row is prepended to the Tree
		// pane's rows, costing 1 line of its bodyH content budget -- the actual
		// tree rows window to bodyH-1 (PF-10, bean bt-uyzf, widened from bodyH-3
		// now that renderPane no longer reserves its own title+separator lines)
		// so the combined [searchLine, ...treeRows] slice still fits renderPane's
		// own Golden-Rule-#1 line cap.
		headRows := m.treePaneHeadRows(lw - 2) // D02: Tree never shows a sort suffix
		treeRowsWithHead := append(headRows, m.treeRows(nodes, !m.detailFocus, bodyH-len(headRows), lw-2)...)
		treeBox := renderPane(pane{rows: treeRowsWithHead}, lw, bodyH, !m.detailFocus)
		body = treeBox
		if rw > 0 {
			detailBox := m.renderDetailPane(nodes, rw, bodyH, m.detailFocus)
			body = lipgloss.JoinHorizontal(lipgloss.Top, treeBox, detailBox)
		}
	}

	content := head + "\n" + div + "\n" + filterBarRow
	content = appendStatusLine(content+"\n"+body+"\n"+div+"\n"+localKeys, status)
	out := outerBorder(content, innerW, true)

	return m.composeOverlays(out, w, h)
}

// treeClickRow maps a mouse click to a Tree node index (E5 Task 4, bean
// bt-mne6, design decision f; caller: mouse.go's mouseTreeClick) --
// bodyH/lw/originX/originY are RECONSTRUCTED identically to viewBrowseRepo's
// own render formula (above) via the shared browseRepoChrome +
// clickPaneGeometry helpers (Golden-Rule-Drift-Schutz, Kommentar-Pflicht
// analog windowStart's own doc comment): if viewBrowseRepo's algebra ever
// changes, it changes HERE too automatically (single source), so a click can
// never silently land on the wrong row through independent drift. Row 0
// (clickRow==0, right below the pane's own top border -- PF-10, bean bt-uyzf,
// removed the title+separator lines that used to sit there) is the search
// head line (treeSearchLine) -- never a node target. Row 1+ maps via
// windowStart(len(nodes), bodyH-1, cursorPos) + (clickRow-1), the SAME
// bodyH-1 window height treeRows itself windows to (treeRowsWithHead's own
// budget trade, above). ok=false for a click outside the Tree pane's column
// span, on/above the search line, or past the last actually-rendered row.
func treeClickRow(m model, nodes []treeNode, msg tea.MouseMsg) (idx int, ok bool) {
	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	innerW := w - 2
	head, localKeys := m.browseRepoChrome(innerW)

	bodyH, lw, _, originX, originY := clickPaneGeometry(w, h, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.detailHidden)
	// B6 (S6, jira-style-ui experiment): the persistent
	// filter bar (box_filter_bar.go) sits between the header divider and
	// the Tree/Detail split (viewBrowseRepo, above) -- it reclaims its 3
	// rows from bodyH BEFORE the split renders, but clickPaneGeometry's own
	// originY has no knowledge of it (that helper is shared with Backlog,
	// which never shows the bar). treeClickRow is BROWSE-only
	// (mouseTreeClick's own call site, mouse.go).
	//
	// bt-vpvu ROOT CAUSE: the originY correction below landed in S6, but
	// the matching bodyH correction did NOT -- the renderer reclaims the
	// bar's 3 rows from bodyH before windowing, this hit-test kept
	// windowing against the UNREDUCED bodyH. Both halves computed a
	// different window start, so every click landed on the wrong bean the
	// moment the list was scrolled far enough for the two starts to
	// diverge.
	originY += filterBarHeight
	bodyH -= filterBarHeight
	if bodyH < 1 {
		bodyH = 1
	}

	if msg.X < originX || msg.X >= originX+lw {
		return 0, false // right Detail pane, or off-screen -- no Tree target
	}
	// The head rows (search head + column header, bean bt-f68z) are never a
	// node target -- and their COUNT is now variable (the boxed search head is
	// 3 lines, the plain one 1), so it is read from the one function that
	// builds them rather than assumed to be 1.
	headRows := m.treePaneHeadRows(lw - 2)
	clickRow := msg.Y - originY - len(headRows)
	if clickRow < 0 {
		return 0, false // above the pane, or on the search head / column header
	}

	// Rows are no longer 1:1 with beans (hanging-indent title wrapping, bean
	// bt-f68z), so the clicked LINE is resolved through the very same
	// blockWindow the renderer used -- not by dividing by one.
	_, rowElem := blockWindow(m.treeBlocks(nodes, !m.detailFocus, lw-2), bodyH-len(headRows), m.cursorPos(nodes))
	return blockWindowElemAt(rowElem, clickRow)
}
