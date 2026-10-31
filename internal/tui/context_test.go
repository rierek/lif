package tui

// context_test.go — Yank (yy/ym/yt): yankText's three variants plus the
// y-prefix dispatch. The dispatch tests exercise the REAL clip.Copy call and
// assert on the resulting toast, not on clipboard content; content
// assertions are made against yankText directly.

import (
	"strings"
	"testing"

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
	out, err := yankText(idx, idx.ByID[id], mode)
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

// TestYankSequenceCopies: y then m copies and confirms via a toastInfo
// "Copied: <id>" toast.
func TestYankSequenceCopies(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	nm, _ := m.handleKey(runeMsg('y'))
	mm := nm.(model)
	if !mm.yankPrefix {
		t.Fatal("y must arm the yank prefix")
	}
	nm, _ = mm.handleKey(runeMsg('m'))
	mm = nm.(model)
	if mm.yankPrefix || mm.toast == nil || mm.toast.kind != toastInfo || mm.toast.title != "Copied: tk-2" {
		t.Fatalf("ym did not confirm the copy: prefix=%v toast=%+v", mm.yankPrefix, mm.toast)
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
