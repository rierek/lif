package tui

// Direct primitive tests beyond the golden chrome frame — covers the
// acceptance list in bean bt-5544: breadcrumb, footer hints,
// masterDetailWidths, modalBox, overlayModal (placeOverlay), listState (see
// list_test.go), keymap (see keymap_test.go).

import (
	"strings"
	"testing"

	keybind "github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/rierek/lif/internal/tui/theme"
)

// TestBreadcrumbFormat guards the beans-tui breadcrumb format `> repo: Title`
// (port-adaptation vs. devd's `> slug: Title`) plus the right-aligned global
// hint, and the title-less `> repo` form.
func TestBreadcrumbFormat(t *testing.T) {
	out := breadcrumb("lif", "Browse", "q:quit", 60)
	if !strings.Contains(out, "> lif") {
		t.Errorf("breadcrumb missing '> lif': %q", out)
	}
	if !strings.Contains(out, "Browse") {
		t.Errorf("breadcrumb missing title: %q", out)
	}
	if !strings.Contains(out, "q:quit") {
		t.Errorf("breadcrumb missing global hint: %q", out)
	}

	bare := breadcrumb("lif", "", "q:quit", 60)
	if strings.Contains(bare, "> lif:") {
		t.Errorf("title-less breadcrumb must not render a trailing colon: %q", bare)
	}
}

// TestBreadcrumbNarrowStacks guards the narrow-terminal fallback: when left+
// right don't fit on one line, breadcrumb stacks them on two lines instead of
// overflowing/truncating silently.
func TestBreadcrumbNarrowStacks(t *testing.T) {
	out := breadcrumb("lif", "A Very Long Screen Title Indeed", "ctrl+k:cmd  p:project  q:quit", 20)
	if !strings.Contains(out, "\n") {
		t.Errorf("expected narrow breadcrumb to stack onto two lines, got single line: %q", out)
	}
}

// TestRenderBindingsSkipsHelplessAndColorSeparatesNoColon guards D06 (design-
// spec.md §15 PF-16, bean bt-ntoz/bt-d8kc Grilling-Nachtrag: "Taste in TEAL,
// Aktions-Wort grau (subtext), KEIN ':' mehr — Farbtrennung ersetzt den
// Doppelpunkt"), superseding the previous plain "key:desc" colon-joined
// rendering: bindings without a Help().Key are still skipped (devd DD2-175:
// derived from the keymap, not hand-maintained strings), entries now join on
// " · " (Q06's own verbatim separator), and the ANSI-stripped plain text of
// each entry is "key desc" (single space, no colon) -- the color split IS
// the separator now.
func TestRenderBindingsSkipsHelplessAndColorSeparatesNoColon(t *testing.T) {
	bs := []keybind.Binding{
		keybind.NewBinding(keybind.WithKeys("enter"), keybind.WithHelp("enter", "open")),
		keybind.NewBinding(keybind.WithKeys("x")), // no Help() → must be skipped
		keybind.NewBinding(keybind.WithKeys("esc"), keybind.WithHelp("esc", "back")),
	}
	got := renderBindings(bs)
	plain := stripHint(got)
	want := "enter open · esc back"
	if plain != want {
		t.Errorf("stripHint(renderBindings(bs))=%q, want %q (full=%q)", plain, want, got)
	}
	if strings.Contains(plain, ":") {
		t.Errorf("renderBindings=%q, must not contain ':' anymore (D06)", plain)
	}
}

