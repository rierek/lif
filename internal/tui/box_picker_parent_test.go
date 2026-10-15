package tui

// box_picker_parent_test.go — TDD coverage for the Parent-Picker (`a`, E3
// Task 3, bean bt-p1uz): single-select over data.EligibleParents (self/
// descendants/invalid-types pre-filtered, design decision f) plus a
// "(No parent)" clear row pinned first. Immediate-apply Enter semantics
// (like box_menu_value.go's value menu, design decision a3) -- NOT the
// Pending-Diff pattern box_picker_tag.go/box_picker_blocking.go use, since a
// bean has exactly one parent, nothing to diff.

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// fixtureBeansForParentPicker is a deliberately deeper/wider hierarchy than
// fixtureBeans() (update_test.go): it needs a "feature" type (absent from
// fixtureBeans) AND a hand-edited structural oddity -- ms-2, a MILESTONE
// nested under ep-1 -- to make the descendant-exclusion half of
// TestParentPickerExcludesSelfDescendantsAndInvalidTypes meaningful: ms-2's
// TYPE (milestone) would otherwise be perfectly eligible for an epic's
// parent, so only CollectDescendants (not the type filter) can be excluding
// it. Beans-legal per this app's own philosophy (hand-editable frontmatter,
// tolerated the same way collectCycleOrphans/appendBeanNode's cycle guards
// already tolerate structurally odd hierarchies, view_browse_repo.go).
func fixtureBeansForParentPicker() []data.Bean {
	return []data.Bean{
		{ID: "ms-1", Title: "Milestone One", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Epic One", Status: "todo", Type: "epic", Priority: "normal", Parent: "ms-1"},
		{ID: "ms-2", Title: "Milestone Two", Status: "todo", Type: "milestone", Priority: "normal", Parent: "ep-1"},
		{ID: "ft-1", Title: "Feature One", Status: "todo", Type: "feature", Priority: "normal", Parent: "ep-1"},
		{ID: "tk-1", Title: "Task One", Status: "todo", Type: "task", Priority: "normal", Parent: "ft-1"},
	}
}

// focusBeanFull expands EVERY ancestor of id (walking Parent up through
// m.idx) so id is visible in m.visibleNodes() -- a generic version of
// box_menu_value_test.go's focusBean, which hardcodes fixtureBeans' fixed
// ms-1/ep-1 ancestor set. This file's fixture has a deeper/wider hierarchy,
// so each test walks its own bean's real ancestor chain instead.
func focusBeanFull(m model, id string) model {
	expanded := map[string]bool{}
	cur := id
	for {
		b, ok := m.idx.ByID[cur]
		if !ok || b.Parent == "" {
			break
		}
		expanded[b.Parent] = true
		cur = b.Parent
	}
	m.expanded = expanded
	m.cursorID = id
	return m
}

// pickerItemIDs is a small assertion helper -- the set of ids in items.
func pickerItemIDs(items []pickerItem) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, it := range items {
		out[it.id] = true
	}
	return out
}

// --- openParentPicker ---

// TestParentPickerExcludesSelfDescendantsAndInvalidTypes guards the
// combined eligibility rule (design decision f): focused on ep-1 (an epic,
// validParentTypes -> [milestone] only), the picker must offer ms-1 and
// NOTHING else -- ep-1 itself (self), ms-2 (a descendant, DESPITE being a
// type-eligible milestone), ft-1 and tk-1 (both descendants AND wrong type)
// are all excluded.
func TestParentPickerExcludesSelfDescendantsAndInvalidTypes(t *testing.T) {
	m := fixtureModel(t, fixtureBeansForParentPicker())
	m = focusBeanFull(m, "ep-1")

	m = step(t, m, runeMsg('a'))

	if m.overlay != overlayParentPicker {
		t.Fatalf("overlay = %v, want overlayParentPicker", m.overlay)
	}
	ids := pickerItemIDs(m.parentItems)
	if !ids["ms-1"] {
		t.Fatalf("parentItems missing ms-1 (eligible: type milestone, not a descendant), got %+v", m.parentItems)
	}
	for _, excluded := range []string{"ep-1", "ms-2", "ft-1", "tk-1"} {
		if ids[excluded] {
			t.Errorf("parentItems must exclude %q, got %+v", excluded, m.parentItems)
		}
	}
	// self + descendants + invalid types all excluded -> only the clear row
	// ("") plus ms-1 remain.
	if len(m.parentItems) != 2 {
		t.Fatalf("len(parentItems) = %d, want 2 (clear row + ms-1), got %+v", len(m.parentItems), m.parentItems)
	}
}

