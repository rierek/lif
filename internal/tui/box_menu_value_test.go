package tui

// box_menu_value_test.go — TDD coverage for the combined Status/Type/
// Priority value menu (`s`, E3 Task 1, bean bt-dlgk, design decision a3).
// Reuses fixtureBeans/fixtureModel/step/keyMsg/runeMsg from update_test.go
// (same package).

import (
	"errors"
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

// focusBean expands every ancestor fixtureBeans' tk-1/tk-2 need (ms-1 ->
// ep-1 -> tk-*, both start collapsed) so id actually appears in
// visibleNodes() and cursorPos can find it -- mirrors the same
// ancestor-expand dance update_test.go's detail-focus tests already use
// (e.g. TestDetailFocusRightEntersFieldLevelOnlyForBeziehungenSection).
// Setting m.cursorID alone is not enough: cursorPos falls back to index 0
// (ms-1) for a cursorID absent from the current visibleNodes() slice.
func focusBean(m model, id string) model {
	m.expanded = map[string]bool{"ms-1": true, "ep-1": true}
	m.cursorID = id
	return m
}

// --- openValueMenu / buildValueMenuItems ---

// TestOpenValueMenuBuildsGroupedItemsCursorOnCurrentStatus guards `s` on a
// focused bean: 5 rows (B11/B12, design-spec.md §15 PF-16, bean bt-ntoz --
// `s` is Status-only, the menu used to always return all 15 rows regardless
// of the requested group before this task), mutTarget captures the bean's
// ID, and the cursor seeds onto the bean's CURRENT status within the status
// group (port beans-src statuspicker.go's selectedIndex seed).
func TestOpenValueMenuBuildsGroupedItemsCursorOnCurrentStatus(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	m = step(t, m, runeMsg('s'))

	if len(m.menuItems) != 5 {
		t.Fatalf("menuItems len = %d, want 5 (status group only, B11/B12)", len(m.menuItems))
	}
	if m.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", m.mutTarget)
	}
	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	want := valueMenuCursorFor(m.menuItems, "status", "todo")
	if m.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (todo within the status group)", m.menu.cursor, want)
	}
	if got := m.menuItems[m.menu.cursor]; got.group != "status" || got.value != "todo" {
		t.Fatalf("cursored item = %+v, want {status todo}", got)
	}
}

// TestBuildValueMenuItemsReturnsOnlyRequestedGroup guards B11/B12's core fix:
// buildValueMenuItems(group) returns EXACTLY that group's 5 rows, never the
// other two -- the combined-15-rows behavior it replaces.
func TestBuildValueMenuItemsReturnsOnlyRequestedGroup(t *testing.T) {
	cases := []struct {
		group string
		want  int
	}{
		{"status", len(data.StatusValues())},
		{"type", len(data.TypeValues())},
		{"priority", len(data.PriorityValues())},
	}
	for _, tc := range cases {
		t.Run(tc.group, func(t *testing.T) {
			items := buildValueMenuItems(tc.group)
			if len(items) != tc.want {
				t.Fatalf("buildValueMenuItems(%q) len = %d, want %d", tc.group, len(items), tc.want)
			}
			for _, it := range items {
				if it.group != tc.group {
					t.Fatalf("buildValueMenuItems(%q) leaked item from group %q: %+v", tc.group, it.group, it)
				}
			}
		})
	}
}