// TestRenderBindingsColorsKeyTealDescSubtext guards D06's actual color
// split: the key half carries theme.BindingKey's Teal escape, the
// description half carries theme.BindingDesc's Subtext escape -- not just
// that the plain text reads right (the test above), but that the RIGHT
// substring carries EACH color, so a regression that swapped the two styles
// (or dropped color entirely) would be caught here even though the
// ANSI-stripped text alone wouldn't reveal it.
func TestRenderBindingsColorsKeyTealDescSubtext(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	bs := []keybind.Binding{keybind.NewBinding(keybind.WithKeys("q"), keybind.WithHelp("q", "quit"))}
	got := renderBindings(bs)

	wantKey := theme.BindingKey.Render("q")
	wantDesc := theme.BindingDesc.Render("quit")
	if !strings.Contains(got, wantKey) {
		t.Errorf("renderBindings=%q, want it to contain the Teal-rendered key %q", got, wantKey)
	}
	if !strings.Contains(got, wantDesc) {
		t.Errorf("renderBindings=%q, want it to contain the Subtext-rendered desc %q", got, wantDesc)
	}
}

// TestRenderBindingsKeepsKeyAndDescTogetherAcrossWrap guards a regression
// found during this task's own tmux smoke test (D06, design-spec.md §15
// PF-16, bean bt-ntoz/bt-d8kc): the pre-D06 "key:desc" rendering GLUED the
// key to its desc's first word via ":" (no internal space -- wordwrap could
// never split them). Replacing ":" with a plain " " (D06's own optic)
// reintroduced a real breakable space there, so footer()'s own wordwrap
// (view.go) could orphan a key alone on one line with its desc starting the
// NEXT line (observed live: Ready footer at 80 cols split "S" from
// "Sort"). renderBindings must glue key+desc (and any multi-word desc's own
// internal spaces) with a non-breaking space (U+00A0, x/ansi's own
// wordwrap.go explicitly excludes nbsp from its breakpoint set) so an
// entire binding's hint is always wrap-atomic -- only the " · "
// between-entries separator may ever become a line break.
func TestRenderBindingsKeepsKeyAndDescTogetherAcrossWrap(t *testing.T) {
	bs := []keybind.Binding{
		keybind.NewBinding(keybind.WithKeys("a"), keybind.WithHelp("a", "Alpha")),
		keybind.NewBinding(keybind.WithKeys("S"), keybind.WithHelp("S", "Sort")),
	}
	hint := renderBindings(bs) // "a Alpha · S Sort"
	wrapped := wrapText(hint, 11)
	for _, line := range strings.Split(wrapped, "\n") {
		if plain := strings.TrimSpace(ansi.Strip(line)); plain == "Sort" {
			t.Fatalf("wrapText separated desc %q from its key %q onto the PREVIOUS line: %q", "Sort", "S", wrapped)
		}
	}
}

// TestRenderBindingsKeepsMultiWordDescTogether guards the same NBSP fix for
// a multi-word Help().Desc (e.g. FocusIn's "next focus"): none of its own
// internal words may split across a wrap boundary either.
func TestRenderBindingsKeepsMultiWordDescTogether(t *testing.T) {
	bs := []keybind.Binding{
		keybind.NewBinding(keybind.WithKeys("tab"), keybind.WithHelp("tab", "focus in")),
	}
	hint := "x " + renderBindings(bs) // "x " prefix forces a real wrap decision ahead of the entry
	wrapped := wrapText(hint, 12)     // >= len("tab focus in")=12: exercises wordwrap's own
	// atomicity, not the separate Hardwrap over-long-token fallback (which
	// legitimately hard-chops ANY atomic unit, nbsp included, once it alone
	// exceeds the width budget -- an unrelated, unavoidable edge case at
	// pathologically narrow widths, out of scope here).
	for _, line := range strings.Split(wrapped, "\n") {
		plain := strings.TrimSpace(ansi.Strip(line))
		if plain == "in" || plain == "focus" || plain == "tab" || plain == "tab focus" {
			t.Fatalf("wrapText split the multi-word desc apart: line %q (full=%q)", plain, wrapped)
		}
	}
}

// TestMasterDetailWidthsIsOneThirdTwoThirds guards the 1fr:2fr split on a
// wide terminal where the floor/cap don't kick in.
func TestMasterDetailWidthsIsOneThirdTwoThirds(t *testing.T) {
	lw, rw := masterDetailWidths(90, 0)
	if lw != 30 {
		t.Errorf("lw=%d, want 30 (90/3)", lw)
	}
	if rw != 90-30-4 {
		t.Errorf("rw=%d, want %d", rw, 90-30-4)
	}
}

