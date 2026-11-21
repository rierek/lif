package tui

// context_test.go — Yank (yy/ym/yt/yi/yl): yankText's variants, the filtered
// child set of yl, and the y-prefix dispatch. TestYankDispatch runs the real
// handleKey/clip.Copy in a child process (see yankDispatchCases) and compares
// the OSC52 clipboard payload it writes to stderr.

import (
	"bytes"
	"encoding/base64"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

const yankBody = "Intro sentence.\n\n## Goal\n\nText.\n\n```sh\n# not a heading\n```\n\n### Detail ##\n\n#nospace\n"

// tableRows returns every Markdown table body row (lines starting with
// "| <prefix>") in md -- used to pin down exact row counts/order without
// coupling to the exact column layout.
func tableRows(md, prefix string) []string {
	var rows []string
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "| "+prefix) {
			rows = append(rows, line)
		}
	}
	return rows
}

func mustYank(t *testing.T, idx *data.Index, id string, mode yankMode) string {
	t.Helper()
	out, err := yankText(idx, idx.ByID[id], mode, nil)
	if err != nil {
		t.Fatalf("yankText(%s, %d): %v", id, mode, err)
	}
	return out
}

// TestYankFullLeafIsTheItemFile: a leaf yields exactly the rendered item
// file -- no Children table, nothing hand-picked.
func TestYankFullLeafIsTheItemFile(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	b := idx.ByID["tk-2"]
	b.Body = yankBody
	want, err := b.Render()
	if err != nil {
		t.Fatal(err)
	}
	if got := mustYank(t, idx, "tk-2", yankFull); got != string(want) {
		t.Fatalf("yy on a leaf = %q, want the item file %q", got, want)
	}
}

// TestYankFullParentHasChildrenTable: 2 children -> exactly 2 table rows,
// in idx.Children (sortBeans) order -- tk-1 (in-progress/high) before tk-2.
func TestYankFullParentHasChildrenTable(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	md := mustYank(t, idx, "ep-1", yankFull)
	rows := tableRows(md, "tk-")
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "| tk-1 ") || !strings.HasPrefix(rows[1], "| tk-2 ") {
		t.Fatalf("Children rows = %v, want [tk-1, tk-2]:\n%s", rows, md)
	}
}

// TestYankMetaIsTheFullFrontMatter: ym parses back to the same front matter
// (incl. fields the old curated header dropped) with an empty body, and the
// source item keeps its body.
func TestYankMetaIsTheFullFrontMatter(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	b := idx.ByID["tk-1"]
	b.Body = yankBody
	b.Extra = map[string]any{"goal": "ship it"}
	md := mustYank(t, idx, "tk-1", yankMeta)
	if strings.Contains(md, "---") {
		t.Fatalf("ym must not carry the --- fence:\n%s", md)
	}
	got, err := item.Parse(strings.NewReader("---\n" + md + "---\n"))
	if err != nil {
		t.Fatalf("ym output does not parse as front matter: %v\n%s", err, md)
	}
	if got.Body != "" || got.Title != b.Title || got.Parent != b.Parent || got.Extra["goal"] != "ship it" {
		t.Fatalf("ym front matter lost data, parsed %+v from:\n%s", got, md)
	}
	if b.Body != yankBody {
		t.Fatal("ym mutated the source item's body")
	}
}

// TestYankTOCListsBodyHeadings: yt = ym + the heading outline, nested from
// the shallowest level; fenced code and "#nospace" are not headings, closing
// hashes are stripped, body prose is absent.
func TestYankTOCListsBodyHeadings(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	idx.ByID["tk-1"].Body = yankBody
	meta := mustYank(t, idx, "tk-1", yankMeta)
	got := mustYank(t, idx, "tk-1", yankTOC)
	want := strings.TrimRight(meta, "\n") + "\n\n## TOC\n\n- Goal\n  - Detail\n"
	if got != want {
		t.Fatalf("yt = %q, want %q", got, want)
	}
}

// TestYankSequenceCancels: y then an unbound second key copies nothing and
// drops the hint toast.
func TestYankSequenceCancels(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	nm, _ := m.handleKey(runeMsg('y'))
	nm, _ = nm.(model).handleKey(runeMsg('x'))
	mm := nm.(model)
	if mm.yankPrefix || mm.toast != nil {
		t.Fatalf("yx must cancel silently: prefix=%v toast=%+v", mm.yankPrefix, mm.toast)
	}
}

