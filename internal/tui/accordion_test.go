package tui

// accordion_test.go — TDD-first tests for the Detail-Accordion port (E2 Task
// 1, bean bt-ms0k): renderAccordion's exclusive-open/digit-numbering algebra
// (port of devd accordion.go:309-373, minus the edit-field concept) and
// glowRender (port of devd editor.go:102-126). E7 Task 4 (bean bt-kyj5,
// PF-1/PF-12) added: section 1 (Meta) always renders its body regardless of
// `open` (PF-1, "nicht kollabierbar"), and renderAccordion's header styling
// reserves a stable 1-column gutter for its active/inactive marker (PF-12)
// instead of conditionally prefixing it only when active (fieldStrip ALSO
// carried this same PF-12 fix once, but fieldStrip itself was removed
// wholesale by B04, design-spec.md §15 PF-17, bean bt-b0w0 -- see
// TestRenderAccordionOmitsFieldStripForRelations, below).
//
// PF-1 REVISED by PF-18 (design-spec.md §15, PO-Feedback 2026-07-16, bean
// bt-98cb): Meta's "always open" exception is GONE -- Meta now behaves like
// every other section, exclusive-open, closed by default until actively
// selected. See TestRenderAccordionSectionOneExclusiveLikeOthers, below.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/theme"
)

// TestRenderAccordionExclusiveOpen guards the exclusive-open contract: with 3
// sections and open=2, ONLY section 2's body renders (the open target) --
// section 1 (Meta) and section 3 both stay header-only. PF-18 (design-
// spec.md §15, PO-Feedback 2026-07-16, bean bt-98cb) REVISED PF-1's "Meta
// (section 1) is never collapsible" special case -- Meta now behaves like
// any other section, exclusive-open, no forced-open exception.
func TestRenderAccordionExclusiveOpen(t *testing.T) {
	secs := []accordionSection{
		{title: "One", body: "body-one-content"},
		{title: "Two", body: "body-two-content"},
		{title: "Three", body: "body-three-content"},
	}
	out := renderAccordion(secs, 2, 60, false, 0, 0)

	if strings.Contains(out, "body-one-content") {
		t.Error("PF-18: section 1 (Meta) body must NOT render when another section is open (PF-1 exception revised)")
	}
	if !strings.Contains(out, "body-two-content") {
		t.Error("open section 2's body must render")
	}
	if strings.Contains(out, "body-three-content") {
		t.Error("closed section 3's body must not render")
	}
	// B05 (design-spec.md §15 PF-16, bean bt-ntoz/bt-czpf, 2026-07-16):
	// the old assertions here required the "▸"/"▾" chevron hint suffix to
	// be PRESENT -- that suffix was removed as redundant (open/closed
	// state is already visible from whether the body renders below the
	// header). See TestRenderAccordionNoChevronSuffix below for the
	// current (inverse) contract.
}

// TestRenderAccordionNoChevronSuffix guards B05 (design-spec.md §15 PF-16,
// bean bt-ntoz/bt-czpf, 2026-07-16): the accordion section header's
// trailing hint suffix ("  ▾" open / "  ▸" closed) is redundant -- a
// section's open/closed state is already visible from whether its body
// renders below the header. Neither glyph may appear in the rendered
// header output any more, regardless of open/closed state.
func TestRenderAccordionNoChevronSuffix(t *testing.T) {
	secs := []accordionSection{
		{title: "One", body: "body-one-content"},
		{title: "Two", body: "body-two-content"},
		{title: "Three", body: "body-three-content"},
	}
	out := ansi.Strip(renderAccordion(secs, 2, 60, false, 0, 0))
	if strings.Contains(out, "▾") {
		t.Error("B05: open section header must no longer carry the '▾' chevron suffix")
	}
	if strings.Contains(out, "▸") {
		t.Error("B05: closed section header must no longer carry the '▸' chevron suffix")
	}
}