// TestParentPickerMilestoneShowsNoEligibleParents guards the nil-
// validParentTypes short-circuit: a milestone can never take a parent, so
// only the "(No parent)" clear row remains.
func TestParentPickerMilestoneShowsNoEligibleParents(t *testing.T) {
	m := fixtureModel(t, fixtureBeansForParentPicker())
	m = focusBeanFull(m, "ms-1")

	m = step(t, m, runeMsg('a'))

	if len(m.parentItems) != 1 {
		t.Fatalf("len(parentItems) = %d, want 1 (clear row only), got %+v", len(m.parentItems), m.parentItems)
	}
	if m.parentItems[0].id != "" {
		t.Fatalf("parentItems[0].id = %q, want \"\" (clear row)", m.parentItems[0].id)
	}
}

// TestParentPickerClearRowFirstAndCursorOnCurrentParent guards two upstream
// invariants at once (port beans-src parentpicker.go clearParentItem +
// newParentPickerModel's selectedIndex seed): the clear row is ALWAYS index
// 0, and the cursor starts on the focused bean's CURRENT parent's row.
func TestParentPickerClearRowFirstAndCursorOnCurrentParent(t *testing.T) {
	m := fixtureModel(t, fixtureBeansForParentPicker())
	m = focusBeanFull(m, "tk-1") // parent: ft-1

	m = step(t, m, runeMsg('a'))

	if len(m.parentItems) == 0 || m.parentItems[0].id != "" {
		t.Fatalf("parentItems[0] = %+v, want the \"(No parent)\" clear row first", m.parentItems)
	}
	wantIdx := -1
	for i, it := range m.parentItems {
		if it.id == "ft-1" {
			wantIdx = i
		}
	}
	if wantIdx < 0 {
		t.Fatalf("ft-1 (current parent) not found in parentItems %+v", m.parentItems)
	}
	if m.menu.cursor != wantIdx {
		t.Fatalf("menu.cursor = %d, want %d (ft-1's row)", m.menu.cursor, wantIdx)
	}
}

// --- keyParentPicker: enter applies immediately ---

// TestParentPickerEnterAppliesSetParentOrRemoveParent covers both branches
// of the immediate-apply Enter (design decision a3 parity): cursoring onto a
// real bean row fires SetParent, cursoring onto the clear row fires
// RemoveParent. Verified the same way box_picker_tag.go's
// TestTagPickerEnterDiffsFireOneSetTagsCall is: a real data.Client pointed
// at a nonexistent repo dir, no beans binary required -- cmd() resolves
// DIRECTLY to a mutationDoneMsg whose error text names the dispatched CLI
// subcommand.
func TestParentPickerEnterAppliesSetParentOrRemoveParent(t *testing.T) {
	t.Run("SetParent", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeansForParentPicker())
		m.client = &data.Client{RepoDir: "/nonexistent-bt-e3-t3-scratch-dir"}
		m = focusBeanFull(m, "ep-1")
		m = step(t, m, runeMsg('a'))

		for i, it := range m.parentItems {
			if it.id == "ms-1" {
				m.menu.cursor = i
			}
		}

		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		nm, ok := tm.(model)
		if !ok {
			t.Fatalf("Update(enter) did not return a model, got %T", tm)
		}
		if nm.overlay != overlayNone {
			t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
		}
		if cmd == nil {
			t.Fatal("enter on a real row must return a Cmd")
		}
		msg := cmd()
		mdm, ok := msg.(mutationDoneMsg)
		if !ok {
			t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
		}
		if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif update") {
			t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves SetParent dispatched)", mdm.err, "lif update")
		}
	})

	t.Run("RemoveParent", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeansForParentPicker())
		m.client = &data.Client{RepoDir: "/nonexistent-bt-e3-t3-scratch-dir"}
		m = focusBeanFull(m, "ep-1")
		m = step(t, m, runeMsg('a'))
		m.menu.cursor = 0 // the clear row

		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		nm := tm.(model)
		if nm.overlay != overlayNone {
			t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
		}
		if cmd == nil {
			t.Fatal("enter on the clear row must return a Cmd")
		}
		msg := cmd()
		mdm, ok := msg.(mutationDoneMsg)
		if !ok {
			t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
		}
		if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif update") {
			t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves RemoveParent dispatched)", mdm.err, "lif update")
		}
	})
}

