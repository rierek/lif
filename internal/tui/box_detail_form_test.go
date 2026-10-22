package tui

// box_detail_form_test.go — S2 (jira-style experiment, design-spec.md D01/
// D03, mockup §6.3): tests for detailBoxForm, the jira-style scalar-fields
// grid built from S1's dropdownBox primitive. Structural (width-agnostic
// content) + responsive (perRow column count) + golden snapshot, mirroring
// tree_golden_test.go's own -update flag pattern (chrome_test.go's shared
// package-level `update` flag).

import (
	"github.com/rierek/lif/pkg/item"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/data"
)

// detailBoxFormFixture is the fixed bean fixture shared by all three tests
// below (structural/responsive/golden) — same ID/field shape as tree_golden_
// test.go's gld-tsk1 (Parent="gld-epic") so a future cross-golden comparison
// stays apples-to-apples, but Tags nil here (Parent test coverage already
// exercises tags via the tree golden; this fixture instead covers the "no
// tags" Dim-placeholder path of detailBoxForm's own Tags box).
func detailBoxFormFixture() *item.Item {
	return &item.Item{
		ID:       "gld-tsk1",
		Title:    "First golden task",
		Status:   "todo",
		Type:     "task",
		Priority: "",
		Parent:   "gld-epic",
		Tags:     nil,
	}
}

// detailBoxFormIndex builds the *data.Index fixture detailBoxForm needs to
// resolve Relations (S2c) — a minimal parent/child pair around the fixture
// bean's own ID/Parent ("gld-tsk1" parented under "gld-epic"), mirroring
// view_detail_bean_test.go's own data.NewIndex(beans) construction pattern.
func detailBoxFormIndex() *data.Index {
	beans := []*item.Item{
		{ID: "gld-epic", Title: "Golden epic", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "gld-tsk1", Title: "First golden task", Status: "todo", Type: "task", Priority: "", Parent: "gld-epic"},
	}
	return data.NewIndex(beans)
}

