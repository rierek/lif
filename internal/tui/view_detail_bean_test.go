package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/data"
)

// TestBeanSectionsBodyEmptyShowsPlaceholder guards the empty-body path: no
// Body field set -> a placeholder, not an empty string (digit-jump 2 must
// always show something).
func TestBeanSectionsBodyEmptyShowsPlaceholder(t *testing.T) {
	beans := []data.Bean{
		{ID: "nobody-1", Title: "No Body", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := panelSections(idx, idx.ByID["nobody-1"], 60)

	if strings.TrimSpace(secs[1].body) == "" {
		t.Fatal("Body section must show a placeholder for an empty bean body, got empty string")
	}
}

// TestBeanSectionsBodyRendersMarkdown guards the glamour wiring: markdown
// heading/bold syntax survives as rendered ANSI under TrueColor.
func TestBeanSectionsBodyRendersMarkdown(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	beans := []data.Bean{
		{ID: "body-1", Title: "Has Body", Status: "todo", Type: "task", Priority: "normal", Body: "# Heading\n\nSome **bold** text."},
	}
	idx := data.NewIndex(beans)
	secs := panelSections(idx, idx.ByID["body-1"], 60)

	if !strings.Contains(secs[1].body, "\x1b[") {
		t.Errorf("Body section under TrueColor must contain ANSI escapes (glamour-rendered), got: %q", secs[1].body)
	}
	if !strings.Contains(secs[1].body, "Heading") {
		t.Errorf("Body section missing rendered heading text: %q", secs[1].body)
	}
}

// TestBeanSectionsRelationsListsParentChildrenBlockingBlockedByInCanonicalOrder
// guards I03: Blocking/BlockedBy resolved beans must render in data.SortBeans
// canonical tier order (Status -> Priority -> Type -> Title), NOT insertion
// order. Children arrives pre-sorted via idx.Children; this test forces
// Blocking to disagree with insertion order (in-progress bean listed second
// in the raw slice, must still render first). Renamed from ...Beziehungen...
// (PF-7, section title RELATIONS).
func TestBeanSectionsRelationsListsParentChildrenBlockingBlockedByInCanonicalOrder(t *testing.T) {
	beans := []data.Bean{
		{ID: "rel-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "rel-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rel-parent",
			Blocking: []string{"rel-block-todo", "rel-block-inprog"}},
		{ID: "rel-child", Title: "Child Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rel-main"},
		// Blocking targets: listed todo-first in Blocking slice above, but
		// in-progress must sort first per data.SortBeans canonical order.
		{ID: "rel-block-todo", Title: "Blocked Todo", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "rel-block-inprog", Title: "Blocked InProgress", Status: "in-progress", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := panelSections(idx, idx.ByID["rel-main"], 60)

	rel := secs[2]
	if rel.title != "RELATIONS" {
		t.Fatalf("section 2 title = %q, want RELATIONS", rel.title)
	}
	if !strings.Contains(rel.body, "Parent Bean") {
		t.Errorf("Relations body missing Parent: %q", rel.body)
	}
	if !strings.Contains(rel.body, "Child Bean") {
		t.Errorf("Relations body missing Children: %q", rel.body)
	}

	// Find the relationField order for the two Blocking targets among
	// rel.fields (Parent is 1 field, Children 1 field, then Blocking x2).
	var blockingIDs []string
	for _, f := range rel.fields {
		if f.beanID == "rel-block-todo" || f.beanID == "rel-block-inprog" {
			blockingIDs = append(blockingIDs, f.beanID)
		}
	}
	if len(blockingIDs) != 2 {
		t.Fatalf("expected 2 Blocking relationFields, got %v", blockingIDs)
	}
	if blockingIDs[0] != "rel-block-inprog" {
		t.Errorf("Blocking field order = %v, want rel-block-inprog first (canonical status tier, not insertion order)", blockingIDs)
	}
}

// TestBeanSectionsRelationsDanglingReferenceShowsUnresolvedNotJumpable
// guards the dangling-BlockedBy path: an ID with no match in idx.ByID renders
// as a muted "(unresolved: <id>)" row with relationField.beanID == "" (Task
// 2's jump-guard checks this). Renamed from ...Beziehungen... (PF-7).
func TestBeanSectionsRelationsDanglingReferenceShowsUnresolvedNotJumpable(t *testing.T) {
	beans := []data.Bean{
		{ID: "dangling-1", Title: "Dangling Bean", Status: "todo", Type: "task", Priority: "normal",
			BlockedBy: []string{"does-not-exist"}},
	}
	idx := data.NewIndex(beans)
	secs := panelSections(idx, idx.ByID["dangling-1"], 60)

	rel := secs[2]
	if !strings.Contains(rel.body, "does-not-exist") {
		t.Errorf("Relations body missing the unresolved reference id: %q", rel.body)
	}
	found := false
	for _, f := range rel.fields {
		if strings.Contains(f.label, "does-not-exist") {
			found = true
			if f.beanID != "" {
				t.Errorf("unresolved relationField.beanID = %q, want empty (not jumpable)", f.beanID)
			}
		}
	}
	if !found {
		t.Fatal("expected a relationField for the unresolved BlockedBy reference")
	}
}

// TestBeanSectionsHistorieShowsCreatedUpdatedETag guards the History
// section: nil CreatedAt/UpdatedAt render a placeholder, never a zero-time
// string ("0001-01-01..."). Section title assertion updated to HISTORY
// (PF-7).
func TestBeanSectionsHistorieShowsCreatedUpdatedETag(t *testing.T) {
	beans := []data.Bean{
		{ID: "hist-nil", Title: "No Timestamps", Status: "todo", Type: "task", Priority: "normal"},
	}
	idx := data.NewIndex(beans)
	secs := panelSections(idx, idx.ByID["hist-nil"], 60)

	if secs[3].title != "HISTORY" {
		t.Fatalf("section 3 title = %q, want HISTORY", secs[3].title)
	}

	histNil := secs[3].body
	if strings.Contains(histNil, "0001-01-01") {
		t.Errorf("Historie must not render Go's zero-time string for nil timestamps: %q", histNil)
	}

	created := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 1, 3, 11, 0, 0, 0, time.UTC)
	beansSet := []data.Bean{
		{ID: "hist-set", Title: "With Timestamps", Status: "todo", Type: "task", Priority: "normal",
			CreatedAt: &created, UpdatedAt: &updated, ETag: "abc123"},
	}
	idx2 := data.NewIndex(beansSet)
	secs2 := panelSections(idx2, idx2.ByID["hist-set"], 60)
	hist := secs2[3].body
	if !strings.Contains(hist, "2026-01-02") {
		t.Errorf("Historie missing CreatedAt: %q", hist)
	}
	if !strings.Contains(hist, "2026-01-03") {
		t.Errorf("Historie missing UpdatedAt: %q", hist)
	}
	if !strings.Contains(hist, "abc123") {
		t.Errorf("Historie missing ETag: %q", hist)
	}
}

// TestBeanSectionsHistorieWrapsLongETagToBodyWidth guards B01 (E2-T1-quality-
// review finding, MANDATORY carry-over into Task 2): historieSectionBody must
// wrap via wrapText -- otherwise a long ETag overflows the Detail pane's
// bordered width.
func TestBeanSectionsHistorieWrapsLongETagToBodyWidth(t *testing.T) {
	beans := []data.Bean{
		{ID: "wrap-hist", Title: "Wrap Hist", Status: "todo", Type: "task", Priority: "normal",
			ETag: strings.Repeat("etagchunk-", 5)},
	}
	idx := data.NewIndex(beans)
	const bodyW = 20
	secs := panelSections(idx, idx.ByID["wrap-hist"], bodyW)

	for _, line := range strings.Split(ansi.Strip(secs[3].body), "\n") {
		if w := lipgloss.Width(line); w > bodyW {
			t.Errorf("Historie line exceeds bodyW=%d (got %d): %q", bodyW, w, line)
		}
	}
}

// --- hangingIndentWrap (B04.3, design-spec.md §15 PF-17, bean bt-b0w0) ---
//
// hangingIndentWrap wraps text to width w, with the FIRST line prefixed by
// prefix (already-styled) and every CONTINUATION line left-padded to match
// the prefix's own VISIBLE width instead of falling back to column 0 -- the
// actual PO-Mockup bug: relationsSectionBody's old blanket wrapText() over
// the whole joined block re-wrapped already-composed rows with no notion of
// their own per-row prefix width, so a long title's continuation wrapped
// back to column 0 and "unterwanderte" the following row's Meta-Spalten
// (glyph/status/ID). Reused verbatim by Task 5/bt-4mo9's Relations-Picker
// (same row shape) -- kept as an independent, well-tested helper per that
// task's own Notes-for-T5 requirement.

// TestHangingIndentWrapShortTitleNoWrap guards the trivial no-wrap path: a
// title that fits on the first line renders as prefix+text, single line, no
// continuation indent logic engaged at all.
func TestHangingIndentWrapShortTitleNoWrap(t *testing.T) {
	got := hangingIndentWrap("t bt-1 ", "Short title", 40)
	want := "t bt-1 Short title"
	if got != want {
		t.Errorf("hangingIndentWrap = %q, want %q", got, want)
	}
}

// TestHangingIndentWrapLongTitleAlignsContinuationUnderPrefixEnd guards the
// actual fix: a title long enough to wrap must have its continuation
// line(s) left-padded with exactly indentW spaces (indentW == the prefix's
// own visible width via lipgloss.Width), never column 0.
func TestHangingIndentWrapLongTitleAlignsContinuationUnderPrefixEnd(t *testing.T) {
	prefix := "t bt-apmy "
	indentW := lipgloss.Width(prefix)
	got := hangingIndentWrap(prefix, "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", 40)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the long title to wrap into >=2 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], prefix) {
		t.Errorf("first line must start with the prefix, got: %q", lines[0])
	}
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != indentW {
			t.Errorf("continuation line %d indent = %d spaces, want %d (prefix width): %q", i+1, gotIndent, indentW, line)
		}
	}
}

