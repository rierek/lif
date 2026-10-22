package tui

// box_form_page_test.go — TDD coverage for bean bt-adkn (jira-style-ui
// experiment, epic bt-vy1q): seitenweises Body-Blaettern in der Box-Form.
// pgup/pgdn scroll the Detail pane by one FULL page (== the pane's own
// visible line budget) instead of the line-at-a-time up/down of bt-ze10.
//
// Load-bearing (SSTD C): paging MUST run through the SAME adjustBoxFormScroll
// mutation point the wheel and up/down use -- no second scroll path, so
// line-wise and page-wise scrolling can never drift on the reset/clamp rules.
// These tests assert exactly that: the page delta is boxFormScrollBounds' own
// reported height, applied via adjustBoxFormScroll, clamped identically.
//
// Reuses box_form_scroll_test.go's fixture (boxFormScrollModel /
// boxFormLongBodyBeans / requireOverflow): tk-2 carries an 80-line Body that
// reliably overflows the Detail pane by more than 2x, so ONE page down never
// itself hits the clamp ceiling (the assertion below depends on that).

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// dotRun extracts the first contiguous run of page-indicator glyphs
// (pageDotFilled/pageDotEmpty) from s, or "" if none -- lets the render tests
// below assert on the indicator without depending on its exact frame column.
func dotRun(s string) string {
	start := strings.IndexAny(s, pageDotFilled+pageDotEmpty)
	if start < 0 {
		return ""
	}
	run := []rune{}
	for _, r := range s[start:] {
		if string(r) == pageDotFilled || string(r) == pageDotEmpty {
			run = append(run, r)
			continue
		}
		break
	}
	return string(run)
}

// TestBoxFormPageDownUpScrollsByPage guards the headline Akzeptanz: while the
// Detail pane is focused, pgdn moves boxFormScroll down by
// exactly one page (the pane's visible line budget) and pgup moves it back --
// both through adjustBoxFormScroll, which records the owning bean.
func TestBoxFormPageDownUpScrollsByPage(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	if total-height < height {
		t.Fatalf("fixture must overflow by >= one full page (total=%d, height=%d) for a page-down to land un-clamped", total, height)
	}

	m.detailFocus = true
	m = step(t, m, keyMsg(tea.KeyPgDown))
	if m.boxFormScroll != height {
		t.Fatalf("boxFormScroll after one pgdn = %d, want exactly one page (%d)", m.boxFormScroll, height)
	}
	if m.boxFormScrollBean != b.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q (paging must record its owning bean via adjustBoxFormScroll)", m.boxFormScrollBean, b.ID)
	}

	m = step(t, m, keyMsg(tea.KeyPgUp))
	if m.boxFormScroll != 0 {
		t.Fatalf("boxFormScroll after pgup back = %d, want 0 (one page up returns to the top)", m.boxFormScroll)
	}
}

// TestBoxFormPageKeysPageBodyAtTreeFocus is the rework Akzeptanz (bean bt-adkn
// B1, PO-Reject 2026-07-21: "wenn ich PageDown/PageUp verwende, dann scrollt
// das gesamte linke pane"): pgup/pgdn must page the visible Body WITHOUT a
// prior tab into the Detail region -- exactly like the mouse wheel, which is
// focus-independent. The DEFAULT focus is the Tree (detailFocus == false), so
// this drives the FULL handleKey at Tree focus. The earlier
// TestBoxFormPageDownUpScrollsByPage set m.detailFocus = true directly, which
// bypassed the routing and is WHY the focus-dependency bug shipped green.
//
// Two things asserted together: (a) the Body pages by one page through
// adjustBoxFormScroll, and (b) the Tree cursor does NOT move -- paging is a
// Detail-viewport action, never a Tree-cursor action, even at Tree focus.
func TestBoxFormPageKeysPageBodyAtTreeFocus(t *testing.T) {
	m := boxFormScrollModel(t)
	if m.detailFocus {
		t.Fatal("setup: expected Tree-focused (detailFocus == false) -- this test guards paging WITHOUT a prior tab")
	}
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	if total-height < height {
		t.Fatalf("fixture must overflow by >= one full page (total=%d, height=%d)", total, height)
	}

	beforeCursor := m.cursorID
	m = step(t, m, keyMsg(tea.KeyPgDown))
	if m.boxFormScroll != height {
		t.Fatalf("boxFormScroll after tree-focused pgdn = %d, want one page (%d) -- paging must not require detailFocus", m.boxFormScroll, height)
	}
	if m.boxFormScrollBean != b.ID {
		t.Fatalf("boxFormScrollBean = %q, want %q (paging must record its owning bean via adjustBoxFormScroll)", m.boxFormScrollBean, b.ID)
	}
	if m.cursorID != beforeCursor {
		t.Fatalf("tree cursorID moved to %q on pgdn, want unchanged %q (paging is a Detail action, never a Tree-cursor move)", m.cursorID, beforeCursor)
	}

	m = step(t, m, keyMsg(tea.KeyPgUp))
	if m.boxFormScroll != 0 {
		t.Fatalf("boxFormScroll after tree-focused pgup back = %d, want 0", m.boxFormScroll)
	}
	if m.cursorID != beforeCursor {
		t.Fatalf("tree cursorID moved to %q on pgup, want unchanged %q", m.cursorID, beforeCursor)
	}
}

