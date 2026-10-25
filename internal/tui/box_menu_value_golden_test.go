package tui

// box_menu_value_golden_test.go — golden snapshot for the field-anchored
// value menu (Slice C, bt-f0y9 "feld-verankertes Inline-Dropdown", D09
// revidiert, B01/B02 fix round, Reviewer Q02, 2026-07-20): freezes the
// CORRECT post-fix rendering (content-width popup, left-aligned to the
// Status field, no chrome corruption) at normal size -- added AFTER the
// B01/B02 fix, not before, so it never froze the buggy geometry the
// Reviewer caught. Mirrors box_form_golden_test.go's own goldenTreeModel/
// TrueColor/-update pattern exactly.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestValueMenuAnchoredGolden renders goldenTreeModel's own 100x30 fixture
// with Detail-Pane focused on gld-tsk1, and the Status value
// menu opened via the real `s` keypress -- compares against
// testdata/value_menu_anchored.golden.
// Regenerate with: go test ./internal/tui/ -run TestValueMenuAnchoredGolden -update
func TestValueMenuAnchoredGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	m := goldenTreeModel(t)
	m.detailFocus = true
	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Fatalf("setup: overlay = %v, want overlayValueMenu", m.overlay)
	}
	out := m.View()

	if h := lipgloss.Height(out); h != 30 {
		t.Errorf("value-menu-anchored view height=%d, want 30 (full terminal height)", h)
	}

	path := filepath.Join("testdata", "value_menu_anchored.golden")
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
		t.Errorf("value-menu-anchored view output differs from golden %q.\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}