// --- keyParentPicker: esc discards ---

func TestParentPickerEscNoMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeansForParentPicker())
	m = focusBeanFull(m, "ep-1")
	m = step(t, m, runeMsg('a'))

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("esc did not close the parent picker")
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// --- vanished-target guard on enter (E3-T3-Review PFLICHT, carried into
// bean bt-ppzb/E3 Task 6: "Vanished-mutTarget-Regressionstests für Parent-
// UND Blocking-Picker (Muster TestValueMenuTargetVanished...)") ---

// TestParentPickerEnterTargetVanishedClosesGracefully mirrors
// TestValueMenuTargetVanishedClosesGracefully/
// TestTagPickerEnterTargetVanishedClosesGracefully (design decision d): the
// focused bean disappears (external delete + reload) between open and enter
// -- enter must close the overlay and set a status-line note, WITHOUT firing
// a doomed SetParent/RemoveParent.
func TestParentPickerEnterTargetVanishedClosesGracefully(t *testing.T) {
	beans := fixtureBeansForParentPicker()
	m := fixtureModel(t, beans)
	m = focusBeanFull(m, "ep-1")
	m = step(t, m, runeMsg('a'))
	if m.overlay != overlayParentPicker {
		t.Fatal("setup: a did not open the parent picker")
	}

	// ep-1 vanishes externally; a reload lands while the picker is still open
	// (m.overlay survives a reload untouched, applyLoaded never touches it).
	var remaining []data.Bean
	for _, b := range beans {
		if b.ID != "ep-1" {
			remaining = append(remaining, b)
		}
	}
	m = step(t, m, beansLoadedMsg{beans: remaining})

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter on a vanished target must close the overlay")
	}
	if nm.err == "" {
		t.Fatal("enter on a vanished target must set a status-line note (m.err)")
	}
	// bt-81f0: cmd is no longer nil -- it is now the Toast's own
	// auto-dismiss tick (non-sticky), NOT a doomed mutation (structurally
	// guaranteed: this branch returns before any mutateCmd(...) is built).
	// Not invoked here -- toastError's own 8s duration would block the test.
	if cmd == nil {
		t.Fatal("enter on a vanished target must still fire a Cmd (the Toast's own auto-dismiss tick, bt-81f0)")
	}
	// bt-81f0: m.err no longer renders anywhere -- Toast is the ONE visible
	// channel, this guard must not go silent.
	if nm.toast == nil {
		t.Fatal("enter on a vanished target must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if nm.toast.kind != toastError {
		t.Errorf("toast.kind = %v, want toastError", nm.toast.kind)
	}
}

// --- current-parent-out-of-eligibility cursor fallback (E3-T3-Review
// PFLICHT, carried into bean bt-ppzb/E3 Task 6: "Test: current-parent-out-
// of-eligibility -> Cursor-Fallback 0") ---