// TestOpenValueMenuStatusShowsOnlyStatusItems is openValueMenu's own
// end-to-end counterpart to TestBuildValueMenuItemsReturnsOnlyRequestedGroup
// above -- guards that the group parameter actually reaches
// buildValueMenuItems (not just seeds the cursor, the OLD bug this task
// fixes) for all three groups via the direct method call (T6's own seeding
// test, TestOpenValueMenuSeedsOnGivenGroup, already covers "type" the same
// way -- this test is the explicit B11/B12 "only 5, not 15" regression pin
// the bean's own TDD-Schritte name).
func TestOpenValueMenuStatusShowsOnlyStatusItems(t *testing.T) {
	for _, group := range []string{"status", "type", "priority"} {
		t.Run(group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-2")

			m = m.openValueMenu(group)

			if len(m.menuItems) != 5 {
				t.Fatalf("openValueMenu(%q): menuItems len = %d, want 5", group, len(m.menuItems))
			}
			for _, it := range m.menuItems {
				if it.group != group {
					t.Fatalf("openValueMenu(%q) leaked item from group %q: %+v", group, it.group, it)
				}
			}
		})
	}
}

// TestOpenValueMenuSeedsOnGivenGroup guards T6's openValueMenu(group string)
// signature change (design-spec.md §15 PF-5: the Meta field-level enter
// cascade must be able to seed the combined menu on "type"/"priority", not
// only the `s`-key's hardcoded "status"): calling m.openValueMenu("type")
// against a bean with Type "bug" seeds the cursor onto the type/bug row, not
// status.
func TestOpenValueMenuSeedsOnGivenGroup(t *testing.T) {
	beans := []*item.Item{
		{ID: "bug-1", Title: "A Bug", Status: "todo", Type: "bug", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.cursorID = "bug-1"

	m = m.openValueMenu("type")

	if m.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", m.overlay)
	}
	want := valueMenuCursorFor(m.menuItems, "type", "bug")
	if m.menu.cursor != want {
		t.Fatalf("menu.cursor = %d, want %d (type/bug)", m.menu.cursor, want)
	}
	if got := m.menuItems[m.menu.cursor]; got.group != "type" || got.value != "bug" {
		t.Fatalf("cursored item = %+v, want {type bug}", got)
	}
}

// --- keyValueMenu: enter applies + closes ---

// TestValueMenuEnterAppliesCursoredValueAndCloses guards the immediate-apply
// Enter semantics (design decision a3): moving the cursor onto "in-progress"
// (status group) and pressing enter closes the overlay and dispatches
// SetStatus via a mutateCmd. The dispatch itself is verified through the
// REAL data.Store's error message (pointed at a non-existent repo dir, no
// beans binary required) rather than a mock -- the error text starting with
// "lif update" proves SetStatus (not SetType/SetPriority) actually ran.
func TestValueMenuEnterAppliesCursoredValueAndCloses(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	store := fixtureStore(t)
	m.store = store
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	m = step(t, m, runeMsg('s'))
	// "in-progress" sits BEFORE "todo" in tier order (data.StatusValues()),
	// so it cannot be reached by driving KeyDown from the seeded cursor --
	// set the cursor directly via the same lookup openValueMenu itself uses.
	m.menu.cursor = valueMenuCursorFor(m.menuItems, "status", "in-progress")

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
	}
	if cmd == nil {
		t.Fatal("enter must return a Cmd (mutateCmd)")
	}

	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err != nil {
		t.Fatalf("mutationDoneMsg.err = %v, want nil", mdm.err)
	}
	if got := storeItem(t, store, "tk-2").Status; got != "in-progress" {
		t.Fatalf("status on disk = %q, want in-progress (proves SetStatus dispatched)", got)
	}
}

