package tui

// view_browse_repo_test.go — TDD coverage for viewBrowseRepo's own Chrome
// (browseRepoChrome). Originally PF-11's Header/Footer-Keybinding-Split
// (design-spec.md §15, epic-E7-plan.md Task 7, bean bt-m6at); D04/D06/Q06
// (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc) SUPERSEDE that: the
// Header now carries EXACTLY 4 globals (Palette/Picker/Help/Quit), the
// Footer drops Navigation entirely and renders Q06's PO-verbatim list
// (FocusIn/FocusOut first, then actions), and every hint is Teal-key/
// Subtext-desc with no ':' (D06 optic, renderBindings). Still swaps to a
// context's own bindings while a Filter-Menu/Overlay/Search/Palette/Help is
// open (Q04-Antwort, unchanged).

import (
	"fmt"
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// TestBrowseRepoChromeHeaderShowsExactlyFourGlobals guards D04 (design-
// spec.md §15 PF-16, bean bt-ntoz/bt-d8kc, SUPERSEDES the previous 7-Globals
// header): the Header now carries EXACTLY Palette/Picker/Help/Quit, no
// colon (D06 optic) -- ctrl+r/esc/enter are gone.
func TestBrowseRepoChromeHeaderShowsExactlyFourGlobals(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	head, _ := m.browseRepoChrome(200) // wide enough to never trigger breadcrumb's narrow-stack fallback
	plain := stripHint(head)
	for _, want := range []string{"K commands", "p repos", "? help", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Errorf("browseRepoChrome header = %q, want it to contain %q", plain, want)
		}
	}
	for _, absent := range []string{"ctrl+r", "esc back", "enter open"} {
		if strings.Contains(plain, absent) {
			t.Errorf("browseRepoChrome header = %q, must NOT contain %q (D04 degrades it out of the header)", plain, absent)
		}
	}
}

func TestBrowseRepoChromeFooterOmitsRefreshAndEnter(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	_, localKeys := m.browseRepoChrome(200)
	plain := stripHint(localKeys)
	if strings.Contains(plain, "ctrl+r") {
		t.Errorf("browseRepoChrome footer = %q, must NOT contain ctrl+r (now header-only, D04)", plain)
	}
	if strings.Contains(plain, "enter open") {
		t.Errorf("browseRepoChrome footer = %q, must NOT contain enter open (now header-only, D04)", plain)
	}
}

func TestBrowseRepoChromeFooterShowsFocusInFocusOut(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	_, localKeys := m.browseRepoChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "tab focus in") {
		t.Errorf("browseRepoChrome footer = %q, want FocusIn's real hint (not a hand-typed suffix)", plain)
	}
	if !strings.Contains(plain, "shift+tab focus out") {
		t.Errorf("browseRepoChrome footer = %q, want shift+tab visible for the first time (PF-13)", plain)
	}
}

// TestBrowseRepoChromeFooterMatchesQ06List guards Q06's finale, PO-verbatim
// Footer-Liste (design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc): browseRepoLocalBindings()
// renders EXACTLY "tab focus in · shift+tab focus out · / search · f Filter
// · s Status · c Create · d Delete · e Edit · b Backlog · t Tags · y Yank ·
// a Parent · r Relations" -- Navigation-Keys (Up/Down/Left/Right) are
// entirely gone.
func TestBrowseRepoChromeFooterMatchesQ06List(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	_, localKeys := m.browseRepoChrome(500) // wide enough for the whole list on one line
	plain := stripHint(localKeys)
	want := "tab focus in · shift+tab focus out · / search · f Filter · s Status · c Create · d Delete · e Edit · b Backlog · t Tags · y Yank · a Parent · r Relations"
	if plain != want {
		t.Errorf("browseRepoChrome footer = %q, want exactly %q", plain, want)
	}
}

// TestBrowseRepoLocalBindingsOmitsNavigation guards Q06's explicit removal
// of Up/Down/Left/Right ("intuitiv genug", PO-Begruendung) from the Footer.
func TestBrowseRepoLocalBindingsOmitsNavigation(t *testing.T) {
	for _, nav := range []keybind.Binding{keys.Up, keys.Down, keys.Left, keys.Right} {
		for _, b := range browseRepoLocalBindings() {
			if strings.Join(b.Keys(), ",") == strings.Join(nav.Keys(), ",") {
				t.Errorf("browseRepoLocalBindings() still contains navigation binding %v, want it removed (Q06)", nav.Keys())
			}
		}
	}
}

