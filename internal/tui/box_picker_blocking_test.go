package tui

// box_picker_blocking_test.go — TDD coverage for the Blocking-Picker (`r`,
// remapped from `B` by Q06, design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc;
// E3 Task 3, bean bt-p1uz): toggle-multi-select over every OTHER bean
// (design decision g: deliberately NO cycle/descendant exclusion, port-
// parity with beans-src blockingpicker.go). Pending-Diff pattern ported
// verbatim from box_picker_tag.go (T2) -- space toggles pending, enter diffs
// pending against original and fires ONE combined data.SetBlocking
// mutateCmd, esc discards.

import (
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// contentIndent measures a rendered picker row's hangingIndentWrap indent,
// skipping modalBox's own constant 2-cell frame (1 border rune + 1
// Padding(0,1) rune) that prefixes every content line inside the box (T5,
// bean bt-4mo9, B06) -- a bare TrimLeft(line, " ") from column 0 would stop
// immediately at the leading "│" border rune (not a space) and always
// report 0, hiding the real hanging-indent width.
func contentIndent(t *testing.T, line string) int {
	t.Helper()
	runes := []rune(line)
	const frameW = 2
	if len(runes) < frameW {
		t.Fatalf("line shorter than the modal frame (%d cells): %q", frameW, line)
	}
	rest := string(runes[frameW:])
	return len(rest) - len(strings.TrimLeft(rest, " "))
}

// --- buildBlockingItems / openBlockingPicker ---

// TestBlockingPickerExcludesOnlySelf guards design decision g directly:
// focused on ep-1 (whose own CHILDREN are tk-1/tk-2, fixtureBeans), the
// picker must offer tk-1 AND tk-2 (its own descendants!) alongside ms-1 --
// only ep-1 itself is missing. This is the exact opposite of the
// Parent-Picker's descendant exclusion (box_picker_parent_test.go), proving
// the two pickers really do use different eligibility rules.
func TestBlockingPickerExcludesOnlySelf(t *testing.T) {
	m := fixtureModel(t, fixtureBeans()) // ms-1 -> ep-1 -> tk-1, tk-2
	m = focusBean(m, "ep-1")

	m = step(t, m, runeMsg('r'))

	if m.overlay != overlayBlockingPicker {
		t.Fatalf("overlay = %v, want overlayBlockingPicker", m.overlay)
	}
	ids := pickerItemIDs(m.blockItems)
	for _, want := range []string{"ms-1", "tk-1", "tk-2"} {
		if !ids[want] {
			t.Errorf("blockItems missing %q, got %+v", want, m.blockItems)
		}
	}
	if ids["ep-1"] {
		t.Fatal("blockItems must exclude the focused bean itself (ep-1)")
	}
	if len(m.blockItems) != 3 {
		t.Fatalf("len(blockItems) = %d, want 3 (every other bean, no cycle exclusion), got %+v", len(m.blockItems), m.blockItems)
	}
}

// TestBlockingPickerSeedsPendingFromBlockingField guards the wholesale-
// replace + independence convention (mirrors
// TestOpenTagPickerSeedsPendingFromFocusedBean): bean-a's Blocking field
// (fixtureBeansWithBlocking, update_test.go) seeds BOTH blockOriginal and
// blockPending, as two independent maps.
func TestBlockingPickerSeedsPendingFromFocusedBean(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking()) // bean-a Blocking: [bean-b]
	m = focusBeanFull(m, "bean-a")

	m = step(t, m, runeMsg('r'))

	if m.overlay != overlayBlockingPicker {
		t.Fatalf("overlay = %v, want overlayBlockingPicker", m.overlay)
	}
	if m.mutTarget != "bean-a" {
		t.Fatalf("mutTarget = %q, want bean-a", m.mutTarget)
	}
	if len(m.blockOriginal) != 1 || !m.blockOriginal["bean-b"] {
		t.Fatalf("blockOriginal = %v, want {bean-b:true}", m.blockOriginal)
	}
	if len(m.blockPending) != 1 || !m.blockPending["bean-b"] {
		t.Fatalf("blockPending = %v, want {bean-b:true}", m.blockPending)
	}

	m.blockPending["scratch-independence-probe"] = true
	if m.blockOriginal["scratch-independence-probe"] {
		t.Fatal("blockOriginal aliases blockPending's backing map -- must be independent")
	}
}

