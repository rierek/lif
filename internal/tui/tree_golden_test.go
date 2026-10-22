package tui

// tree_golden_test.go — golden snapshot for the Browse Primat-View (T8,
// implementation-plan.md »E1 Task 8«). Mirrors chrome_test.go's pattern
// (same package-level `update` flag, defined there). 100x30, TrueColor
// forced for determinism.

import (
	"github.com/rierek/lif/pkg/item"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// goldenTreeModel builds the deterministic fixture rendered by
// TestTreeGolden: a milestone -> epic -> two tasks (one bug), plus one
// orphan -- every row kind (root/branch/leaf/orphan) is on screen at once,
// both branches expanded, cursor parked on a leaf.
func goldenTreeModel(t *testing.T) model {
	t.Helper()
	beans := []*item.Item{
		{ID: "gld-mlst", Title: "Golden Milestone", Status: "in-progress", Type: "milestone", Priority: "high"},
		{ID: "gld-epic", Title: "Golden Epic", Status: "todo", Type: "epic", Priority: "normal", Parent: "gld-mlst", Tags: []string{"backend"}},
		{ID: "gld-tsk1", Title: "First golden task", Status: "todo", Type: "task", Priority: "normal", Parent: "gld-epic"},
		{ID: "gld-tsk2", Title: "Second golden task", Status: "completed", Type: "bug", Priority: "critical", Parent: "gld-epic"},
		{ID: "gld-orph", Title: "Golden orphan", Status: "draft", Type: "task", Priority: "low", Parent: "gld-missing"},
	}

	m := newModel(nil, "/tmp/bt-golden-repo")
	m = step(t, m, beansLoadedMsg{beans: beans})
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.detailHidden = false
	m.expanded["gld-mlst"] = true
	m.expanded["gld-epic"] = true
	m.expanded[notOnRoadmapRootID] = true
	m.cursorID = "gld-tsk1"
	return m
}

// TestTreeGolden renders a full 100x30 frame of the Browse view and compares
// it against testdata/tree.golden.
// Regenerate with: go test ./internal/tui/ -run TestTreeGolden -update
func TestTreeGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("tree view height=%d, want 30 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Errorf("line %d overflows width (%d > 100): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "tree.golden")
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
		t.Errorf("tree view output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// TestTreeGoldenDeterministic guards that View() is a pure function of the
// model: repeated calls on identical state must byte-for-byte agree.
func TestTreeGoldenDeterministic(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	a := m.View()
	b := m.View()
	if a != b {
		t.Error("View() is not deterministic across repeated calls with identical model state")
	}
}