func TestBrowseRepoChromeFooterIsContextSensitiveOnFilterOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.filterOpen = true
	_, localKeys := m.browseRepoChrome(200)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "space/x Toggle facet") {
		t.Errorf("browseRepoChrome footer while filterOpen = %q, want the Toggle hint (Q04)", plain)
	}
	if strings.Contains(plain, "c Create") {
		t.Errorf("browseRepoChrome footer while filterOpen = %q, must not leak the (now irrelevant) view-local Create hint", plain)
	}
}

// --- treeSearchLine sortSuffix (D02, design-spec.md §15 PF-16, bean
// bt-ntoz/bt-d8kc): the Backlog-Sort-Indicator. treeSearchLine gained a
// second `sortSuffix string` parameter -- Tree's own call site
// (viewBrowseRepo) always passes "" (No-Op guarantee below), Backlog's call
// site (viewBacklog) passes "sort "+backlogSortDisplayLabel(m.backlogSort).

// TestTreeSearchLineEmptySuffixUnchanged is the Tree-side No-Op guarantee:
// an empty sortSuffix must render BYTE-IDENTICAL to the pre-D02 idle hint
// (tree.golden's own search-line fixture depends on this).
func TestTreeSearchLineEmptySuffixUnchanged(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	got := m.treeSearchLine(40, "")
	want := truncate(theme.Muted.Render(searchShield+" / search"), 40)
	if got != want {
		t.Errorf("treeSearchLine(40, \"\") = %q, want %q (byte-identical idle hint)", got, want)
	}
}

// TestBacklogSortSuffixShowsAbbreviatedPriority guards the PO's own literal
// example (bean bt-ntoz Grilling-Nachtrag): idle Backlog search line with a
// non-empty sortSuffix renders "⌕ / search · sort prio" (ANSI-stripped).
func TestBacklogSortSuffixShowsAbbreviatedPriority(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	got := m.treeSearchLine(40, "sort "+backlogSortDisplayLabel("priority"))
	plain := ansi.Strip(got)
	want := "⌕ / search · sort prio"
	if plain != want {
		t.Errorf("treeSearchLine(40, \"sort prio\") = %q, want %q", plain, want)
	}
}

// TestTreeSearchLineSortSuffixAppendsInActiveSearchBranch guards that the
// sortSuffix is appended in the searchActive branch too (bean bt-d8kc: "in
// ALLEN DREI bestehenden Render-Zweigen"), not just the idle one.
func TestTreeSearchLineSortSuffixAppendsInActiveSearchBranch(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.searchActive = true
	got := m.treeSearchLine(120, "sort status") // wide enough: searchInput's own placeholder text is long
	plain := ansi.Strip(got)
	if !strings.HasSuffix(plain, "sort status") {
		t.Errorf("treeSearchLine (searchActive) = %q, want it to end with the sortSuffix", plain)
	}
}

// TestTreeSearchLineSortSuffixAppendsInTreeActiveBranch guards the same for
// the treeActive (committed search/facet) branch.
func TestTreeSearchLineSortSuffixAppendsInTreeActiveBranch(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "task")
	got := m.treeSearchLine(60, "sort created")
	plain := ansi.Strip(got)
	if !strings.HasSuffix(plain, "sort created") {
		t.Errorf("treeSearchLine (treeActive) = %q, want it to end with the sortSuffix", plain)
	}
}