// TestHangingIndentWrapVaryingPrefixWidths guards the "PRO ZEILE
// individuell" requirement (design-spec.md §15 PF-17 B04.3): the indent is
// NOT a shared table column across rows -- bean IDs vary in length
// ("<prefix>-<n-chars>"), so a short-ID prefix and a long-ID prefix must
// each produce a continuation indent matching THEIR OWN prefix width.
func TestHangingIndentWrapVaryingPrefixWidths(t *testing.T) {
	shortPrefix := "t bt-1 "
	longPrefix := "t bt-abcdef12 "
	longTitle := "This title is intentionally long enough that it must wrap across more than one line for sure"

	shortOut := hangingIndentWrap(shortPrefix, longTitle, 30)
	longOut := hangingIndentWrap(longPrefix, longTitle, 30)

	shortLines := strings.Split(shortOut, "\n")
	longLines := strings.Split(longOut, "\n")
	if len(shortLines) < 2 || len(longLines) < 2 {
		t.Fatalf("setup: both outputs must wrap, got %d/%d lines", len(shortLines), len(longLines))
	}
	shortIndent := len(shortLines[1]) - len(strings.TrimLeft(shortLines[1], " "))
	longIndent := len(longLines[1]) - len(strings.TrimLeft(longLines[1], " "))
	if shortIndent != lipgloss.Width(shortPrefix) {
		t.Errorf("short-prefix continuation indent = %d, want %d", shortIndent, lipgloss.Width(shortPrefix))
	}
	if longIndent != lipgloss.Width(longPrefix) {
		t.Errorf("long-prefix continuation indent = %d, want %d", longIndent, lipgloss.Width(longPrefix))
	}
	if shortIndent == longIndent {
		t.Errorf("short and long prefix indents must differ (per-row, not a shared column): both = %d", shortIndent)
	}
}

