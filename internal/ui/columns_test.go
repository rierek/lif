// internal/ui/columns_test.go
package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

func TestConnectorIsEmptyAtRoot(t *testing.T) {
	r := Row{Depth: 0}
	if got := r.Connector(); got != "" {
		t.Errorf("Connector() = %q, want empty at depth 0", got)
	}
}

func TestConnectorShapes(t *testing.T) {
	cases := []struct {
		name          string
		ancestorsLast []bool
		isLast        bool
		want          string
	}{
		{"first level, has siblings", []bool{true}, false, "├─ "},
		{"first level, last child", []bool{true}, true, "└─ "},
		{"second level under a parent with siblings", []bool{true, false}, false, "│  ├─ "},
		{"second level under a last child", []bool{true, true}, true, "   └─ "},
	}
	for _, c := range cases {
		r := Row{Depth: len(c.ancestorsLast), AncestorsLast: c.ancestorsLast, IsLast: c.isLast}
		if got := r.Connector(); got != c.want {
			t.Errorf("%s: Connector() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestStemContinuesOnlyWhileSiblingsFollow(t *testing.T) {
	withSiblings := Row{Depth: 1, AncestorsLast: []bool{true}, IsLast: false}
	if got := withSiblings.Stem(); got != "│  " {
		t.Errorf("Stem() = %q, want %q for a row with siblings after it", got, "│  ")
	}

	last := Row{Depth: 1, AncestorsLast: []bool{true}, IsLast: true}
	if got := last.Stem(); got != "   " {
		t.Errorf("Stem() = %q, want three spaces for a last child", got)
	}
}

func TestStemAndConnectorHaveTheSameWidth(t *testing.T) {
	// A continuation line must start exactly under its own title, so the stem
	// has to occupy the cells the connector occupied.
	r := Row{Depth: 2, AncestorsLast: []bool{true, false}, IsLast: false}
	if DisplayWidth(r.Stem()) != DisplayWidth(r.Connector()) {
		t.Errorf("stem %d cells, connector %d cells — must match",
			DisplayWidth(r.Stem()), DisplayWidth(r.Connector()))
	}
}

// TestRowsFromFlatItemsResetsAncestryOnShallowerDepth guards the ancestry
// stack against carrying a stale entry forward when a flat list returns to a
// shallower depth. Without the truncation, a grandchild's ancestry would leak
// into its uncle's row.
func TestRowsFromFlatItemsResetsAncestryOnShallowerDepth(t *testing.T) {
	items := []FlatItem{
		{Depth: 0, IsLast: false}, // root, has a sibling root after it
		{Depth: 1, IsLast: false}, // root's first child, has a sibling
		{Depth: 2, IsLast: true},  // grandchild, last at its level
		{Depth: 1, IsLast: true},  // back to depth 1: root's second (last) child
	}
	rows := RowsFromFlatItems(items)

	last := rows[3]
	if len(last.AncestorsLast) != 1 {
		t.Fatalf("returning to depth 1 kept a stale ancestor: AncestorsLast = %v, want length 1",
			last.AncestorsLast)
	}
	if last.AncestorsLast[0] != false {
		t.Errorf("AncestorsLast[0] = %v, want %v (the root's own IsLast)", last.AncestorsLast[0], false)
	}
}

// TestFlatRowsAreAllDepthZero pins down the table form's contract: no tree
// shape survives, every item lands at depth 0 in its original order.
func TestFlatRowsAreAllDepthZero(t *testing.T) {
	items := []*item.Item{{ID: "a"}, {ID: "b"}}
	rows := FlatRows(items)

	if len(rows) != 2 {
		t.Fatalf("FlatRows returned %d rows, want 2", len(rows))
	}
	for i, r := range rows {
		if r.Depth != 0 {
			t.Errorf("row %d: Depth = %d, want 0", i, r.Depth)
		}
		if !r.IsLast {
			t.Errorf("row %d: IsLast = %v, want true", i, r.IsLast)
		}
		if r.Item != items[i] {
			t.Errorf("row %d: Bean pointer mismatch", i)
		}
	}
}

func testRows(titles ...string) []Row {
	rows := make([]Row, 0, len(titles))
	for _, ti := range titles {
		rows = append(rows, Row{Item: &item.Item{
			ID: "beans-abcd", Title: ti, Type: "task", Status: "todo", Priority: "normal",
		}})
	}
	return rows
}

func TestNarrowTerminalKeepsEveryAxisShort(t *testing.T) {
	c := NewColumns(testRows("a short title"), 80, true, config.Default())
	if c.LongStatus || c.LongType || c.LongPrio {
		t.Errorf("at 80 columns nothing should be long: type=%v status=%v prio=%v",
			c.LongType, c.LongStatus, c.LongPrio)
	}
}

func TestStatusIsBoughtBeforeType(t *testing.T) {
	// The buying order is status, then type, then priority. At a width that
	// affords exactly one upgrade, it must be status.
	rows := testRows(strings.Repeat("x", 200))
	for w := 80; w <= 200; w++ {
		c := NewColumns(rows, w, true, config.Default())
		if c.LongType && !c.LongStatus {
			t.Fatalf("at width %d type went long before status", w)
		}
		if c.LongPrio && !c.LongType {
			t.Fatalf("at width %d priority went long before type", w)
		}
	}
}

func TestTitleNeverDropsBelowTheFloorForAnUpgrade(t *testing.T) {
	rows := testRows(strings.Repeat("x", 200))
	for w := 80; w <= 200; w++ {
		c := NewColumns(rows, w, true, config.Default())
		if (c.LongStatus || c.LongType || c.LongPrio) && c.Title < minTitleWidth {
			t.Fatalf("width %d: bought a long form and left the title at %d", w, c.Title)
		}
	}
}

func TestColumnsNeverExceedTheTerminal(t *testing.T) {
	// Progress rows are in the sweep on purpose: Title is width minus every
	// other column's cost, so a sum built only from the fields NewColumns
	// just returned is an identity — it balances no matter what those fields
	// are, even if a whole column's cost silently stopped being subtracted.
	// wantProgress is computed here from the same inputs (progressBarWidth
	// plus the counter text), never read off c.ProgressWidth, so it stays
	// correct even if NewColumns forgets to reserve room for it — which is
	// exactly the prototype's overflow: a milestone row's progress column
	// pushed the line past the terminal because nothing had reserved its
	// width.
	rows := []Row{
		{Item: &item.Item{ID: "beans-abcd", Title: strings.Repeat("x", 200), Type: "milestone"},
			Progress: &Progress{Done: 3, Total: 10}},
		{Item: &item.Item{ID: "beans-abcd", Title: "short", Type: "task"}},
	}
	wantProgress := progressBarWidth + 2 + DisplayWidth(fmt.Sprintf("%d/%d", 3, 10)) // 6 + 2 + 4 = 12
	for w := 60; w <= 200; w++ {
		c := NewColumns(rows, w, true, config.Default())
		total := c.Indent + c.Type + c.Gap + c.ID + c.Gap + c.Title +
			c.Gap + c.Status + c.Gap + c.Prio + c.Gap + wantProgress
		if c.Tags > 0 {
			total += c.Gap + c.Tags
		}
		if total > w {
			t.Errorf("width %d: columns plus the reserved progress column sum to %d", w, total)
		}
	}
}

// TestFixedCostIsPinnedNotJustBalanced pins every column's width at one known
// fixture against numbers worked out independently of NewColumns, so a change
// to Gap or a tags width is caught rather than silently absorbed into a
// bigger or smaller Title. TestColumnsNeverExceedTheTerminal cannot do this:
// Title is defined as "whatever budget() leaves over", so any sum built from
// NewColumns's own output fields balances to the width by construction,
// whatever those fields happen to be — that identity is what let Gap going
// from 2 to 3, or the wide-tags width going from 24 to 28, leave every test
// in this file green.
//
// At width 200 with a 10-cell ID and tags showing, the budget affords every
// upgrade: Indent 0 (depth 0), Type 10 (long), ID 10, Status 11 (long),
// Prio 8 (long), Tags 24 (width >= 120), Gap 2, six gaps (before type, ID,
// title, status, prio, tags). These numbers were worked out by hand from the
// brief's constants, not read back from a run of NewColumns.
func TestFixedCostIsPinnedNotJustBalanced(t *testing.T) {
	rows := testRows(strings.Repeat("x", 200), "short")
	c := NewColumns(rows, 200, true, config.Default())

	wantIndent, wantGap, wantID, wantType, wantStatus, wantPrio, wantTags := 0, 2, 10, 4, 6, 1, 24
	if c.Indent != wantIndent {
		t.Errorf("Indent = %d, want %d", c.Indent, wantIndent)
	}
	if c.Gap != wantGap {
		t.Errorf("Gap = %d, want %d", c.Gap, wantGap)
	}
	if c.ID != wantID {
		t.Errorf("ID = %d, want %d", c.ID, wantID)
	}
	if c.Type != wantType || !c.LongType {
		t.Errorf("Type = %d (long=%v), want %d long", c.Type, c.LongType, wantType)
	}
	if c.Status != wantStatus || !c.LongStatus {
		t.Errorf("Status = %d (long=%v), want %d long", c.Status, c.LongStatus, wantStatus)
	}
	if c.Prio != wantPrio || c.LongPrio {
		t.Errorf("Prio = %d (long=%v), want %d short (no row has a priority)", c.Prio, c.LongPrio, wantPrio)
	}
	if c.Tags != wantTags {
		t.Errorf("Tags = %d, want %d", c.Tags, wantTags)
	}

	// Six gaps: before type, before ID, before title, before status, before
	// prio, before tags. Title is what is left after every other pinned
	// number is subtracted from the terminal width — a literal number, not a
	// re-derivation of budget().
	wantFixed := wantIndent + wantType + wantGap + wantID + wantGap + wantGap +
		wantStatus + wantGap + wantPrio + wantGap + wantTags
	wantTitle := 200 - wantFixed
	if c.Title != wantTitle {
		t.Errorf("Title = %d, want %d (200 minus the independently pinned fixed cost of %d)",
			c.Title, wantTitle, wantFixed)
	}
}

func TestTagsGiveWayBeforeTheTitleIsCrushed(t *testing.T) {
	c := NewColumns(testRows(strings.Repeat("x", 200)), 62, true, config.Default())
	if c.Tags != 0 {
		t.Errorf("at 62 columns the tags column should have been dropped, got %d", c.Tags)
	}
}

func TestIndentFollowsTheDeepestRow(t *testing.T) {
	rows := []Row{
		{Item: &item.Item{ID: "a", Title: "root", Type: "epic"}, Depth: 0},
		{Item: &item.Item{ID: "b", Title: "kid", Type: "task"}, Depth: 1, AncestorsLast: []bool{true}},
		{Item: &item.Item{ID: "c", Title: "gk", Type: "task"}, Depth: 2, AncestorsLast: []bool{true, true}},
	}
	c := NewColumns(rows, 110, false, config.Default())
	if c.Indent != 6 {
		t.Errorf("Indent = %d, want 6 for a depth-2 tree", c.Indent)
	}
}

func TestFlatRowsNeedNoIndent(t *testing.T) {
	c := NewColumns(FlatRows([]*item.Item{{ID: "a", Title: "x", Type: "task"}}), 110, false, config.Default())
	if c.Indent != 0 {
		t.Errorf("Indent = %d, want 0 for flat rows", c.Indent)
	}
}

func TestCounterWidthIsMeasuredNotAssumed(t *testing.T) {
	// A real store reached 131/139 and burst a fixed five-cell counter.
	//
	// The n/m counter itself is digits and a slash — always one byte per
	// cell, so len() and DisplayWidth() can never disagree on it; no fixture
	// could make that half of this test catch a DisplayWidth-for-len swap.
	// The same row-scanning loop also measures the item ID, and IDs are free
	// text (a slug carrying an umlaut is realistic in this codebase), so
	// "beans-über" here is what actually exercises DisplayWidth: 11 bytes,
	// 10 cells, because ü is two UTF-8 bytes but one terminal cell.
	rows := []Row{
		{Item: &item.Item{ID: "beans-über", Title: "m", Type: "milestone"}, Progress: &Progress{Done: 131, Total: 139}},
		{Item: &item.Item{ID: "b", Title: "n", Type: "milestone"}, Progress: &Progress{Done: 0, Total: 5}},
	}
	c := NewColumns(rows, 110, false, config.Default())
	if c.ID != 10 {
		t.Errorf("ID = %d, want 10 cells for \"beans-über\" (11 bytes, 10 cells) — len() would report 11", c.ID)
	}
	if c.Counter != 7 {
		t.Errorf("Counter = %d, want 7 for \"131/139\"", c.Counter)
	}
	if c.ProgressWidth != 6+2+7 {
		t.Errorf("ProgressWidth = %d, want bar(6) + gap(2) + counter(7)", c.ProgressWidth)
	}
}

func TestTypeTextFollowsTheLongFlag(t *testing.T) {
	b := &item.Item{Type: "milestone"}
	short := Columns{LongType: false}
	if got := short.TypeText(b); got != "M" {
		t.Errorf("short type = %q, want %q", got, "M")
	}
	long := Columns{LongType: true}
	if got := long.TypeText(b); got != "milestone" {
		t.Errorf("long type = %q, want %q", got, "milestone")
	}
}

func TestPrioTextHidesNormal(t *testing.T) {
	c := Columns{LongPrio: true}
	if got := c.PrioText(&item.Item{Priority: "normal"}); got != "" {
		t.Errorf("normal priority = %q, want empty", got)
	}
	if got := c.PrioText(&item.Item{Priority: "high"}); got != "high" {
		t.Errorf("high priority = %q, want %q", got, "high")
	}
	short := Columns{LongPrio: false}
	if got := short.PrioText(&item.Item{Priority: "critical"}); got != "‼" {
		t.Errorf("short critical = %q, want %q", got, "‼")
	}
}

func rowsWithTags(title string, tags []string) []Row {
	return []Row{{Item: &item.Item{
		ID: "beans-abcd", Title: title, Type: "task", Status: "todo",
		Priority: "normal", Tags: tags,
	}}}
}

func TestRebalanceMovesUnusedTitleWidthToTags(t *testing.T) {
	rows := rowsWithTags("short", []string{"a-rather-long-tag-name", "second-tag"})
	c := NewColumns(rows, 110, true, config.Default())
	titleBefore, tagsBefore := c.Title, c.Tags

	c.Rebalance(rows)

	if c.Title >= titleBefore {
		t.Errorf("title stayed at %d, want it to shrink from %d", c.Title, titleBefore)
	}
	if c.Tags <= tagsBefore {
		t.Errorf("tags stayed at %d, want them to grow from %d", c.Tags, tagsBefore)
	}
	if c.Title+c.Tags != titleBefore+tagsBefore {
		t.Errorf("rebalance changed the total: %d+%d vs %d+%d",
			c.Title, c.Tags, titleBefore, tagsBefore)
	}
}

func TestRebalanceHandsReleasedTagsWidthToAStarvedTitle(t *testing.T) {
	rows := rowsWithTags(strings.Repeat("x", 300), []string{"tag"})
	c := NewColumns(rows, 110, true, config.Default())
	title, tags := c.Title, c.Tags
	c.Rebalance(rows)
	if c.Title <= title || c.Title+c.Tags != title+tags {
		t.Errorf("title %d->%d tags %d->%d, want released tags width moved to title", title, c.Title, tags, c.Tags)
	}
}

// TestRebalanceGivesNoMoreThanTheTagsNeedOrTheHeaderLabel guards both bounds
// of the shrink branch: tags never grow past what a single "#ab" tag needs
// by content alone, but never shrink below "TAGS" — its own header label —
// either, or the header word would overflow the column it labels
// (beans-kdkp).
func TestRebalanceGivesNoMoreThanTheTagsNeedOrTheHeaderLabel(t *testing.T) {
	rows := rowsWithTags("x", []string{"ab"})
	c := NewColumns(rows, 110, true, config.Default())
	c.Rebalance(rows)
	if c.Tags > 4 {
		t.Errorf("tags grew to %d for a single #ab, want at most 4 (the TAGS header floor)", c.Tags)
	}
	if c.Tags < len(headerTagsLabel) {
		t.Errorf("tags shrank to %d, narrower than its own %q header label", c.Tags, headerTagsLabel)
	}
}

func TestRebalanceIsANoopWithoutTags(t *testing.T) {
	rows := testRows("short")
	c := NewColumns(rows, 110, false, config.Default())
	before := c.Title
	c.Rebalance(rows)
	// Split rather than a compound ||: a mutation that only breaks one half
	// (e.g. the guard fires but something downstream still stamps a tag
	// width in) needs its own assertion to be provable.
	if c.Title != before {
		t.Errorf("rebalance moved the title in a tagless layout: %d -> %d", before, c.Title)
	}
	if c.Tags != 0 {
		t.Errorf("rebalance gave a tagless layout a tag column: %d", c.Tags)
	}
}

// TestRebalanceMeasuresTagWidthInDisplayCells guards against a byte-count
// regression: "beans-über" is 11 bytes but only 10 display cells (ü is a
// single cell but two UTF-8 bytes). The single tag here needs exactly 11
// cells ("#" plus the 10-cell tag) — a literal, not a value read back from
// c. If Rebalance measured with len() instead of DisplayWidth, the tag
// column would land on 12, one cell short of the real content because it
// thinks the tag needs one more byte-cell than it does.
func TestRebalanceMeasuresTagWidthInDisplayCells(t *testing.T) {
	rows := rowsWithTags("x", []string{"beans-über"})
	c := NewColumns(rows, 110, true, config.Default())
	c.Rebalance(rows)
	if c.Tags != 11 {
		t.Errorf("tags = %d, want exactly 11 display cells for #beans-über", c.Tags)
	}
}

// TestRebalanceNeverGrowsTheColumnTotal pins the conservation half of the
// contract for the case Task 7's flagship test got backwards: Title and Tags
// must never sum to more after Rebalance than before it, whether width moved
// between them (case: tags need more) or was released (case: tags need
// less). The expected bound is titleBefore+tagsBefore, captured before
// Rebalance runs — not anything read back from c afterward — so a mutation
// that grows one column without shrinking the other by the same amount
// trips it.
func TestRebalanceNeverGrowsTheColumnTotal(t *testing.T) {
	rows := rowsWithTags("short", []string{
		"a-rather-long-tag-name", "second-tag-that-is-long", "third-tag-also-long",
	})
	c := NewColumns(rows, 110, true, config.Default())
	titleBefore, tagsBefore := c.Title, c.Tags
	total := titleBefore + tagsBefore

	c.Rebalance(rows)

	if c.Title+c.Tags > total {
		t.Errorf("title+tags grew from %d to %d", total, c.Title+c.Tags)
	}
}

// TestRebalanceNeverExceedsTheTerminal is TestColumnsNeverExceedTheTerminal's
// sibling for Rebalance: it sums every column plus every gap — including the
// reserved progress column, the exact term Task 7's flagship test left
// unguarded — and checks the total against c.Width itself, after Rebalance
// has run, across the same width sweep. TestRebalanceNeverGrowsTheColumnTotal
// only proves Title+Tags conservation between the two columns Rebalance
// touches; it says nothing about whether the row as a whole still fits the
// terminal once ID, gaps and the progress column are counted back in. This
// test closes that gap by assertion rather than by inference.
//
// Both rows carry short titles on purpose, so the title floor never claims
// the whole budget the way a 200-cell title would — that leaves Title with
// real spare across most of the sweep, and the second row's three long tags
// need far more than the initial tag allocation, so Rebalance actually
// transfers width from Title to Tags for nearly the whole sweep (checked
// below), not just at one width.
func TestRebalanceNeverExceedsTheTerminal(t *testing.T) {
	rows := []Row{
		{Item: &item.Item{ID: "beans-abcd", Title: "short milestone", Type: "milestone"},
			Progress: &Progress{Done: 3, Total: 10}},
		{Item: &item.Item{ID: "beans-abcd", Title: "short task", Type: "task", Tags: []string{
			"a-rather-long-tag-name", "second-tag-that-is-long", "third-tag-also-long",
		}}},
	}
	wantProgress := progressBarWidth + 2 + DisplayWidth(fmt.Sprintf("%d/%d", 3, 10)) // 6 + 2 + 4 = 12

	moved := false
	for w := 60; w <= 200; w++ {
		c := NewColumns(rows, w, true, config.Default())
		tagsBefore := c.Tags
		c.Rebalance(rows)
		if c.Tags != tagsBefore {
			moved = true
		}

		total := c.Indent + c.Type + c.Gap + c.ID + c.Gap + c.Title +
			c.Gap + c.Status + c.Gap + c.Prio + c.Gap + wantProgress
		if c.Tags > 0 {
			total += c.Gap + c.Tags
		}
		if total > c.Width {
			t.Errorf("width %d: after Rebalance, columns plus the reserved progress column sum to %d, want at most %d", w, total, c.Width)
		}
	}
	if !moved {
		t.Fatal("fixture never exercised Rebalance's transfer across the sweep — test proves nothing")
	}
}

// stripANSI is already defined in markdown_test.go (same package); reused
// here rather than redeclared.

func TestHeaderNamesTheShortForms(t *testing.T) {
	c := NewColumns(testRows("x"), 80, true, config.Default())
	got := stripANSI(c.Header())
	for _, want := range []string{"T", "ID", "TITLE", "S", "P", "TAGS"} {
		if !strings.Contains(got, want) {
			t.Errorf("header %q is missing %q", got, want)
		}
	}
}

func TestHeaderNamesTheLongForms(t *testing.T) {
	rows := testRows("x")
	rows[0].Item.Priority = "high"
	c := NewColumns(rows, 160, true, config.Default())
	got := stripANSI(c.Header())
	for _, want := range []string{"TYPE", "STATUS", "PRIORITY"} {
		if !strings.Contains(got, want) {
			t.Errorf("header %q is missing %q", got, want)
		}
	}
}

func TestHeaderMatchesTheColumnWidths(t *testing.T) {
	c := NewColumns(testRows("x"), 110, true, config.Default())
	if w := DisplayWidth(stripANSI(c.Header())); w > c.Width {
		t.Errorf("header is %d cells, terminal is %d", w, c.Width)
	}
}

// TestHeaderHasNoTrailingWhitespace guards the TrimRight call in Header(): the
// last cell (TAGS, here) is padded to its full column width like any other,
// so without the trim the header would end in a run of spaces that is
// invisible in a diff but glaring next to a flat raster in a terminal.
func TestHeaderHasNoTrailingWhitespace(t *testing.T) {
	c := NewColumns(testRows("x"), 80, true, config.Default())
	plain := stripANSI(c.Header())
	if plain != strings.TrimRight(plain, " ") {
		t.Errorf("header has trailing whitespace: %q", plain)
	}
}

// TestHeaderHasNoTrailingWhitespaceWithProgressNoTags covers the branch
// TestHeaderHasNoTrailingWhitespace does not: every existing trailing-
// whitespace test uses showTags=true, so TAGS is always the last cell and
// the tag-absent paths — PRIORITY or PROGRESS as the last cell — are never
// exercised. Header() applies TrimRight to the whole joined string
// regardless of which cell ends it, so this is currently safe, but a future
// change that moves the trim into the tag-cell branch specifically would
// break exactly this case while every existing test stayed green.
//
// showTags=false alone is not enough to prove the point: PRIORITY's column
// width is always exactly its own label's width (1 for "P", 8 for
// "PRIORITY"), so it never carries trailing padding to begin with and a
// missing trim would not show up there. A progress-carrying row is what
// forces a genuinely padded last cell without tags: ProgressWidth
// (progressBarWidth + gap + counter) is wider than "PROGRESS" itself, so
// PROGRESS is padded, and it is the last cell when tags are off.
func TestHeaderHasNoTrailingWhitespaceWithProgressNoTags(t *testing.T) {
	rows := []Row{{
		Item:     &item.Item{ID: "beans-abcd", Title: "x", Type: "task", Status: "todo", Priority: "normal"},
		Progress: &Progress{Done: 3, Total: 10},
	}}
	c := NewColumns(rows, 80, false, config.Default())
	if c.ProgressWidth == 0 {
		t.Fatal("fixture does not carry a progress column — adjust it")
	}
	if c.Tags != 0 {
		t.Fatal("fixture unexpectedly has a tags column — PROGRESS would not be last")
	}
	plain := stripANSI(c.Header())
	if plain != strings.TrimRight(plain, " ") {
		t.Errorf("header has trailing whitespace with PROGRESS as the last cell: %q", plain)
	}
}

// TestHeaderExactLayoutWithMultibyteID pins the header to an exact string
// built entirely from literals, not from any field read back off c — a
// mutation that reorders columns, drops the gap, or forgets the trailing
// trim changes the rendered header without changing the sum NewColumns
// already guarantees, which is exactly the failure mode that made an
// earlier task's overflow test unable to go red.
//
// The ID is "beans-über": 11 bytes, but 10 display cells because ü is a
// single-width rune. Using it here — rather than an ASCII ID of the same
// visual width — means a header built on len() instead of DisplayWidth
// would compute idWidth as 11, shift every column after it, and no longer
// match the literal below.
func TestHeaderExactLayoutWithMultibyteID(t *testing.T) {
	rows := []Row{{Item: &item.Item{
		ID: "beans-über", Title: "x", Type: "task", Status: "todo", Priority: "normal",
	}}}
	c := NewColumns(rows, 80, true, config.Default())
	got := stripANSI(c.Header())

	// Hand-derived from the documented algorithm at width 80, showTags true:
	// Type=1, ID=10 (DisplayWidth of "beans-über"), Status=1, Prio=1 all stay
	// short (the first upgrade already costs more than the 80-cell budget
	// allows past minTitleWidth); Tags=18 because width < 120; Title is what
	// remains of the budget once the fixed columns and five 2-space gaps are
	// subtracted. The trailing padding TrimRight removes from the last
	// (TAGS) cell is why the literal ends at "TAGS", not "TAGS" plus spaces.
	want := "T" + "  " +
		"ID" + strings.Repeat(" ", 8) + "  " +
		"TITLE" + strings.Repeat(" ", 34) + "  " +
		"S" + "  " +
		"P" + "  " +
		"TAGS"

	if got != want {
		t.Errorf("Header() =\n%q\nwant\n%q", got, want)
	}
}

func TestLegendIsEmptyWhenNothingIsShortened(t *testing.T) {
	c := NewColumns(testRows("x"), 200, false, config.Default())
	if !c.LongType || !c.LongStatus || !c.LongPrio {
		t.Skip("200 columns did not buy every long form; adjust the fixture")
	}
	if got := c.Legend(config.Default()); len(got) != 0 {
		t.Errorf("Legend = %#v, want empty when nothing is short", got)
	}
}

func TestLegendNamesEveryShortenedAxis(t *testing.T) {
	c := NewColumns(testRows(strings.Repeat("x", 200)), 80, true, config.Default())
	lines := c.Legend(config.Default())
	joined := stripANSI(strings.Join(lines, "\n"))

	for _, want := range []string{"type", "status", "priority"} {
		if !strings.Contains(joined, want) {
			t.Errorf("legend is missing the %s axis:\n%s", want, joined)
		}
	}
	for _, want := range []string{"milestone", "draft", "critical"} {
		if !strings.Contains(joined, want) {
			t.Errorf("legend does not name %q:\n%s", want, joined)
		}
	}
}

func TestLegendOmitsAxesThatAreAlreadyLong(t *testing.T) {
	c := NewColumns(testRows(strings.Repeat("x", 200)), 120, true, config.Default())
	if !c.LongStatus {
		t.Skip("120 columns did not buy the long status; adjust the fixture")
	}
	joined := stripANSI(strings.Join(c.Legend(config.Default()), "\n"))
	if strings.Contains(joined, "in-progress") {
		t.Errorf("legend explains the status axis though it is written out:\n%s", joined)
	}
}

// TestLegendWrapsInsteadOfOverflowing guards the defect this task fixes:
// Legend built each line as a label plus every entry joined by " · " with no
// regard for c.Width at all, so on any terminal narrower than roughly 68
// cells — the width the five default types need on one line — the legend ran
// past the right edge. The sweep below starts at 80 (wide enough that no
// wrapping should be needed) and goes well under 68, into the range where
// the old code actually overflowed, down to 20.
func TestLegendWrapsInsteadOfOverflowing(t *testing.T) {
	widths := []int{80, 60, 50, 40, 30, 20}
	for _, width := range widths {
		c := NewColumns(testRows(strings.Repeat("x", 200)), width, true, config.Default())
		lines := c.Legend(config.Default())
		for _, line := range lines {
			plain := stripANSI(line)
			if plain == "" {
				continue // the blank separator line Legend prepends
			}
			got := DisplayWidth(plain)
			if got <= c.Width {
				continue
			}
			if !strings.Contains(plain, "·") {
				// A single entry is never split mid-word — the same
				// exception WrapText documents for an unbreakable word. At
				// pathologically narrow widths one entry alone (lead plus
				// code plus name) can still exceed width; that is
				// unavoidable without either truncating the mapping away
				// or breaking a name mid-character, both worse than a
				// slightly wide lone line.
				continue
			}
			t.Errorf("width %d: legend line %d cells wide, want <= %d: %q",
				width, got, c.Width, plain)
		}
	}
}

// TestLegendContinuationLineIndentsUnderTheEntries guards the specific shape
// the wrap must take: a continuation line carries the same 9 cells of lead
// the axis label occupies on the first line, so entries stay aligned under
// entries rather than sliding left under the label.
func TestLegendContinuationLineIndentsUnderTheEntries(t *testing.T) {
	c := NewColumns(testRows(strings.Repeat("x", 200)), 30, true, config.Default())
	lines := c.Legend(config.Default())

	sawContinuation := false
	for _, line := range lines {
		plain := stripANSI(line)
		if plain == "" || strings.HasPrefix(plain, "type") ||
			strings.HasPrefix(plain, "status") || strings.HasPrefix(plain, "priority") {
			continue
		}
		sawContinuation = true
		if !strings.HasPrefix(plain, strings.Repeat(" ", 9)) {
			t.Errorf("continuation line does not carry the 9-cell lead: %q", plain)
		}
	}
	if !sawContinuation {
		t.Fatal("width 30 with the full default type/status/priority lists produced no continuation line — adjust the fixture")
	}
}

// TestLegendUsesTheCellsColours guards against a vacuous colour assertion:
// under `go test` there is no tty, so lipgloss suppresses ANSI entirely and
// a plain "contains this escape sequence" check would pass whether or not
// the code colours anything. withTrueColor (declared in markdown_test.go)
// forces real colour output for the duration of this test, so the exact
// styled substrings below only appear if Legend actually applies the same
// foreground/bold styling the table cells use for that axis.
func TestLegendUsesTheCellsColours(t *testing.T) {
	withTrueColor(t)
	c := NewColumns(testRows(strings.Repeat("x", 200)), 80, true, config.Default())
	joined := strings.Join(c.Legend(config.Default()), "\n")

	// milestone: bold mauve, per DefaultTypes' Color/Emphasis.
	wantMilestone := lipgloss.NewStyle().Foreground(ResolveColor("mauve")).Bold(true).Render("M")
	if !strings.Contains(joined, wantMilestone) {
		t.Errorf("legend does not render the milestone type in its cell colour (bold mauve):\n%q", joined)
	}

	// critical: bold red, per DefaultPriorities' Color and the critical/high
	// bold rule.
	wantCritical := lipgloss.NewStyle().Foreground(ResolveColor("red")).Bold(true).Render("‼")
	if !strings.Contains(joined, wantCritical) {
		t.Errorf("legend does not render the critical priority in its cell colour (bold red):\n%q", joined)
	}

	// task: no configured colour, so styleFor renders it with the terminal's
	// own text colour and no bold — not some colour ResolveColor("") happens
	// to resolve to. A legend entry styled any other way would name "task"
	// with a swatch the actual cell never shows (beans-5451).
	wantTask := lipgloss.NewStyle().Render("T")
	if !strings.Contains(joined, wantTask) {
		t.Errorf("legend does not render the task type unstyled, matching the cell (beans-5451):\n%q", joined)
	}
	wrongTask := lipgloss.NewStyle().Foreground(ResolveColor("")).Render("T")
	if wrongTask != wantTask && strings.Contains(joined, wrongTask) {
		t.Errorf("legend renders the task type in ResolveColor(\"\")'s colour instead of unstyled:\n%q", joined)
	}
}

// TestWrapLegendLineEmitsNothingForNoEntries guards the unconditional
// trailing flush this replaces: with an empty entry list the loop never
// runs, so entriesOnLine stays 0 and the old code still flushed once,
// emitting a line carrying only the axis label and no entries (beans-rxyz).
func TestWrapLegendLineEmitsNothingForNoEntries(t *testing.T) {
	lines := wrapLegendLine("priority", nil, nil, 80)
	if len(lines) != 0 {
		t.Errorf("wrapLegendLine(nil, nil) = %#v, want no lines", lines)
	}
}
