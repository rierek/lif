package tui

// view_browse_flat_test.go — jira-style-ui experiment, Slice 5 (S5): update-
// tests + golden coverage for the Nested/Flat Browse toggle (`G`,
// keymap.go/view_browse_flat.go). Mirrors tree_golden_test.go's/
// box_form_golden_test.go's fixture+harness pattern for the golden half.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestKeyGTogglesFlatView guards the basic toggle contract: `G` flips
// m.flatView, and the rendered LEFT pane switches from the Tree's indented/
// expand-marker rows to flatRows' un-indented, readyRowText-based rows.
func TestKeyGTogglesFlatView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	m.cursorID = "tk-1"
	if m.flatView {
		t.Fatal("setup: expected flatView false (default)")
	}

	before := ansi.Strip(m.View())

	m = step(t, m, runeMsg('G'))

	if !m.flatView {
		t.Fatal("flatView = false after G, want true")
	}

	after := ansi.Strip(m.View())

	if before == after {
		t.Fatal("View() unchanged after G toggle -- flat mode must render differently from the Tree")
	}
	// The Tree renders "tk-1" indented two levels deep with an expand
	// marker column ("    " for a depth-2 leaf, treeRowText); flat mode has
	// no hierarchy, so "tk-1"'s row starts right after the cursor bar/status
	// glyph with no such indent. Cheap, golden-independent signal: the
	// depth-2 indent ("      ", 3x"  ") that treeRowText would have produced
	// for tk-1 (Milestone->Epic->Task) must be gone from the flat render.
	if strings.Contains(after, "      "+"tk-1") {
		t.Error("flat render still shows Tree-style deep indentation before tk-1")
	}

	// Toggling back returns byte-identical to the original Tree render (a
	// pure, reversible mode switch -- Tree's own cursorID/expanded state was
	// never touched while flat was active).
	m = step(t, m, runeMsg('G'))
	if m.flatView {
		t.Fatal("flatView = true after second G, want false (toggle)")
	}
	if got := ansi.Strip(m.View()); got != before {
		t.Error("toggling G back to nested did not restore the original Tree render byte-for-byte")
	}
}

// TestKeyFlatNavMovesCursorAndSelection guards flat mode's own up/down
// cursor: it moves over flatVisible()'s bean slice (independent of the
// Tree's cursorID) and focusedBean() resolves through it, feeding the
// Detail pane the correct bean at every step.
func TestKeyFlatNavMovesCursorAndSelection(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('G'))
	if !m.flatView {
		t.Fatal("setup: expected flatView true")
	}

	vis := m.flatVisible()
	if len(vis) < 2 {
		t.Fatalf("setup: need >=2 flat-visible beans, got %d", len(vis))
	}

	if m.flatList.cursor != 0 {
		t.Fatalf("flatList.cursor = %d, want 0 before any nav", m.flatList.cursor)
	}
	if got := m.focusedBean(); got == nil || got.ID != vis[0].ID {
		t.Fatalf("focusedBean() = %v, want %s (flatList.cursor 0)", got, vis[0].ID)
	}

	m = step(t, m, runeMsg('k')) // down (jkli layout, keymap.go)
	if m.flatList.cursor != 1 {
		t.Fatalf("flatList.cursor = %d after down, want 1", m.flatList.cursor)
	}
	if got := m.focusedBean(); got == nil || got.ID != vis[1].ID {
		t.Fatalf("focusedBean() = %v after down, want %s", got, vis[1].ID)
	}

	m = step(t, m, runeMsg('i')) // up (jkli layout, keymap.go)
	if m.flatList.cursor != 0 {
		t.Fatalf("flatList.cursor = %d after up, want 0", m.flatList.cursor)
	}
	if got := m.focusedBean(); got == nil || got.ID != vis[0].ID {
		t.Fatalf("focusedBean() = %v after up, want %s", got, vis[0].ID)
	}
}

// TestBrowseFlatGolden renders the SAME fixture TestTreeGolden uses, but
// with m.flatView toggled on, and compares it against
// testdata/browse_flat.golden.
// Regenerate with: go test ./internal/tui/ -run TestBrowseFlatGolden -update
func TestBrowseFlatGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	m.flatView = true
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("browse flat view height=%d, want 30 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Errorf("line %d overflows width (%d > 100): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "browse_flat.golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (%s) — regenerate with -update: %v", path, err)
	}
	if out != string(want) {
		t.Errorf("browse flat view output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// TestBrowseFlatDefaultOffMatchesTree guards flatView's OFF-by-default
// contract at the golden level: WITHOUT toggling G, the exact same fixture
// must still render byte-identically to tree.golden -- this new code path
// is fully inert unless explicitly toggled on (mirrors
// TestBrowseBoxFormDefaultOffMatchesTree, box_form_golden_test.go).
func TestBrowseFlatDefaultOffMatchesTree(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	if m.flatView {
		t.Fatal("setup: expected flatView false (default)")
	}
	out := m.View()

	want, err := os.ReadFile(filepath.Join("testdata", "tree.golden"))
	if err != nil {
		t.Fatalf("tree.golden missing: %v", err)
	}
	if out != string(want) {
		t.Errorf("default (flatView off) browse view diverged from tree.golden -- G toggle must be fully inert when off")
	}
}
