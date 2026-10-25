package tui

// box_form_golden_test.go — golden snapshot for the Browse Primat-View with
// the jira-style box-form detail render (S2b). Mirrors tree_golden_test.go's
// TestTreeGolden EXACTLY (same fixture via goldenTreeModel, same 100x30
// frame, same TrueColor/-update pattern).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestBrowseBoxFormGolden renders the SAME fixture TestTreeGolden uses
// and compares it against testdata/browse_boxform.golden.
// Regenerate with: go test ./internal/tui/ -run TestBrowseBoxFormGolden -update
func TestBrowseBoxFormGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("browse boxform view height=%d, want 30 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Errorf("line %d overflows width (%d > 100): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "browse_boxform.golden")
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
		t.Errorf("browse boxform view output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}