// TestHangingIndentWrapNeverCollapsesBelowMinimumContinuationWidth guards
// the narrow-terminal floor (code sketch, bean bt-b0w0): when w is small
// enough that w-indentW would go below 8, the continuation width floors at
// 8 instead of collapsing to zero/negative -- indent itself still reflects
// the REAL (unclamped) prefix width, only the wrap width is floored.
func TestHangingIndentWrapNeverCollapsesBelowMinimumContinuationWidth(t *testing.T) {
	prefix := "t bt-averylongid1234 " // wider than w below
	indentW := lipgloss.Width(prefix)
	got := hangingIndentWrap(prefix, "some continuation text that needs room to wrap across lines", 10)

	if strings.TrimSpace(got) == "" {
		t.Fatal("hangingIndentWrap must not collapse to empty output on a narrow width")
	}
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected wrapping to still occur at the floored width, got 1 line: %q", got)
	}
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != indentW {
			t.Errorf("continuation line %d indent = %d, want %d (unclamped prefix width)", i+1, gotIndent, indentW)
		}
	}
}

// TestHangingIndentWrapHardBreaksSpacelessLongToken guards F01 (bt-b0w0
// Review-Findings, Fix-Runde 1, medium/blockierend): a single token with NO
// whitespace (URL, German compound, long ID) longer than the continuation
// width must be HARD-broken at the width boundary -- ansi.Wordwrap alone
// only breaks at whitespace and lets such a token overflow the target width
// unbounded (reviewer repro: a 103-char word at w=30 produced one 104-cell
// line). The fix mirrors wrapText's (view.go) exact two-pass pattern:
// ansi.Hardwrap(ansi.Wordwrap(text, contW, ""), contW, true).
func TestHangingIndentWrapHardBreaksSpacelessLongToken(t *testing.T) {
	prefix := "t bt-1 "
	const w = 30
	got := hangingIndentWrap(prefix, strings.Repeat("x", 103), w)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the 103-char spaceless token to hard-break into >=2 lines at w=%d, got 1 line (%d cells wide)", w, lipgloss.Width(got))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("line %d is %d cells wide, want <= %d (spaceless token must hard-break, F01): %q", i, lw, w, line)
		}
	}
}

