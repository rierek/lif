package tui

// context_test.go — TDD coverage for Yank `y` (E5 Task 3, bean bt-e4a6,
// epic-E5-plan.md »Task 3«): beanContext (design decision b, ONE
// view-agnostic Markdown generator covering both a leaf Issue and an
// Epic/Milestone via an optional Children table) + the y-dispatch itself
// (keyNodeAction). Mirrors devd's own precedent (dd2_217_test.go: "der
// clip.Copy-Pfad selbst wird nicht getestet, Seiteneffekt OSC52/pbcopy") --
// the toast-dispatch tests below exercise the REAL clip.Copy call (same as
// devd's TestBacklogYankKeyWired) and assert on the resulting toast, not on
// actual clipboard content; a pure content assertion is only made against
// beanContext directly, which takes no clipboard action.

import (
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
)

// contextFixtureBeans is a small Epic -> 2 Tasks hierarchy with a resolvable
// Blocking relation -- the exact shape TestBeanContextResolvesParentTitle
// AndRelations needs (fixtureBeans has no Blocking set on any bean).
func contextFixtureBeans() []data.Bean {
	return []data.Bean{
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: "tk-1", Title: "Task One", Status: "in-progress", Type: "task", Priority: "high", Parent: "ep-1", Blocking: []string{"tk-2"}},
		{ID: "tk-2", Title: "Task Two", Status: "todo", Type: "task", Priority: "normal", Parent: "ep-1"},
	}
}

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

// TestBeanContextLeafHasNoChildrenTable guards the leaf case (design
// decision b): a Task with no children must not render a "## Children"
// section at all.
func TestBeanContextLeafHasNoChildrenTable(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	b := idx.ByID["tk-2"]
	md := beanContext(idx, b)
	if strings.Contains(md, "## Children") {
		t.Fatalf("leaf bean must not render a Children table:\n%s", md)
	}
}

// TestBeanContextParentHasChildrenTable guards the Epic/Milestone case
// (design decision b): 2 children -> exactly 2 table rows, in
// idx.Children (sortBeans) order -- fixtureBeans' own doc comment pins
// tk-1 (in-progress/high) before tk-2 (todo/normal).
func TestBeanContextParentHasChildrenTable(t *testing.T) {
	idx := data.NewIndex(fixtureBeans())
	b := idx.ByID["ep-1"]
	md := beanContext(idx, b)
	if !strings.Contains(md, "## Children") {
		t.Fatalf("parent bean must render a Children table:\n%s", md)
	}
	rows := tableRows(md, "tk-")
	if len(rows) != 2 {
		t.Fatalf("Children table rows = %d, want 2:\n%s", len(rows), md)
	}
	if !strings.HasPrefix(rows[0], "| tk-1 ") || !strings.HasPrefix(rows[1], "| tk-2 ") {
		t.Fatalf("Children table order = %v, want [tk-1, tk-2] (sortBeans order)", rows)
	}
}

// TestBeanContextResolvesParentTitleAndRelations guards that Parent/
// Blocking render as resolved "ID Title" labels, not bare IDs.
func TestBeanContextResolvesParentTitleAndRelations(t *testing.T) {
	idx := data.NewIndex(contextFixtureBeans())
	b := idx.ByID["tk-1"]
	md := beanContext(idx, b)
	if !strings.Contains(md, "Parent: ep-1 Epic One") {
		t.Fatalf("Parent not resolved to ID+Title:\n%s", md)
	}
	if !strings.Contains(md, "Blocking: tk-2 Task Two") {
		t.Fatalf("Blocking not resolved to ID+Title:\n%s", md)
	}
}

// TestYankShowsConfirmationToast (US-11 Kern): `y` on a focused bean fires
// clip.Copy and confirms via a toastInfo "Copied: <id>" toast.
func TestYankShowsConfirmationToast(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")

	handled, nm, _ := m.keyNodeAction(runeMsg('y'))
	if !handled {
		t.Fatal("y must be handled by keyNodeAction")
	}
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("keyNodeAction did not return a model, got %T", nm)
	}
	if mm.toast == nil {
		t.Fatal("y did not show a confirmation toast")
	}
	if mm.toast.kind != toastInfo {
		t.Fatalf("toast.kind = %v, want toastInfo", mm.toast.kind)
	}
	if !strings.Contains(mm.toast.title, "Copied") || !strings.Contains(mm.toast.title, "tk-2") {
		t.Fatalf("toast.title = %q, want it to mention Copied + tk-2", mm.toast.title)
	}
}

// TestYankOnOrphanRootNoop guards the orphan-root cursor (b == nil): no
// toast, no crash (Plan Step 6).
func TestYankOnOrphanRootNoop(t *testing.T) {
	beans := append(fixtureBeans(), fixtureOrphanBean())
	m := fixtureModel(t, beans)
	m.cursorID = orphanRootID
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