// TestParentPickerCurrentParentOutOfEligibilityCursorFallsBackToZero guards
// openParentPicker's cursor seed (box_picker_parent.go) for a bean whose
// CURRENT Parent field is set but points at a bean that is beans-legal
// (hand-editable frontmatter) yet not a legal parent TYPE for it --
// tk-1.Parent is hand-edited to tk-9, ANOTHER task (validParentTypes("task")
// = [milestone, epic, feature] -- a task can never legally parent a task),
// so tk-9 never appears in parentItems at all. The seed loop
// (openParentPicker) must fall back to its listState{} zero value (index 0,
// the "(No parent)" clear row) rather than leaving the cursor on a stale
// position -- this is already the loop's own documented behavior (no
// explicit "not found" branch needed), pinned here as an end-to-end
// regression.
func TestParentPickerCurrentParentOutOfEligibilityCursorFallsBackToZero(t *testing.T) {
	beans := append(fixtureBeansForParentPicker(),
		data.Bean{ID: "tk-9", Title: "Task Nine", Status: "todo", Type: "task", Priority: "normal", Parent: "ft-1"},
	)
	for i := range beans {
		if beans[i].ID == "tk-1" {
			beans[i].Parent = "tk-9" // out-of-eligibility: a task can't legally parent a task
		}
	}
	m := fixtureModel(t, beans)
	m = focusBeanFull(m, "tk-1")

	m = step(t, m, runeMsg('a'))

	ids := pickerItemIDs(m.parentItems)
	if ids["tk-9"] {
		t.Fatalf("parentItems must exclude tk-9 (wrong type for a task's parent), got %+v", m.parentItems)
	}
	if m.menu.cursor != 0 {
		t.Fatalf("menu.cursor = %d, want 0 (fallback: current parent tk-9 is not in parentItems)", m.menu.cursor)
	}
}

// --- parentPickerBox width/wrap (T5, bean bt-4mo9, B06) ---

// TestParentPickerBoxUsesWideModalWidth mirrors
// TestBlockingPickerBoxUsesWideModalWidth: the rendered overlay must scale
// with the terminal instead of staying pinned to the old fixed
// clampModalWidth(48, ...).
func TestParentPickerBoxUsesWideModalWidth(t *testing.T) {
	m := fixtureModel(t, fixtureBeansForParentPicker())
	m.width = 120
	m = focusBeanFull(m, "ep-1")
	m = step(t, m, runeMsg('a'))

	out := m.parentPickerBox()
	lines := strings.Split(out, "\n")
	maxW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > maxW {
			maxW = w
		}
	}
	oldFixed := clampModalWidth(48, m.width) + 2 // +2 border, byte-parity with modalBox's own overhead
	if maxW <= oldFixed {
		t.Fatalf("parentPickerBox max line width = %d, want > %d (old fixed-48 box) at m.width=120", maxW, oldFixed)
	}
	wantW := wideModalWidth(m.width) + 2
	if maxW != wantW {
		t.Errorf("parentPickerBox max line width = %d, want %d (wideModalWidth(120)+2 border)", maxW, wantW)
	}
	// F02 (T5-Review, bean bt-4mo9): wantW above is self-referential (derived
	// from the SAME wideModalWidth call under test) -- a bug inside
	// wideModalWidth itself (e.g. a wrong percentage or clamp) would still
	// pass. Pin an independent literal, mirrors the Blocking-Picker's own
	// TestBlockingPickerBoxUsesWideModalWidth pin: wideModalWidth(120) =
	// 120*85/100 = 102 (no floor-60/ceiling-(termW-4)/floor-24 clamp engages
	// at this width) + 2 border = 104.
	const wantLiteral = 104
	if maxW != wantLiteral {
		t.Errorf("parentPickerBox max line width = %d, want %d (independent literal pin at m.width=120, T5-Review F02)", maxW, wantLiteral)
	}
}