// TestValueMenuEnterOnTypeGroupDispatchesSetType mirrors the status-group
// test for the type group, guarding that the group->Set* dispatch itself is
// keyed correctly (not just "some mutation fired"). B11/B12 (bean bt-ntoz):
// `s` now opens the status group ONLY (buildValueMenuItems is filtered), so
// unlike before this task, driving KeyDown from an `s`-opened menu can never
// reach the type group anymore -- the type group is reached directly via
// m.openValueMenu("type"), the same call the PF-5 Meta field-level enter
// cascade (keyDetailFocus) and the Palette's new "set type" action use.
func TestValueMenuEnterOnTypeGroupDispatchesSetType(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	store := fixtureStore(t)
	m.store = store
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	m = m.openValueMenu("type")
	if m.menuItems[m.menu.cursor].group != "type" {
		t.Fatalf("setup: cursored group = %q, want type", m.menuItems[m.menu.cursor].group)
	}
	m.menu.cursor = valueMenuCursorFor(m.menuItems, "type", "bug")

	_, cmd := m.Update(keyMsg(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter must return a Cmd")
	}
	mdm := cmd().(mutationDoneMsg)
	if mdm.err != nil {
		t.Fatalf("mutationDoneMsg.err = %v, want nil", mdm.err)
	}
	if got := storeItem(t, store, "tk-2").Type; got != "bug" {
		t.Fatalf("type on disk = %q, want bug (proves SetType dispatched)", got)
	}
}

// TestValueMenuEnterOnPriorityGroupDispatchesSetPriority mirrors the above
// for the priority group (same B11/B12 rationale: reached directly via
// m.openValueMenu("priority"), not via `s` + KeyDown).
func TestValueMenuEnterOnPriorityGroupDispatchesSetPriority(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	store := fixtureStore(t)
	m.store = store
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	m = m.openValueMenu("priority")
	if m.menuItems[m.menu.cursor].group != "priority" {
		t.Fatalf("setup: cursored group = %q, want priority", m.menuItems[m.menu.cursor].group)
	}
	m.menu.cursor = valueMenuCursorFor(m.menuItems, "priority", "high")

	_, cmd := m.Update(keyMsg(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("enter must return a Cmd")
	}
	mdm := cmd().(mutationDoneMsg)
	if mdm.err != nil {
		t.Fatalf("mutationDoneMsg.err = %v, want nil", mdm.err)
	}
	if got := storeItem(t, store, "tk-2").Priority; got != "high" {
		t.Fatalf("priority on disk = %q, want high (proves SetPriority dispatched)", got)
	}
}

// --- keyValueMenu: esc/s close without mutation ---

// TestValueMenuEscClosesWithoutMutation guards esc: overlay closes, no Cmd.
func TestValueMenuEscClosesWithoutMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)
	m = step(t, m, runeMsg('s'))

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("esc did not close the value menu")
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// TestValueMenuSReclosesWithoutMutation guards `s` itself as the second
// close key (design decision a3: "esc/s schließt ohne Mutation") -- mirrors
// keyFilterMenu's own multi-key-closes-without-clearing precedent.
func TestValueMenuSReclosesWithoutMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)
	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: s did not open the value menu")
	}

	tm, cmd := m.Update(runeMsg('s'))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("s did not close the already-open value menu")
	}
	if cmd != nil {
		t.Fatal("s-close must not fire a mutation Cmd")
	}
}

// --- valueMenuBox rendering ---

// TestValueMenuCurrentValueMarked guards the "(current)" marker (port
// beans-src statuspicker.go isCurrent) on the focused bean's actual
// status/type/priority values.
func TestValueMenuCurrentValueMarked(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)
	m = step(t, m, runeMsg('s'))

	out := m.valueMenuBox()
	if !strings.Contains(out, "(current)") {
		t.Fatalf("valueMenuBox() missing a (current) marker:\n%s", out)
	}
}

// TestValueMenuBoxTitleReflectsGroup guards the Planner addition to B11/B12
// (bean bt-y2iw, commit-body-documented as beyond the bare wording): the
// modalPanel title names the open group ("Set status"/"Set type"/
// "Set priority") instead of the old hard-coded "Set value" -- so which of
// the three now-separate menus is open is never ambiguous. An empty
// menuItems slice (defensive, e.g. a zero-value model) falls back to the old
// "Set value" wording.
func TestValueMenuBoxTitleReflectsGroup(t *testing.T) {
	cases := []struct {
		group string
		want  string
	}{
		{"status", "Set status"},
		{"type", "Set type"},
		{"priority", "Set priority"},
	}
	for _, tc := range cases {
		t.Run(tc.group, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-2")
			m = m.openValueMenu(tc.group)

			out := m.valueMenuBox()
			if !strings.Contains(out, tc.want) {
				t.Fatalf("valueMenuBox() for group %q missing title %q:\n%s", tc.group, tc.want, out)
			}
		})
	}

	if got := valueMenuTitle(nil); got != "Set value" {
		t.Fatalf("valueMenuTitle(nil) = %q, want %q (defensive fallback)", got, "Set value")
	}
}