// --- keyBlockingPicker: toggle ---

func TestBlockingPickerToggleFlipsPendingOnly(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m = focusBeanFull(m, "bean-a") // Blocking: [bean-b]
	m = step(t, m, runeMsg('r'))

	// bean bt-a3a8: the cursor addresses blockFiltered (what is on screen),
	// not the unfiltered blockItems -- with no query typed the two lists are
	// identical, so this helper simply follows the picker's real contract.
	cursorTo := func(m model, id string) model {
		for i, it := range m.blockFiltered {
			if it.id == id {
				m.menu.cursor = i
			}
		}
		return m
	}

	m = cursorTo(m, "bean-b")
	m = step(t, m, runeMsg(' '))
	if m.blockPending["bean-b"] {
		t.Fatal("space did not toggle bean-b OFF in blockPending")
	}
	if !m.blockOriginal["bean-b"] {
		t.Fatal("blockOriginal must stay unchanged by a pending toggle")
	}

	// ERRATUM (bean bt-a3a8): this used to press 'x', keys.Toggle's second
	// alias. The picker now hosts an always-focused search field, so 'x'
	// belongs to the INPUT -- intercepting it as a toggle would make it a
	// permanently untypeable character, the exact Review-R1 B01 bug the
	// Tag-Picker already paid for (bean bt-9ipw). Toggle is space-only here
	// now; TestBlockingPickerLettersStayTypeable
	// (box_picker_filter_test.go) guards the other half of that trade.
	m = cursorTo(m, "ep-1")
	m = step(t, m, runeMsg(' '))
	if !m.blockPending["ep-1"] {
		t.Fatal("space did not toggle ep-1 ON in blockPending")
	}
	if m.blockOriginal["ep-1"] {
		t.Fatal("blockOriginal must stay unchanged (ep-1 was never original)")
	}
}

// --- keyBlockingPicker: enter diffs via ONE SetBlocking call ---

// TestBlockingPickerEnterDiffsViaSetBlocking mirrors
// TestTagPickerEnterDiffsFireOneSetTagsCall's verification shape exactly: a
// real data.Store pointed at a nonexistent repo dir (no beans binary
// required), cmd() resolves DIRECTLY to a mutationDoneMsg (not a
// tea.BatchMsg), whose error text names the dispatched CLI subcommand.
func TestBlockingPickerEnterDiffsViaSetBlocking(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	store := newTestStore(t, fixtureBeansWithBlocking()...)
	m.store = store
	m = focusBeanFull(m, "bean-a") // Blocking: [bean-b]
	m = step(t, m, runeMsg('r'))

	for i, it := range m.blockItems {
		switch it.id {
		case "bean-b": // originally present -> toggle off = remove
			m.menu.cursor = i
		}
	}
	m = step(t, m, runeMsg(' '))
	for i, it := range m.blockItems {
		if it.id == "ep-2" { // not originally present -> toggle on = add
			m.menu.cursor = i
		}
	}
	m = step(t, m, runeMsg(' '))

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
	}
	if cmd == nil {
		t.Fatal("enter with pending changes must return a Cmd")
	}

	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg (ONE combined SetBlocking call, not tea.Batch)", msg)
	}
	if mdm.err != nil {
		t.Fatalf("mutationDoneMsg.err = %v, want nil", mdm.err)
	}
	if got := storeItem(t, store, "bean-a"); !got.IsBlocking("ep-2") || got.IsBlocking("bean-b") {
		t.Fatalf("bean-a Blocking on disk = %v, want [ep-2] (proves SetBlocking dispatched)", got.Blocking)
	}
}

func TestBlockingPickerEnterNoChangesNoMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m = focusBeanFull(m, "bean-a")
	m = step(t, m, runeMsg('r'))

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter must close the picker even with no changes")
	}
	if cmd != nil {
		t.Fatal("enter with no pending changes must not fire a mutation Cmd")
	}
}

// --- keyBlockingPicker: esc discards ---