// TestYankOnOrphanRootNoop guards the orphan-root cursor (b == nil): no
// toast, no crash (Plan Step 6).
func TestYankOnOrphanRootNoop(t *testing.T) {
	beans := append(fixtureBeans(), fixtureOrphanBean())
	m := fixtureModel(t, beans)
	m.cursorID = notOnRoadmapRootID
	if m.focusedBean() != nil {
		t.Fatal("test setup invalid: focusedBean() != nil on the orphan root")
	}

	handled, nm, cmd := m.keyNodeAction(runeMsg('y'))
	if !handled {
		t.Fatal("y must still be reported handled (silent no-op), not fall through")
	}
	if cmd != nil {
		t.Fatal("y on the orphan root must not return a Cmd")
	}
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("keyNodeAction did not return a model, got %T", nm)
	}
	if mm.toast != nil {
		t.Fatalf("y on the orphan root must not show a toast, got %+v", mm.toast)
	}
}

// yankFixtureBeans extends fixtureBeans: ep-1 gets a completed direct child
// (tk-done), tk-1 a grandchild (gc-1), plus an unrelated item. tk-1 is the
// "share"-tagged high-priority task, tk-2 the normal-priority bug.
func yankFixtureBeans() []*item.Item {
	beans := fixtureBeans()
	for _, b := range beans {
		switch b.ID {
		case "tk-1":
			b.Tags = []string{"share"}
		case "tk-2":
			b.Type = "bug"
		}
	}
	return append(beans,
		&item.Item{ID: "tk-done", Title: "Task Done", Status: "completed", Type: "task", Priority: "normal", Parent: "ep-1"},
		&item.Item{ID: "gc-1", Title: "Grandchild", Status: "todo", Type: "task", Priority: "normal", Parent: "tk-1"},
		&item.Item{ID: "un-1", Title: "Unrelated", Status: "todo", Type: "task", Priority: "normal"},
	)
}

const (
	yankHead  = "ep-1 - Epic One\n"
	yankTk1   = "  +-- tk-1 - Task One\n"
	yankTk2   = "  +-- tk-2 - Task Two\n"
	yankTkDon = "  +-- tk-done - Task Done\n"
)