// --- keyNodeAction: focused-bean guard ---

// TestKeyNodeActionRequiresFocusedBeanExceptCreate guards that every T1
// node-action key except Create (s/t/a/r/d/e) is a handled-but-silent no-op
// with no focused bean (an empty repo / the orphan-root cursor), while
// Create (c) is handled regardless -- it works on an empty repo (T4).
// Blocking-Picker key per Q06 remap (design-spec.md §15 PF-16, bean
// bt-ntoz/bt-d8kc): was `B`, now `r`.
func TestKeyNodeActionRequiresFocusedBeanExceptCreate(t *testing.T) {
	m := fixtureModel(t, nil) // no beans -> focusedBean() == nil
	if m.focusedBean() != nil {
		t.Fatal("setup: expected focusedBean() == nil with zero beans loaded")
	}

	for _, k := range []tea.KeyMsg{runeMsg('s'), runeMsg('t'), runeMsg('a'), runeMsg('r'), runeMsg('d'), runeMsg('e')} {
		handled, nm, cmd := m.keyNodeAction(k)
		if !handled {
			t.Fatalf("key %v: handled = false, want true (handled-but-silent no-op)", k)
		}
		if cmd != nil {
			t.Fatalf("key %v: cmd != nil, want nil (no-op, no focused bean)", k)
		}
		mm, ok := nm.(model)
		if !ok {
			t.Fatalf("key %v: keyNodeAction did not return a model, got %T", k, nm)
		}
		if mm.overlay != overlayNone {
			t.Fatalf("key %v: overlay = %v, want overlayNone (no focused bean -- must not open)", k, mm.overlay)
		}
	}

	handled, _, _ := m.keyNodeAction(runeMsg('c'))
	if !handled {
		t.Fatal("c (Create) must be handled even with no focused bean")
	}
}

// TestKeyNodeActionStatusOpensMenuWithFocusedBean is the positive-path
// counterpart: `s` with a real focused bean opens the value menu (already
// covered end-to-end above, but this pins keyNodeAction's own dispatch
// directly, independent of handleKey's capture order).
func TestKeyNodeActionStatusOpensMenuWithFocusedBean(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	handled, nm, cmd := m.keyNodeAction(runeMsg('s'))
	if !handled {
		t.Fatal("s must be handled")
	}
	if cmd != nil {
		t.Fatal("s (open) must not itself fire a Cmd")
	}
	mm := nm.(model)
	if mm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", mm.overlay)
	}
}

// TestKeyNodeActionTypeOpensMenuSeededToType is jira-style-ui experiment
// S4's positive-path pin for the new global `o` key: mirrors
// TestKeyNodeActionStatusOpensMenuWithFocusedBean exactly, but asserts the
// value menu opens seeded to the TYPE group (
// valid in both accordion and box-form modes, same dispatch checkpoint as
// Status).
func TestKeyNodeActionTypeOpensMenuSeededToType(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	handled, nm, cmd := m.keyNodeAction(runeMsg('o'))
	if !handled {
		t.Fatal("o must be handled")
	}
	if cmd != nil {
		t.Fatal("o (open) must not itself fire a Cmd")
	}
	mm := nm.(model)
	if mm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", mm.overlay)
	}
	if len(mm.menuItems) == 0 || mm.menuItems[0].group != "type" {
		t.Fatalf("menuItems seeded group = %+v, want group \"type\"", mm.menuItems)
	}
	if mm.menuItems[mm.menu.cursor].value != "task" {
		t.Fatalf("cursor value = %q, want %q (tk-2's current type)", mm.menuItems[mm.menu.cursor].value, "task")
	}
}

