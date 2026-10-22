package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/roadmap"
)

const ppLongTitle = "Eine sehr lange Überschrift für das Feature mit Umlauten ÄÖÜ äöü ß und noch mehr Text damit es abgeschnitten wird"

// ppFixture is a store with three hierarchy levels (milestone > epic >
// feature > task), an archived feature, an orphan epic and a long German
// title. Milestones have no valid parent type in the default profile.
func ppFixture(t *testing.T) (*itemgraph.CoreResolver, *config.Config) {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultWithPrefix("pp-")
	core := itemcore.New(itemsDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatal(err)
	}
	mk := func(id, title, typ, status, parent string) *item.Item {
		return &item.Item{ID: id, Slug: item.Slugify(title), Title: title, Type: typ, Status: status, Parent: parent}
	}
	for _, b := range []*item.Item{
		mk("pp-m1", "Alpha release", "milestone", "todo", ""),
		mk("pp-m2", "Beta release", "milestone", "in-progress", ""),
		mk("pp-e1", "Search", "epic", "todo", "pp-m1"),
		mk("pp-e2", "Capture", "epic", "in-progress", "pp-m1"),
		mk("pp-e3", "Orphan epic", "epic", "todo", ""),
		mk("pp-f1", "Index", "feature", "todo", "pp-e1"),
		mk("pp-f2", "Done feature", "feature", "completed", "pp-e1"),
		mk("pp-f3", "Tree", "feature", "todo", "pp-e2"),
		mk("pp-f4", ppLongTitle, "feature", "todo", "pp-e3"),
		mk("pp-t1", "Leaf task", "task", "todo", "pp-f1"),
	} {
		if err := core.Create(b); err != nil {
			t.Fatalf("create %s: %v", b.ID, err)
		}
	}
	return &itemgraph.CoreResolver{Core: core}, cfg
}

// ppRun executes cmd and returns the messages it yields (BatchMsg
// flattened). A command that blocks (cursor blink) is dropped after 150ms.
func ppRun(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- cmd() }()
	select {
	case m := <-ch:
		if b, ok := m.(tea.BatchMsg); ok {
			var out []tea.Msg
			for _, c := range b {
				out = append(out, ppRun(c)...)
			}
			return out
		}
		if m == nil {
			return nil
		}
		return []tea.Msg{m}
	case <-time.After(150 * time.Millisecond):
		return nil
	}
}

// ppSend feeds msgs to f and every message the resulting commands yield.
func ppSend(f *huh.Form, msgs ...tea.Msg) {
	queue := append([]tea.Msg(nil), msgs...)
	for i := 0; len(queue) > 0 && i < 60; i++ {
		m := queue[0]
		queue = queue[1:]
		_, cmd := f.Update(m)
		queue = append(queue, ppRun(cmd)...)
	}
}

func ppInit(f *huh.Form) { ppSend(f, ppRun(f.Init())...) }

func ppKey(k tea.KeyType) tea.Msg { return tea.KeyMsg{Type: k} }

