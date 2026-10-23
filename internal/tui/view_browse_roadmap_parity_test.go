package tui

// view_browse_roadmap_parity_test.go — bean beans-6ccp (U4): the TUI roadmap
// list renders through the same row builder and row renderer as lif roadmap,
// without a column header, and keeps the compact rows beside the detail pane.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// listPaneWidth is the cell width the list pane hands its rows (frame removed).
func listPaneWidth(m model) int {
	innerW := m.width - 2
	head, localKeys := m.browseRepoChrome(innerW)
	_, lw, _, _, _ := clickPaneGeometry(m.width, m.height, head, localKeys, m.statusLine(innerW), m.settings.Layout.TreeWidth, m.layout)
	return lw - 2
}

// cliRoadmapLines is what lif roadmap prints for the rows at the given width:
// the CLI builder and the CLI renderer, minus title, rule and legend.
func cliRoadmapLines(items []*item.Item, width int) []string {
	cfg := data.ActiveConfig()
	rows := ui.RoadmapRows(roadmap.Build(cfg, items, true, nil, nil))
	var out []string
	for _, b := range ui.TreeBlocks(rows, width, false, cfg) {
		for _, l := range b {
			out = append(out, strings.TrimRight(ansi.Strip(l), " "))
		}
	}
	return out
}

// roadmapPaneLines are the list pane's content lines of the rendered screen in
// a layout without a detail pane: frames and cursor column removed, expand
// markers blanked (the CLI has none; the marker cells stay blank).
func roadmapPaneLines(m model) []string {
	var out []string
	for _, l := range screenLines(m) {
		if !strings.HasPrefix(l, "││") || !strings.HasSuffix(l, "││") {
			continue
		}
		l = strings.TrimSuffix(strings.TrimPrefix(l, "││"), "││")
		l = string([]rune(l)[1:]) // cursor column: exactly one cell
		l = strings.NewReplacer("▾", " ", "▸", " ").Replace(l)
		out = append(out, strings.TrimRight(l, " "))
	}
	return out
}

// containsInOrder reports whether want occurs, in order, in some line of got
// each (the screen line may carry the detail pane or chrome next to it).
func containsInOrder(got, want []string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && strings.Contains(g, strings.TrimLeft(want[i], " ")) {
			i++
		}
	}
	return i == len(want)
}

// hasLines reports whether want occurs as consecutive lines of got.
func hasLines(got, want []string) bool {
	for i := 0; i+len(want) <= len(got); i++ {
		if strings.Join(got[i:i+len(want)], "\n") == strings.Join(want, "\n") {
			return true
		}
	}
	return false
}

func TestRoadmapLayoutRendersTheRowsOfLifRoadmap(t *testing.T) {
	m := longTitleModel(t)
	m = m.setLayout(layoutRoadmap)
	// The TUI list spends two more cells on the marker column, left of the
	// tree lines; blanked, it is two leading spaces on every line of lif roadmap.
	want := cliRoadmapLines(longTitleBeans(), listPaneWidth(m)-3)
	for i := range want {
		want[i] = "  " + want[i]
	}
	if len(want) < 6 {
		t.Fatalf("fixture too small for a parity check: %q", want)
	}
	got := roadmapPaneLines(m)
	if !hasLines(got, want) {
		t.Errorf("TUI roadmap rows differ from lif roadmap\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestListsRenderNoColumnHeader(t *testing.T) {
	for _, flat := range []bool{false, true} {
		m := longTitleModel(t)
		m.flatView = flat
		for _, l := range screenLines(m) {
			if strings.Contains(l, "S T ID") || strings.Contains(l, "Status Type ID") {
				t.Errorf("flat=%v: column header still rendered: %q", flat, l)
			}
		}
	}
}

func TestCollapsedNodeHidesChildrenAndKeepsItsMarker(t *testing.T) {
	m := longTitleModel(t)
	m.expanded["ep-1"] = false
	lines := roadmapPaneLines(m)
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "tk-1") {
		t.Errorf("collapsed epic still shows its children:\n%s", joined)
	}
	raw := strings.Join(screenLines(m), "\n")
	if !strings.Contains(raw, "▸") || !strings.Contains(raw, "▾") {
		t.Errorf("expected ▾ on the open milestone and ▸ on the collapsed epic:\n%s", raw)
	}
}

func TestCursorHighlightsEveryLineOfAMultiLineBlock(t *testing.T) {
	m := longTitleModel(t)
	m.cursorID = "tk-1"
	lw := listPaneWidth(m)
	var block []string
	for _, b := range m.treeBlocks(m.visibleNodes(), true, lw) {
		if strings.Contains(ansi.Strip(b[0]), "tk-1") {
			block = b
		}
	}
	if len(block) < 2 {
		t.Fatalf("tk-1 must span several lines at this width, got %q", block)
	}
	for i, l := range block {
		if !strings.HasPrefix(ansi.Strip(l), "▌") {
			t.Errorf("line %d of the cursor block lacks the cursor bar: %q", i, ansi.Strip(l))
		}
	}
}

func TestSplitLayoutKeepsCompactRows(t *testing.T) {
	m := longTitleModel(t)
	m = m.setLayout(layoutFilterDetail)
	nodes := m.visibleNodes()
	w := listPaneWidth(m)
	var want []string
	for _, n := range nodes {
		if n.id == "tk-2" {
			for _, l := range treeRowLines(n, m.beanIDPrefix(), w-1) {
				want = append(want, strings.TrimRight(ansi.Strip(l), " "))
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("tk-2 not in the visible nodes")
	}
	if !containsInOrder(screenLines(m), want) {
		t.Errorf("split view lost the compact rows: want %q\n%s", want, strings.Join(screenLines(m), "\n"))
	}
}

// Opening or closing a node must not move a column: the milestone row is the
// same line with its children shown or hidden.
func TestRoadmapColumnsStayPutWhenANodeOpens(t *testing.T) {
	line := func(open bool) string {
		m := longTitleModel(t).setLayout(layoutRoadmap)
		m.expanded["ms-1"], m.expanded["ep-1"] = open, open
		for _, l := range roadmapPaneLines(m) {
			if strings.Contains(l, "ms-1") {
				return l
			}
		}
		t.Fatalf("ms-1 row missing (open=%v)", open)
		return ""
	}
	if closed, open := line(false), line(true); closed != open {
		t.Errorf("milestone row moved when it opened:\nclosed: %q\nopen:   %q", closed, open)
	}
}