// TestBoxFormPageDownClampsAtEnd guards that page-wise scrolling honours the
// SAME clamp ceiling as line-wise/wheel scrolling: repeated pgdn pins at
// total-height and never overshoots.
func TestBoxFormPageDownClampsAtEnd(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	wantMax := total - height

	m.detailFocus = true
	for i := 0; i < 50; i++ {
		m = step(t, m, keyMsg(tea.KeyPgDown))
	}
	if m.boxFormScroll != wantMax {
		t.Fatalf("boxFormScroll after many pgdn = %d, want clamped ceiling %d", m.boxFormScroll, wantMax)
	}
}

// --- Page indicator (part b) ---

// TestBoxFormPageIndexMapsOffsetToPage guards the pure page-arithmetic: count
// is ceil(total/height), the current page is offset/height, and the CLAMPED
// ceiling (offset == total-height) always maps to the LAST page even when
// total is not a whole multiple of height -- so paging to the end lights the
// last dot (bean bt-adkn).
func TestBoxFormPageIndexMapsOffsetToPage(t *testing.T) {
	cases := []struct {
		total, height, offset int
		wantPage, wantCount   int
	}{
		{total: 10, height: 20, offset: 0, wantPage: 0, wantCount: 1}, // fits -> single page
		{total: 102, height: 18, offset: 0, wantPage: 0, wantCount: 6},
		{total: 102, height: 18, offset: 18, wantPage: 1, wantCount: 6},
		{total: 102, height: 18, offset: 72, wantPage: 4, wantCount: 6},
		{total: 102, height: 18, offset: 84, wantPage: 5, wantCount: 6}, // maxOff -> last page (not 4)
		{total: 36, height: 18, offset: 18, wantPage: 1, wantCount: 2},  // exact multiple
	}
	for _, c := range cases {
		page, count := boxFormPageIndex(c.total, c.height, c.offset)
		if page != c.wantPage || count != c.wantCount {
			t.Errorf("boxFormPageIndex(%d,%d,%d) = (%d,%d), want (%d,%d)",
				c.total, c.height, c.offset, page, count, c.wantPage, c.wantCount)
		}
	}
}

// TestBoxFormPageIndicatorTracksPage is the headline Akzeptanz through the REAL
// render pipeline: a long Body shows a dot row (one dot per page) on the pane's
// fixed bottom frame, the FIRST dot filled at the top -- and after paging to
// the end the LAST dot is filled, while the row stays visible throughout
// (bean bt-adkn "Seiten-Indikator sichtbar, auch waehrend des Blaetterns").
func TestBoxFormPageIndicatorTracksPage(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	_, wantCount := boxFormPageIndex(total, height, 0)
	if wantCount < 2 {
		t.Fatalf("fixture must span >= 2 pages (count=%d) for the indicator to be meaningful", wantCount)
	}

	top := dotRun(ansi.Strip(m.View()))
	if len([]rune(top)) != wantCount {
		t.Fatalf("dot run at top = %q (%d dots), want %d dots (one per page)", top, len([]rune(top)), wantCount)
	}
	if strings.Count(top, pageDotFilled) != 1 {
		t.Fatalf("dot run at top = %q, want exactly one filled dot", top)
	}
	if !strings.HasPrefix(top, pageDotFilled) {
		t.Fatalf("dot run at top = %q, want the FIRST dot filled (page 0)", top)
	}

	end := m.adjustBoxFormScroll(b, total) // oversized -> clamps to the last page
	endRun := dotRun(ansi.Strip(end.View()))
	if len([]rune(endRun)) != wantCount {
		t.Fatalf("dot run at end = %q (%d dots), want %d (indicator must stay visible while paging)", endRun, len([]rune(endRun)), wantCount)
	}
	if !strings.HasSuffix(endRun, pageDotFilled) {
		t.Fatalf("dot run at end = %q, want the LAST dot filled (paged to the end)", endRun)
	}
}