func ppType(f *huh.Form, s string) {
	for _, r := range s {
		ppSend(f, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// ppEnv wires a picker into a form, optionally behind a type select.
type ppEnv struct {
	p      *parentPicker
	f      *huh.Form
	typ    string
	parent string
}

func ppNew(t *testing.T, typ, parent string, w, h int, withTypeStep bool) *ppEnv {
	t.Helper()
	t.Setenv("TERM", "xterm-256color") // TERM=dumb pins huh to width 80
	resolver, cfg := ppFixture(t)
	e := &ppEnv{typ: typ, parent: parent}
	e.p = newParentPicker(context.Background(), resolver, cfg, &e.typ, &e.parent)
	var groups []*huh.Group
	if withTypeStep {
		groups = append(groups, huh.NewGroup(huh.NewSelect[string]().
			Options(huh.NewOptions("task", "feature", "epic", "milestone")...).
			Value(&e.typ)))
	}
	groups = append(groups, e.p.group)
	e.f = huh.NewForm(groups...).
		WithLayout(e.p).
		WithTheme(ui.FormTheme(ui.FlavourFormPalette(ui.ActiveTheme())))
	ppInit(e.f)
	ppSend(e.f, tea.WindowSizeMsg{Width: w, Height: h})
	return e
}

func (e *ppEnv) view() string { return ansi.Strip(e.f.View()) }

// ppRowLines returns the view lines on which the short ID token id appears
// as a row (token followed by a space and a type letter).
func ppRowLines(view, short string) []string {
	re := regexp.MustCompile(`^\s*┃ [▸ ] [├└│─ ]{0,12}` + regexp.QuoteMeta(short) + ` [A-Z?] `)
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if re.MatchString(l) {
			out = append(out, l)
		}
	}
	return out
}

func ppIDsInOrder(view string, shorts []string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		for _, s := range shorts {
			if len(ppRowLines(l, s)) > 0 {
				out = append(out, s)
			}
		}
	}
	return out
}

var ppAll = []string{"m1", "m2", "e1", "e2", "e3", "f1", "f2", "f3", "f4", "t1"}

func TestParentPickerCandidateRowsPerType(t *testing.T) {
	cases := []struct {
		typ  string
		want []string
	}{
		{"task", []string{"m1", "m2", "e1", "e2", "e3", "f1", "f3", "f4"}},
		{"feature", []string{"m1", "m2", "e1", "e2", "e3"}},
		{"epic", []string{"m1", "m2"}},
		{"milestone", nil},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			e := ppNew(t, c.typ, "", 150, 40, false)
			v := e.view()
			if !strings.Contains(v, "(no parent)") {
				t.Fatalf("no \"(no parent)\" option:\n%s", v)
			}
			want := map[string]bool{}
			for _, s := range c.want {
				want[s] = true
			}
			for _, s := range ppAll {
				n := len(ppRowLines(v, s))
				if want[s] && n != 1 {
					t.Errorf("%s: want exactly one row, got %d\n%s", s, n, v)
				}
				if !want[s] && n != 0 {
					t.Errorf("%s: unexpected row (archived, leaf or invalid type)\n%s", s, v)
				}
			}
		})
	}
}

func TestParentPickerRowsHaveTreePrefixShortIDTypeAndTitle(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	v := e.view()
	if l := ppRowLines(v, "m1"); len(l) != 1 || strings.Contains(l[0], "├─") || strings.Contains(l[0], "└─") {
		t.Fatalf("root row m1 must carry no connector: %q", l)
	}
	for _, c := range []struct{ id, typ, title string }{
		{"e1", "E", "Search"}, {"f1", "F", "Index"}, {"m2", "M", "Beta release"},
	} {
		l := ppRowLines(v, c.id)
		if len(l) != 1 || !regexp.MustCompile(`[├└]─ `+c.id+` `+c.typ+` `+c.title).MatchString(l[0]) && c.id != "m2" {
			t.Errorf("row %s: %q", c.id, l)
		}
		if len(l) == 1 && !strings.Contains(l[0], c.id+" "+c.typ+" "+c.title) {
			t.Errorf("row %s lacks short id, type, title: %q", c.id, l[0])
		}
	}
	// f1 sits one level below e1: its connector is indented past e1's.
	f1, e1 := ppRowLines(v, "f1")[0], ppRowLines(v, "e1")[0]
	if strings.Index(f1, "f1") <= strings.Index(e1, "e1") {
		t.Errorf("f1 not indented under e1:\n%s\n%s", e1, f1)
	}
}

