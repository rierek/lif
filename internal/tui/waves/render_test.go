package waves

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/pkg/item"
)

var update = flag.Bool("update", false, "regenerate golden files")

func TestMain(m *testing.M) {
	// Golden bytes must not depend on the terminal that runs the tests.
	lipgloss.SetColorProfile(termenv.TrueColor)
	theme.SetConfiguredColors(map[string]lipgloss.Color{
		"todo": theme.Blue, "in-progress": theme.Yellow, "completed": theme.Green,
	}, nil, nil)
	os.Exit(m.Run())
}

// named makes a todo item with a readable title.
func named(id, title string, blockedBy ...string) *item.Item {
	it := mk(id, "root", blockedBy...)
	it.Title = title
	return it
}

// render renders and fails on a four-way crossing glyph.
func render(t *testing.T, b *Board, o RenderOpts) string {
	t.Helper()
	out := b.Render(o)
	if strings.ContainsRune(out, '┼') {
		t.Fatalf("output contains ┼:\n%s", ansi.Strip(out))
	}
	if again := b.Render(o); again != out {
		t.Fatal("Render is not deterministic")
	}
	return out
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden missing (%s), regenerate with -update: %v", path, err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s.\n--- got ---\n%s\n--- want ---\n%s", path, ansi.Strip(got), ansi.Strip(string(want)))
	}
}

// prefix is the escape sequence that starts the given style.
func prefix(c lipgloss.Color) string {
	s := lipgloss.NewStyle().Foreground(c).Render("x")
	return s[:strings.Index(s, "x")]
}

func lines(s string) []string { return strings.Split(ansi.Strip(s), "\n") }

func TestRenderOneEdge(t *testing.T) {
	b := board(named("t-a", "Design API"), named("t-b", "Build API", "t-a"))
	out := render(t, b, RenderOpts{Width: 80, Height: 10})
	golden(t, "one_edge", out)
	plain := ansi.Strip(out)
	if n := strings.Count(plain, "▶"); n != 1 {
		t.Errorf("%d arrowheads, want 1", n)
	}
	if !strings.Contains(plain, "▶│") {
		t.Error("arrowhead does not stand directly in front of the target card")
	}
}

func TestRenderFocusChangesStrokeWeight(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-c", "Three"), named("t-d", "Four", "t-c"))
	_, rowA := pos(t, b, "t-a")
	_, rowC := pos(t, b, "t-c")
	for _, c := range []struct {
		focus       string
		heavy, thin int // rows y of the heavy and the thin edge
	}{{"t-a", midY(rowA), midY(rowC)}, {"t-c", midY(rowC), midY(rowA)}} {
		out := render(t, b, RenderOpts{Width: 80, Height: 14, Focus: c.focus})
		golden(t, "focus_"+c.focus, out)
		ls := lines(out)
		if !strings.Contains(ls[c.heavy], "━▶") || strings.Contains(ls[c.heavy], "─▶") {
			t.Errorf("focus %s: focus edge row %q should be heavy", c.focus, ls[c.heavy])
		}
		if !strings.Contains(ls[c.thin], "─▶") || strings.Contains(ls[c.thin], "━▶") {
			t.Errorf("focus %s: other edge row %q should be thin", c.focus, ls[c.thin])
		}
	}
}

// colourBefore is the escape sequence in force where sub first appears in line.
func colourBefore(t *testing.T, line, sub string) string {
	t.Helper()
	i := strings.Index(line, sub)
	if i < 0 {
		t.Fatalf("%q not found in %q", sub, line)
	}
	start := strings.LastIndex(line[:i], "\x1b[")
	return line[start : start+strings.Index(line[start:], "m")+1]
}

