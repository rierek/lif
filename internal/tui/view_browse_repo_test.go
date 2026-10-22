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
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	want := "tab focus in · shift+tab focus out · / search · f Filter · c Create · d Delete · b Backlog · y Yank · r Relations · v fullscreen · D toggle detail"
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
