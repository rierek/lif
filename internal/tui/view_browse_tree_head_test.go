package tui

// view_browse_tree_head_test.go — bean bt-f68z (PO-Befunde #11/#12/#13):
// the boxed search head, the width-dependent column header, and the
// hanging-indent title wrap, each asserted against the REAL rendered View()
// rather than against the helper that produces it (the render-grounded
// pattern mouse_test.go's own doc comment establishes).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// longTitleBeans is a fixture whose titles are guaranteed to wrap in the
// ~30-cell left pane of a 100-column split -- the condition PO-Befund #13
// describes and every pre-existing fixture (all titles < 15 cells) avoids.
func longTitleBeans() []data.Bean {
	return []data.Bean{
		{ID: "ms-1", Title: "Milestone One", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		{ID: "tk-1", Title: "God Delete Purge crasht ORM nullt child FK statt Cascade", Status: "in-progress", Type: "task", Priority: "high", Parent: "ep-1"},
		{ID: "tk-2", Title: "Canary Instanz sproutling test Staged Rollout und Dogfood auf NAS", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-1"},
	}
}

func longTitleModel(t *testing.T) model {
	t.Helper()
	m := fixtureModel(t, longTitleBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	return m
}

// --- #13: hanging-indent title wrap ---

func TestTreeTitlesWrapWithHangingIndent(t *testing.T) {
	m := longTitleModel(t)
	lines := screenLines(m)

	// The first line of tk-1's row carries the ID; the wrap continuation must
	// follow directly below, indented to the title column and carrying NO id.
	idx := -1
	for i, l := range lines {
		if strings.Contains(l, "tk-1") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("tk-1's row not found in the rendered view")
	}
	head := lines[idx]
	at := strings.Index(head, "God")
	if at < 0 {
		t.Fatalf("tk-1's row does not start the title: %q", head)
	}
	// Columns are CELL widths, not byte offsets -- the glyph/marker columns
	// are multi-byte runes (the mistake leftPaneClickAt's own ansi.StringWidth
	// call already avoids, mouse_test.go).
	titleCol := ansi.StringWidth(head[:at])

	cont := lines[idx+1]
	if strings.Contains(cont, "tk-2") || strings.Contains(cont, "ep-1") {
		t.Fatalf("tk-1's title did not wrap — the next line is already another bean: %q", cont)
	}
	if got := firstWordColumn(cont); got != titleCol {
		t.Errorf("continuation is hanging-indented to column %d, want the title column %d: %q", got, titleCol, cont)
	}
}

// firstWordColumn returns the CELL column of the first alphanumeric rune in a
// rendered (ANSI-stripped) screen line -- i.e. where the content starts once
// the frame borders and the hanging indent are skipped.
func firstWordColumn(line string) int {
	col := 0
	for _, r := range line {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return col
		}
		col += ansi.StringWidth(string(r))
	}
	return -1
}

func TestTreeWrapNeverGrowsTheFrame(t *testing.T) {
	// The parentPickerRowBudget lesson: wrapping must be absorbed by the row
	// WINDOW, never by the frame. Height and width stay pinned at 80x24 --
	// the tightest terminal the project supports.
	m := fixtureModel(t, longTitleBeans())
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m.expanded["ms-1"] = true
	m.expanded["ep-1"] = true
	out := m.View()
	if h := lipgloss.Height(out); h != 24 {
		t.Errorf("wrapping titles changed the frame height: %d, want 24", h)
	}
	for i, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > 80 {
			t.Errorf("line %d overflows 80 columns (%d): %q", i, w, l)
		}
	}
}

// --- #13 follow-up: cursor moves over BEANS, not lines ---

func TestCursorDownMovesOneBeanNotOneLine(t *testing.T) {
	m := longTitleModel(t)
	nodes := m.visibleNodes()
	if len(nodes) < 3 {
		t.Fatalf("fixture produced %d visible nodes, need >= 3", len(nodes))
	}
	m.cursorID = nodes[0].id
	m = m.treeCursorMove(nodes, 1)
	if m.cursorID != nodes[1].id {
		t.Errorf("down moved to %q, want the NEXT BEAN %q — cursor must step over beans, not screen lines", m.cursorID, nodes[1].id)
	}
	// ...including when the bean it leaves occupies several screen lines.
	for i := range nodes {
		if nodes[i].bean != nil && len(treeRowLines(nodes[i], "", 28)) > 1 {
			m.cursorID = nodes[i].id
			m = m.treeCursorMove(nodes, 1)
			if i+1 < len(nodes) && m.cursorID != nodes[i+1].id {
				t.Errorf("down from a MULTI-LINE bean moved to %q, want %q", m.cursorID, nodes[i+1].id)
			}
			break
		}
	}
}

// --- #13 follow-up: the click must still hit the right bean ---

// TestClickOnWrappedContinuationSelectsSameBean is the acceptance the bean
// spells out ("Klick trifft nach Umbruch weiterhin das richtige bean"): a
// click on a title's SECOND line belongs to the bean that title came from,
// not to the bean whose head row happens to sit that many rows down.
func TestClickOnWrappedContinuationSelectsSameBean(t *testing.T) {
	m := longTitleModel(t)
	m.cursorID = "ms-1"

	// "ORM nullt" appears only on a continuation line of tk-1's wrapped title.
	msg := treeClickAt(t, m, "ORM nullt")
	tm, _ := m.handleMouse(msg)
	m2 := tm.(model)
	if m2.cursorID != "tk-1" {
		t.Errorf("click on tk-1's wrapped continuation selected %q, want tk-1", m2.cursorID)
	}
}

// TestClickBelowAWrappedBeanSelectsTheRightBean is the counterpart: with a
// 3-line bean above it, the row/element offset diverges by 2 -- a hit-test
// that still divided screen rows by one would land two beans too early.
func TestClickBelowAWrappedBeanSelectsTheRightBean(t *testing.T) {
	m := longTitleModel(t)
	m.cursorID = "ms-1"

	msg := treeClickAt(t, m, "tk-2")
	tm, _ := m.handleMouse(msg)
	m2 := tm.(model)
	if m2.cursorID != "tk-2" {
		t.Errorf("click on tk-2's row (below a 3-line bean) selected %q, want tk-2", m2.cursorID)
	}
}

func TestClickOnColumnHeaderIsNoOp(t *testing.T) {
	m := longTitleModel(t)
	m.cursorID = "ep-1"
	msg := treeClickAt(t, m, "S T ID")
	tm, _ := m.handleMouse(msg)
	if got := tm.(model).cursorID; got != "ep-1" {
		t.Errorf("a click on the column header moved the cursor to %q — it is not a bean target", got)
	}
}

// --- #12: column header ---

func TestTreeColumnHeaderNarrowUsesKeyLetters(t *testing.T) {
	got := ansi.Strip(treeColumnHeader(30))
	if !strings.Contains(got, "S") || !strings.Contains(got, "Title") {
		t.Errorf("narrow header must abbreviate to the key letters and keep Title: %q", got)
	}
	if strings.Contains(got, "Status") {
		t.Errorf("narrow header must NOT spell Status out (PO: auf die Keybindings kuerzen): %q", got)
	}
}

func TestTreeColumnHeaderWideSpellsOut(t *testing.T) {
	got := ansi.Strip(treeColumnHeader(90))
	if !strings.Contains(got, "Status") || !strings.Contains(got, "Title") {
		t.Errorf("wide header must spell the labels out (PO: Header ausschreiben): %q", got)
	}
}

func TestTreeColumnHeaderNeverExceedsWidth(t *testing.T) {
	for _, w := range []int{4, 10, 17, 30, 47, 48, 90} {
		if got := lipgloss.Width(treeColumnHeader(w)); got > w {
			t.Errorf("treeColumnHeader(%d) is %d cells wide", w, got)
		}
	}
}

func TestTreeColumnHeaderIsRenderedInTheSplit(t *testing.T) {
	m := longTitleModel(t)
	lines := screenLines(m)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "S T ID") && strings.Contains(l, "Title") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("the Tree pane renders no column header (PO-Befund #12)")
	}
}