// TestRenderAccordionClosedHeaderUsesTealNotMuted guards B06 (EXPERIMENT,
// design-spec.md §15 PF-16, bean bt-ntoz/bt-czpf, 2026-07-16): a CLOSED
// section's header title renders with theme.HeaderInactive's own ANSI
// styling (Teal), not theme.Muted's (Hint-grey) any more -- render-grounded
// so a regression in accordion.go's own render path (not just the token's
// theme.go definition) is caught too. OPEN headers (theme.Header/Mauve) are
// untouched by B06 -- only the closed/inactive branch changes.
func TestRenderAccordionClosedHeaderUsesTealNotMuted(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	secs := []accordionSection{
		{title: "Meta", body: "meta-body"},
		{title: "Closed", body: "closed-body"},
	}
	out := renderAccordion(secs, 1, 60, false, 0, 0) // open=1 -- section 2 ("Closed") stays closed

	wantTeal := theme.HeaderInactive.Render("Closed")
	wantMuted := theme.Muted.Render("Closed")
	if !strings.Contains(out, wantTeal) {
		t.Errorf("B06: closed section header must render with theme.HeaderInactive (Teal) styling, got: %q", out)
	}
	if strings.Contains(out, wantMuted) {
		t.Errorf("B06: closed section header must NOT still render with theme.Muted styling, got: %q", out)
	}
}

// TestRenderAccordionSectionOneExclusiveLikeOthers guards PF-18 (design-
// spec.md §15, PO-Feedback 2026-07-16, bean bt-98cb -- REVISES PF-1):
// section 1 (Meta) is no longer a forced-open exception -- its body renders
// ONLY when open==1 (Meta itself is the active section), exactly like any
// other section. open==0 (no section active/selected, e.g. before the
// Detail-Pane has focus) and open pointing at another section must both
// leave Meta's body closed.
func TestRenderAccordionSectionOneExclusiveLikeOthers(t *testing.T) {
	secs := []accordionSection{
		{title: "Meta", body: "meta-body-content"},
		{title: "Body", body: "body-body-content"},
	}
	cases := []struct {
		open     int
		wantMeta bool
	}{
		{open: 0, wantMeta: false},
		{open: 1, wantMeta: true},
		{open: 2, wantMeta: false},
	}
	for _, c := range cases {
		out := renderAccordion(secs, c.open, 60, false, 0, 0)
		got := strings.Contains(out, "meta-body-content")
		if got != c.wantMeta {
			t.Errorf("open=%d: meta body rendered=%v, want %v (PF-18: Meta exclusive like every other section)", c.open, got, c.wantMeta)
		}
	}
}

// TestRenderAccordionHeaderGutterWidthStableAcrossActiveState guards PF-12
// for section headers. The OLD asymmetry (design-spec.md §15 PF-12 item 1):
// the active branch prefixes "▌" (1 reserved column) then truncates to w-1,
// while the inactive branch has NO prefix and truncates to the FULL w --
// so a header's own title text starts 1 column EARLIER when inactive than
// when active (verified below: fails under the old code, col 6 vs col 7 for
// a "[1] Solo" header at w=40). The fix reserves the SAME 1 column either
// way (" " inactive, "▌" active) so the title always starts at the same
// column regardless of the row's own active state.
func TestRenderAccordionHeaderGutterWidthStableAcrossActiveState(t *testing.T) {
	secs := []accordionSection{{title: "Solo", body: "solo-body"}}
	const w = 40

	activeOut := ansi.Strip(renderAccordion(secs, 1, w, true, 0, 0))    // the only section IS active
	inactiveOut := ansi.Strip(renderAccordion(secs, 1, w, false, 0, 0)) // active=false globally -- no section active

	activeCol := cellCol(t, activeOut, "Solo")
	inactiveCol := cellCol(t, inactiveOut, "Solo")
	if activeCol != inactiveCol {
		t.Errorf("PF-12: header title starts at column %d when active vs column %d when inactive -- gutter must reserve the same 1 column either way", activeCol, inactiveCol)
	}
}

// cellCol locates substr in s (both already ANSI-stripped) and returns its
// starting terminal CELL column (lipgloss.Width of the preceding text) --
// NOT strings.Index's raw byte offset, which would over-count multi-byte
// gutter glyphs like "▌" (3 UTF-8 bytes) against a 1-byte " " and produce a
// false mismatch between two otherwise column-equal renders.
func cellCol(t *testing.T, s, substr string) int {
	t.Helper()
	idx := strings.Index(s, substr)
	if idx < 0 {
		t.Fatalf("substring %q not found in %q", substr, s)
	}
	return lipgloss.Width(s[:idx])
}

