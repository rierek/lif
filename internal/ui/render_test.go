// internal/ui/render_test.go
package ui

import (
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

func TestTaskCarriesNoColour(t *testing.T) {
	s := styleFor(&item.Item{Type: "task"}, config.Default())
	if s.hasColor {
		t.Error("task should stay in the terminal's own text colour")
	}
	if s.bold {
		t.Error("task should not be emphasised")
	}
}

func TestContainersAreEmphasised(t *testing.T) {
	for _, typ := range []string{"milestone", "epic"} {
		if !styleFor(&item.Item{Type: typ}, config.Default()).bold {
			t.Errorf("%s should be emphasised", typ)
		}
	}
	for _, typ := range []string{"feature", "bug"} {
		if styleFor(&item.Item{Type: typ}, config.Default()).bold {
			t.Errorf("%s should not be emphasised", typ)
		}
	}
}

func TestEveryColouredTypeTintsItsTitle(t *testing.T) {
	// The title is the same thing as the type word; feature and bug went
	// uncoloured once because tinting was tied to emphasis.
	for _, typ := range []string{"milestone", "epic", "feature", "bug"} {
		s := styleFor(&item.Item{Type: typ}, config.Default())
		if !s.hasColor {
			t.Errorf("%s title should carry the type colour", typ)
		}
	}
}

func TestTagCellShowsTheFirstTagEvenWhenItMustBeCut(t *testing.T) {
	// A column holding nothing but "+3" tells the reader nothing.
	got := stripANSI(tagCell([]string{"a-very-long-tag-indeed", "b", "c"}, 12))
	if !strings.HasPrefix(got, "#a-very") {
		t.Errorf("tag cell = %q, want it to start with the elided first tag", got)
	}
}

func TestTagCellMarksTheRemainder(t *testing.T) {
	got := stripANSI(tagCell([]string{"one", "two", "three", "four"}, 10))
	if !strings.Contains(got, "+") {
		t.Errorf("tag cell = %q, want a +N marker", got)
	}
	if DisplayWidth(got) > 10 {
		t.Errorf("tag cell = %q is %d cells, want at most 10", got, DisplayWidth(got))
	}
}

func TestTagCellNeverExceedsItsWidth(t *testing.T) {
	sets := [][]string{
		{"short"},
		{"one", "two"},
		{"a-really-quite-long-tag-name"},
		{"note-intern", "slug-tailwind-upgrade"},
	}
	for _, tags := range sets {
		for w := 1; w <= 30; w++ {
			if got := DisplayWidth(stripANSI(tagCell(tags, w))); got > w {
				t.Errorf("tagCell(%v, %d) is %d cells wide", tags, w, got)
			}
		}
	}
}

func TestProgressCellShowsBarAndCounter(t *testing.T) {
	c := Columns{ProgressWidth: 15, Counter: 7}
	got := stripANSI(progressCell(&Progress{Done: 131, Total: 139}, c))
	if !strings.Contains(got, "131/139") {
		t.Errorf("progress cell = %q, want the counter", got)
	}
	if !strings.Contains(got, "█") {
		t.Errorf("progress cell = %q, want a filled bar", got)
	}
}

func TestProgressCellHandlesZeroTotal(t *testing.T) {
	c := Columns{ProgressWidth: 13, Counter: 5}
	got := stripANSI(progressCell(&Progress{Done: 0, Total: 0}, c))
	if strings.Contains(got, "█") {
		t.Errorf("progress cell = %q, want an empty bar for 0/0", got)
	}
}

// The tests above never assert on colour itself — they strip ANSI and check
// content, so they cannot be fooled by the no-tty vacuous-colour trap, but
// they also never prove any colour is actually applied. The brief specifies
// no test for statusCell or prioCell at all, and cellStyle.render is only
// exercised indirectly through styleFor's returned fields. These companions
// close that gap: withTrueColor forces real ANSI so a missing- or
// wrong-colour bug shows up rather than being silently absorbed by the
// no-tty default.

func TestCellStyleRenderCarriesColourOnlyWhenResolved(t *testing.T) {
	withTrueColor(t)
	// feature is coloured but not emphasised (unlike epic/milestone, which
	// are both) — isolating the two lets this test catch a bug that drops
	// the foreground while bold still leaves its own escape code behind.
	coloured := styleFor(&item.Item{Type: "feature"}, config.Default())
	plain := styleFor(&item.Item{Type: "task"}, config.Default())

	got := coloured.render("feature-42")
	if stripANSI(got) == got {
		t.Errorf("coloured type rendered with no ANSI codes: %q", got)
	}
	if want := "task-1"; plain.render(want) != want {
		t.Errorf("uncoloured, unemphasised style should pass text through unchanged, got %q", plain.render(want))
	}
}

func TestStatusCellColoursVaryByStatus(t *testing.T) {
	withTrueColor(t)
	cfg := config.Default()
	c := Columns{Status: 11}
	codeFor := func(status string) string {
		out := statusCell(&item.Item{Status: status}, c, cfg)
		codes := ansiEscape.FindAllString(out, -1)
		if len(codes) == 0 {
			t.Fatalf("status cell for %q carries no ANSI colour under a true-colour profile: %q", status, out)
		}
		return codes[0]
	}
	if codeFor("todo") == codeFor("in-progress") {
		t.Error("todo and in-progress should not render with the same colour")
	}
}

func TestPrioCellHidesNormalPriority(t *testing.T) {
	// The density decision from the design: normal repeats on nearly every
	// row in the table, so it earns no ink there (unlike the detail view).
	c := Columns{Prio: 3}
	got := prioCell(&item.Item{Priority: "normal"}, c, config.Default())
	if stripANSI(got) != "   " {
		t.Errorf("normal priority = %q, want a blank cell", stripANSI(got))
	}
}

func TestPrioCellColoursVaryByPriority(t *testing.T) {
	withTrueColor(t)
	cfg := config.Default()
	c := Columns{Prio: 3}
	codeFor := func(pri string) string {
		out := prioCell(&item.Item{Priority: pri}, c, cfg)
		codes := ansiEscape.FindAllString(out, -1)
		if len(codes) == 0 {
			t.Fatalf("prio cell for %q carries no ANSI colour under a true-colour profile: %q", pri, out)
		}
		return codes[0]
	}
	// critical and high are both bold, so a mismatch here can only come from
	// colour, not weight — low would also differ from critical on bold alone
	// and mask a hardcoded-colour bug the way it did during mutation testing.
	if codeFor("critical") == codeFor("high") {
		t.Error("critical and high should not render with the same colour")
	}
}

func demoRows() []Row {
	m := &item.Item{ID: "beans-fexy", Title: "0.5 Output alignment", Type: "milestone", Status: "in-progress", Priority: "normal", Tags: []string{"release"}}
	e := &item.Item{ID: "beans-9m0d", Title: "Shared presentation vocabulary", Type: "epic", Status: "in-progress", Priority: "normal", Tags: []string{"ui", "accepted"}}
	t1 := &item.Item{ID: "beans-9zpz", Title: "Extract the colour resolution into one place", Type: "task", Status: "in-progress", Priority: "high"}
	b1 := &item.Item{ID: "beans-wa9y", Title: "Status column drifts by one cell when tags are on", Type: "bug", Status: "todo", Priority: "critical", Tags: []string{"regression"}}
	return []Row{
		{Item: m, Depth: 0, IsLast: false},
		{Item: e, Depth: 1, AncestorsLast: []bool{false}, IsLast: false},
		{Item: t1, Depth: 2, AncestorsLast: []bool{false, false}, IsLast: false},
		{Item: b1, Depth: 2, AncestorsLast: []bool{false, false}, IsLast: true},
	}
}

func TestNoLineExceedsTheWidthInEitherForm(t *testing.T) {
	// The original sweep started at 70 and never reached the tree's title-body
	// floor, which only misfires below width 47 (tags off) or 67 (tags on)
	// for demoRows() — a Task-10-shaped gap: the right property, swept in a
	// range the guarded code never runs in. 40 sits inside that window (and
	// is the exact value traced by hand in review) without falling into the
	// narrower width below 39 where even the fix legitimately still overflows
	// — demoRows()'s longest id is 10 cells wide, and minRenderableTitle is
	// the documented last resort for exactly that case (see columns.go): a
	// terminal too narrow for id+status+priority to fit at all is not this
	// bug, it is the accepted floor doing its job. 20 and 30 were tried first
	// and both land in that legitimately-still-overflowing zone for *both*
	// forms, which is why they are not in this list.
	//
	// Legend() (columns.go) used to be excluded here: its lines were a fixed
	// width regardless of the width argument. wrapLegendLine (beans-whly)
	// made it width-aware, so the legend block is now covered by the same
	// sweep as the row/tree layout instead of being carved out (beans-b0bx).
	for _, form := range []Form{FormTable, FormTree} {
		for _, w := range []int{40, 70, 80, 100, 110, 130, 160} {
			for _, tags := range []bool{false, true} {
				out := Render(demoRows(), form, "Demo", w, tags, config.Default())
				for i, line := range strings.Split(out, "\n") {
					if got := DisplayWidth(stripANSI(line)); got > w {
						t.Errorf("form=%s width=%d tags=%v line %d is %d cells:\n%s",
							form, w, tags, i, got, stripANSI(line))
					}
				}
			}
		}
	}
}

func TestTableFormHasAHeaderAndNoTreeCharacters(t *testing.T) {
	out := stripANSI(Render(demoRows(), FormTable, "Demo", 110, false, config.Default()))
	if !strings.Contains(out, "TITLE") {
		t.Error("table form must carry a header")
	}
	for _, glyph := range []string{"├", "└", "│"} {
		if strings.Contains(out, glyph) {
			t.Errorf("table form must be flat, found %q", glyph)
		}
	}
}

// TestTableFormFlattensBeforeMeasuringColumns guards render.go:185's flat
// argument to NewColumns. demoRows() carries real depth (0, 1, 2, 2), and
// the flat-vs-tree distinction has no tree glyphs to give it away in table
// form (renderTable never calls Connector/Stem) — the only symptom of
// passing rows instead of flat is that NewColumns' Indent (3*maxDepth) is
// folded into the header's TYPE column but not into the row's own type
// cell, so the header silently drifts out of alignment with its column.
func TestTableFormFlattensBeforeMeasuringColumns(t *testing.T) {
	out := stripANSI(Render(demoRows(), FormTable, "Demo", 110, false, config.Default()))
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected at least a title, rule, header and one row, got %d lines:\n%s", len(lines), out)
	}
	header, firstRow := lines[2], lines[3]
	headerID := strings.Index(header, "ID")
	rowID := strings.Index(firstRow, "beans-fexy")
	if headerID < 0 || rowID < 0 {
		t.Fatalf("could not locate ID column in header %q or row %q", header, firstRow)
	}
	if headerID != rowID {
		t.Errorf("header's ID column starts at %d, first row's id at %d — the tree depth carried by demoRows() must be dropped before NewColumns measures, not just hidden by the absence of glyphs", headerID, rowID)
	}
}