// TestMasterDetailWidthsFloorAndCap guards the two guard rails: a
// treeWidthFloor raises a too-narrow 1fr share (readability on narrow
// terminals), but never above w*2/5 (devd DD2-91 rework: floor, not fixed).
func TestMasterDetailWidthsFloorAndCap(t *testing.T) {
	// w/3 = 20 < floor 36 → floor wins.
	lw, _ := masterDetailWidths(60, 36)
	if lw != 24 {
		// 60*2/5 = 24 caps the floor here — guards the cap-over-floor rule.
		t.Errorf("lw=%d, want 24 (cap w*2/5 overrides floor)", lw)
	}

	// On a wide terminal the floor must not force a fixed narrow column.
	lw2, _ := masterDetailWidths(300, 36)
	if lw2 != 100 {
		t.Errorf("lw2=%d, want 100 (300/3, floor irrelevant on wide terminal)", lw2)
	}
}

// TestModalBoxHasRoundedBorder guards modal.go's chrome: modalBox must carry
// a RoundedBorder around the (rebased) inner content.
func TestModalBoxHasRoundedBorder(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	out := modalBox("line one\nline two", 30, theme.Mauve)
	if !strings.ContainsAny(out, "╭╮╰╯") {
		t.Error("modalBox output missing RoundedBorder corner glyphs")
	}
}

// TestRebaseBgReopensAfterReset guards the B02/B03 fix directly: under a
// color profile, every inner ESC[0m reset must be followed by a re-opened
// Base background sequence — otherwise cells after an inner reset (e.g. a
// theme.Header.Render call, later a huh form) fall back to the terminal's
// default background instead of the modal's Base.
func TestRebaseBgReopensAfterReset(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	in := "a\x1b[0mb"
	out := rebaseBg(in)
	if out == in {
		t.Fatal("rebaseBg did not modify the string under a TrueColor profile")
	}
	if !strings.Contains(out, "\x1b[0m\x1b[") {
		t.Errorf("rebaseBg did not insert a re-open sequence right after the reset: %q", out)
	}
}

// TestRebaseBgAsciiInsertsNoRealColor guards the Ascii-profile path: termenv
// still resolves theme.Base to a (non-nil) NoColor under Ascii, so rebaseBg
// does rewrite the string — but the re-opened sequence must carry no real
// color escape (no visible color can leak into an Ascii terminal).
func TestRebaseBgAsciiInsertsNoRealColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	in := "a\x1b[0mb"
	out := rebaseBg(in)
	if strings.Contains(out, "\x1b[38") || strings.Contains(out, "\x1b[48") {
		t.Errorf("rebaseBg inserted a real color escape under the Ascii profile: %q", out)
	}
}

// TestMenuListMarksCursor guards menuList's ▸-cursor placement on the
// selected entry only.
func TestMenuListMarksCursor(t *testing.T) {
	out := menuList(3, 1, func(i int, sel bool) string {
		if sel {
			return "SELECTED"
		}
		return "plain"
	})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("menuList produced %d lines, want 3", len(lines))
	}
	if !strings.Contains(lines[1], "▸") || !strings.Contains(lines[1], "SELECTED") {
		t.Errorf("line 1 (cursor) = %q, want ▸ + SELECTED", lines[1])
	}
	if strings.Contains(lines[0], "▸") || strings.Contains(lines[2], "▸") {
		t.Error("non-cursor lines must not carry the ▸ marker")
	}
}

