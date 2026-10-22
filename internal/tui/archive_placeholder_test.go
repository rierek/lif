package tui

// archive_placeholder_test.go — TDD coverage for D01 (bean bt-39cl, closes
// the Investigation 2026-07-16 root cause): when an expanded Epic's Direct-
// Children are ALL hidden by the archive default (m.showArchived == false),
// filteredBeanNode (view_browse_repo.go) now renders a synthetic, non-
// selectable placeholder row ("N archiviert — f→Archive") in the child
// indent instead of silently rendering zero rows below a marker that still
// claims ▾ (hasKids=true). Reuses fixtureModel/step/keyMsg/nodeIDs
// (update_test.go, same package) and mirrors archive_visibility_test.go's
// own fixture style.

import (
	"github.com/rierek/lif/pkg/item"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// placeholderFixtureBeans builds the exact repro shape from the bean's own
// Investigation (repro-39cl): one Epic whose 3 Direct-Children are ALL
// archived (2x completed, 1x scrapped), plus one open sibling Epic with NO
// children at all (D01 acceptance box's own "Epic ohne Children" control).
func placeholderFixtureBeans() []*item.Item {
	return []*item.Item{
		{ID: "ep-full", Title: "Epic Full Archive", Status: "in-progress", Type: "epic", Priority: "normal"},
		{ID: "ch-1", Title: "Child One", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-full"},
		{ID: "ch-2", Title: "Child Two", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-full"},
		{ID: "ch-3", Title: "Child Three", Status: "scrapped", Type: "task", Priority: "normal", Parent: "ep-full"},
		{ID: "ep-empty", Title: "Epic No Children", Status: "in-progress", Type: "epic", Priority: "normal"},
	}
}

// mixFixtureBeans is the D01 acceptance box's "Mix" case: 1 open + 2
// archived Direct-Children -- the open child must stay visible, NO
// placeholder (only the Voll-Verdeckungs-Fall gets one).
func mixFixtureBeans() []*item.Item {
	return []*item.Item{
		{ID: "ep-mix", Title: "Epic Mix", Status: "in-progress", Type: "epic", Priority: "normal"},
		{ID: "ch-open", Title: "Child Open", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-mix"},
		{ID: "ch-done1", Title: "Child Done One", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-mix"},
		{ID: "ch-done2", Title: "Child Done Two", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-mix"},
	}
}

// findNode locates the row with the given id (empty id never matches --
// placeholders are id-less by design, use findPlaceholder below for those).
func findNode(nodes []treeNode, id string) (treeNode, bool) {
	for _, n := range nodes {
		if n.id == id {
			return n, true
		}
	}
	return treeNode{}, false
}

// findPlaceholder returns the first placeholder row directly following
// parentID's own row, or ok=false if none.
func findPlaceholder(nodes []treeNode, parentID string) (treeNode, bool) {
	for i, n := range nodes {
		if n.id == parentID && i+1 < len(nodes) && nodes[i+1].placeholder {
			return nodes[i+1], true
		}
	}
	return treeNode{}, false
}

// TestFilteredBeanNodePlaceholderCases is the D01 RED-anchor table test
// (Akzeptanz-Checkliste items 1-3): drives m.visibleNodes() (the real,
// fully-wired path filteredBeanNode/flattenTreeFiltered feed) across the
// four scenarios the bean's Akzeptanz box enumerates.
func TestFilteredBeanNodePlaceholderCases(t *testing.T) {
	t.Run("full obscuring renders placeholder with correct N", func(t *testing.T) {
		m := fixtureModel(t, placeholderFixtureBeans())
		m.expanded["ep-full"] = true

		nodes := m.visibleNodes()
		epic, ok := findNode(nodes, "ep-full")
		if !ok || !epic.hasKids || !epic.open {
			t.Fatalf("ep-full node = %+v, ok=%v -- want hasKids=true, open=true (marker stays ▾)", epic, ok)
		}
		ph, ok := findPlaceholder(nodes, "ep-full")
		if !ok {
			t.Fatalf("visibleNodes() = %v, want a placeholder row directly after ep-full", nodeIDs(nodes))
		}
		if ph.hiddenCount != 3 {
			t.Errorf("placeholder hiddenCount = %d, want 3 (all 3 Direct-Children archived)", ph.hiddenCount)
		}
		for _, childID := range []string{"ch-1", "ch-2", "ch-3"} {
			if _, present := findNode(nodes, childID); present {
				t.Errorf("archived child %s rendered as a real row, want it replaced by the placeholder", childID)
			}
		}
	})

	t.Run("mix shows open child, no placeholder", func(t *testing.T) {
		m := fixtureModel(t, mixFixtureBeans())
		m.expanded["ep-mix"] = true

		nodes := m.visibleNodes()
		if _, present := findNode(nodes, "ch-open"); !present {
			t.Fatalf("visibleNodes() = %v, want ch-open (the one non-archived child) visible", nodeIDs(nodes))
		}
		if _, ok := findPlaceholder(nodes, "ep-mix"); ok {
			t.Fatalf("visibleNodes() = %v, want NO placeholder in the Mix case (only Voll-Verdeckung gets one)", nodeIDs(nodes))
		}
		for _, n := range nodes {
			if n.placeholder {
				t.Fatalf("visibleNodes() contains a placeholder node %+v, want none anywhere in the Mix case", n)
			}
		}
	})

	t.Run("showArchived=true unchanged, no placeholder", func(t *testing.T) {
		m := fixtureModel(t, placeholderFixtureBeans())
		m.expanded["ep-full"] = true
		m.showArchived = true

		nodes := m.visibleNodes()
		for _, childID := range []string{"ch-1", "ch-2", "ch-3"} {
			if _, present := findNode(nodes, childID); !present {
				t.Errorf("visibleNodes() (showArchived=true) = %v, want real child row %s (not a placeholder)", nodeIDs(nodes), childID)
			}
		}
		if _, ok := findPlaceholder(nodes, "ep-full"); ok {
			t.Fatalf("visibleNodes() (showArchived=true) = %v, must NOT contain a placeholder", nodeIDs(nodes))
		}
	})

	t.Run("epic without children unaffected", func(t *testing.T) {
		m := fixtureModel(t, placeholderFixtureBeans())
		m.expanded["ep-empty"] = true // expanding a leaf is legal state, must stay a no-op visually

		nodes := m.visibleNodes()
		epic, ok := findNode(nodes, "ep-empty")
		if !ok {
			t.Fatalf("visibleNodes() = %v, want ep-empty present", nodeIDs(nodes))
		}
		if epic.hasKids {
			t.Errorf("ep-empty.hasKids = true, want false (no children at all -- marker must stay blank, B03)")
		}
		if _, ok := findPlaceholder(nodes, "ep-empty"); ok {
			t.Fatalf("visibleNodes() = %v, ep-empty must NOT get a placeholder (it has zero children, not archived ones)", nodeIDs(nodes))
		}
	})

	t.Run("search active scopes the placeholder out (archive-only guard)", func(t *testing.T) {
		// D01 Implementer-Vorgabe: "bei aktiver Suche/Facette gilt der
		// bestehende Pfad -- Platzhalter NUR für den Archiv-Filter-Fall".
		// A search query matching the epic itself (but none of its archived
		// children, irrelevant here since they're excluded by archive
		// anyway) switches treeActive() true -> archiveOnly=false -> the
		// pre-existing (placeholder-less) path applies unchanged.
		m := fixtureModel(t, placeholderFixtureBeans())
		m.expanded["ep-full"] = true
		m = setSearchQuery(m, "Epic Full Archive")

		nodes := m.visibleNodes()
		if _, ok := findPlaceholder(nodes, "ep-full"); ok {
			t.Fatalf("visibleNodes() (search active) = %v, must NOT render a placeholder (out of scope while search/facets narrow the tree)", nodeIDs(nodes))
		}
	})
}

// TestTreeRowTextPlaceholderPattern guards the exact D01 render pattern
// ("N archiviert — f→Archive", theme.Muted) at the pure-function level, plus
// the indent contract (depth-1 indent + a blank 2-char marker, same width
// TestTreeNodeMarkerBlankForLeaf already locks for leaves -- B03 precedent).
func TestTreeRowTextPlaceholderPattern(t *testing.T) {
	n := treeNode{depth: 1, placeholder: true, hiddenCount: 3}
	got := stripHint(treeRowText(n, ""))
	want := "  " + "  " + "3 archiviert — f→Archive" // depth(1)*"  " indent + blank marker "  "
	if got != want {
		t.Fatalf("treeRowText(placeholder, hiddenCount=3) = %q, want %q", got, want)
	}
}

// --- Cursor navigation over the placeholder (Implementer-Entscheidung:
// nicht selektierbar, Cursor überspringt) ---

// TestTreeCursorMoveSkipsPlaceholderDownward guards the down-arrow case:
// moving from the epic row must land PAST the placeholder onto the next
// real row, never ON the placeholder itself.
func TestTreeCursorMoveSkipsPlaceholderDownward(t *testing.T) {
	m := fixtureModel(t, placeholderFixtureBeans())
	m.expanded["ep-full"] = true
	m.cursorID = "ep-full"

	nodes := m.visibleNodes()
	if _, ok := findPlaceholder(nodes, "ep-full"); !ok {
		t.Fatal("setup: no placeholder after ep-full")
	}

	m2 := m.treeCursorMove(nodes, 1)
	if m2.cursorID == "" {
		t.Fatalf("cursorID = %q (landed on the id-less placeholder), want it skipped to a real row", m2.cursorID)
	}
	landed, ok := findNode(nodes, m2.cursorID)
	if !ok || landed.placeholder {
		t.Fatalf("cursor landed on %+v, want a non-placeholder row", landed)
	}
}

// TestTreeCursorMoveSkipsPlaceholderUpward guards the up-arrow case
// (opposite-direction fallback in skipPlaceholder when the placeholder sits
// as the LAST visible row -- ep-full is the only root, so its placeholder is
// the tree's final row).
func TestTreeCursorMoveSkipsPlaceholderUpward(t *testing.T) {
	beans := []*item.Item{
		{ID: "ep-full", Title: "Epic Full Archive", Status: "in-progress", Type: "epic", Priority: "normal"},
		{ID: "ch-1", Title: "Child One", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-full"},
	}
	m := fixtureModel(t, beans)
	m.expanded["ep-full"] = true

	nodes := m.visibleNodes()
	if len(nodes) != 2 || !nodes[1].placeholder {
		t.Fatalf("setup: nodes = %+v, want exactly [ep-full, placeholder]", nodes)
	}
	m.cursorID = nodes[1].id // can't legitimately happen via real navigation, but pins the defensive fallback directly

	m2 := m.treeCursorMove(nodes, 1) // delta=+1 (down) runs off the end -> must fall back UPWARD onto ep-full
	if m2.cursorID != "ep-full" {
		t.Fatalf("cursorID = %q, want ep-full (fallback scan when forward runs off the list end)", m2.cursorID)
	}
}

// --- Fix-Runde R1 (Review B01): applyLoaded cursor-restore must be
// placeholder-safe on BOTH paths ---

// TestApplyLoadedCursorRestoreSkipsPlaceholderOnEmptyCursorID is the
// reviewer's own repro (B01 path a): applyLoaded(empty) leaves
// m.cursorID == "" -- a follow-up applyLoaded(same beans) with the epic
// expanded must NOT "find" that empty cursorID on the id-less placeholder
// row (old code: cursorFound=true against n.id == "", cursor bar parked on
// the non-selectable row, Detail "(no selection)").
func TestApplyLoadedCursorRestoreSkipsPlaceholderOnEmptyCursorID(t *testing.T) {
	m := fixtureModel(t, placeholderFixtureBeans())
	m.expanded["ep-full"] = true

	m = step(t, m, beansLoadedMsg{beans: []*item.Item{}}) // -> cursorID = ""
	if m.cursorID != "" {
		t.Fatalf("setup: cursorID = %q after empty load, want \"\"", m.cursorID)
	}
	m = step(t, m, beansLoadedMsg{beans: placeholderFixtureBeans()})

	nodes := m.visibleNodes()
	pos := m.cursorPos(nodes)
	if nodes[pos].placeholder {
		t.Fatalf("cursor restored onto the placeholder row (pos %d), want a real row", pos)
	}
	if m.cursorID == "" {
		t.Fatal("cursorID still \"\" after reload with non-empty tree, want a real bean ID")
	}
}

// TestApplyLoadedOldPosFallbackSkipsPlaceholder covers B01 path b: the
// positional oldPos fallback (cursored bean vanished in the reload) must
// not land on a placeholder index in the NEW filtered tree. Old tree:
// [ep-full, tk-open] with cursor on tk-open (pos 1); reload removes
// tk-open and archives ep-full's remaining children -> new tree:
// [ep-full, placeholder] -- oldPos 1 hits the placeholder exactly.
func TestApplyLoadedOldPosFallbackSkipsPlaceholder(t *testing.T) {
	before := []*item.Item{
		{ID: "ep-full", Title: "Epic", Status: "in-progress", Type: "epic", Priority: "normal"},
		{ID: "tk-open", Title: "Open Task", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-full"},
	}
	after := []*item.Item{
		{ID: "ep-full", Title: "Epic", Status: "in-progress", Type: "epic", Priority: "normal"},
		{ID: "ch-1", Title: "Child One", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-full"},
	}
	m := fixtureModel(t, before)
	m.expanded["ep-full"] = true
	m.cursorID = "tk-open"

	m = step(t, m, beansLoadedMsg{beans: after})

	nodes := m.visibleNodes()
	if len(nodes) != 2 || !nodes[1].placeholder {
		t.Fatalf("setup: new tree = %+v, want exactly [ep-full, placeholder]", nodes)
	}
	if m.cursorID != "ep-full" {
		t.Fatalf("cursorID = %q, want ep-full (oldPos fallback slid off the placeholder)", m.cursorID)
	}
}

// TestKeyTreeExpandKeysNoOpOnPlaceholder locks the I01 explicit guards
// (keyTree right/left/enter): with the cursor position resolving to the
// placeholder row (cursorID "" + id-less placeholder, the exact pre-B01
// stale state), the expand keys must be pure no-ops -- no panic, no
// expanded-map mutation.
func TestKeyTreeExpandKeysNoOpOnPlaceholder(t *testing.T) {
	m := fixtureModel(t, placeholderFixtureBeans())
	m.expanded["ep-full"] = true
	m.cursorID = "" // cursorPos resolves "" onto the id-less placeholder row

	nodes := m.visibleNodes()
	if !nodes[m.cursorPos(nodes)].placeholder {
		t.Fatal("setup: cursorPos does not resolve to the placeholder row")
	}
	expandedBefore := len(m.expanded)

	for _, key := range []tea.KeyMsg{runeMsg('l'), runeMsg('j'), keyMsg(tea.KeyEnter)} {
		m2 := step(t, m, key)
		if len(m2.expanded) != expandedBefore {
			t.Fatalf("key %v on placeholder mutated m.expanded (%d -> %d), want no-op", key, expandedBefore, len(m2.expanded))
		}
	}
}

// TestMouseTreeClickOnPlaceholderIsNoOp guards the mouse-side counterpart of
// the "nicht selektierbar" decision (mouse.go mouseTreeClick): a real click
// (geometry-resolved via treeClickAt, mouse_test.go's own render-grounded
// helper) on the placeholder row's own rendered text must not move the
// cursor at all.
func TestMouseTreeClickOnPlaceholderIsNoOp(t *testing.T) {
	m := fixtureModel(t, placeholderFixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ep-full"] = true
	m.cursorID = "ep-full"

	msg := treeClickAt(t, m, "archiviert")
	tm, _ := m.handleMouse(msg)
	m2, ok := tm.(model)
	if !ok {
		t.Fatalf("handleMouse(click) did not return a model, got %T", tm)
	}
	if m2.cursorID != "ep-full" {
		t.Fatalf("click on the placeholder row: cursorID = %q, want unchanged (ep-full, No-Op)", m2.cursorID)
	}
}
