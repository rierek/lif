package tui

// box_filter_bar_test.go — S3 (jira-style experiment, design-spec.md D02/
// D08): tests for filterBar, the persistent Type/Status/Priority/Tags chip
// row built from S2e's gridRow (box_detail_form.go) + S1's dropdownBox
// (box_dropdown.go). Structural only (width-agnostic content, ansi.Strip'd)
// — no golden here, the Roadmap-view-level golden (box_form_golden_test.go's
// TestRoadmapBoxFormGolden, testdata/roadmap_boxform.golden) already covers
// filterBar composed into the real frame.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TestFilterBarActiveFacet asserts an active facet (m.filterStatus set)
// shows both its label ("Status") and its value ("todo") in the rendered
// bar, and that every line is exactly the requested width (review B1/B5
// convention this experiment's other box tests already established:
// underflow, not just overflow, must be caught).
func TestFilterBarActiveFacet(t *testing.T) {
	m := model{filterStatus: map[string]bool{"todo": true}}
	const w = 100
	out := m.filterBar(w)
	plain := ansi.Strip(out)

	for _, want := range []string{"Status", "todo", "Type", "Priority", "Tags"} {
		if !strings.Contains(plain, want) {
			t.Errorf("output missing %q\n--- got ---\n%s", want, plain)
		}
	}
	for i, ln := range strings.Split(out, "\n") {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want exactly %d: %q", i, got, w, ln)
		}
	}
}

// TestFilterBarAllEmpty asserts that with no facets active, all four chips
// fall back to the "(any)" placeholder — four occurrences, one per facet —
// and every line still renders at exactly the requested width.
func TestFilterBarAllEmpty(t *testing.T) {
	m := model{}
	const w = 100
	out := m.filterBar(w)
	plain := ansi.Strip(out)

	if n := strings.Count(plain, "(any)"); n != 4 {
		t.Errorf("want 4 occurrences of \"(any)\" (one per empty facet), got %d\n--- got ---\n%s", n, plain)
	}
	for i, ln := range strings.Split(out, "\n") {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want exactly %d: %q", i, got, w, ln)
		}
	}
}

// TestFilterBarNarrowWidth mirrors TestDetailBoxFormFixedGridNoCollapse's
// own narrow-width check: at 50 cells, all four chips must still fit
// exactly (gridRow's own contract), no overflow/underflow.
func TestFilterBarNarrowWidth(t *testing.T) {
	m := model{filterTag: map[string]bool{"backend": true, "frontend": true}}
	const w = 50
	out := m.filterBar(w)
	for i, ln := range strings.Split(out, "\n") {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want exactly %d: %q", i, got, w, ln)
		}
	}
}