func TestRenderEdgeColours(t *testing.T) {
	direct := board(named("t-a", "One"), named("t-b", "Two", "t-a"))
	edgeRow := func(b *Board, o RenderOpts) string {
		return strings.Split(render(t, b, o), "\n")[midY(0)]
	}
	if got := colourBefore(t, edgeRow(direct, RenderOpts{Width: 80, Height: 10}), "─▶"); got != prefix(theme.Text) {
		t.Errorf("direct edge colour %q, want Text %q", got, prefix(theme.Text))
	}
	if got := colourBefore(t, edgeRow(direct, RenderOpts{Width: 80, Height: 10, Focus: "t-a"}), "━▶"); got != prefix(theme.Mauve) {
		t.Errorf("focus edge colour %q, want Mauve %q", got, prefix(theme.Mauve))
	}

	a1, b1 := mk("t-a1", "t-a"), mk("t-b1", "t-b", "t-a1")
	lifted := board(named("t-a", "One"), named("t-b", "Two"), a1, b1)
	if got := colourBefore(t, edgeRow(lifted, RenderOpts{Width: 80, Height: 10}), "─▶"); got != prefix(theme.Overlay) {
		t.Errorf("lifted edge colour %q, want Overlay %q", got, prefix(theme.Overlay))
	}
}

func TestRenderContainerShowsNoChildren(t *testing.T) {
	c := named("t-c", "Container epic")
	c.Type = "epic"
	k1, k2 := mk("t-k1", "t-c"), mk("t-k2", "t-c")
	k1.Title, k2.Title = "Child alpha", "Child beta"
	b := board(c, named("t-d", "After", "t-c"), k1, k2)
	out := render(t, b, RenderOpts{Width: 80, Height: 10})
	golden(t, "container", out)
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "Container epic") || strings.Contains(plain, "Child") {
		t.Errorf("container card must show its own title and no child titles:\n%s", plain)
	}
}