// TestTreeNodeMarkerBlankForLeaf is B03's regression lock (bean bt-ntoz,
// design-spec.md §15 PF-16): kinderlose Beans dürfen NIE ein Expand-Dreieck
// zeigen -- nur gleich breiten Leerraum. ERRATUM (bean bt-e6q9, B03): der
// PO-gemeldete Bug reproduziert NICHT gegen den aktuellen Code-Stand --
// treeNodeMarker prüft bereits `!n.hasKids` zuerst und gibt dann "  " (2
// Leerzeichen, dieselbe Breite wie "▾ "/"▸ ") zurück. Kein Code-Fix, nur
// dieser Regressionstest, der das bereits-korrekte Verhalten festschreibt.
func TestTreeNodeMarkerBlankForLeaf(t *testing.T) {
	leaf := treeNode{id: "leaf-1", hasKids: false, open: false}
	if got := treeNodeMarker(leaf); got != "  " {
		t.Errorf("treeNodeMarker(leaf, hasKids=false) = %q, want \"  \" (blank, no expand triangle)", got)
	}
	if w := lipgloss.Width(treeNodeMarker(leaf)); w != lipgloss.Width("▾ ") {
		t.Errorf("treeNodeMarker(leaf) cell-width = %d, want %d (same width as the open/closed markers, no layout shift)", w, lipgloss.Width("▾ "))
	}

	// Regression guard for the branch cases too, so a future change can't
	// silently swap the hasKids-gate for something else without this test
	// catching it.
	openBranch := treeNode{id: "branch-open", hasKids: true, open: true}
	if got := treeNodeMarker(openBranch); got != "▾ " {
		t.Errorf("treeNodeMarker(hasKids=true, open=true) = %q, want \"▾ \"", got)
	}
	closedBranch := treeNode{id: "branch-closed", hasKids: true, open: false}
	if got := treeNodeMarker(closedBranch); got != "▸ " {
		t.Errorf("treeNodeMarker(hasKids=true, open=false) = %q, want \"▸ \"", got)
	}
}

// --- NB-2 (Reopen bt-b0w0, PO-Review Runde 3, US-05): RELATIONS-Accordion
// scrollt statt abzuschneiden ---
//
// windowRelationsSection is the new pure helper (view_browse_repo.go, next to
// renderAccordionPane) that composes windowStart (cursor-centered, the SAME
// convention treeRows already uses) with scrollView (Chrome/Lobby/Help's own
// ↑/↓ indicator, view.go) -- no new windowing algorithm, per the bean's own
// Planner-Konkretisierung. Tested in isolation first (pure function, no
// model/idx needed), then via renderAccordionPane end-to-end (the real
// beanSections/relationsSectionBody/renderAccordion wiring).

// fixtureBeanWithManyChildren builds a parent epic with n children (mirrors
// the smoke fixture bt-apmy, 11 Children, at unit-test scale) -- SortBeans
// (status/priority/type/title, all equal here) leaves them ordered by Title,
// so "Child 00".."Child NN" sorts numerically ascending: fieldCursor i always
// addresses child i directly, no reshuffling to account for in assertions.
func fixtureBeanWithManyChildren(n int) (parentID string, all []data.Bean) {
	parentID = "ep-many"
	all = append(all, data.Bean{ID: parentID, Title: "Epic With Many Children", Status: "todo", Type: "epic", Priority: "normal"})
	for i := 0; i < n; i++ {
		all = append(all, data.Bean{
			ID:       fmt.Sprintf("ch-%02d", i),
			Title:    fmt.Sprintf("Child %02d", i),
			Status:   "todo",
			Type:     "task",
			Priority: "normal",
			Parent:   parentID,
		})
	}
	return parentID, all
}

// TestWindowRelationsSectionNoopWhenBodyFitsBudget guards the "kein Golden-
// Bruch bei wenigen Relations" Akzeptanzkriterium at the pure-function level:
// a body shorter than the pane's own budget must come back byte-identical,
// no indicator appended.
func TestWindowRelationsSectionNoopWhenBodyFitsBudget(t *testing.T) {
	body := "line0\nline1\nline2"
	h := 20 // avail = 20 - 5 (headerBlock) - 4 (beanSectionCount headers) = 11 >> 3
	if got := windowRelationsSection(body, h, 0); got != body {
		t.Fatalf("windowRelationsSection must be a no-op when body fits the budget,\ngot:\n%s\nwant (unchanged):\n%s", got, body)
	}
}