// TestRenderAccordionOmitsFieldStripForRelations guards B04 (design-spec.md
// §15 PF-17, bean bt-b0w0, 2026-07-16): the fieldStrip mechanism is REMOVED
// entirely -- a section carrying fields (RELATIONS was fieldStrip's only
// remaining caller; Meta's own field cursor already lived inline via
// metaSectionBody, PF-4) must never render a "Fields:" line, active or not.
// Replaces TestFieldStripGutterWidthStable (DELETED, not adapted -- it
// called fieldStrip() directly, which this task deletes wholesale, design-
// spec.md §15 PF-17 "Muster PF-14/B13-Removal": compiler-verified, no
// caller left standing).
func TestRenderAccordionOmitsFieldStripForRelations(t *testing.T) {
	secs := []accordionSection{
		{title: "Meta", body: "meta-body"},
		{
			title:  "Relations",
			body:   "▷ relation-row-one\n▶ relation-row-two",
			fields: []relationField{{beanID: "a", label: "one"}, {beanID: "b", label: "two"}},
		},
	}
	// Relations (i=1) active, fieldIdx=1 -- exactly the state that used to
	// trigger fieldStrip's own render branch (activeSec && i != 0 && len(s.
	// fields) > 0).
	out := renderAccordion(secs, 2, 60, true, 1, 1)

	if strings.Contains(out, "Fields:") {
		t.Errorf("B04: renderAccordion must never emit a 'Fields:' strip line any more, got: %q", out)
	}
}

// TestRenderAccordionDigitHeaderNumbering guards the ziffern-Header format:
// header lines carry [1]..[3] in order (ansi-stripped, per primitives_test.go
// convention).
func TestRenderAccordionDigitHeaderNumbering(t *testing.T) {
	secs := []accordionSection{
		{title: "One", body: "b1"},
		{title: "Two", body: "b2"},
		{title: "Three", body: "b3"},
	}
	out := ansi.Strip(renderAccordion(secs, 1, 60, false, 0, 0))

	i1 := strings.Index(out, "[1]")
	i2 := strings.Index(out, "[2]")
	i3 := strings.Index(out, "[3]")
	if i1 < 0 || i2 < 0 || i3 < 0 {
		t.Fatalf("expected [1]..[3] digit headers in output, got: %q", out)
	}
	if !(i1 < i2 && i2 < i3) {
		t.Errorf("digit headers out of order: [1]@%d [2]@%d [3]@%d", i1, i2, i3)
	}
}

// TestRenderAccordionEmptySectionsShowsPlaceholder guards the nil/empty input
// path: a placeholder hint renders instead of an empty string or a panic.
func TestRenderAccordionEmptySectionsShowsPlaceholder(t *testing.T) {
	out := renderAccordion(nil, 0, 60, false, 0, 0)
	if strings.TrimSpace(out) == "" {
		t.Fatal("renderAccordion(nil, ...) must render a placeholder hint, got empty string")
	}
}

// TestGlowRenderEmptyBodyReturnsEmpty guards glowRender's empty-input
// short-circuit (port devd editor.go:102-126).
func TestGlowRenderEmptyBodyReturnsEmpty(t *testing.T) {
	if got := glowRender("", 80); got != "" {
		t.Errorf("glowRender(\"\", 80) = %q, want empty string", got)
	}
}

// TestGlowRenderAsciiProfileProducesNoEscapeSequences guards Golden-
// Determinism for future Detail goldens: under the Ascii color profile,
// glowRender's glamour output must never contain a raw ESC[ sequence (notty
// style).
func TestGlowRenderAsciiProfileProducesNoEscapeSequences(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)

	out := glowRender("# Heading\n\nSome **bold** body text.", 80)
	if strings.Contains(out, "\x1b[") {
		t.Errorf("glowRender under Ascii profile must contain no escape sequences, got: %q", out)
	}
	if !strings.Contains(out, "Heading") {
		t.Errorf("glowRender output missing rendered content: %q", out)
	}
}