// TestHangingIndentWrapCJKDoubleWidthTitleStaysWithinWidth guards F02
// (bt-b0w0 Review-Findings, Fix-Runde 1): CJK/Emoji titles are double-width
// per CELL, not per rune -- the wrap math must budget by lipgloss.Width
// (terminal cells), so no rendered line may exceed w even though the RUNE
// count per line is roughly half a Latin title's. CJK prose also carries no
// spaces, so this doubles as the double-width arm of F01's hard-break pin.
func TestHangingIndentWrapCJKDoubleWidthTitleStaysWithinWidth(t *testing.T) {
	prefix := "t bt-1 "
	const w = 30
	title := "漢字のタイトルが長くて折り返しが必要になるほど長い場合の確認テスト🎉🚀"
	got := hangingIndentWrap(prefix, title, w)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the double-width title to wrap into >=2 lines at w=%d, got 1 line (%d cells wide)", w, lipgloss.Width(got))
	}
	indentW := lipgloss.Width(prefix)
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("line %d is %d cells wide, want <= %d (double-width cells must be budgeted per CELL, F02): %q", i, lw, w, line)
		}
		if i > 0 {
			if gotIndent := len(line) - len(strings.TrimLeft(line, " ")); gotIndent != indentW {
				t.Errorf("continuation line %d indent = %d spaces, want %d (prefix width)", i, gotIndent, indentW)
			}
		}
	}
}