func TestTreeFormHasTreeCharactersAndNoHeader(t *testing.T) {
	out := stripANSI(Render(demoRows(), FormTree, "Demo", 110, false, config.Default()))
	if strings.Contains(out, "TITLE") {
		t.Error("tree form promises no columns and must carry no header")
	}
	if !strings.Contains(out, "├─ ") {
		t.Error("tree form must draw its connectors")
	}
}

func TestTreeFormRunsTheConnectorIntoTheTypeWord(t *testing.T) {
	// The connector must not end two columns short of the word it connects.
	out := stripANSI(Render(demoRows(), FormTree, "Demo", 110, false, config.Default()))
	if !strings.Contains(out, "├─ epic") && !strings.Contains(out, "├─ E") {
		t.Errorf("connector does not lead into the type word:\n%s", out)
	}
}

func TestWrappedTitlesKeepTheVerticalLines(t *testing.T) {
	rows := []Row{
		{Item: &item.Item{ID: "a", Title: "root", Type: "epic", Status: "todo"}, Depth: 0, IsLast: false},
		{Item: &item.Item{ID: "b", Title: strings.Repeat("word ", 30), Type: "task", Status: "todo"},
			Depth: 1, AncestorsLast: []bool{false}, IsLast: false},
		{Item: &item.Item{ID: "c", Title: "after", Type: "task", Status: "todo"},
			Depth: 1, AncestorsLast: []bool{false}, IsLast: true},
	}
	out := stripANSI(Render(rows, FormTree, "Demo", 80, false, config.Default()))
	lines := strings.Split(out, "\n")
	found := false
	for i, l := range lines {
		if strings.Contains(l, "├─ ") && i+1 < len(lines) {
			if strings.Contains(lines[i+1], "│") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("a wrapped row severed its branch:\n%s", out)
	}
}

// TestColourSurvivesAWrappedTitleInBothForms guards the prototype regression
// the user caught by eye: only the first line of a wrapped title took the
// type's colour, continuation lines fell back to the terminal's default. All
// other colour-bearing tests in this file strip ANSI first and would not
// notice this; this one deliberately does not.
func TestColourSurvivesAWrappedTitleInBothForms(t *testing.T) {
	withTrueColor(t)
	rows := []Row{
		{Item: &item.Item{ID: "beans-x", Title: strings.Repeat("word ", 30), Type: "epic", Status: "todo"}, Depth: 0, IsLast: true},
	}
	colourBefore := func(t *testing.T, line, target string) string {
		t.Helper()
		idx := strings.Index(line, target)
		if idx < 0 {
			t.Fatalf("could not find %q in line %q", target, line)
		}
		matches := ansiEscape.FindAllString(line[:idx], -1)
		if len(matches) == 0 {
			t.Fatalf("line %q carries no ANSI colour before %q under a true-colour profile", line, target)
		}
		return matches[len(matches)-1]
	}
	for _, form := range []Form{FormTable, FormTree} {
		out := Render(rows, form, "Demo", 40, false, config.Default())
		lines := strings.Split(out, "\n")
		firstIdx := -1
		for i, l := range lines {
			if strings.Contains(l, "beans-x") {
				firstIdx = i
				break
			}
		}
		if firstIdx < 0 || firstIdx+1 >= len(lines) {
			t.Fatalf("form=%s: expected a wrapped title with at least one continuation line:\n%s", form, out)
		}
		first, cont := lines[firstIdx], lines[firstIdx+1]
		firstColour := colourBefore(t, first, "word")
		contColour := colourBefore(t, cont, "word")
		if firstColour != contColour {
			t.Errorf("form=%s: title colour changed across the wrap: first line %q, continuation %q", form, firstColour, contColour)
		}
	}
}

func TestSectionHeadingsAppear(t *testing.T) {
	rows := demoRows()
	rows[2].Section = "No Milestone"
	out := stripANSI(Render(rows, FormTree, "Roadmap", 110, false, config.Default()))
	if !strings.Contains(out, "No Milestone") {
		t.Error("section heading missing")
	}
}

func TestParseForm(t *testing.T) {
	for _, s := range []string{"table", "tree"} {
		if _, ok := ParseForm(s); !ok {
			t.Errorf("ParseForm(%q) rejected a valid form", s)
		}
	}
	if _, ok := ParseForm("grid"); ok {
		t.Error(`ParseForm("grid") accepted an unknown form`)
	}
}

// richRows is the golden-width fixture: a milestone with a progress bar, a
// long-title epic with tags, a critical bug two levels deep, and a plain
// short task — enough shape to exercise the tree stem, the tag column, the
// progress counter and the type/status/priority long-vs-short axes at once.
func richRows() []Row {
	long := "Canary-Instanz sproutling-test — Staged-Rollout & Dogfood auf NAS"
	return []Row{
		{Item: &item.Item{ID: "SPF-fexy", Title: "v0.5.0 — Kind-Administration", Type: "milestone",
			Status: "draft", Priority: "normal", Tags: []string{"rel-0-5-0"}},
			Depth: 0, IsLast: false, Progress: &Progress{Done: 131, Total: 139}},
		{Item: &item.Item{ID: "SPF-9m0d", Title: long, Type: "epic", Status: "in-progress",
			Priority: "high", Tags: []string{"note-intern", "slug-tailwind-upgrade"}},
			Depth: 1, AncestorsLast: []bool{false}, IsLast: false},
		{Item: &item.Item{ID: "SPF-wa9y", Title: "Status column drifts by one cell when tags are on",
			Type: "bug", Status: "todo", Priority: "critical", Tags: []string{"regression"}},
			Depth: 2, AncestorsLast: []bool{false, false}, IsLast: true},
		{Item: &item.Item{ID: "SPF-635g", Title: "short", Type: "task", Status: "completed",
			Priority: "deferred"}, Depth: 1, AncestorsLast: []bool{false}, IsLast: true},
	}
}

// TestNoLineEverExceedsItsTerminal guards the third prototype defect: a line
// that ran past the terminal edge because a *minimum* width was used as an
// actual width. The swept widths straddle the thresholds NewColumns actually
// consults: the inline `width >= 120` that buys the roomy layout (110 sits
// just below, 130 just above), minTitleWidth=45 which gates every long-form
// upgrade, and tagsCrushWidth=25 below which the tags column is dropped —
// reached at 70 and 80 with tags on. The constants in styles.go's
// CalculateResponsiveColumns are NOT in play here: that function serves the
// TUI and neither NewColumns nor Render ever calls it.
func TestNoLineEverExceedsItsTerminal(t *testing.T) {
	widths := []int{70, 80, 100, 110, 130, 160}
	for _, form := range []Form{FormTable, FormTree} {
		for _, w := range widths {
			for _, tags := range []bool{false, true} {
				out := Render(richRows(), form, "Golden", w, tags, config.Default())
				for i, line := range strings.Split(out, "\n") {
					if got := DisplayWidth(stripANSI(line)); got > w {
						t.Errorf("form=%s width=%d tags=%v: line %d is %d cells\n%s",
							form, w, tags, i, got, stripANSI(line))
					}
				}
			}
		}
	}
}

// tableColumnsFor mirrors renderTable's own rows-to-flat step (Table drops
// tree shape, not columns math) so the Columns computed here match exactly
// what Render(..., FormTable, ...) computes internally — a tree-indented
// Columns would consume extra title budget and disagree on which axes are
// long. No column-width or legend decision is reimplemented; only the
// tree-field drop that renderTable itself performs is repeated here so the
// test can call the production Columns.Legend on the same input.
func tableColumnsFor(rows []Row, width int, showTags bool, cfg *config.Config) Columns {
	flat := make([]Row, 0, len(rows))
	for _, r := range rows {
		flat = append(flat, Row{Item: r.Item, Depth: 0, IsLast: true, Section: r.Section, Progress: r.Progress})
	}
	return NewColumns(flat, width, showTags, cfg)
}

// TestLegendAppearsExactlyWhenSomethingIsShort checks the legend through the
// production function that emits it, Columns.Legend, rather than by
// grepping for a type/status name that also appears in the table itself
// whenever that axis renders long-form — that string-sniffing approach
// can never fail, since the name is present in the table's own cell.
func TestLegendAppearsExactlyWhenSomethingIsShort(t *testing.T) {
	for _, w := range []int{70, 80, 100, 110, 130, 160} {
		c := tableColumnsFor(richRows(), w, true, config.Default())
		out := stripANSI(Render(richRows(), FormTable, "Golden", w, true, config.Default()))
		// `out` is stripped, so the needle must be too -- otherwise the
		// comparison is asymmetric and would start failing the moment a
		// test forces a colour profile.
		legend := stripANSI(strings.Join(c.Legend(config.Default()), "\n"))
		shortSomewhere := !c.LongType || !c.LongStatus || !c.LongPrio

		if shortSomewhere {
			if legend == "" {
				t.Fatalf("width %d: an axis is short but Columns.Legend returned nothing", w)
			}
			if !strings.Contains(out, legend) {
				t.Errorf("width %d: an axis is short but the render does not contain what Legend() produced", w)
			}
		} else {
			if legend != "" {
				t.Errorf("width %d: every axis is long but Columns.Legend still produced text: %q", w, legend)
			}
		}
	}
}

// TestProgressCounterNeverWraps guards the counter against being cut, which
// is a different failure from the line overflow TestNoLineEverExceedsItsTerminal
// catches: too narrow a Counter makes the line too WIDE (PadRight never
// truncates), so test 1 catches that on its own. What only this test catches
// is the opposite repair -- someone forcing the column to fit by truncating
// it. Mutation: in progressCell, replace PadRight(..., c.Counter) with
// Truncate(..., c.Counter-1); "131/139" becomes "131/1…", this goes red and
// test 1 stays green because the line got narrower, not wider.
func TestProgressCounterNeverWraps(t *testing.T) {
	for _, w := range []int{80, 110, 160} {
		out := stripANSI(Render(richRows(), FormTable, "Golden", w, false, config.Default()))
		if !strings.Contains(out, "131/139") {
			t.Errorf("width %d: the counter 131/139 did not survive on one line:\n%s", w, out)
		}
	}
}

// TestTagCellIsAlwaysASingleLine carries what the brief's
// TestLongTagsNeverBreakMidWord was aiming at, in the form the current
// implementation makes provable. The defect it guards was real: against the
// 759-item store, tags were torn mid-word across three lines. That is
// impossible now for a structural reason — tagCell truncates and never
// wraps — and this test pins exactly that structure, so a future switch to
// WrapText inside the cell is caught here rather than in someone's terminal.
// Mutation: give tagCell's `cell` closure a WrapText-based body joined with
// "\n" and this goes red; the brief's version stays green.
func TestTagCellIsAlwaysASingleLine(t *testing.T) {
	tagSets := [][]string{
		{"slug-tailwind-upgrade"},
		{"note-intern", "slug-tailwind-upgrade"},
		{"a-very-long-tag-that-cannot-possibly-fit", "b", "c", "d"},
		{"kurz", "auch-kurz"},
	}
	for _, tags := range tagSets {
		for w := 1; w <= 40; w++ {
			got := stripANSI(tagCell(tags, w))
			if strings.Contains(got, "\n") {
				t.Errorf("tagCell(%v, %d) spans more than one line: %q", tags, w, got)
			}
		}
	}
}

// TestLongTagsNeverBreakMidWord from the brief is deliberately not shipped
// here — see task-21-report.md, and TestTagCellIsAlwaysASingleLine above for
// the property it was reaching for. Given richRows() and widths {80,110,160},
// tagCell (render.go) is structurally binary for any tag past the first:
// it renders the tag in full or collapses it behind a "+N" marker, never a
// partial/wrapped form, because its overflow check compares a fully-sized
// candidate against a strictly smaller budget (width-reserve), so a
// truncated candidate can never pass. The first tag, "#note-intern" at 12
// cells, never gets truncated either: the observed non-zero Tags column
// width never drops below 18 for this fixture. No single-line production
// mutation was found that turns a check for a "rade"/"upgrade"
// line-start fragment red without additionally rigging the fixture.

// TestTreeIDColumnIsPaddedLikeTheTable pins a misalignment found in the
// final review: renderTree charged the title budget c.ID -- the widest ID in
// the set -- but rendered the cell as st.render(r.Item.ID) with no PadRight,
// while renderTable padded it correctly. With IDs of differing width in one
// set (this workspace has SPF-, beans- and prefix-less legacy IDs at once)
// everything to the right of a short ID shifted left, and the width was
// still taken from the title rather than used by the ID.
//
// Mutation: drop the PadRight around r.Item.ID in renderTree and this goes
// red; the table stays green, which is the asymmetry that hid the defect.
func TestTreeIDColumnIsPaddedLikeTheTable(t *testing.T) {
	rows := []Row{
		{Item: &item.Item{ID: "beans-wa9y", Title: "long identifier", Type: "epic",
			Status: "todo", Tags: []string{"alpha"}}, Depth: 0, IsLast: false},
		{Item: &item.Item{ID: "old1", Title: "short identifier", Type: "task",
			Status: "todo", Tags: []string{"beta"}}, Depth: 1, AncestorsLast: []bool{false}, IsLast: true},
	}
	out := stripANSI(Render(rows, FormTree, "T", 110, true, config.Default()))

	// Measured in CELLS, not bytes: strings.Index returns a byte offset and
	// the tree connector "└─ " is 7 bytes wide but 3 cells, which reads as a
	// 4-cell misalignment that is not there.
	var cols []int
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			cols = append(cols, DisplayWidth(line[:i]))
		}
	}
	if len(cols) != 2 {
		t.Fatalf("expected two tag cells, found %d in:\n%s", len(cols), out)
	}
	if cols[0] != cols[1] {
		t.Errorf("tag column starts at cell %d and %d: the ID cell is not padded\n%s", cols[0], cols[1], out)
	}
}