// TestParentPickerBoxLongTitleWrapsWithHangingIndent guards the actual
// PO-reported bug (B06 screenshot: IDs breaking mid-word) end-to-end,
// exercising the NON-cursor row path deliberately (ep-1 has no parent set,
// so openParentPicker's cursor seed lands on the clear row, index 0 -- the
// long-title eligible-parent row sits at index 1, never the cursor): the
// full ID must stay intact on the row's first line, and the continuation
// line must hang-indent under the title's own start column, never column 0.
func TestParentPickerBoxLongTitleWrapsWithHangingIndent(t *testing.T) {
	longID := "ms-averylongidentifier99"
	beans := []data.Bean{
		{ID: longID, Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Focus", Status: "todo", Type: "epic", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.width = 70 // narrow enough that wideModalWidth(70)=60 forces the long title to wrap
	m = focusBean(m, "ep-1")
	m = step(t, m, runeMsg('a'))

	if m.menu.cursor != 0 {
		t.Fatalf("setup: expected cursor on the clear row (index 0, ep-1 has no parent set), got %d -- this test exercises the NON-cursor wrap path", m.menu.cursor)
	}

	out := ansi.Strip(m.parentPickerBox())
	lines := strings.Split(out, "\n")

	idLine := -1
	for i, l := range lines {
		if strings.Contains(l, longID) {
			idLine = i
			break
		}
	}
	if idLine < 0 {
		t.Fatalf("parent picker output missing the full intact ID %q on any single line: %q", longID, out)
	}
	for i, l := range lines {
		if i == idLine {
			continue
		}
		if strings.Contains(l, "ms-a") || strings.Contains(l, "verylongidentifier") {
			t.Errorf("ID fragment leaked onto a different line %d (mid-ID break): %q (full output: %q)", i, l, out)
		}
	}
	if idLine+1 >= len(lines) {
		t.Fatalf("expected the long title to wrap onto a continuation line after line %d, got only %d lines: %q", idLine, len(lines), out)
	}
	titleCol := cellCol(t, lines[idLine], "Hier") // absolute column, includes the modal's 2-cell frame
	gotIndent := contentIndent(t, lines[idLine+1])
	wantIndent := titleCol - 2
	if gotIndent != wantIndent {
		t.Errorf("continuation line indent = %d, want %d (title-start column on the ID's own line): %q", gotIndent, wantIndent, lines[idLine+1])
	}
}

// TestParentPickerRowIndentJitterParityCursorVsNonCursor mirrors
// TestBlockingPickerRowIndentJitterParityCursorVsNonCursor
// (box_picker_blocking_test.go, T5-Review F03, bean bt-4mo9, Mutation-Beweis
// D): the SAME wrapped long-title row's continuation-line indent must not
// shift depending on whether the row currently carries the cursor. Reuses
// TestParentPickerBoxLongTitleWrapsWithHangingIndent's own fixture (cursor
// seeds onto the "(No parent)" clear row at index 0, the long-title eligible
// parent sits at index 1) so the non-cursor state is exercised for free at
// setup, and adds the cursor-on-the-row state explicitly.
func TestParentPickerRowIndentJitterParityCursorVsNonCursor(t *testing.T) {
	longID := "ms-averylongidentifier99"
	beans := []data.Bean{
		{ID: longID, Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "milestone", Priority: "normal"},
		{ID: "ep-1", Title: "Focus", Status: "todo", Type: "epic", Priority: "normal"},
	}

	open := func(t *testing.T) model {
		t.Helper()
		m := fixtureModel(t, beans)
		m.width = 70 // wideModalWidth(70)=60, forces the long title to wrap
		m = focusBean(m, "ep-1")
		return step(t, m, runeMsg('a'))
	}

	setup := open(t)
	targetIdx := -1
	for i, it := range setup.parentItems {
		if it.id == longID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		t.Fatalf("setup: parentItems missing %q, got %+v", longID, setup.parentItems)
	}
	if targetIdx == 0 {
		t.Fatal("setup: targetIdx must differ from the guaranteed non-cursor index 0 (the \"(No parent)\" clear row)")
	}

	indentAt := func(cursor int) int {
		m := open(t)
		m.menu.cursor = cursor

		out := ansi.Strip(m.parentPickerBox())
		lines := strings.Split(out, "\n")
		idLine := -1
		for i, l := range lines {
			if strings.Contains(l, longID) {
				idLine = i
				break
			}
		}
		if idLine < 0 {
			t.Fatalf("cursor=%d: parent picker output missing the full intact ID %q: %q", cursor, longID, out)
		}
		if idLine+1 >= len(lines) {
			t.Fatalf("cursor=%d: expected the long title to wrap onto a continuation line after line %d, got only %d lines: %q", cursor, idLine, len(lines), out)
		}
		return contentIndent(t, lines[idLine+1])
	}

	cursorIndent := indentAt(targetIdx)
	nonCursorIndent := indentAt(0) // "(No parent)" clear row, always index 0
	if cursorIndent != nonCursorIndent {
		t.Errorf("row indent jitters between cursor state (%d) and non-cursor state (%d) -- want identical (T5-Review F03, Mutation-Beweis D)", cursorIndent, nonCursorIndent)
	}
}