// yankListOf renders yl for id exactly as keyYank does.
func yankListOf(t *testing.T, m model, id string) string {
	t.Helper()
	b := m.idx.ByID[id]
	out, err := yankText(m.idx, b, yankList, m.yankChildren(id))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestYankItemIsIDAndTitleLine(t *testing.T) {
	idx := data.NewIndex(yankFixtureBeans())
	if got := mustYank(t, idx, "ep-1", yankItem); got != yankHead {
		t.Fatalf("yi = %q, want %q", got, yankHead)
	}
	if got, err := yankText(idx, nil, yankList, nil); got != "" || err != nil {
		t.Fatalf("nil item = %q, %v", got, err)
	}
}

func TestYankListDefaultIsOpenDirectChildren(t *testing.T) {
	m := fixtureModel(t, yankFixtureBeans())
	if got, want := yankListOf(t, m, "ep-1"), yankHead+yankTk1+yankTk2; got != want {
		t.Fatalf("yl = %q, want %q", got, want)
	}
}

func TestYankListFilters(t *testing.T) {
	tests := []struct {
		name  string
		setup func(m model) model
		want  string
	}{
		{"status facet", func(m model) model { m.filterStatus = map[string]bool{"todo": true}; return m }, yankHead + yankTk2},
		{"type facet", func(m model) model { m.filterType = map[string]bool{"bug": true}; return m }, yankHead + yankTk2},
		{"priority facet", func(m model) model { m.filterPriority = map[string]bool{"high": true}; return m }, yankHead + yankTk1},
		{"tag facet", func(m model) model { m.filterTag = map[string]bool{"share": true}; return m }, yankHead + yankTk1},
		{"search st:", func(m model) model { return setSearchQuery(m, "st:todo") }, yankHead + yankTk2},
		{"search ty:", func(m model) model { return setSearchQuery(m, "ty:bug") }, yankHead + yankTk2},
		{"search pr:", func(m model) model { return setSearchQuery(m, "pr:high") }, yankHead + yankTk1},
		{"search tag:", func(m model) model { return setSearchQuery(m, "tag:share") }, yankHead + yankTk1},
		{"search text", func(m model) model { return setSearchQuery(m, "task two") }, yankHead + yankTk2},
		{"search text and prefix", func(m model) model { return setSearchQuery(m, "st:todo task") }, yankHead + yankTk2},
		{"all filtered out", func(m model) model { m.filterTag = map[string]bool{"nope": true}; return m }, yankHead},
		{"archive shows completed", func(m model) model { m.showArchived = true; return m }, yankHead + yankTk1 + yankTk2 + yankTkDon},
		{"parent fails own filter", func(m model) model {
			m.showArchived = true
			m.filterStatus = map[string]bool{"completed": true}
			return m
		}, yankHead + yankTkDon},
		{"exact ID search keeps children relation", func(m model) model { return setSearchQuery(m, "ep-1") }, yankHead + yankTk1 + yankTk2},
		{"exact ID of a child", func(m model) model { return setSearchQuery(m, "tk-1") }, yankHead + yankTk1},
		{"body-only hit", func(m model) model {
			m.idx.ByID["tk-2"].Body = "mentions a zebra"
			return setSearchQuery(m, "zebra")
		}, yankHead + yankTk2},
		{"current Bleve hit", func(m model) model {
			m.searchBleveFor, m.searchBleveIDs = "quartz", map[string]bool{"tk-1": true, "gc-1": true}
			return setSearchQuery(m, "quartz")
		}, yankHead + yankTk1},
		{"stale Bleve hit", func(m model) model {
			m.searchBleveFor, m.searchBleveIDs = "other", map[string]bool{"tk-1": true}
			return setSearchQuery(m, "quartz")
		}, yankHead},
		{"parent context without own match", func(m model) model { return setSearchQuery(m, "epic one") }, yankHead},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.setup(fixtureModel(t, yankFixtureBeans()))
			if got := yankListOf(t, m, "ep-1"); got != tc.want {
				t.Fatalf("yl = %q, want %q", got, tc.want)
			}
		})
	}
	// clearing the query restores both open children
	m := setSearchQuery(fixtureModel(t, yankFixtureBeans()), "task two")
	if got := yankListOf(t, setSearchQuery(m, ""), "ep-1"); got != yankHead+yankTk1+yankTk2 {
		t.Fatalf("cleared filters: yl = %q", got)
	}
}

func TestYankListNoChildrenNilIndexAndUnicode(t *testing.T) {
	m := fixtureModel(t, yankFixtureBeans())
	if got := yankListOf(t, m, "tk-2"); got != "tk-2 - Task Two\n" {
		t.Fatalf("leaf yl = %q", got)
	}
	if got := (model{}).yankChildren("ep-1"); got != nil {
		t.Fatalf("nil index children = %v", got)
	}
	if got := m.yankChildren("missing"); got != nil {
		t.Fatalf("missing parent children = %v", got)
	}
	long := "Ünïcødé 日本語 " + strings.Repeat("x", 300)
	m = fixtureModel(t, append(yankFixtureBeans(), &item.Item{ID: "tk-9", Title: long, Status: "todo", Type: "task", Priority: "low", Parent: "ep-1"}))
	if got := yankListOf(t, m, "ep-1"); !strings.HasSuffix(got, "  +-- tk-9 - "+long+"\n") {
		t.Fatalf("long/unicode title not preserved: %q", got)
	}
}

// yl depends on filters only, never on folds, layout, flat or Ready view.
func TestYankListIgnoresFoldsViewsAndReady(t *testing.T) {
	base := fixtureModel(t, yankFixtureBeans())
	want := yankListOf(t, base, "ep-1")
	variants := map[string]func(m model) model{
		"folded":  func(m model) model { m.expanded = map[string]bool{}; return m },
		"flat":    func(m model) model { m.flatView = true; return m },
		"ready":   func(m model) model { m.view = viewReady; return m },
		"detail":  func(m model) model { m.layout = layoutDetail; m.detailFocus = true; return m },
		"blocked": func(m model) model { m.idx.ByID["tk-2"].BlockedBy = []string{"tk-1"}; return m },
	}
	for name, f := range variants {
		if got := yankListOf(t, f(fixtureModel(t, yankFixtureBeans())), "ep-1"); got != want {
			t.Errorf("%s: yl = %q, want %q", name, got, want)
		}
	}
}