func TestParentPickerRowsFollowRoadmapOrder(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	resolver, _ := ppFixture(t)
	all, err := resolver.Items(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := ppIDsInOrder(e.view(), []string{"m1", "m2", "e1", "e2", "e3", "f1", "f3", "f4"})
	var want []string
	inRoadmap := map[string]bool{}
	for _, r := range roadmapRows(roadmap.Build(cfg, all, true, nil, nil)) {
		id := strings.TrimPrefix(r.Item.ID, "pp-")
		inRoadmap[id] = true
		for _, g := range got {
			if g == id {
				want = append(want, id)
			}
		}
	}
	// Containers both show: the roadmap hides the empty milestone m2.
	var shared []string
	for _, g := range got {
		if inRoadmap[g] {
			shared = append(shared, g)
		}
	}
	if len(got) != 8 || len(want) < 7 || strings.Join(shared, ",") != strings.Join(want, ",") {
		t.Fatalf("picker order %v (shared %v), roadmap order %v", got, shared, want)
	}
}

func TestParentPickerOneRowPerCandidateAtBothWidths(t *testing.T) {
	for _, w := range []int{95, 150} {
		e := ppNew(t, "task", "", w, 40, false)
		v := e.view()
		l := ppRowLines(v, "f4")
		if len(l) != 1 {
			t.Fatalf("w=%d: f4 rows = %d\n%s", w, len(l), v)
		}
		if !strings.Contains(l[0], "…") || !strings.Contains(l[0], "Eine sehr") {
			t.Errorf("w=%d: title not ellipsised: %q", w, l[0])
		}
		if strings.Contains(v, "abgeschnitten wird") {
			t.Errorf("w=%d: title wrapped instead of truncated", w)
		}
		if !regexp.MustCompile(`└─ f4 F Eine`).MatchString(l[0]) {
			t.Errorf("w=%d: prefix or short id damaged: %q", w, l[0])
		}
		for _, line := range strings.Split(v, "\n") {
			if ui.DisplayWidth(line) > w {
				t.Errorf("w=%d: line wider than terminal (%d): %q", w, ui.DisplayWidth(line), line)
			}
		}
		for _, s := range []string{"m1", "m2", "e1", "e2", "e3", "f1", "f3", "f4"} {
			if n := len(ppRowLines(v, s)); n != 1 {
				t.Errorf("w=%d: %s rows = %d", w, s, n)
			}
		}
	}
}

func TestParentPickerBoundTypeChangeReplacesOptions(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, true)
	// Type step: task -> feature (down), then Enter into the parent step.
	ppSend(e.f, ppKey(tea.KeyDown), ppKey(tea.KeyEnter))
	v := e.view()
	if e.typ != "feature" {
		t.Fatalf("bound type = %q", e.typ)
	}
	if len(ppRowLines(v, "f1")) != 0 || len(ppRowLines(v, "e1")) != 1 {
		t.Fatalf("feature candidates wrong:\n%s", v)
	}
	// Back to the type step, pick milestone, forward again.
	ppSend(e.f, tea.KeyMsg{Type: tea.KeyShiftTab})
	ppSend(e.f, ppKey(tea.KeyDown), ppKey(tea.KeyDown), ppKey(tea.KeyEnter))
	v = e.view()
	if e.typ != "milestone" {
		t.Fatalf("bound type = %q\n%s", e.typ, v)
	}
	if len(ppRowLines(v, "m1"))+len(ppRowLines(v, "e1")) != 0 || !strings.Contains(v, "(no parent)") {
		t.Fatalf("milestone must offer only (no parent):\n%s", v)
	}
}

func TestParentPickerPrefilledParentStartsHighlighted(t *testing.T) {
	e := ppNew(t, "task", "pp-e2", 150, 40, false)
	ppSend(e.f, ppKey(tea.KeyEnter))
	if e.parent != "pp-e2" {
		t.Fatalf("prefilled candidate lost: parent = %q", e.parent)
	}
}

func TestParentPickerSelectionWritesParentID(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	// "(no parent)" is row 0; the first candidate follows.
	ppSend(e.f, ppKey(tea.KeyDown), ppKey(tea.KeyEnter))
	if e.parent == "" {
		t.Fatal("selecting a candidate wrote no parent")
	}
	e2 := ppNew(t, "task", "", 150, 40, false)
	ppSend(e2.f, ppKey(tea.KeyEnter))
	if e2.parent != "" {
		t.Fatalf("(no parent) wrote %q", e2.parent)
	}
}

