package tui

// Golden test for the shared chrome layer (Chrome/breadcrumb/footer/outer
// frame). View() is not a thing yet (no model until T8) — Chrome itself is
// the pure function under test.

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var update = flag.Bool("update", false, "regenerate golden files")

// goldenChromeOpts is the deterministic (no time/randomness) frame used by
// TestChromeGolden/TestChromeDeterministic.
func goldenChromeOpts() ChromeOpts {
	return ChromeOpts{
		Width: 80, Height: 24, Bordered: true,
		Repo:       "bt",
		Title:      "Test",
		GlobalHint: "ctrl+k:cmd  p:project  b:ready  R:reviews  ?:help  q:quit",
		Body:       "dummy body content for the golden chrome frame — enough text to see wrapping behave.",
		FooterHint: "enter: open  esc: back",
	}
}

// TestChromeGolden renders a full 80×24 frame (breadcrumb "bt: Test", dummy
// body, footer hints) and compares it against testdata/chrome.golden.
// Regenerate with: go test ./internal/tui/ -run TestChromeGolden -update
func TestChromeGolden(t *testing.T) {
	// Force a deterministic color profile: golden bytes must not depend on
	// the machine/terminal running the test (COLORTERM, TTY-ness, ...).
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	out := Chrome(goldenChromeOpts())

	if h := lipgloss.Height(out); h != 24 {
		t.Errorf("chrome height=%d, want 24 (full terminal height)", h)
	}
	for i, ln := range strings.Split(out, "\n") {
		if w := lipgloss.Width(ln); w > 80 {
			t.Errorf("line %d overflows width (%d > 80): %q", i, w, ln)
		}
	}

	path := filepath.Join("testdata", "chrome.golden")
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
		t.Errorf("chrome output differs from golden %q (frame/width/truncation?).\n--- got ---\n%s\n--- want ---\n%s", path, out, string(want))
	}
}

// TestChromeDeterministic guards that Chrome is a pure function of its
// inputs: repeated calls with identical opts must byte-for-byte agree (no
// hidden time/random/map-iteration-order dependency slipping in).
func TestChromeDeterministic(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	o := goldenChromeOpts()
	a := Chrome(o)
	b := Chrome(o)
	if a != b {
		t.Error("Chrome(o) is not deterministic across repeated calls with identical opts")
	}
}

// TestChromeNeverOverflowsWidth mirrors devd's DD2-60 review guard: no line
// overflows the terminal width across a range of narrow/typical widths, and
// the frame always fills the requested height exactly.
func TestChromeNeverOverflowsWidth(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	body := "Goal:\n" + strings.Repeat("x", 200) + "\nA longer sentence with many words to wrap right here."
	hint := strings.Repeat("k:action  ", 20)
	for _, w := range []int{30, 40, 80} {
		out := Chrome(ChromeOpts{
			Width: w, Height: 24, Bordered: true,
			Repo: "bt", Title: "A rather long screen title",
			GlobalHint: "q:quit",
			Body:       body,
			FooterHint: hint,
		})
		for i, ln := range strings.Split(out, "\n") {
			if lw := lipgloss.Width(ln); lw > w {
				t.Errorf("w=%d: line %d overflows (%d > %d)", w, i, lw, w)
			}
		}
		if h := lipgloss.Height(out); h != 24 {
			t.Errorf("w=%d: height=%d, want 24", w, h)
		}
	}
}

// TestChromeUnborderedNoOuterFrame guards outerBorder()'s no-op path: with
// Bordered=false the content is returned without the RoundedBorder wrap.
func TestChromeUnborderedNoOuterFrame(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	o := goldenChromeOpts()
	o.Bordered = false
	out := Chrome(o)
	if strings.ContainsAny(out, "╭╮╰╯") {
		t.Error("unbordered Chrome output contains RoundedBorder corner glyphs")
	}
}

// TestChromeFallbackAvailOverride guards the avail<4 fallback path (T7
// follow-up I03, bean bt-7jr8): ChromeOpts.fallbackAvail is a test hook that
// overrides the hardcoded 18-line default once the computed body height
// drops below 4 (a real terminal too short to fit the chrome). A tiny Height
// forces that branch; the two calls share every other opt, so the ONLY
// difference in output height must be exactly (default 18) - (override 5).
func TestChromeFallbackAvailOverride(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	base := ChromeOpts{
		Width: 80, Height: 4, Bordered: true, // tiny height forces avail<4
		Repo: "bt", Body: "body", FooterHint: "enter: open",
	}
	withDefault := Chrome(base)

	overridden := base
	overridden.fallbackAvail = 5
	withOverride := Chrome(overridden)

	if lipgloss.Height(withDefault) == lipgloss.Height(withOverride) {
		t.Fatal("fallbackAvail override had no effect on Chrome() output height -- avail<4 path may not be exercised, or the field is dead")
	}
	if diff := lipgloss.Height(withDefault) - lipgloss.Height(withOverride); diff != 13 {
		t.Errorf("height delta between default fallback (18) and override (5) = %d, want 13", diff)
	}
}