// TestHangingIndentWrapIndentMatchesStyledPrefixUnderTrueColor guards F03
// (bt-b0w0 Review-Findings, Fix-Runde 1): the indent computation
// (lipgloss.Width(ansi.Strip(prefix))) must count VISIBLE cells of a REAL,
// ANSI-styled relationRow prefix -- every other hangingIndentWrap test runs
// under the Ascii ambient default (zero ANSI bytes in the prefix), so a
// regression that counted raw bytes instead of stripped cells would slip
// through them. Render-grounded through relationRow itself (the actual
// production prefix construction), under a forced TrueColor profile.
func TestHangingIndentWrapIndentMatchesStyledPrefixUnderTrueColor(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	rel := &data.Bean{ID: "tc-1", Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "task", Priority: "normal"}
	out := relationRow(rel, relationRowMarker(false, 0, 1), 40)

	if !strings.Contains(out, "\x1b[") {
		t.Fatal("setup: TrueColor relationRow output must contain ANSI escapes (the styled-prefix premise of this test)")
	}
	stripped := ansi.Strip(out)
	lines := strings.Split(stripped, "\n")
	if len(lines) < 2 {
		t.Fatalf("setup: expected the long title to wrap into >=2 lines, got 1: %q", stripped)
	}
	titleCol := cellCol(t, lines[0], "Hier")
	for i, line := range lines[1:] {
		gotIndent := len(line) - len(strings.TrimLeft(line, " "))
		if gotIndent != titleCol {
			t.Errorf("continuation line %d indent = %d, want %d (title-start column of the STYLED prefix, F03): %q", i+1, gotIndent, titleCol, line)
		}
	}
}

// --- relationsSectionBody cursor markers + fieldStrip removal (B04,
// design-spec.md §15 PF-17, bean bt-b0w0) ---