func TestParentPickerPanelFollowsCursor(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	if v := e.view(); strings.Contains(v, "Open children") || strings.Contains(v, "Path") {
		t.Fatalf("(no parent) must show no path and no children:\n%s", v)
	}
	// Down once: first candidate row (m1).
	ppSend(e.f, ppKey(tea.KeyDown))
	v := e.view()
	if !strings.Contains(v, "Path") || !strings.Contains(v, "Open children") {
		t.Fatalf("panel missing for highlighted row:\n%s", v)
	}
}

func TestParentPickerFilterDrivesPanelWithoutCursorMove(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	ppType(e.f, "/e1")
	v := e.view()
	if !strings.Contains(v, "/e1") {
		t.Fatalf("filter line missing:\n%s", v)
	}
	if len(ppRowLines(v, "e1")) != 1 || len(ppRowLines(v, "e2")) != 0 || len(ppRowLines(v, "m1")) != 0 {
		t.Fatalf("filter rows wrong:\n%s", v)
	}
	// Panel: path root -> container, open children only.
	for _, want := range []string{"Path", "Alpha release", "Search", "Open children", "Index"} {
		if !strings.Contains(v, want) {
			t.Errorf("panel lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Done feature") {
		t.Errorf("archived child shown in panel:\n%s", v)
	}
	// The panel names Alpha release although m1 is filtered out of the list.
	if len(ppRowLines(v, "m1")) != 0 {
		t.Errorf("m1 should be filtered out of the list")
	}
}

func TestParentPickerPanelPlacementAndFilterLineVisible(t *testing.T) {
	for _, c := range []struct {
		w, h   int
		beside bool
	}{{95, 30, false}, {99, 30, false}, {100, 40, true}, {150, 40, true}} {
		e := ppNew(t, "task", "", c.w, c.h, false)
		ppType(e.f, "/e1")
		lines := strings.Split(e.view(), "\n")
		if len(lines) > c.h {
			t.Errorf("%dx%d: %d rendered rows exceed the terminal", c.w, c.h, len(lines))
		}
		filterAt, panelAt, rowAt := -1, -1, -1
		for i, l := range lines {
			if strings.Contains(l, "/e1") && filterAt < 0 {
				filterAt = i
			}
			if strings.Contains(l, "╭") && panelAt < 0 {
				panelAt = i
			}
			if len(ppRowLines(l, "e1")) > 0 {
				rowAt = i
			}
		}
		if filterAt < 0 || filterAt >= c.h {
			t.Errorf("%dx%d: filter line not within rendered rows", c.w, c.h)
		}
		if c.beside && panelAt != filterAt {
			t.Errorf("%dx%d: panel should stand beside the list (panel row %d, filter row %d)", c.w, c.h, panelAt, filterAt)
		}
		if !c.beside && panelAt <= rowAt {
			t.Errorf("%dx%d: panel should stand under the list (panel row %d, list row %d)", c.w, c.h, panelAt, rowAt)
		}
	}
}

func TestParentPickerPanelCutsLongContentWithMarker(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	resolver, cfg := ppFixture(t)
	typ, parent := "task", ""
	for i := range 30 {
		if err := resolver.Core.Create(&item.Item{ID: "pp-x" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Title: "Child", Type: "task", Status: "todo", Parent: "pp-e1"}); err != nil {
			t.Fatal(err)
		}
	}
	p := newParentPicker(context.Background(), resolver, cfg, &typ, &parent)
	f := huh.NewForm(p.group).WithLayout(p).WithTheme(ui.FormTheme(ui.FlavourFormPalette(ui.ActiveTheme())))
	ppInit(f)
	ppSend(f, tea.WindowSizeMsg{Width: 95, Height: 30})
	ppType(f, "/e1")
	v := ansi.Strip(f.View())
	if !strings.Contains(v, "more") {
		t.Fatalf("no cut marker for over-long panel:\n%s", v)
	}
	if n := len(strings.Split(v, "\n")); n > 30 {
		t.Fatalf("view has %d rows at 95x30", n)
	}
}

func TestParentPickerEscEndsFilterAndKeepsStepOpen(t *testing.T) {
	e := ppNew(t, "task", "", 150, 40, false)
	ppType(e.f, "/e1")
	ppSend(e.f, ppKey(tea.KeyEsc))
	if e.f.State != huh.StateNormal {
		t.Fatalf("Esc ended the form: state %v", e.f.State)
	}
	if e.p.field.GetFiltering() {
		t.Fatal("filter entry still active after Esc")
	}
	if e.parent != "" {
		t.Fatalf("Esc wrote parent %q", e.parent)
	}
	if v := e.view(); len(ppRowLines(v, "e1")) != 1 {
		t.Fatalf("parent step lost after Esc:\n%s", v)
	}
}

func TestParentPickerOtherGroupRendersAsLayoutDefault(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	resolver, cfg := ppFixture(t)
	typ, parent, title := "task", "", ""
	p := newParentPicker(context.Background(), resolver, cfg, &typ, &parent)
	f := huh.NewForm(p.group, huh.NewGroup(huh.NewInput().Title("Title").Value(&title))).
		WithLayout(p).
		WithTheme(ui.FormTheme(ui.FlavourFormPalette(ui.ActiveTheme())))
	ppInit(f)
	ppSend(f, tea.WindowSizeMsg{Width: 150, Height: 40})
	ppSend(f, ppKey(tea.KeyEnter))
	if _, ok := f.GetFocusedField().(*huh.Input); !ok {
		t.Fatalf("second group not focused: %T", f.GetFocusedField())
	}
	got := f.View()
	f.WithLayout(huh.LayoutDefault)
	if want := f.View(); got != want {
		t.Fatalf("other group differs from LayoutDefault:\n%q\n%q", got, want)
	}
}

func TestParentPickerColoursComeFromFlavour(t *testing.T) {
	withTrueColor(t)
	hexRe := regexp.MustCompile(`\x1b\[[0-9;]*?(?:38|48);2;(\d+);(\d+);(\d+)m`)
	for _, flavour := range []string{"mocha", "latte"} {
		t.Run(flavour, func(t *testing.T) {
			ui.SetTheme(flavour)
			t.Cleanup(func() { ui.SetTheme("mocha") })
			e := ppNew(t, "task", "", 150, 40, false)
			ppType(e.f, "/e1")
			raw := e.f.View()
			allowed := map[string]bool{}
			for _, hex := range ui.ActiveTheme().Colors {
				allowed[strings.ToLower(hex)] = true
			}
			ms := hexRe.FindAllStringSubmatch(raw, -1)
			if len(ms) == 0 {
				t.Fatal("no colour escapes under forced TrueColor")
			}
			for _, m := range ms {
				r, _ := strconv.Atoi(m[1])
				g, _ := strconv.Atoi(m[2])
				b, _ := strconv.Atoi(m[3])
				if !ppNearFlavour(allowed, r, g, b) {
					t.Errorf("colour #%02x%02x%02x is not in flavour %s", r, g, b, flavour)
				}
			}
		})
	}
}

// ppNearFlavour reports whether rgb is a flavour hex up to the +-1 per
// channel rounding of the terminal colour conversion.
func ppNearFlavour(allowed map[string]bool, r, g, b int) bool {
	for hex := range allowed {
		var hr, hg, hb int
		if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &hr, &hg, &hb); err != nil {
			continue
		}
		if max(hr-r, r-hr) <= 1 && max(hg-g, g-hg) <= 1 && max(hb-b, b-hb) <= 1 {
			return true
		}
	}
	return false
}