// TestWindowRelationsSectionWindowsAroundActiveLine is the RED-anchor for the
// actual windowing: 20 synthetic rows, activeLine=15 passed in numerically
// (bt-se4q, bt-b0w0-Review Follow-up B01 -- windowRelationsSection no longer
// derives the cursor by rescanning `body` for a "▶" glyph, the caller hands
// it the exact display-line index instead) -- the window must contain that
// row PLUS a trailing "L n–m/total" indicator line (scrollView's own
// format), never just the first h rows (the old blunt renderPane line-cap
// this bean's Root-Cause section describes).
func TestWindowRelationsSectionWindowsAroundActiveLine(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("row-%02d", i))
	}
	body := strings.Join(lines, "\n")
	h := 15 // avail = 15-5-4 = 6, winH = 5 (1 reserved for the indicator)

	got := windowRelationsSection(body, h, 15)
	if !strings.Contains(got, "row-15") {
		t.Fatalf("window must contain the active row (row-15), got:\n%s", got)
	}
	gotLines := strings.Split(got, "\n")
	if len(gotLines) != 6 {
		t.Fatalf("windowed output has %d lines, want 6 (5-row window + 1 indicator line)", len(gotLines))
	}
	ind := ansi.Strip(gotLines[len(gotLines)-1])
	if !strings.Contains(ind, "/20") {
		t.Fatalf("last line must be a more-entries indicator mentioning the total (20), got %q", ind)
	}
}

// TestWindowRelationsSectionIgnoresGlyphInsideUnrelatedLine guards the same
// bt-se4q contract one layer up: a body line UNRELATED to the real cursor
// happens to contain a literal "▶" character (e.g. a relation title with
// that glyph) at an EARLIER index than the numeric activeLine passed in --
// the window must still center on activeLine, never on the earlier glyph
// occurrence (the exact class of bug the removed activeRelationLine's
// strings.Contains(l, "▶") rescan was exposed to).
func TestWindowRelationsSectionIgnoresGlyphInsideUnrelatedLine(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		text := fmt.Sprintf("row-%02d", i)
		if i == 2 {
			text = "row-02 ▶ glyph inside an unrelated title"
		}
		lines = append(lines, text)
	}
	body := strings.Join(lines, "\n")
	h := 15 // avail = 15-5-4 = 6, winH = 5

	got := windowRelationsSection(body, h, 15) // real cursor is row-15, NOT row-02's embedded glyph
	if !strings.Contains(got, "row-15") {
		t.Fatalf("window must center on the passed activeLine (row-15), got:\n%s", got)
	}
	if strings.Contains(got, "row-02") {
		t.Fatalf("window must NOT center on the earlier glyph-in-title line (row-02), got:\n%s", got)
	}
}

// TestWindowRelationsSectionDefaultsTopWhenNoActiveMarker covers the
// accOpen-persists-without-focus case (Prelude doc note, design-spec.md
// §15 PF-18): RELATIONS can be open with NO row active (focus elsewhere) --
// there is no cursor to center on, so the window must deterministically
// default to the top instead of an arbitrary/stale position. activeLine=0
// is relationsSectionBody's own default in that case (view_detail_bean.go).
func TestWindowRelationsSectionDefaultsTopWhenNoActiveMarker(t *testing.T) {
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("row-%02d", i))
	}
	body := strings.Join(lines, "\n")
	got := windowRelationsSection(body, 15, 0)
	gotLines := strings.Split(got, "\n")
	if !strings.Contains(gotLines[0], "row-00") {
		t.Fatalf("with no active marker, window must default to the top (row-00 first), first line got %q", gotLines[0])
	}
}

// TestRenderAccordionPaneRelationsKeepsSelectedChildVisibleAcrossAllPositions
// is the end-to-end acceptance guard (Akzeptanzkriterium 1+2: Fenster um den
// selektierten Eintrag, Pfeiltasten-Auto-Scroll) -- run through the REAL
// beanSections/relationsSectionBody/renderAccordion/renderPane pipeline (not
// just the isolated helper), for EVERY reachable fieldCursor position (0..11,
// every position `up`/`down` navigation could ever park on), not just a
// hand-picked sample.
func TestRenderAccordionPaneRelationsKeepsSelectedChildVisibleAcrossAllPositions(t *testing.T) {
	const n = 12
	_, all := fixtureBeanWithManyChildren(n)
	m := fixtureModel(t, all)
	b := m.idx.ByID["ep-many"]
	w, h := 60, 15 // avail = 15-5-4 = 6, winH = 5 -- forces windowing (12 > 5)

	for i := 0; i < n; i++ {
		out := renderAccordionPane(m.idx, b, w, h, relationsSectionIdx+1, relationsSectionIdx, i, 1, true, 0, -1)
		plain := ansi.Strip(out)
		wantID := fmt.Sprintf("ch-%02d", i)
		if !strings.Contains(plain, wantID) {
			t.Fatalf("fieldCursor=%d: rendered RELATIONS pane must keep the selected child (%s) visible, got:\n%s", i, wantID, plain)
		}
	}
}