func TestRenderCompletedCardIsDimmedButKeepsDot(t *testing.T) {
	a := named("t-a", "Shipped thing")
	a.Status = "completed"
	b := board(a, named("t-b", "Open thing", "t-a"))
	out := render(t, b, RenderOpts{Width: 80, Height: 10})
	golden(t, "completed", out)
	if !strings.Contains(out, lipgloss.NewStyle().Foreground(theme.StatusColor("completed")).Render("●")) {
		t.Error("status dot lost its status colour")
	}
	for _, want := range []string{
		prefix(theme.Surface) + "╭", // frame
		prefix(theme.Overlay) + "Shipped thing",
		prefix(theme.Text) + "Open thing",
		prefix(theme.Overlay) + " t-a ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
}

func TestRenderLongTitleIsCutAfterThreeRows(t *testing.T) {
	long := named("t-a", "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima mike november oscar papa")
	b := board(long)
	out := render(t, b, RenderOpts{Width: 80, Height: 10})
	golden(t, "long_title", out)
	ls := lines(out)
	if len(ls) != 5 {
		t.Fatalf("card is %d rows high, want 5:\n%s", len(ls), strings.Join(ls, "\n"))
	}
	for i := 1; i <= 3; i++ {
		if strings.TrimSpace(ls[i][len("│"):]) == "" {
			t.Errorf("title row %d is empty", i)
		}
	}
	if !strings.Contains(ls[3], "…") || strings.Contains(strings.Join(ls, ""), "papa") {
		t.Errorf("title is not cut after row 3: %q", ls[3])
	}
	if !strings.Contains(ls[4], "t-a") {
		t.Errorf("bottom border lacks the ID: %q", ls[4])
	}
}

func TestRenderLooseColumnSeparator(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-l", "Loose one"))
	out := render(t, b, RenderOpts{Width: 90, Height: 12})
	golden(t, "loose", out)
	line := []rune(lines(out)[0])
	x := -1
	for i, r := range line {
		if r == '┊' {
			x = i
		}
	}
	if x < b.colX[0]+cardW || x >= b.colX[1] {
		t.Errorf("┊ at %d, want inside the gap [%d,%d)", x, b.colX[0]+cardW, b.colX[1])
	}
	if got := strings.Count(ansi.Strip(out), "┊"); got != b.height() {
		t.Errorf("separator covers %d rows, want %d", got, b.height())
	}
}

func TestRenderEdgeAcrossTwoColumns(t *testing.T) {
	b := board(named("t-p", "P"), named("t-x", "X"), named("t-q", "Q", "t-p"), named("t-t", "T", "t-q", "t-x"))
	assertNoEdgeInsideCards(t, b)
	golden(t, "long_edge", render(t, b, RenderOpts{Width: 120, Height: 14}))

	below := board(named("t-p", "P"), named("t-x", "X"), named("t-q", "Q", "t-p"), named("t-r", "R", "t-p"), named("t-t", "T", "t-q", "t-x"))
	assertNoEdgeInsideCards(t, below)
	golden(t, "long_edge_below", render(t, below, RenderOpts{Width: 120, Height: 20}))
}

func TestRenderEntryDirections(t *testing.T) {
	b := board(named("t-p", "P"), named("t-s", "S"), named("t-q", "Q", "t-p", "t-s"), named("t-r", "R", "t-p"))
	golden(t, "entry_directions", render(t, b, RenderOpts{Width: 80, Height: 14}))
}

func TestRenderMarkAndDimmed(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-c", "Three", "t-b"))
	out := render(t, b, RenderOpts{Width: 100, Height: 8, Focus: "t-b", Mark: "t-a", Dimmed: map[string]bool{"t-a": true, "t-c": true}})
	golden(t, "mark_dimmed", out)
	if !strings.Contains(out, prefix(theme.Teal)+"┏") {
		t.Error("marked card has no Teal frame")
	}
	if !strings.Contains(out, prefix(theme.Surface)+"╭") {
		t.Error("dimmed card frame is not dimmed")
	}
	if !strings.Contains(out, prefix(theme.Overlay)+"Three") {
		t.Error("dimmed card title is not dimmed")
	}
}

func TestRenderCriticalPathOnAndOff(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-c", "Three", "t-b"), named("t-d", "Four"), named("t-e", "Five", "t-d"))
	off := render(t, b, RenderOpts{Width: 100, Height: 14, Focus: "t-d"})
	on := render(t, b, RenderOpts{Width: 100, Height: 14, Focus: "t-d", Critical: true})
	golden(t, "critical_off", off)
	golden(t, "critical_on", on)
	red := prefix(theme.Red)
	if strings.Contains(off, red) {
		t.Error("critical path is red while switched off")
	}
	for _, want := range []string{red + "─", red + " t-a ", red + " t-c "} {
		if !strings.Contains(on, want) {
			t.Errorf("critical path on lacks %q", want)
		}
	}
	if strings.Contains(on, red+" t-d ") {
		t.Error("card outside the critical path has a red ID")
	}
}

func TestRenderNarrowViewportFollowsFocusWithMarkers(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-c", "Three", "t-b"), named("t-d", "Four", "t-c"), named("t-e", "Five", "t-d"))
	out := render(t, b, RenderOpts{Width: 60, Height: 7, Focus: "t-c"})
	golden(t, "narrow", out)
	ls := lines(out)
	if len(ls) > 7 {
		t.Fatalf("%d lines, want at most 7", len(ls))
	}
	for _, l := range ls {
		if n := len([]rune(l)); n > 60 {
			t.Errorf("line of %d cells exceeds Width: %q", n, l)
		}
	}
	mid := []rune(ls[len(ls)/2])
	if mid[0] != '◀' || mid[len(mid)-1] != '▶' {
		t.Errorf("middle row lacks the edge markers: %q", string(mid))
	}
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "Two") || !strings.Contains(plain, "Three") || strings.Contains(plain, "One") {
		t.Errorf("focus card is not in the second visible column:\n%s", plain)
	}
}

func TestRenderFirstColumnsStandLeft(t *testing.T) {
	b := board(named("t-a", "One"), named("t-b", "Two", "t-a"), named("t-c", "Three", "t-b"))
	for _, focus := range []string{"t-a", "t-b"} {
		out := render(t, b, RenderOpts{Width: 64, Height: 7, Focus: focus})
		if !strings.Contains(ansi.Strip(out), "One") {
			t.Errorf("focus %s: column 0 is not at the left edge", focus)
		}
	}
	out := render(t, b, RenderOpts{Width: 64, Height: 7, Focus: "t-c"})
	if strings.Contains(ansi.Strip(out), "One") {
		t.Error("focus t-c: column 0 should be cut off")
	}
}