// TestKeyNodeActionPriorityOpensMenuSeededToPriority mirrors
// TestKeyNodeActionTypeOpensMenuSeededToType for the new global `u` key,
// asserting the value menu opens seeded to the PRIORITY group.
func TestKeyNodeActionPriorityOpensMenuSeededToPriority(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)

	handled, nm, cmd := m.keyNodeAction(runeMsg('u'))
	if !handled {
		t.Fatal("u must be handled")
	}
	if cmd != nil {
		t.Fatal("u (open) must not itself fire a Cmd")
	}
	mm := nm.(model)
	if mm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu", mm.overlay)
	}
	if len(mm.menuItems) == 0 || mm.menuItems[0].group != "priority" {
		t.Fatalf("menuItems seeded group = %+v, want group \"priority\"", mm.menuItems)
	}
	if mm.menuItems[mm.menu.cursor].value != "normal" {
		t.Fatalf("cursor value = %q, want %q (tk-2's current priority)", mm.menuItems[mm.menu.cursor].value, "normal")
	}
}

// TestKeyNodeActionTypeAndPriorityNoFocusedBeanIsSilentNoOp mirrors
// TestKeyNodeActionRequiresFocusedBeanExceptCreate for the two new keys:
// handled=true, no overlay opens, without a focused bean.
func TestKeyNodeActionTypeAndPriorityNoFocusedBeanIsSilentNoOp(t *testing.T) {
	m := fixtureModel(t, nil) // no beans -> focusedBean() == nil
	if m.focusedBean() != nil {
		t.Fatal("setup: expected focusedBean() == nil with zero beans loaded")
	}

	for _, k := range []tea.KeyMsg{runeMsg('o'), runeMsg('u')} {
		handled, nm, cmd := m.keyNodeAction(k)
		if !handled {
			t.Fatalf("key %v: handled = false, want true (handled-but-silent no-op)", k)
		}
		if cmd != nil {
			t.Fatalf("key %v: cmd != nil, want nil (no-op, no focused bean)", k)
		}
		mm, ok := nm.(model)
		if !ok {
			t.Fatalf("key %v: keyNodeAction did not return a model, got %T", k, nm)
		}
		if mm.overlay != overlayNone {
			t.Fatalf("key %v: overlay = %v, want overlayNone (no focused bean -- must not open)", k, mm.overlay)
		}
	}
}

// --- applyMutationResult ---

// TestApplyMutationResultConflictSetsStatusLineAndReloads guards the
// ErrConflict branch: m.err carries a "Conflict" note (Toast is E5;
// the status line is the interim channel) and a reload Cmd fires regardless.
func TestApplyMutationResultConflictSetsStatusLineAndReloads(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	err := fmt.Errorf("%w: bean x: boom", data.ErrConflict)

	tm, cmd := m.applyMutationResult(err, "")
	nm := tm.(model)
	if !strings.Contains(nm.err, "Conflict") {
		t.Fatalf("m.err = %q, want it to contain %q", nm.err, "Conflict")
	}
	if cmd == nil {
		t.Fatal("cmd == nil, want a reload Cmd (loadCmd) even on conflict")
	}
}

// TestApplyMutationResultSuccessClearsErrAndReloads guards the success path:
// a stale m.err from a previous failure is cleared and a reload still fires
// (design decision d: unconditional reload, success or failure alike).
func TestApplyMutationResultSuccessClearsErrAndReloads(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.err = "stale error from a previous mutation"

	tm, cmd := m.applyMutationResult(nil, "")
	nm := tm.(model)
	if nm.err != "" {
		t.Fatalf("m.err = %q, want empty (cleared on success)", nm.err)
	}
	if cmd == nil {
		t.Fatal("cmd == nil, want a reload Cmd (loadCmd) on success")
	}
}