// TestRenderAccordionPaneRelationsShowsMoreEntriesIndicatorWhenOverflowing
// guards Akzeptanzkriterium 3 (sichtbarer Mehr-Einträge-Hinweis).
func TestRenderAccordionPaneRelationsShowsMoreEntriesIndicatorWhenOverflowing(t *testing.T) {
	const n = 12
	_, all := fixtureBeanWithManyChildren(n)
	m := fixtureModel(t, all)
	b := m.idx.ByID["ep-many"]

	out := renderAccordionPane(m.idx, b, 60, 15, relationsSectionIdx+1, relationsSectionIdx, 0, 1, true, 0, -1)
	plain := ansi.Strip(out)
	// scrollView's indicator format is "L n–m/total" (view.go) -- total here
	// counts BODY LINES (the "Children" subheader plus its 12 rows == 13),
	// not raw relation-entry count, an implementation detail the
	// Akzeptanzkriterium itself doesn't pin down ("sichtbarer Hinweis... dass
	// mehr Einträge liegen") -- assert the indicator's shape (L-prefix + more-
	// below arrow), not a specific number that would need updating the
	// moment the body composition (e.g. a future Parent group) changes.
	if !strings.Contains(plain, "L ") || !strings.Contains(plain, "↓") {
		t.Fatalf("overflowing RELATIONS must show a more-entries indicator (\"L n–m/total\" + ↓), got:\n%s", plain)
	}
}

// TestRenderAccordionPaneRelationsFewEntriesUnchangedByWindowing guards
// Akzeptanzkriterium 4 (kein Golden-Bruch bei wenigen Relations): 2 children
// fit well inside the pane's budget -- BOTH must render simultaneously, no
// windowing, no more-entries indicator, byte-for-byte the pre-NB-2 shape.
func TestRenderAccordionPaneRelationsFewEntriesUnchangedByWindowing(t *testing.T) {
	_, all := fixtureBeanWithManyChildren(2)
	m := fixtureModel(t, all)
	b := m.idx.ByID["ep-many"]

	out := renderAccordionPane(m.idx, b, 60, 15, relationsSectionIdx+1, relationsSectionIdx, 0, 1, true, 0, -1)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "ch-00") || !strings.Contains(plain, "ch-01") {
		t.Fatalf("few relations (2, under the pane's budget) must show ALL of them simultaneously (no windowing), got:\n%s", plain)
	}
	if strings.Contains(plain, "/2") {
		t.Fatalf("few relations must NOT show a more-entries indicator, got:\n%s", plain)
	}
}

// TestRenderFullscreenBodyRelationsWindowsSameAsSplitPane guards against the
// Fullscreen-drift risk this bean's Prelude/Notes flagged explicitly:
// renderFullscreenBody (view_fullscreen.go) calls the SAME renderAccordionPane
// as the Split pane, no separate scroll path -- if a future change ever forks
// that, this breaks.
func TestRenderFullscreenBodyRelationsWindowsSameAsSplitPane(t *testing.T) {
	const n = 12
	_, all := fixtureBeanWithManyChildren(n)
	m := fixtureModel(t, all)
	b := m.idx.ByID["ep-many"]

	out := renderFullscreenBody(fullscreenDetail, 60, 15, nil, true, m.idx, b, relationsSectionIdx, relationsSectionIdx+1, 7, 1, 0)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "ch-07") {
		t.Fatalf("Vollbild-Detail must window RELATIONS the SAME way the Split pane does -- fieldCursor=7 must stay visible, got:\n%s", plain)
	}
}