// TestDetailBoxFormStructure asserts (width-agnostic, ansi.Strip'd) that all
// 5 scalar fields plus Title render with their labels/values/hotkey badges,
// and that EVERY produced line is exactly the requested width (100) --
// review B5/B1: overflow-only (`> width`) checks miss underflow, so this
// asserts `== width`.
func TestDetailBoxFormStructure(t *testing.T) {
	b := detailBoxFormFixture()
	idx := detailBoxFormIndex()
	const w = 100
	out := detailBoxForm(idx, b, w, -1)
	plain := ansi.Strip(out)

	for _, want := range []string{
		"Title", "First golden task",
		"Status", "todo",
		"Type", "task",
		"Priority",
		"Parent", "gld-epic",
		"Tags",
		"(e)", "(s)", "(o)", "(u)", "(a)", "(t)",
		"Body", "Relations", "History",
	} {
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

// TestDetailBoxFormFixedGridNoCollapse asserts D12: the scalar grid is FIXED
// (Row A = Status|Type|Priority, Row B = Parent|Tags) at ANY width, no
// responsive perRow collapse to 1-up. At a narrow width (50) Row A must
// still render 3 columns (3 "▾" arrows / the (s)(o)(u) hotkeys on adjacent
// columns) and Row B must still render 2 columns ((a)(t) hotkeys) -- and
// every produced line is still exactly the requested width (review B1/B5).
func TestDetailBoxFormFixedGridNoCollapse(t *testing.T) {
	b := detailBoxFormFixture()
	idx := detailBoxFormIndex()
	const w = 50
	out := detailBoxForm(idx, b, w, -1)
	lines := strings.Split(out, "\n")

	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != w {
			t.Errorf("line %d width = %d, want exactly %d: %q", i, got, w, ln)
		}
	}

	// Layout is fixed: Title (3 lines: top/mid/bot), then Row A (3 lines),
	// then Row B (3 lines), then the full-width panels.
	if len(lines) < 9 {
		t.Fatalf("want at least 9 lines (Title+RowA+RowB), got %d: %q", len(lines), out)
	}
	rowAMid := ansi.Strip(lines[4])
	rowABot := ansi.Strip(lines[5])
	if n := strings.Count(rowAMid, "▾"); n != 3 {
		t.Errorf("Row A mid line must have 3 ▾ arrows (no 1-up collapse), got %d: %q", n, rowAMid)
	}
	for _, hk := range []string{"(s)", "(o)", "(u)"} {
		if !strings.Contains(rowABot, hk) {
			t.Errorf("Row A bottom border missing hotkey %q (no 1-up collapse): %q", hk, rowABot)
		}
	}

	rowBMid := ansi.Strip(lines[7])
	rowBBot := ansi.Strip(lines[8])
	if n := strings.Count(rowBMid, "▾"); n != 2 {
		t.Errorf("Row B mid line must have 2 ▾ arrows (no 1-up collapse), got %d: %q", n, rowBMid)
	}
	for _, hk := range []string{"(a)", "(t)"} {
		if !strings.Contains(rowBBot, hk) {
			t.Errorf("Row B bottom border missing hotkey %q (no 1-up collapse): %q", hk, rowBBot)
		}
	}
}

// TestDetailBoxFormGolden renders detailBoxForm(fixture, 100) at TrueColor
// and compares it against testdata/detail_boxform.golden. Mirrors tree_
// golden_test.go's pattern exactly (shared `update` flag from chrome_test.go).
// Regenerate with: go test ./internal/tui/ -run TestDetailBoxFormGolden -update
func TestDetailBoxFormGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	b := detailBoxFormFixture()
	idx := detailBoxFormIndex()
	out := detailBoxForm(idx, b, 100, -1)

	path := filepath.Join("testdata", "detail_boxform.golden")
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
		t.Errorf("detailBoxForm output differs from golden %q.\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// --- bean bt-oox1 (#1): the bean ID next to the title ---

// TestDetailBoxFormShowsBeanIDNextToTitle guards PO finding #1: the Detail
// pane showed no ID at all, so the one identifier the PO needs to quote in a
// commit, a bean reference or a CLI call was nowhere on screen.
//
// It is the FULL ID, deliberately. bt-pl5p shortened the LIST rows
// (sproutling-eq67 -> eq67) because the repo slug repeats on every row there
// and costs the title its columns -- but that same decision explicitly kept
// the complete, copyable ID readable in the Detail pane
// (TestRelationRowKeepsFullIDInDetail is its counter-guard). The Detail pane
// shows exactly one bean, so the slug costs nothing here and the ID stays
// copy-paste-able. Do not "unify" this with the list shortening.
func TestDetailBoxFormShowsBeanIDNextToTitle(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "sproutling-eq67", Title: "A Bean With A Title", Status: "todo", Type: "task"},
	})
	b := idx.ByID["sproutling-eq67"]

	out := ansi.Strip(detailBoxForm(idx, b, 60, -1))
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[0], "sproutling-eq67") {
		t.Errorf("the Title box's own border does not carry the bean ID:\n%s", lines[0])
	}
	if !strings.Contains(lines[0], "Title") {
		t.Errorf("the Title box lost its label:\n%s", lines[0])
	}
	if !strings.Contains(out, "A Bean With A Title") {
		t.Errorf("the title text itself is gone:\n%s", out)
	}
}

// TestDetailBoxFormIDClampsGracefullyWhenNarrow: the ID rides in the frame
// label, which boxTopBorder clamps -- a narrow pane must still produce an
// exactly-width frame rather than an overflowing one.
func TestDetailBoxFormIDClampsGracefullyWhenNarrow(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "a-very-long-repo-slug-indeed-eq67", Title: "T", Status: "todo", Type: "task"},
	})
	b := idx.ByID["a-very-long-repo-slug-indeed-eq67"]

	// Widths at/above the pane's own usable floor: below ~30 the scalar
	// gridRow already refuses to shrink its three columns further (a
	// pre-existing, unrelated property), so narrower widths would measure
	// that instead of the ID's clamping.
	for _, w := range []int{30, 45, 60, 100} {
		for i, ln := range strings.Split(detailBoxForm(idx, b, w, -1), "\n") {
			if got := lipgloss.Width(ln); got != w {
				t.Errorf("width %d: line %d width = %d, want %d: %q", w, i, got, w, ln)
			}
		}
	}
}