func TestBlockingPickerEscDiscards(t *testing.T) {
	m := fixtureModel(t, fixtureBeansWithBlocking())
	m = focusBeanFull(m, "bean-a")
	m = step(t, m, runeMsg('r'))
	for i, it := range m.blockItems {
		if it.id == "ep-1" {
			m.menu.cursor = i
		}
	}
	m = step(t, m, runeMsg(' ')) // toggle ep-1 on in pending

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("esc did not close the blocking picker")
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// --- blockingPickerBox width/wrap (T5, bean bt-4mo9, B06) ---

// TestBlockingPickerBoxUsesWideModalWidth guards the actual B06 fix: on a
// wide terminal (120 cols), the rendered overlay's border line must be
// substantially wider than the old fixed clampModalWidth(48, ...) box
// (which renders 50 cells wide, see modalBox/TestModalBoxHasRoundedBorder)
// -- ~wideModalWidth(120)+2=104 instead.
func TestBlockingPickerBoxUsesWideModalWidth(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width = 120
	m = focusBean(m, "ep-1")
	m = step(t, m, runeMsg('r'))

	out := m.blockingPickerBox()
	lines := strings.Split(out, "\n")
	maxW := 0
	for _, l := range lines {
		if w := lipgloss.Width(l); w > maxW {
			maxW = w
		}
	}
	oldFixed := clampModalWidth(48, m.width) + 2 // +2 border, byte-parity with modalBox's own overhead
	if maxW <= oldFixed {
		t.Fatalf("blockingPickerBox max line width = %d, want > %d (old fixed-48 box) at m.width=120", maxW, oldFixed)
	}
	wantW := wideModalWidth(m.width) + 2
	if maxW != wantW {
		t.Errorf("blockingPickerBox max line width = %d, want %d (wideModalWidth(120)+2 border)", maxW, wantW)
	}
	// F02 (T5-Review, bean bt-4mo9): wantW above is self-referential (derived
	// from the SAME wideModalWidth call under test) -- a bug inside
	// wideModalWidth itself (e.g. a wrong percentage or clamp) would still
	// pass. Pin an independent literal: wideModalWidth(120) = 120*85/100 =
	// 102 (no floor-60/ceiling-(termW-4)/floor-24 clamp engages at this
	// width) + 2 border = 104.
	const wantLiteral = 104
	if maxW != wantLiteral {
		t.Errorf("blockingPickerBox max line width = %d, want %d (independent literal pin at m.width=120, T5-Review F02)", maxW, wantLiteral)
	}
}

// TestBlockingPickerBoxLongTitleWrapsWithHangingIndent guards the actual
// PO-reported bug (B06 screenshot: IDs breaking mid-word) end-to-end: a
// bean with a long ID and a long title, rendered at a picker width narrow
// enough to force a wrap, must keep the FULL ID intact on the row's first
// line and hanging-indent the continuation line -- never fall back to
// column 0 or split the ID itself.
func TestBlockingPickerBoxLongTitleWrapsWithHangingIndent(t *testing.T) {
	longID := "bt-averylongidentifier99"
	beans := []*item.Item{
		{ID: "ep-1", Title: "Focus", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: longID, Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.width = 70 // narrow enough that wideModalWidth(70)=60 forces the long title to wrap
	m = focusBean(m, "ep-1")
	m = step(t, m, runeMsg('r'))

	out := ansi.Strip(m.blockingPickerBox())
	lines := strings.Split(out, "\n")

	idLine := -1
	for i, l := range lines {
		if strings.Contains(l, longID) {
			idLine = i
			break
		}
	}
	if idLine < 0 {
		t.Fatalf("blocking picker output missing the full intact ID %q on any single line: %q", longID, out)
	}
	for i, l := range lines {
		if i == idLine {
			continue
		}
		if strings.Contains(l, "bt-a") || strings.Contains(l, "verylongidentifier") {
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

// TestBlockingPickerRowIndentJitterParityCursorVsNonCursor is the Jitter-
// Parity regression test the T5-Review demanded (F03, bean bt-4mo9,
// "wertvollste" finding, Mutation-Beweis D): the reviewer reverted the
// non-cursor row's leading pad from 1 space back to 2 (byte-width parity
// with the cursor bar "▌", which is also exactly 1 cell) and the FULL suite
// stayed green -- nothing asserted that a wrapped row's hanging indent
// stays IDENTICAL whether or not that row currently carries the cursor. A 1-
// space regression there would shift the continuation line sideways every
// time the cursor moves onto/off of a wrapped row (visible jitter). This
// renders the SAME wrapped long-title row twice -- once as the cursor row,
// once as a non-cursor row -- and asserts the indent doesn't move.
func TestBlockingPickerRowIndentJitterParityCursorVsNonCursor(t *testing.T) {
	longID := "bt-averylongidentifier99"
	beans := []*item.Item{
		{ID: "ep-1", Title: "Focus", Status: "todo", Type: "epic", Priority: "normal"},
		{ID: longID, Title: "Hier steht ein langer Titel eines beans der so umbricht dass die Uebersicht gewahrt ist", Status: "todo", Type: "task", Priority: "normal"},
		{ID: "bt-other", Title: "Short", Status: "todo", Type: "task", Priority: "normal"},
	}

	open := func(t *testing.T) model {
		t.Helper()
		m := fixtureModel(t, beans)
		m.width = 70 // wideModalWidth(70)=60, forces the long title to wrap (mirrors TestBlockingPickerBoxLongTitleWrapsWithHangingIndent)
		m = focusBean(m, "ep-1")
		return step(t, m, runeMsg('r'))
	}

	// Locate the target row's index by content, not a hardcoded position --
	// buildBlockingItems sorts via data.SortBeans, whose order this test
	// must not have to predict.
	setup := open(t)
	targetIdx := -1
	for i, it := range setup.blockItems {
		if it.id == longID {
			targetIdx = i
			break
		}
	}
	if targetIdx < 0 {
		t.Fatalf("setup: blockItems missing %q, got %+v", longID, setup.blockItems)
	}
	otherIdx := -1
	for i := range setup.blockItems {
		if i != targetIdx {
			otherIdx = i
			break
		}
	}
	if otherIdx < 0 {
		t.Fatal("setup: need at least 2 blockItems to compare cursor vs. non-cursor indent")
	}

	indentAt := func(cursor int) int {
		m := open(t)
		m.menu.cursor = cursor

		out := ansi.Strip(m.blockingPickerBox())
		lines := strings.Split(out, "\n")
		idLine := -1
		for i, l := range lines {
			if strings.Contains(l, longID) {
				idLine = i
				break
			}
		}
		if idLine < 0 {
			t.Fatalf("cursor=%d: blocking picker output missing the full intact ID %q: %q", cursor, longID, out)
		}
		if idLine+1 >= len(lines) {
			t.Fatalf("cursor=%d: expected the long title to wrap onto a continuation line after line %d, got only %d lines: %q", cursor, idLine, len(lines), out)
		}
		return contentIndent(t, lines[idLine+1])
	}

	cursorIndent := indentAt(targetIdx)
	nonCursorIndent := indentAt(otherIdx)
	if cursorIndent != nonCursorIndent {
		t.Errorf("row indent jitters between cursor state (%d) and non-cursor state (%d) -- want identical (T5-Review F03, Mutation-Beweis D)", cursorIndent, nonCursorIndent)
	}
}

// --- vanished-target guard on enter (E3-T3-Review PFLICHT, carried into
// bean bt-ppzb/E3 Task 6: "Vanished-mutTarget-Regressionstests für Parent-
// UND Blocking-Picker (Muster TestValueMenuTargetVanished...)") ---

// TestBlockingPickerEnterTargetVanishedClosesGracefully mirrors
// TestValueMenuTargetVanishedClosesGracefully/
// TestTagPickerEnterTargetVanishedClosesGracefully/
// TestParentPickerEnterTargetVanishedClosesGracefully (design decision d):
// the focused bean disappears (external delete + reload) between open and
// enter -- enter must close the overlay and set a status-line note, WITHOUT
// firing a doomed SetBlocking.
func TestBlockingPickerEnterTargetVanishedClosesGracefully(t *testing.T) {
	beans := fixtureBeansWithBlocking()
	m := fixtureModel(t, beans)
	m = focusBeanFull(m, "bean-a") // Blocking: [bean-b]
	m = step(t, m, runeMsg('r'))
	if m.overlay != overlayBlockingPicker {
		t.Fatal("setup: r did not open the blocking picker")
	}
	for i, it := range m.blockItems {
		if it.id == "ep-1" {
			m.menu.cursor = i
		}
	}
	m = step(t, m, runeMsg(' ')) // ensure a pending change exists

	// bean-a vanishes externally; a reload lands while the picker is still
	// open (m.overlay survives a reload untouched, applyLoaded never touches
	// it).
	var remaining []*item.Item
	for _, b := range beans {
		if b.ID != "bean-a" {
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