// TestPlaceOverlayCentersAndKeepsBackgroundVisible guards overlay.go's
// painter's algorithm: the background remains visible around the overlay,
// and the overlay's own content is present in the composited output.
func TestPlaceOverlayCentersAndKeepsBackgroundVisible(t *testing.T) {
	bg := strings.Repeat("background-row-of-filler-text\n", 9)
	bg = strings.TrimSuffix(bg, "\n")
	fg := "TOP\nMID\nBOT"

	out := placeOverlay(bg, fg, 30, 9)
	if !strings.Contains(out, "TOP") || !strings.Contains(out, "MID") || !strings.Contains(out, "BOT") {
		t.Errorf("composited output missing overlay content: %q", out)
	}
	if !strings.Contains(out, "background-row-of-filler-text") {
		t.Error("composited output lost the background entirely (painter's algorithm broken)")
	}
	if got := len(strings.Split(out, "\n")); got != 9 {
		t.Errorf("composited output has %d lines, want 9 (background line count preserved)", got)
	}
}

// TestCanvasLinesPadsToRectangle guards canvasLines: every line must be
// padded to tw cells and there must be at least th lines, so the overlay
// canvas is a gapless rectangle (devd DD2-216).
func TestCanvasLinesPadsToRectangle(t *testing.T) {
	lines := canvasLines("short\nlonger-row", 20, 5)
	if len(lines) < 5 {
		t.Errorf("canvasLines produced %d lines, want >= 5", len(lines))
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != 20 {
			t.Errorf("line %d width=%d, want 20", i, w)
		}
	}
}

// --- T7 follow-up I01 (bean bt-7jr8, MANDATORY in T8): renderPane
// (focused-border), borderedPane, tagsInline/tagSwatch, modalPanel ---

// fgEscape returns the raw foreground color escape sequence lipgloss/termenv
// emit for c under the CURRENT color profile -- the same primitive
// BorderForeground/Foreground resolve to internally, so tests can assert a
// specific color is (or isn't) present without hardcoding ANSI byte layout.
func fgEscape(c lipgloss.Color) string {
	col := lipgloss.ColorProfile().Color(string(c))
	return termenv.CSI + col.Sequence(false) + "m"
}

// TestRenderPaneFocusedBorderColor guards render_shared.go's focused/
// unfocused border-color swap: focused=true borders Mauve, focused=false
// borders Overlay (devd D03 pattern: only the focused pane is highlighted).
func TestRenderPaneFocusedBorderColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	p := pane{rows: []string{"row one", "row two"}}
	focused := renderPane(p, 20, 4, true)
	unfocused := renderPane(p, 20, 4, false)

	mauve := fgEscape(theme.Mauve)
	overlay := fgEscape(theme.Overlay)

	// Mauve only ever appears via the focused BORDER now (PF-10, bean bt-uyzf,
	// removed the title line that used to also carry it via theme.Header) --
	// it must be exclusive to the focused render.
	if !strings.Contains(focused, mauve) {
		t.Errorf("focused renderPane missing the Mauve border escape %q", mauve)
	}
	if strings.Contains(unfocused, mauve) {
		t.Error("unfocused renderPane unexpectedly carries the Mauve (focused) border color")
	}
	if !strings.Contains(unfocused, overlay) {
		t.Errorf("unfocused renderPane missing the Overlay border escape %q", overlay)
	}

	if focused == unfocused {
		t.Fatal("renderPane output identical for focused vs unfocused")
	}
}

// TestRenderPaneNoTitleLine guards PF-10 (design-spec.md §15, epic-E7-plan.md
// »Task 5«, bean bt-uyzf): renderPane must no longer render a title line +
// underline-separator ahead of its rows -- the Breadcrumb already carries the
// view identity (PO-Nachtrag 4, PO wörtlich: "Es genügt, wenn es in den
// Breadcrumbs ... angezeigt wird. Dann die Suche - sonst ist es obsolet.").
// The FIRST content line inside the border must be rows[0] verbatim, not an
// empty/short title+separator pair consuming 2 lines ahead of it.
func TestRenderPaneNoTitleLine(t *testing.T) {
	p := pane{rows: []string{"first content row", "second content row"}}
	out := renderPane(p, 30, 4, false)

	lines := strings.Split(out, "\n")
	if len(lines) != 6 { // border(1) + h=4 content + border(1), Golden Rule #1
		t.Fatalf("renderPane produced %d lines, want 6 (h=4 + 2 border rows): %q", len(lines), out)
	}
	if !strings.Contains(lines[1], "first content row") {
		t.Fatalf("renderPane's first content line (row 1, right after the top border) must be rows[0] verbatim -- no title/separator line ahead of it. got line 1: %q\nfull:\n%s", lines[1], out)
	}
	if !strings.Contains(lines[2], "second content row") {
		t.Fatalf("renderPane's second content line (row 2) must be rows[1] verbatim, right after rows[0] -- no title/separator line consuming budget. got line 2: %q\nfull:\n%s", lines[2], out)
	}
}