// yankDispatchCase is one real handleKey run in a child process. want is the
// exact clipboard text, "" when nothing is copied.
type yankDispatchCase struct {
	name  string
	focus string // "" = orphan root (no focused item)
	keys  []tea.KeyMsg
	want  string
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func yankDispatchCases(t *testing.T) []yankDispatchCase {
	list := yankHead + yankTk1 + yankTk2
	return []yankDispatchCase{
		{"ym sequence", "tk-2", []tea.KeyMsg{runes("y"), runes("m")}, mustYank(t, data.NewIndex(yankFixtureBeans()), "tk-2", yankMeta)},
		{"yi sequence", "ep-1", []tea.KeyMsg{runes("y"), runes("i")}, yankHead},
		{"yl sequence", "ep-1", []tea.KeyMsg{runes("y"), runes("l")}, list},
		{"yi combined", "ep-1", []tea.KeyMsg{runes("yi")}, yankHead},
		{"yl combined", "ep-1", []tea.KeyMsg{runes("yl")}, list},
		{"yx cancels", "tk-2", []tea.KeyMsg{runes("y"), runes("x")}, ""},
		{"no focus", "", []tea.KeyMsg{runes("yl")}, ""},
	}
}

const yankHelperEnv = "LIF_YANK_DISPATCH_CASE"

// TestYankDispatchHelper is the child half of TestYankDispatch; it only runs
// when the parent selects a case through yankHelperEnv.
func TestYankDispatchHelper(t *testing.T) {
	name := os.Getenv(yankHelperEnv)
	if name == "" {
		t.Skip("child of TestYankDispatch")
	}
	for _, c := range yankDispatchCases(t) {
		if c.name != name {
			continue
		}
		m := fixtureModel(t, yankFixtureBeans())
		if c.focus == "" {
			m.cursorID = notOnRoadmapRootID
		} else {
			m = focusBean(m, c.focus)
		}
		var cm tea.Model = m
		for _, k := range c.keys {
			cm, _ = cm.(model).handleKey(k)
		}
		mm := cm.(model)
		if mm.yankPrefix {
			t.Fatal("yank prefix still armed")
		}
		if c.want == "" {
			if mm.toast != nil {
				t.Fatalf("unexpected toast %+v", mm.toast)
			}
		} else if mm.toast == nil || mm.toast.kind != toastInfo || mm.toast.title != "Copied: "+c.focus {
			t.Fatalf("no copy confirmation: toast=%+v", mm.toast)
		}
		return
	}
	t.Fatalf("unknown case %q", name)
}

var osc52Payload = regexp.MustCompile("\x1b\\]52;[a-z]*;([A-Za-z0-9+/=]*)(?:\x07|\x1b\\\\)")

// TestYankDispatch runs each case in a child test binary with an empty PATH
// (no native clipboard tool) and TERM=xterm, so clip.Copy writes only its
// OSC52 sequence to the captured stderr.
func TestYankDispatch(t *testing.T) {
	for _, c := range yankDispatchCases(t) {
		t.Run(c.name, func(t *testing.T) {
			want := c.want
			cmd := exec.Command(os.Args[0], "-test.run=^TestYankDispatchHelper$", "-test.count=1")
			cmd.Env = []string{"PATH=", "TMUX=", "TERM=xterm", yankHelperEnv + "=" + c.name}
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("child failed: %v\nstdout: %s\nstderr: %q", err, stdout.String(), stderr.String())
			}
			matches := osc52Payload.FindAllSubmatch(stderr.Bytes(), -1)
			if want == "" {
				if len(matches) != 0 {
					t.Fatalf("unexpected OSC52 output: %q", stderr.String())
				}
				return
			}
			if len(matches) != 1 {
				t.Fatalf("want exactly one OSC52 sequence, got %d in %q", len(matches), stderr.String())
			}
			got, err := base64.StdEncoding.DecodeString(string(matches[0][1]))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Fatalf("clipboard = %q, want %q", got, want)
			}
		})
	}
}