// paneLines renders the box-form Detail pane in isolation at
// the given scroll offset, using the SAME accW/height the live pipeline hands
// renderDetailBox, and returns its lines ANSI-stripped. Line 0 is the
// outer ╭ top border, line len-1 the outer ╰ bottom border, the rest content.
func paneLines(t *testing.T, m model, b *data.Bean, boxScroll int) []string {
	t.Helper()
	accW, height := boxFormPaneMetrics(m, b)
	pane := renderDetailBox(m.idx, b, accW+2, height, true, boxScroll, -1)
	raw := strings.Split(pane, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		out[i] = ansi.Strip(l)
	}
	return out
}

// bodyHeaderLine returns the rendered pane line that is the Body panel's title
// row ("Body … (e)"), or "" if none is visible.
func bodyHeaderLine(lines []string) string {
	for _, l := range lines {
		if strings.Contains(l, "Body") && strings.Contains(l, "(e)") {
			return l
		}
	}
	return ""
}

// TestBoxFormBodyHeaderCarriesPageDots guards bean bt-adkn Rework B3 (PO
// decision "Sticky Body-Kopfzeile", 2026-07-21): the page indicator rides in
// the BODY panel's title row ("Body … (e)"), not on the outer pane frame. So a
// long body shows the dot run in the Body header, and the outer bottom border
// carries NO dots at all.
func TestBoxFormBodyHeaderCarriesPageDots(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	_, wantCount := boxFormPageIndex(total, height, 0)
	if wantCount < 2 {
		t.Fatalf("fixture must span >= 2 pages (count=%d)", wantCount)
	}

	lines := paneLines(t, m, b, 0)
	hdr := bodyHeaderLine(lines)
	if hdr == "" {
		t.Fatal("no Body header line visible at offset 0 (Title/scalars must not push it off a tall pane)")
	}
	if run := dotRun(hdr); len([]rune(run)) != wantCount {
		t.Fatalf("dot run in Body header = %q (%d), want %d dots (indicator belongs in the Body title row)", run, len([]rune(run)), wantCount)
	} else if !strings.HasPrefix(run, pageDotFilled) {
		t.Fatalf("dot run in Body header = %q, want the FIRST dot filled at offset 0", run)
	}

	bottom := lines[len(lines)-1]
	if run := dotRun(bottom); run != "" {
		t.Fatalf("outer bottom border carries dots %q, want none (indicator moved off the outer frame onto the Body box, B3)", run)
	}
}

// TestBoxFormBodyHeaderStickyWhenPaged guards the STICKY half of B3: once the
// body is paged such that its title row would scroll off the top, the Body
// header is pinned as the FIRST content line of the viewport ("als FIXE Zeile
// oben im Detail-Viewport gerendert … der Body-Text scrollt darunter") -- so
// the section title and its page indicator stay visible while paging.
func TestBoxFormBodyHeaderStickyWhenPaged(t *testing.T) {
	m := boxFormScrollModel(t)
	b := m.focusedBean()
	_, height := requireOverflow(t, m, b)

	// One page down puts the top of the viewport (offset == height) well past
	// the Title/scalar boxes and into the body, so the Body title row has
	// scrolled above the top edge and must be pinned.
	lines := paneLines(t, m, b, height)
	first := lines[1] // line 0 is the outer ╭ border; line 1 is the top content row
	if !strings.Contains(first, "Body") || !strings.Contains(first, "(e)") {
		t.Fatalf("top content row after paging into body = %q, want the pinned Body header (sticky, B3)", first)
	}
	if run := dotRun(first); run == "" {
		t.Fatalf("pinned Body header = %q, want it to still carry the page dots", first)
	}
}