func TestRenderFocusRowsFullyVisibleAndVerticalMarkers(t *testing.T) {
	items := []*item.Item{named("t-p", "P")}
	for _, id := range []string{"t-x1", "t-x2", "t-x3", "t-x4", "t-x5", "t-x6"} {
		items = append(items, named(id, "Child "+id, "t-p"))
	}
	b := board(items...)
	out := render(t, b, RenderOpts{Width: 80, Height: 12, Focus: "t-x4"})
	plain := ansi.Strip(out)
	if !strings.Contains(plain, "t-x4") || !strings.Contains(plain, "Child t-x4") {
		t.Errorf("focus card is not fully visible:\n%s", plain)
	}
	ls := lines(out)
	if len(ls) != 12 || !strings.ContainsRune(ls[0], '▲') || !strings.ContainsRune(ls[11], '▼') {
		t.Errorf("want 12 lines with ▲ on the first and ▼ on the last:\n%s", plain)
	}
}

func TestRenderExternalHints(t *testing.T) {
	a := named("t-a", "One")
	a1 := mk("t-a1", "t-a", "ext", "ext2")
	b := board(a, a1, mk("ext", ""), mk("ext2", ""), mk("ext3", "", "t-a1"))
	plain := ansi.Strip(render(t, b, RenderOpts{Width: 40, Height: 6}))
	if !strings.Contains(plain, "◀2▶1") {
		t.Errorf("hints ◀2▶1 missing:\n%s", plain)
	}
}

func TestGoldensHaveNoCrossingGlyphAndNoForbiddenWord(t *testing.T) {
	word := "wel" + "le"
	var files []string
	for _, pattern := range []string{"*.go", "testdata/*.golden"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(data)), word) {
			t.Errorf("%s contains the forbidden word", f)
		}
		if strings.HasSuffix(f, ".golden") && strings.ContainsRune(string(data), '┼') {
			t.Errorf("%s contains ┼", f)
		}
	}
	if len(files) < 10 {
		t.Errorf("only %d files checked", len(files))
	}
}

func TestRenderBranchKeepsWeightOfEachArm(t *testing.T) {
	// p feeds x straight and y, z below: one shared lane with a junction per target.
	b := board(named("t-p", "P"), named("t-x", "X", "t-p"), named("t-y", "Y", "t-p"), named("t-z", "Z", "t-p"))
	for _, c := range []struct {
		focus string
		want  []rune
	}{
		{"t-y", []rune{'┱', '┡', '╰'}}, // only the arms of p->y are heavy
		{"t-p", []rune{'┳', '┣', '┗'}}, // every arm is heavy
		{"", []rune{'┬', '├', '╰'}},    // every arm is thin
	} {
		plain := ansi.Strip(render(t, b, RenderOpts{Width: 90, Height: 24, Focus: c.focus}))
		for _, r := range c.want {
			if !strings.ContainsRune(plain, r) {
				t.Errorf("focus %q: output lacks %q:\n%s", c.focus, r, plain)
			}
		}
	}
}

func TestRenderHidesRedundantEdge(t *testing.T) {
	b := board(named("t-a", "A"), named("t-b", "B", "t-a"), named("t-c", "C", "t-b", "t-a"))
	plain := ansi.Strip(render(t, b, RenderOpts{Width: 120, Height: 12}))
	if n := strings.Count(plain, "▶"); n != 2 {
		t.Errorf("%d arrowheads, want 2 (the redundant edge is not drawn):\n%s", n, plain)
	}
}

func TestRenderWideRunesKeepCardWidth(t *testing.T) {
	b := board(named("t-a", "日本語のタイトル 漢字 とても長いタイトルです"))
	for i, l := range lines(render(t, b, RenderOpts{Width: 80, Height: 10})) {
		if w := ansi.StringWidth(l); w != cardW {
			t.Errorf("row %d is %d cells wide, want %d: %q", i, w, cardW, l)
		}
	}
}