// TestRelationsSectionBodyShowsCursorMarkerOnActiveRow guards B04.2: with
// active=true and fieldIdx=1, exactly the row at GLOBAL index 1 (the Child,
// row order Parent=0/Children=1) carries ▶ -- every other row carries ▷.
// Mirrors TestMetaSectionBodyShowsSelectedFieldMarker's own contract for
// Meta (PF-4).
func TestRelationsSectionBodyShowsCursorMarkerOnActiveRow(t *testing.T) {
	beans := []data.Bean{
		{ID: "rsb-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "rsb-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rsb-parent"},
		{ID: "rsb-child", Title: "Child Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "rsb-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["rsb-main"]

	body, fields, _ := relationsSectionBody(idx, main, 60, true, 1) // fieldIdx 1 -> the Child row
	if len(fields) != 2 {
		t.Fatalf("setup: expected 2 relationFields (Parent + Child), got %d", len(fields))
	}
	stripped := ansi.Strip(body)
	if n := strings.Count(stripped, "▶"); n != 1 {
		t.Fatalf("active relationsSectionBody has %d ▶ markers, want exactly 1: %q", n, stripped)
	}
	if n := strings.Count(stripped, "▷"); n != 1 {
		t.Fatalf("active relationsSectionBody has %d ▷ markers, want exactly 1 (the other row): %q", n, stripped)
	}
	var childLine string
	for _, l := range strings.Split(stripped, "\n") {
		if strings.Contains(l, "Child Bean") {
			childLine = l
		}
	}
	if !strings.Contains(childLine, "▶") {
		t.Errorf("▶ marker not on the Child row: %q", childLine)
	}

	inactive, _, _ := relationsSectionBody(idx, main, 60, false, 1)
	if strings.Contains(ansi.Strip(inactive), "▶") {
		t.Error("inactive relationsSectionBody must show no ▶ marker anywhere")
	}
}

// TestRelationsSectionBodyNoLongerRendersFieldsStripLine is the B04.1
// regression pin: the separate "Fields:" strip line is gone -- neither the
// active nor the inactive render path may ever emit that substring again.
func TestRelationsSectionBodyNoLongerRendersFieldsStripLine(t *testing.T) {
	beans := []data.Bean{
		{ID: "nofs-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "nofs-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "nofs-parent"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["nofs-main"]

	for _, active := range []bool{true, false} {
		body, _, _ := relationsSectionBody(idx, main, 60, active, 0)
		if strings.Contains(body, "Fields:") {
			t.Errorf("active=%v: relationsSectionBody must never render a 'Fields:' strip line (B04, removed): %q", active, body)
		}
	}
}

// TestRelationsSectionBodyLongTitleAlignsContinuationUnderTitleStartNotColumnZero
// guards B04.3, the actual PO-Mockup bug: a Children-row title long enough
// to wrap must have its continuation line(s) start under the TITLE's own
// column (not column 0, and not the whole-row indent) -- design-spec.md §15
// PF-17's literal mockup ("t M bt-apmy Hier steht ein langer Titel... / der
// so umbricht, dass die Uebersicht gewahrt ist").
func TestRelationsSectionBodyLongTitleAlignsContinuationUnderTitleStartNotColumnZero(t *testing.T) {
	longTitle := "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist"
	beans := []data.Bean{
		{ID: "wrp-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "wrp-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "wrp-parent"},
		{ID: "wrp-child", Title: longTitle, Status: "todo", Type: "task", Priority: "normal", Parent: "wrp-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["wrp-main"]

	const bodyW = 40
	body, _, _ := relationsSectionBody(idx, main, bodyW, false, 0)
	stripped := ansi.Strip(body)

	groups := strings.Split(stripped, "\n\n")
	children := groups[len(groups)-1]
	lines := strings.Split(children, "\n")
	if lines[0] != "Children" {
		t.Fatalf("setup: last group must be Children, got %q", lines[0])
	}
	if len(lines) < 3 {
		t.Fatalf("expected the long child title to wrap into >=1 continuation line at bodyW=%d, got %d lines: %q", bodyW, len(lines), children)
	}
	titleCol := cellCol(t, lines[1], "Hier")
	for i, cl := range lines[2:] {
		indent := len(cl) - len(strings.TrimLeft(cl, " "))
		if indent == 0 {
			t.Errorf("continuation line %d fell back to column 0 (PO-Mockup B04.3 bug): %q", i, cl)
		}
		if indent != titleCol {
			t.Errorf("continuation line %d indent = %d, want %d (aligned under the title's own start)", i, indent, titleCol)
		}
	}
}

// --- bt-se4q (bt-b0w0-Review Follow-up, B01): numeric active-line index
// instead of activeRelationLine's post-render "▶" glyph rescan ---

// TestRelationsSectionBodyActiveLineIgnoresGlyphEmbeddedInTitle is the RED-
// anchor: a relation whose TITLE itself contains the "▶" active-marker
// glyph, rendered BEFORE the actually active row, must not fool the
// cursor-centering. The OLD activeRelationLine (view_browse_repo.go)
// rescanned the ALREADY-RENDERED lines for the first "▶" substring -- a
// title containing that glyph would win over the real marker.
// relationsSectionBody now hands back the numeric display-line index of the
// active row directly (accumulated while the groups are built, never
// rediscovered by scanning rendered text), so a glyph anywhere inside a
// title can never be mistaken for the marker.
func TestRelationsSectionBodyActiveLineIgnoresGlyphEmbeddedInTitle(t *testing.T) {
	beans := []data.Bean{
		{ID: "gl-parent", Title: "Parent Bean", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "gl-main", Title: "Main Bean", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-parent"},
		// Sorts BEFORE gl-child-b (Title ascending, data.SortBeans) -- its
		// title carries the "▶" glyph the old glyph-rescan keyed on, and it
		// renders as an INACTIVE (▷) row, one line above the real cursor.
		{ID: "gl-child-a", Title: "AAA Glyph ▶ Title", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-main"},
		{ID: "gl-child-b", Title: "BBB Real Child", Status: "todo", Type: "task", Priority: "normal", Parent: "gl-main"},
	}
	idx := data.NewIndex(beans)
	main := idx.ByID["gl-main"]

	// fieldIdx 2 -> GLOBAL row order Parent=0, Children=1(child-a)/2(child-b):
	// the REAL active row is child-b, not the glyph-titled child-a.
	body, fields, activeLine := relationsSectionBody(idx, main, 80, true, 2)
	if len(fields) != 3 {
		t.Fatalf("setup: expected 3 relationFields (Parent + 2 Children), got %d", len(fields))
	}

	lines := strings.Split(body, "\n")
	if activeLine < 0 || activeLine >= len(lines) {
		t.Fatalf("activeLine %d out of range for %d body lines:\n%s", activeLine, len(lines), ansi.Strip(body))
	}
	gotLine := ansi.Strip(lines[activeLine])
	if !strings.HasPrefix(gotLine, "▶ ") {
		t.Fatalf("activeLine %d does not carry the real active marker, got %q\nfull body:\n%s", activeLine, gotLine, ansi.Strip(body))
	}
	if !strings.Contains(gotLine, "BBB Real Child") {
		t.Fatalf("activeLine %d must be the REAL active row (child-b), got %q", activeLine, gotLine)
	}

	// Guard the trap itself: a DIFFERENT, EARLIER line must contain "▶" too
	// (embedded in child-a's title) WITHOUT being marked active -- this is
	// exactly the line the old strings.Contains(l, "▶") scan would have
	// (wrongly) returned first.
	trapLineIdx := -1
	for i, l := range lines {
		if i == activeLine {
			continue
		}
		if strings.Contains(ansi.Strip(l), "▶") {
			trapLineIdx = i
			break
		}
	}
	if trapLineIdx == -1 {
		t.Fatalf("setup: expected an EARLIER line containing the glyph inside a title (the old scan's trap), found none:\n%s", ansi.Strip(body))
	}
	if trapLineIdx >= activeLine {
		t.Fatalf("setup: the glyph-in-title trap line (%d) must render BEFORE the real active line (%d) to prove the old first-match scan would have failed", trapLineIdx, activeLine)
	}
	if strings.HasPrefix(ansi.Strip(lines[trapLineIdx]), "▶ ") {
		t.Fatalf("setup: trap line %q must NOT itself start with the active marker (must be the inactive ▷ row)", ansi.Strip(lines[trapLineIdx]))
	}
}

// panelSection is one panel body of the box-form Detail pane, as the tests in
// this file assert against it.
type panelSection struct {
	title  string
	body   string
	fields []relationField
}

// panelSections builds the Body/Relations/History panel bodies for b the way
// detailBoxForm does; index 0 is unused, so the indices stay 1/2/3 for
// Body/Relations/History.
func panelSections(idx *data.Index, b *data.Bean, bodyW int) []panelSection {
	rel, fields, _ := relationsSectionBody(idx, b, bodyW, false, 0)
	return []panelSection{
		{title: "META"},
		{title: "BODY", body: bodySectionBody(b, bodyW)},
		{title: "RELATIONS", body: rel, fields: fields},
		{title: "HISTORY", body: historieSectionBody(b, bodyW)},
	}
}

// cellCol locates substr in s (both already ANSI-stripped) and returns its
// starting terminal CELL column (lipgloss.Width of the preceding text) --
// NOT strings.Index's raw byte offset, which would over-count multi-byte
// glyphs.
func cellCol(t *testing.T, s, substr string) int {
	t.Helper()
	idx := strings.Index(s, substr)
	if idx < 0 {
		t.Fatalf("substring %q not found in %q", substr, s)
	}
	return lipgloss.Width(s[:idx])
}