// TestApplyMutationResultNonConflictErrorSurfacesRawMessage guards a
// non-ErrConflict failure (e.g. VALIDATION_ERROR): the raw error text
// surfaces verbatim, not the "Conflict" wording.
func TestApplyMutationResultNonConflictErrorSurfacesRawMessage(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	tm, cmd := m.applyMutationResult(errors.New("lif update: VALIDATION_ERROR: bad status"), "")
	nm := tm.(model)
	if strings.Contains(nm.err, "Conflict") {
		t.Fatalf("m.err = %q, must not use the Conflict wording for a non-ErrConflict failure", nm.err)
	}
	if !strings.Contains(nm.err, "VALIDATION_ERROR") {
		t.Fatalf("m.err = %q, want it to surface the raw error text", nm.err)
	}
	if cmd == nil {
		t.Fatal("cmd == nil, want a reload Cmd")
	}
}

// --- beanETag ---

// TestBeanETagReadsLiveIndex guards design decision d: beanETag always reads
// the CURRENT index, so a reload between two calls returns the fresh value,
// never a captured/stale copy.
func TestBeanETagReadsLiveIndex(t *testing.T) {
	beans := fixtureBeans()
	m := fixtureModel(t, beans)

	etag1, ok := m.beanETag("tk-1")
	if !ok {
		t.Fatal("beanETag(tk-1) ok = false, want true")
	}
	if etag1 != m.idx.ByID["tk-1"].ETag() {
		t.Fatalf("beanETag(tk-1) = %q, want %q (live index value)", etag1, m.idx.ByID["tk-1"].ETag())
	}

	changed := append([]*item.Item(nil), beans...)
	for i := range changed {
		if changed[i].ID == "tk-1" {
			c := changed[i].Clone()
			c.Title += " (rotated)"
			changed[i] = c
		}
	}
	wantETag := changed[0].ETag()
	for _, b := range changed {
		if b.ID == "tk-1" {
			wantETag = b.ETag()
		}
	}
	if wantETag == etag1 {
		t.Fatal("setup: the rotated item must carry a different etag")
	}
	m = step(t, m, beansLoadedMsg{beans: changed})

	etag2, ok := m.beanETag("tk-1")
	if !ok || etag2 != wantETag {
		t.Fatalf("beanETag(tk-1) after reload = %q, ok=%v, want %q/true", etag2, ok, wantETag)
	}
}

// TestBeanETagVanishedReturnsNotOK guards the vanished-bean path: an ID no
// longer in the index reports ok=false, never a stale/zero ETag treated as
// valid.
func TestBeanETagVanishedReturnsNotOK(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	if _, ok := m.beanETag("does-not-exist"); ok {
		t.Fatal("beanETag for an unknown ID: ok = true, want false")
	}

	m.idx = nil
	if _, ok := m.beanETag("tk-1"); ok {
		t.Fatal("beanETag with a nil index: ok = true, want false")
	}
}

// --- vanished-target guard on enter ---

// TestValueMenuTargetVanishedClosesGracefully guards the vanished-mutTarget
// path (design decision d): the focused bean disappears from the index
// (external delete + reload) between open and enter -- enter must close the
// overlay and set a status-line note, WITHOUT firing a doomed mutation Cmd.
func TestValueMenuTargetVanishedClosesGracefully(t *testing.T) {
	beans := fixtureBeans()
	m := fixtureModel(t, beans)
	m = focusBean(m, "tk-2") // status=todo, type=task, priority=normal (fixtureBeans, ms-1 -> ep-1 -> tk-2)
	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: s did not open the value menu")
	}

	// tk-2 vanishes externally; a reload lands while the menu is still open
	// (m.overlay survives a reload untouched, applyLoaded never touches it).
	remaining := []*item.Item{beans[0], beans[1], beans[2]} // ms-1, ep-1, tk-1 -- no tk-2
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