// --- #11: boxed search head ---

func TestSearchHeadIsBoxedUnderBoxForm(t *testing.T) {
	m := longTitleModel(t)
	lines := screenLines(m)
	found := false
	for _, l := range lines {
		if strings.Contains(l, "─ Search ") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("box-form: the search head is not framed like the other fields (PO-Befund #11)")
	}
}

func TestSearchHeadBoxCarriesHotkeyBadge(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	rows := m.searchHeadBox(30)
	if len(rows) != searchHeadBoxHeight {
		t.Fatalf("searchHeadBox produced %d lines, want %d", len(rows), searchHeadBoxHeight)
	}
	if !strings.Contains(ansi.Strip(rows[2]), "(/)") {
		t.Errorf("the box's bottom frame must carry the / hotkey badge: %q", rows[2])
	}
	for i, r := range rows {
		if w := lipgloss.Width(r); w != 30 {
			t.Errorf("searchHeadBox line %d is %d cells wide, want exactly 30", i, w)
		}
	}
}

func TestSearchHeadTextMatchesTreeSearchLineContent(t *testing.T) {
	// The extraction contract: treeSearchLine's own composition is unchanged,
	// so the shield + the shared text must reproduce it exactly.
	m := fixtureModel(t, fixtureBeans())
	text, state := m.searchHeadText()
	if state != searchHeadIdle {
		t.Fatalf("fresh model should be in the idle search state, got %v", state)
	}
	if !strings.Contains(ansi.Strip(m.treeSearchLine(40, "")), searchShield+" "+text) {
		t.Errorf("treeSearchLine no longer composes shield + searchHeadText(): %q vs %q", m.treeSearchLine(40, ""), text)
	}
}