// TestBorderedPanePadsAndCapsToHeight guards borderedPane's Golden-Rule-#1
// contract: content is padded/capped to exactly h inner lines (never relies
// on Height()), so the RoundedBorder always adds exactly 2 rows on top.
func TestBorderedPanePadsAndCapsToHeight(t *testing.T) {
	short := borderedPane([]string{"one", "two"}, 10, 4, theme.Mauve)
	if h := lipgloss.Height(short); h != 6 { // h=4 + 2 border rows
		t.Errorf("borderedPane height=%d, want 6 (h=4 padded + 2 border rows)", h)
	}

	over := borderedPane([]string{"a", "b", "c", "d", "e"}, 10, 2, theme.Mauve)
	if h := lipgloss.Height(over); h != 4 { // capped to h=2 + 2 border rows
		t.Errorf("borderedPane (over-long input) height=%d, want 4 (capped to h=2 + 2 border rows)", h)
	}
	if !strings.ContainsAny(over, "╭╮╰╯") {
		t.Error("borderedPane output missing RoundedBorder corner glyphs")
	}
}

// TestTagsInlineAndSwatch guards tagsInline/tagSwatch: empty input renders
// nothing, each tag renders its name plus a swatch dot, and the hash-derived
// color is deterministic across calls (fnv is pure -- no time/random leak).
func TestTagsInlineAndSwatch(t *testing.T) {
	if got := tagsInline(nil); got != "" {
		t.Errorf("tagsInline(nil) = %q, want empty string", got)
	}

	single := tagSwatch("urgent")
	if !strings.Contains(single, "urgent") {
		t.Errorf("tagSwatch missing the tag name: %q", single)
	}
	if !strings.Contains(single, "●") {
		t.Errorf("tagSwatch missing the swatch dot: %q", single)
	}
	if tagSwatch("urgent") != tagSwatch("urgent") {
		t.Error("tagSwatch is not deterministic for the same tag name")
	}

	multi := tagsInline([]string{"urgent", "backend"})
	if !strings.Contains(multi, "urgent") || !strings.Contains(multi, "backend") {
		t.Errorf("tagsInline missing one of the tag names: %q", multi)
	}
	if got := strings.Count(multi, "●"); got != 2 {
		t.Errorf("tagsInline rendered %d swatch dots, want 2", got)
	}
}

// TestModalPanelIncludesHeaderBodyFooter guards modal.go's modalPanel:
// header title + body + (optional) footer all show up, wrapped in modalBox's
// RoundedBorder.
func TestModalPanelIncludesHeaderBodyFooter(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	out := modalPanel("Title Here", "body line", "footer hint", 30, theme.Mauve)
	if !strings.Contains(out, "Title Here") {
		t.Error("modalPanel missing the header title")
	}
	if !strings.Contains(out, "body line") {
		t.Error("modalPanel missing the body")
	}
	if !strings.Contains(out, "footer hint") {
		t.Error("modalPanel missing the footer hint")
	}
	if !strings.ContainsAny(out, "╭╮╰╯") {
		t.Error("modalPanel missing RoundedBorder corner glyphs (modalBox)")
	}

	noFooter := modalPanel("T", "body", "", 30, theme.Mauve)
	if !strings.Contains(noFooter, "body") {
		t.Error("modalPanel (no footer) missing the body")
	}
}