// TestBoxFormBodyIndicatorRendersWhenPagesExceedDotBudget guards the exact gap
// the 80c smoke caught (bean bt-adkn Rework): a body with MORE pages than fit
// as dots in the Body header must still show an indicator -- boxFormPageBadge's
// compact "n/N" fallback -- not be dropped whole. The earlier tests all used a
// few-page fixture on a wide pane, where the dot row fit and this failure mode
// was structurally invisible (the same blind spot that shipped B1 green). Here
// a narrow pane makes the page count exceed the header's dot budget so the n/N
// path is exercised.
func TestBoxFormBodyIndicatorRendersWhenPagesExceedDotBudget(t *testing.T) {
	// A 500-item body yields far more pages than fit as dots, at a pane wide
	// enough that the Body header itself still renders its "(e)" badge cleanly
	// (so the only question under test is whether the indicator survives).
	var body strings.Builder
	for i := 1; i <= 330; i++ {
		fmt.Fprintf(&body, "- BODYLINE%03d\n", i)
	}
	beans := fixtureBeans()
	for i := range beans {
		if beans[i].ID == "tk-2" {
			beans[i].Body = body.String()
		}
	}
	m := fixtureModel(t, beans)
	m = step(t, m, tea.WindowSizeMsg{Width: 90, Height: 20})
	m = focusBean(m, "tk-2")
	b := m.focusedBean()
	total, height := requireOverflow(t, m, b)
	_, count := boxFormPageIndex(total, height, 0)
	accW, _ := boxFormPaneMetrics(m, b)
	if count <= accW-21 {
		t.Skipf("config must make count(%d) exceed the header dot budget(%d) to exercise the n/N fallback", count, accW-21)
	}

	// Page into the body so the header is pinned at the top (a tiny pane may not
	// show it at offset 0), then assert it still carries an indicator.
	hdr := bodyHeaderLine(paneLines(t, m, b, height))
	if hdr == "" {
		t.Fatal("no Body header visible even when paged into the body (sticky pin should surface it)")
	}
	if dotRun(hdr) == "" && !pageCountRe.MatchString(hdr) {
		t.Fatalf("Body header %q carries no page indicator; a many-page body must still show the compact n/N form, not drop it (80c smoke regression)", hdr)
	}
}

// pageCountRe matches boxFormPageBadge's compact "page/count" fallback.
var pageCountRe = regexp.MustCompile(`\d+/\d+`)

// TestBoxFormPageIndicatorAbsentWhenFits guards the no-overflow case: a bean
// whose box-form fits the pane shows NO dot row (count==1) -- so short beans in
// box-form mode stay byte-identical to their pre-bt-adkn render (no golden
// drift for the fits case).
func TestBoxFormPageIndicatorAbsentWhenFits(t *testing.T) {
	m := fixtureModel(t, fixtureBeans()) // default fixture bodies are short
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m = focusBean(m, "tk-1")

	b := m.focusedBean()
	total, height := boxFormScrollBounds(m, b)
	if total > height {
		t.Skipf("fixture tk-1 unexpectedly overflows (total=%d height=%d) -- this test needs a fitting bean", total, height)
	}
	if run := dotRun(ansi.Strip(m.View())); run != "" {
		t.Fatalf("dot run = %q, want none (a fitting box-form must not show a page indicator)", run)
	}
}

// TestBoxFormPageBadgeConstantWidthAcrossPages guards bean bt-adkn US-02 (2nd
// PO-Reject 2026-07-21: "(e) springt weiterhin", narrow terminal): as the page
// number widens while paging (1 -> 50 -> 100), the badge width must stay
// CONSTANT so it neither shifts the (e) badge nor crosses the header's fit
// threshold and drops mid-paging. A variable-width "n/N" fails this; the fix is
// to pad the page number to the width of count.
func TestBoxFormPageBadgeConstantWidthAcrossPages(t *testing.T) {
	const count, budget = 102, 7 // budget holds the widest "100/102"
	w := func(page int) int { return len([]rune(ansi.Strip(boxFormPageBadge(page, count, budget)))) }
	w1, w50, w100 := w(0), w(49), w(99)
	if w1 == 0 {
		t.Fatalf("badge dropped at page 1 (width %d budget %d)", w1, budget)
	}
	if w1 != w50 || w50 != w100 {
		t.Fatalf("badge width not constant across pages: 1->%d 50->%d 100->%d -- a widening page number must not shift or drop the indicator mid-paging", w1, w50, w100)
	}
}
