// internal/ui/width_test.go
package ui

import (
	"reflect"
	"testing"
)

func TestDisplayWidthCountsCellsNotBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"abc", 3},
		{"", 0},
		{"├─ ", 3},    // three runes, seven bytes
		{"│  └─ ", 6}, // the deepest connector shape
		{"Größe", 5},  // umlaut is one cell
		{"日本", 4},     // east asian wide: two cells each
	}
	for _, c := range cases {
		if got := DisplayWidth(c.in); got != c.want {
			t.Errorf("DisplayWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestPadRightFillsButNeverCuts(t *testing.T) {
	if got := PadRight("ab", 5); got != "ab   " {
		t.Errorf("PadRight(\"ab\", 5) = %q, want %q", got, "ab   ")
	}
	if got := PadRight("abcdef", 3); got != "abcdef" {
		t.Errorf("PadRight must not cut, got %q", got)
	}
	if got := DisplayWidth(PadRight("├─", 6)); got != 6 {
		t.Errorf("padded multi-byte width = %d, want 6", got)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in    string
		width int
		want  string
	}{
		{"abcdef", 10, "abcdef"},
		{"abcdef", 6, "abcdef"},
		{"abcdef", 4, "abc…"},
		{"abcdef", 2, "a…"},
		{"abcdef", 1, "…"},
		{"abcdef", 0, ""},
		{"abcdef", -3, ""},
	}
	for _, c := range cases {
		if got := Truncate(c.in, c.width); got != c.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

func TestWrapText(t *testing.T) {
	got := WrapText("the quick brown fox", 10)
	want := []string{"the quick", "brown fox"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText = %#v, want %#v", got, want)
	}
}

func TestWrapTextHardBreaksAWordThatCannotFit(t *testing.T) {
	got := WrapText("supercalifragilistic", 8)
	want := []string{"supercal", "ifragili", "stic"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText = %#v, want %#v", got, want)
	}
}

func TestWrapTextNeverReturnsEmpty(t *testing.T) {
	if got := WrapText("", 10); len(got) != 1 || got[0] != "" {
		t.Errorf("WrapText(\"\", 10) = %#v, want one empty line", got)
	}
}

func TestWrapTextNoLineExceedsTheWidth(t *testing.T) {
	long := "Canary-Instanz sproutling-test — Staged-Rollout & Dogfood auf NAS"
	for _, line := range WrapText(long, 24) {
		if w := DisplayWidth(line); w > 24 {
			t.Errorf("line %q is %d cells wide, want at most 24", line, w)
		}
	}
}

// A column narrower than one wide (CJK) character's cell width has no line
// that is both non-empty and within width — the two guarantees conflict.
// WrapText must still terminate and never return an empty line; it is
// allowed to overflow width by one cell in this single degenerate case.
// This must NOT be "fixed" back to strict width compliance here, or the
// underlying loop hangs forever again.
func TestWrapTextNarrowColumnWithWideCharacterTerminates(t *testing.T) {
	in := "日本語"
	got := WrapText(in, 1)
	if len(got) == 0 {
		t.Fatalf("WrapText(%q, 1) returned no lines", in)
	}
	for _, line := range got {
		if line == "" {
			t.Errorf("WrapText(%q, 1) produced an empty line: %#v", in, got)
		}
	}
}

// At width 2 every rune of "日本語" exactly fits its own line, so both
// guarantees hold here: non-empty lines, and no line wider than requested.
func TestWrapTextWideCharacterExactlyFitsTheWidth(t *testing.T) {
	in := "日本語"
	got := WrapText(in, 2)
	for _, line := range got {
		if line == "" {
			t.Errorf("WrapText(%q, 2) produced an empty line: %#v", in, got)
		}
		if w := DisplayWidth(line); w > 2 {
			t.Errorf("WrapText(%q, 2) line %q is %d cells wide, want at most 2", in, line, w)
		}
	}
}

// TestSetTypeColumnWidthsIsFedInCells is the layer-side half of a defect
// found in the final review: root.go measured the longest type name with
// len(), a BYTE count, and handed it to SetTypeColumnWidths as a column
// width. The error was in the safe direction for the default vocabulary
// (bytes >= cells, so the column came out too wide) but it is the exact
// defect class this layer exists to prevent, three lines from the comment
// explaining why the measurement has to live outside internal/ui.
//
// This pins the property at the seam: a name whose byte length and display
// width differ must size the column by cells.
func TestSetTypeColumnWidthsIsFedInCells(t *testing.T) {
	// "Änderung": 8 display cells, 9 bytes. "课题": 2 runes, 4 cells, 6 bytes.
	for _, tc := range []struct {
		name  string
		cells int
	}{
		{"Änderung", 8},
		{"课题", 4},
		{"milestone", 9},
	} {
		if got := DisplayWidth(tc.name); got != tc.cells {
			t.Errorf("DisplayWidth(%q) = %d, want %d", tc.name, got, tc.cells)
		}
		if len(tc.name) == tc.cells && tc.name != "milestone" {
			t.Errorf("%q was chosen because bytes and cells differ; they do not", tc.name)
		}
	}
}
