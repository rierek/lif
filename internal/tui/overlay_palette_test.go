package tui

// overlay_palette_test.go — TDD coverage for the Command-Center (`ctrl+k`/
// `K`, E4 Task 1, bean bt-jpgn). Reuses fixtureBeans/fixtureModel/step/
// keyMsg/runeMsg (update_test.go) and focusBean (box_menu_value_test.go),
// same package. E4 Task 2 (bean bt-yo60) had extended this file with a
// bean-search half (design decision b) -- removed again by B13 (design-
// spec.md §15 PF-16/"US-04-Revision", bean bt-ntoz, E8 Task 7, bean
// bt-yqdy): every test here now pins the action-only contract, permanently
// (not just until a hypothetical future T2).

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/data"
)

// --- paletteActions: context-aware ordering (design decision b) ---

// TestPaletteActionsBeanContextFirst guards that a focused bean's node
// actions (status/type/priority/tags/create_tag/parent/blocking/edit_title/
// delete) come BEFORE the global actions (create/go_backlog/...). type/
// priority (B12, design-spec.md §15 PF-16, bean bt-ntoz, E8 Task 6) sit
// directly after status -- the bean's own wording ("paletteActions(), im
// focused-bean-Block, nach 'status':"). create_tag (B14, design-spec.md §15
// PF-16, bean bt-ntoz, E8 Task 7, bean bt-yqdy) sits directly after tags --
// same "grouped with the related action" precedent.
func TestPaletteActionsBeanContextFirst(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // task bean -- focusedBean() != nil

	items := paletteActions(m)
	if len(items) == 0 {
		t.Fatal("paletteActions returned no items")
	}
	if items[0].actionID != "status" {
		t.Fatalf("items[0].actionID = %q, want %q (first node action)", items[0].actionID, "status")
	}

	wantNodeIDs := []string{"status", "type", "priority", "tags", "create_tag", "parent", "blocking", "edit_title", "delete"}
	for i, want := range wantNodeIDs {
		if items[i].actionID != want {
			t.Fatalf("items[%d].actionID = %q, want %q", i, items[i].actionID, want)
		}
	}
	// The global "create" action must appear strictly AFTER every node action.
	createIdx := -1
	for i, it := range items {
		if it.actionID == "create" {
			createIdx = i
			break
		}
	}
	if createIdx == -1 {
		t.Fatal(`"create" action missing from paletteActions`)
	}
	if createIdx < len(wantNodeIDs) {
		t.Fatalf("create action at index %d, want it after all %d node actions", createIdx, len(wantNodeIDs))
	}
}

// TestPaletteActionsIncludesSetTypeAndSetPriority guards B12's Palette
// addition (design-spec.md §15 PF-16, bean bt-ntoz, E8 Task 6): the
// Command-Center gains "set type"/"set priority" labels alongside the
// pre-existing "set status" -- NOT a new keybinding (design decision a3
// still reserves exactly ONE key, `s`, for the whole cluster; these are
// additional Palette-only entry points to the SAME m.openValueMenu(group)
// handler).
func TestPaletteActionsIncludesSetTypeAndSetPriority(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")

	items := paletteActions(m)
	wantLabels := map[string]string{"status": "set status", "type": "set type", "priority": "set priority"}
	for actionID, label := range wantLabels {
		found := false
		for _, it := range items {
			if it.actionID == actionID {
				found = true
				if it.label != label {
					t.Fatalf("action %q label = %q, want %q", actionID, it.label, label)
				}
			}
		}
		if !found {
			t.Fatalf("paletteActions missing action %q (%q)", actionID, label)
		}
	}
}

// TestDispatchPaletteTypeAndPriorityOpenSeededValueMenu guards
// dispatchPalette's new "type"/"priority" cases (B12): each opens the value
// menu filtered AND seeded on that group, exactly mirroring the pre-existing
// "status" case (m.openValueMenu("status")) -- a genuine second entry point
// to the SAME handler, never a parallel implementation (dispatchPalette's own
// doc-stamp).
func TestDispatchPaletteTypeAndPriorityOpenSeededValueMenu(t *testing.T) {
	cases := []struct {
		actionID string
		want     string
	}{
		{"type", "task"},       // tk-2's Type (fixtureBeans)
		{"priority", "normal"}, // tk-2's Priority (fixtureBeans)
	}
	for _, tc := range cases {
		t.Run(tc.actionID, func(t *testing.T) {
			m := fixtureModel(t, fixtureBeans())
			m = focusBean(m, "tk-2")
			m.paletteOpen = true

			nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: tc.actionID, label: "set " + tc.actionID})
			mm, ok := nm.(model)
			if !ok {
				t.Fatalf("dispatchPalette did not return a model, got %T", nm)
			}
			if mm.overlay != overlayValueMenu {
				t.Fatalf("overlay = %v, want overlayValueMenu", mm.overlay)
			}
			if len(mm.menuItems) != 5 {
				t.Fatalf("menuItems len = %d, want 5 (%s group only, B11/B12)", len(mm.menuItems), tc.actionID)
			}
			for _, it := range mm.menuItems {
				if it.group != tc.actionID {
					t.Fatalf("menuItems leaked group %q, want only %q", it.group, tc.actionID)
				}
			}
			got := mm.menuItems[mm.menu.cursor]
			if got.value != tc.want {
				t.Fatalf("cursored value = %q, want %q (tk-2's current %s)", got.value, tc.want, tc.actionID)
			}
		})
	}
}

// TestPaletteActionsIncludesCreateTag guards B14's Palette addition
// (design-spec.md §15 PF-16, bean bt-ntoz, E8 Task 7, bean bt-yqdy): the
// Command-Center gains a "create tag" node action, alongside the pre-
// existing "set tags" -- both dispatch into box_picker_tag.go, "create tag"
// additionally opening its free-text new-tag sub-mode (dispatchPalette).
func TestPaletteActionsIncludesCreateTag(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")

	items := paletteActions(m)
	found := false
	for _, it := range items {
		if it.actionID == "create_tag" {
			found = true
			if it.label != "create tag" {
				t.Fatalf("action %q label = %q, want %q", "create_tag", it.label, "create tag")
			}
		}
	}
	if !found {
		t.Fatal(`paletteActions missing action "create_tag" ("create tag")`)
	}
}

// TestPaletteActionsNoFocusedBeanOmitsNodeActions guards that the orphan
// root (focusedBean() == nil) yields ONLY global actions -- no node action
// IDs (status/type/priority/tags/create_tag/parent/blocking/edit_title/
// delete) leak in.
func TestPaletteActionsNoFocusedBeanOmitsNodeActions(t *testing.T) {
	beans := append(fixtureBeans(), fixtureOrphanBean())
	m := fixtureModel(t, beans)
	m.cursorID = orphanRootID

	if m.focusedBean() != nil {
		t.Fatal("test setup invalid: focusedBean() != nil on the orphan root")
	}

	items := paletteActions(m)
	nodeActionIDs := map[string]bool{
		"status": true, "type": true, "priority": true, "tags": true, "create_tag": true, "parent": true,
		"blocking": true, "edit_title": true, "delete": true,
	}
	for _, it := range items {
		if nodeActionIDs[it.actionID] {
			t.Fatalf("paletteActions leaked node action %q with no focused bean", it.actionID)
		}
	}
	wantGlobal := []string{"create", "go_backlog", "go_browse", "filter", "search", "reload", "repo_picker", "register_project", "go_tags", "settings"}
	if len(items) != len(wantGlobal) {
		t.Fatalf("len(items) = %d, want %d (%v)", len(items), len(wantGlobal), wantGlobal)
	}
	for i, want := range wantGlobal {
		if items[i].actionID != want {
			t.Fatalf("items[%d].actionID = %q, want %q", i, items[i].actionID, want)
		}
	}
}

// fixtureOrphanBean is a bean with a dangling parent, shared by every test
// here that needs the synthetic orphan root to exist (visibleNodes only
// surfaces orphanRootID once at least one such bean is present, port
// precedent: update_test.go's TestOrphanShownUnderSyntheticRoot).
func fixtureOrphanBean() data.Bean {
	return data.Bean{
		ID: "orph-1", Title: "Orphaned Task", Status: "todo", Type: "task",
		Priority: "normal", Parent: "does-not-exist",
	}
}

// --- palFiltered: fuzzy filter + empty-query contract ---

// TestPalFilteredActionsFuzzyFiltered guards that palFiltered fuzzy-filters
// the action label list against m.palQuery.
func TestPalFilteredActionsFuzzyFiltered(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.palQuery = "bckl"

	items := m.palFiltered()
	if len(items) != 1 {
		t.Fatalf("len(palFiltered) = %d, want 1 (only 'go to backlog' matches %q)", len(items), m.palQuery)
	}
	if items[0].actionID != "go_backlog" {
		t.Fatalf("palFiltered[0].actionID = %q, want go_backlog", items[0].actionID)
	}
}

// TestPalFilteredActionsFuzzyStatMatchesSetStatusAndSetParent guards T3-
// review I01 (bean bt-kyj5 Prelude, carried into E7 T4): Plan-Step 4 of the
// English-i18n task (bt-w9o8) claimed a fuzzy-regression test for the 5
// word-reversed PF-8 labels ("status: setzen" -> "set status" etc.) but only
// ever re-verified "bckl" -> "go to backlog", whose word order PF-8 left
// UNCHANGED (lowest-risk case, doesn't exercise the reversal at all). This
// test exercises a query against the ACTUALLY reversed "set ..." labels.
// "stat" is verified (not assumed) to be a rune SUBSEQUENCE -- not a
// contiguous substring, fuzzyMatch is a subsequence matcher (fuzzy.go) -- of
// BOTH "set status" (s-t-a-t contiguous) AND "set parent" (s-t-a-...-t,
// spanning "seT stAT" via the trailing "t" of "parent"): 2 matches, not the
// single match the Prelude note's phrasing suggested was likely.
func TestPalFilteredActionsFuzzyStatMatchesSetStatusAndSetParent(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2") // node actions ("set status"/"set parent") only exist with a focused bean
	m.palQuery = "stat"

	items := m.palFiltered()
	wantIDs := []string{"status", "parent"}
	if len(items) != len(wantIDs) {
		t.Fatalf("len(palFiltered) = %d, want %d (%v) for query %q", len(items), len(wantIDs), wantIDs, m.palQuery)
	}
	for i, want := range wantIDs {
		if items[i].actionID != want {
			t.Fatalf("palFiltered[%d].actionID = %q, want %q", i, items[i].actionID, want)
		}
	}
}

// TestPalFilteredActionsFuzzyGoMatchesAllGoToEntriesPlusRegisterProject
// guards T3-review I01 (bean bt-kyj5 Prelude): the "go to <entity>" entries
// (backlog/browse/repo picker/tags/settings) share PF-8's UNCHANGED "go to
// X" shape -- a plain "go" query must still fuzzy-match every one of them
// (not fewer), in declaration order, with no bean leaking in. E10 Task 2
// (bean bt-r92i) added a 5th such entry ("go to tags", go_tags) -- widening
// this guard from 4 to 5. bt-d3ps (epic-E13-plan.md Item 4) ERRATUM: the new
// "register project" label ALSO fuzzy-matches "go" as a subsequence (r-e-
// "g"-ister project, g before the "o" in "project") -- not a "go to X" entry
// conceptually, but fuzzyMatch (fuzzy.go) is a plain subsequence matcher
// with no phrase-boundary awareness, so it matches anyway. Widening this
// guard to 6 (not renaming its scope away from "go to") is the honest fix --
// asserting 5 here would just be wrong, not a tighter guard. (T5-mini, bean
// bt-uyzf, optional T4-review I01): none of fixtureBeans' titles ("Milestone
// One"/"Epic One"/"Task One"/"Task Two") contain a 'g' at all, so no bean
// item could ever fuzzy-match "go" and silently inflate the count above --
// the exact-length assertion below implicitly guards that absence too.
func TestPalFilteredActionsFuzzyGoMatchesAllGoToEntriesPlusRegisterProject(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.palQuery = "go"

	items := m.palFiltered()
	wantIDs := []string{"go_backlog", "go_browse", "repo_picker", "register_project", "go_tags", "settings"}
	if len(items) != len(wantIDs) {
		t.Fatalf("len(palFiltered) = %d, want %d (%v) for query %q", len(items), len(wantIDs), wantIDs, m.palQuery)
	}
	for i, want := range wantIDs {
		if items[i].actionID != want {
			t.Fatalf("palFiltered[%d].actionID = %q, want %q", i, items[i].actionID, want)
		}
	}
}

// TestPalFilteredEmptyQueryReturnsAllActions pins the T1 contract (B13,
// design-spec.md §15 PF-16/"US-04-Revision", made permanent -- the palette's
// former second, bean-result pool no longer exists at all, so "no bean
// items" is no longer a query-dependent qualifier worth naming in the test):
// an empty query returns every action.
func TestPalFilteredEmptyQueryReturnsAllActions(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.palQuery = ""

	items := m.palFiltered()
	if len(items) != len(paletteActions(m)) {
		t.Fatalf("len(palFiltered) = %d, want %d (all actions)", len(items), len(paletteActions(m)))
	}
	for _, it := range items {
		if it.kind != paletteKindAction {
			t.Fatalf("palFiltered with empty query contains a non-action item: %+v", it)
		}
	}
}

// --- openPalette / keyPalette: open/nav/filter/dispatch/close ---

func TestOpenPaletteResetsQueryAndSeedsList(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.palQuery = "stale"

	nm, _ := m.openPalette()
	mm := nm.(model)
	if !mm.paletteOpen {
		t.Fatal("openPalette did not set paletteOpen")
	}
	if mm.palQuery != "" {
		t.Fatalf("palQuery = %q, want empty on open", mm.palQuery)
	}
	if mm.palList.length != len(mm.palFiltered()) {
		t.Fatalf("palList.length = %d, want %d", mm.palList.length, len(mm.palFiltered()))
	}
}

func TestKeyPaletteEnterDispatchesAndCloses(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.openPalette()
	m = nm.(model)

	m.palQuery = "bckl"
	m.palList.setLen(len(m.palFiltered()))

	nm2, _ := m.keyPalette(keyMsg(tea.KeyEnter))
	mm := nm2.(model)
	if mm.paletteOpen {
		t.Fatal("keyPalette enter did not close the palette")
	}
	if mm.view != viewBacklog {
		t.Fatalf("view = %v, want viewBacklog (go_backlog dispatched)", mm.view)
	}
}

func TestKeyPaletteEscClosesNoSideEffect(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.openPalette()
	m = nm.(model)
	m.palQuery = "bckl"

	nm2, _ := m.keyPalette(keyMsg(tea.KeyEsc))
	mm := nm2.(model)
	if mm.paletteOpen {
		t.Fatal("esc did not close the palette")
	}
	if mm.view != viewBrowseRepo {
		t.Fatalf("view = %v, want viewBrowseRepo unchanged (esc must not dispatch)", mm.view)
	}
}

func TestKeyPaletteRuneAppendsAndResyncsList(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.openPalette()
	m = nm.(model)

	nm2, _ := m.keyPalette(runeMsg('c'))
	mm := nm2.(model)
	if mm.palQuery != "c" {
		t.Fatalf("palQuery = %q, want %q", mm.palQuery, "c")
	}
	if mm.palList.length != len(mm.palFiltered()) {
		t.Fatalf("palList.length = %d, want %d (resynced)", mm.palList.length, len(mm.palFiltered()))
	}
}

func TestKeyPaletteBackspaceTrims(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.openPalette()
	m = nm.(model)
	m.palQuery = "cr"
	m.palList.setLen(len(m.palFiltered()))

	nm2, _ := m.keyPalette(keyMsg(tea.KeyBackspace))
	mm := nm2.(model)
	if mm.palQuery != "c" {
		t.Fatalf("palQuery = %q, want %q", mm.palQuery, "c")
	}
}

// --- Capture-Order (design decision h) ---

func TestHandleKeyPaletteOpensPaletteFromTree(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('K'))
	if !m.paletteOpen {
		t.Fatal("K from the tree did not open the palette")
	}
}

// TestHandleKeyCtrlKNoLongerBound guards bean bt-mx4k: the Command-Center used
// to answer to BOTH ctrl+k and K. The PO retired ctrl+k (one key, one
// function; D07 case convention: uppercase = view/global) -- K is now the
// sole binding, and the header six characters shorter, which matters at 80
// columns.
func TestHandleKeyCtrlKNoLongerBound(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, tea.KeyMsg{Type: tea.KeyCtrlK})
	if m.paletteOpen {
		t.Fatal("ctrl+k opened the palette -- the binding was retired in bt-mx4k, K is the only key")
	}
}

func TestHandleKeyPaletteUnreachableWhileFilterOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.filterOpen = true
	m = step(t, m, runeMsg('K'))
	if m.paletteOpen {
		t.Fatal("K opened the palette while filterOpen -- capture order violated (design decision h)")
	}
	if !m.filterOpen {
		t.Fatal("filterOpen was cleared -- K must have been swallowed by keyFilterMenu, not routed to the palette")
	}
}

func TestHandleKeyPaletteOpensPaletteEvenWithOverlayOpen(t *testing.T) {
	// Sanity: an E3 node-action overlay (e.g. Value-Menu) captures BEFORE
	// the palette check -- K must NOT reach openPalette while overlay
	// != overlayNone (same capture-order contract as filterOpen above).
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('s')) // opens the value menu overlay
	if m.overlay == overlayNone {
		t.Fatal("test setup invalid: value menu did not open")
	}
	m = step(t, m, runeMsg('K'))
	if m.paletteOpen {
		t.Fatal("K opened the palette while an overlay was open -- capture order violated")
	}
}

// --- F1 in-flight guard (Review-Runde 2, Async-Gap-Clobbering, Finding 1b) ---

// TestDispatchPaletteCreateIgnoredWhileCreateInFlight guards dispatchPalette's
// "create" case OWN copy of the F1 in-flight guard -- a THIRD call site of
// the same single-create invariant keyNodeAction's Create case (update.go)
// and submitForm's "create" case (box_confirm_create.go) already enforce
// (types.go doc-stamp). The Command-Center is a genuine second entry point
// to the SAME handlers (dispatchPalette's own doc-stamp), so its "create"
// case must refuse a second Create-Form exactly like the `c` key does while
// m.pendingCreate != nil -- otherwise ctrl+k -> "create bean" would
// cross-contaminate the single createDraft/pendingCreate slots the same way
// Finding 1b originally closed for the `c` key.
func TestDispatchPaletteCreateIgnoredWhileCreateInFlight(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.pendingCreate = func() tea.Msg { return nil } // simulate an earlier create already in flight
	m.paletteOpen = true

	nm, cmd := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "create", label: "create bean"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	_ = cmd
	if mm.form != nil {
		t.Fatal("palette create while a create is in flight must not open a second Create-Form")
	}
	if mm.overlay == overlayCreateConfirm {
		t.Fatal("palette create while a create is in flight must not open a second Confirm-Gate")
	}
	if mm.err != createInFlightNote {
		t.Fatalf("err = %q, want createInFlightNote %q", mm.err, createInFlightNote)
	}
	if mm.pendingCreate == nil {
		t.Fatal("pendingCreate must remain set -- the original in-flight create must not be forgotten")
	}
	// bt-81f0: m.err no longer renders anywhere -- Toast is the ONE visible
	// channel, this guard must not go silent. kind=toastWarn (bt-tm4a): the
	// in-flight guard is a hint ("please wait"), not a hard error -- all
	// three createInFlightNote guards now agree on toastWarn, matching
	// keyNodeAction's original choice (update.go:747, see also
	// box_confirm_create_test.go's twin assertion).
	if mm.toast == nil {
		t.Fatal("palette create dropping the second create must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if mm.toast.kind != toastWarn {
		t.Errorf("toast.kind = %v, want toastWarn (bt-tm4a: unified in-flight-guard severity)", mm.toast.kind)
	}
}

// --- create_tag dispatch guard (B14, design-spec.md §15 PF-16, bean bt-ntoz, E8 Task 7, bean bt-yqdy) ---

// TestDispatchPaletteCreateTagOpensTagPickerReadyToType guards the happy
// path post-bt-9ipw-consolidation (D01, epic-E12-plan.md »Item 1«): with a
// focused bean, "create tag" opens the Tag-Picker (overlayTagPicker) with
// its search field ALREADY focused and ready to type -- since D01 merged
// the former separate free-text new-tag sub-mode into the Haupt-Picker
// itself, there is no second "input mode" left to enter; opening the picker
// IS the ready-to-create state.
func TestDispatchPaletteCreateTagOpensTagPickerReadyToType(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m.paletteOpen = true

	nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "create_tag", label: "create tag"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	if mm.paletteOpen {
		t.Fatal("dispatchPalette must close the palette")
	}
	if mm.overlay != overlayTagPicker {
		t.Fatalf("overlay = %v, want overlayTagPicker", mm.overlay)
	}
	if !mm.tagInput.Focused() {
		t.Fatal("create_tag must land on the Tag-Picker's search field already focused (D01, no second gate)")
	}
}

// TestDispatchPaletteCreateTagNoFocusedBeanNoOp guards the PFLICHT-Guard
// (bean bt-yqdy's own wording): without a focused bean, "create tag" must be
// a clean no-op -- relying on openTagPicker()'s own internal no-op guard
// (returns m unchanged, overlay untouched, when focusedBean()==nil), the
// SAME guard the plain "tags" case above relies on -- post-D01 the two
// actions are the identical handler, so there is no separate chain left to
// guard here.
func TestDispatchPaletteCreateTagNoFocusedBeanNoOp(t *testing.T) {
	beans := append(fixtureBeans(), fixtureOrphanBean())
	m := fixtureModel(t, beans)
	m.cursorID = orphanRootID
	m.paletteOpen = true

	if m.focusedBean() != nil {
		t.Fatal("test setup invalid: focusedBean() != nil on the orphan root")
	}

	nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "create_tag", label: "create tag"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	if mm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (no focused bean -- must not open the Tag-Picker)", mm.overlay)
	}
}

// --- register_project: bt-d3ps (epic-E13-plan.md Item 4, PO-Redefinition
// Grilling 2026-07-17 -- replaces the earlier discovery-scan design: NO
// scan, NO discovery roots, NO find-persistence) ---

// TestPaletteActionsIncludesRegisterProject guards the new global entry,
// grouped directly after "repo_picker" (same repo-registry neighborhood,
// plan's own "Platzierung neben repo_picker/settings").
func TestPaletteActionsIncludesRegisterProject(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	items := paletteActions(m)

	idx := -1
	for i, it := range items {
		if it.actionID == "register_project" {
			idx = i
			if it.label != "register project" {
				t.Fatalf("register_project label = %q, want %q", it.label, "register project")
			}
		}
	}
	if idx == -1 {
		t.Fatal(`"register_project" missing from paletteActions`)
	}

	repoPickerIdx := -1
	for i, it := range items {
		if it.actionID == "repo_picker" {
			repoPickerIdx = i
		}
	}
	if repoPickerIdx == -1 {
		t.Fatal(`test setup invalid: "repo_picker" missing from paletteActions`)
	}
	if idx <= repoPickerIdx {
		t.Fatalf("register_project at index %d, want it after repo_picker (index %d)", idx, repoPickerIdx)
	}
}

// TestRegisterProjectNilClientNoOp guards the m.client == nil guard (Palette
// opened from the Lobby itself, no live repo) -- no-op, never crash, no
// toast, settings unchanged.
func TestRegisterProjectNilClientNoOp(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolation: this path must never touch real config.yaml
	m := fixtureModel(t, fixtureBeans())
	if m.client != nil {
		t.Fatal("test setup invalid: fixtureModel's client is not nil")
	}
	before := append([]string{}, m.settings.Repos...)

	nm, cmd := m.registerProject()
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("registerProject did not return a model, got %T", nm)
	}
	if cmd != nil {
		t.Fatal("registerProject with nil client must return a nil cmd (no-op)")
	}
	if mm.toast != nil {
		t.Fatalf("toast = %+v, want nil (no-op must not show a toast)", mm.toast)
	}
	if len(mm.settings.Repos) != len(before) {
		t.Fatalf("settings.Repos changed on a nil-client no-op: %v", mm.settings.Repos)
	}
}

// TestRegisterProjectAlreadyRegisteredShowsInfoToastNoDuplicate guards the
// dedup branch: a repo already in m.settings.Repos gets an "already
// registered" toastInfo, no duplicate entry, no disk write.
func TestRegisterProjectAlreadyRegisteredShowsInfoToastNoDuplicate(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolation: dedup branch must never reach SaveUserSettings
	repoDir := "/tmp/bt-d3ps-already-registered"
	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: repoDir}
	m.settings.Repos = []string{repoDir}

	nm, cmd := m.registerProject()
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("registerProject did not return a model, got %T", nm)
	}
	if cmd == nil {
		t.Fatal("registerProject (already registered) must still return a Cmd (toast auto-dismiss timer)")
	}
	if mm.toast == nil {
		t.Fatal("toast is nil, want a toastInfo \"already registered\" toast")
	}
	if mm.toast.kind != toastInfo {
		t.Fatalf("toast.kind = %v, want toastInfo", mm.toast.kind)
	}
	if mm.toast.title != "already registered" {
		t.Fatalf("toast.title = %q, want %q", mm.toast.title, "already registered")
	}
	if len(mm.settings.Repos) != 1 {
		t.Fatalf("settings.Repos = %v, want exactly 1 entry (no duplicate)", mm.settings.Repos)
	}
}

// TestRegisterProjectSuccessSavesSettingsAndToasts guards the success path:
// a NOT-yet-registered repo gets appended to config.yaml (via
// config.SaveUserSettings, existing signature) AND to the in-model
// m.settings.Repos, plus a "Registered: <slug>" toastInfo.
func TestRegisterProjectSuccessSavesSettingsAndToasts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, ".lif.yml"), []byte("lif:\n    prefix: lt-\n"), 0o644); err != nil {
		t.Fatalf("write .lif.yml: %v", err)
	}

	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: repoDir}
	if len(m.settings.Repos) != 0 {
		t.Fatalf("test setup invalid: settings.Repos = %v, want empty", m.settings.Repos)
	}

	nm, cmd := m.registerProject()
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("registerProject did not return a model, got %T", nm)
	}
	if cmd == nil {
		t.Fatal("registerProject (success) must return a Cmd (toast auto-dismiss timer)")
	}
	if mm.toast == nil {
		t.Fatal("toast is nil, want a toastInfo \"Registered: lt\" toast")
	}
	if mm.toast.kind != toastInfo {
		t.Fatalf("toast.kind = %v, want toastInfo", mm.toast.kind)
	}
	if want := "Registered: lt"; mm.toast.title != want {
		t.Fatalf("toast.title = %q, want %q", mm.toast.title, want)
	}

	found := false
	for _, r := range mm.settings.Repos {
		if r == repoDir {
			found = true
		}
	}
	if !found {
		t.Fatalf("settings.Repos = %v, missing %q", mm.settings.Repos, repoDir)
	}

	saved, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("config.LoadSettings() after registerProject: %v", err)
	}
	found = false
	for _, r := range saved.Repos {
		if r == repoDir {
			found = true
		}
	}
	if !found {
		t.Fatalf("config.yaml Repos = %v, missing %q -- SaveUserSettings not persisted", saved.Repos, repoDir)
	}
}

// TestDispatchPaletteRegisterProjectRoutesToRegisterProject guards the
// dispatchPalette wiring itself (thin pass-through, mirrors every other
// action-ID case's own doc-stamp) -- exercised end-to-end via the nil-client
// no-op shape (cheapest deterministic assertion: palette closes, no crash).
func TestDispatchPaletteRegisterProjectRoutesToRegisterProject(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := fixtureModel(t, fixtureBeans())
	m.paletteOpen = true

	nm, cmd := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "register_project", label: "register project"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	if mm.paletteOpen {
		t.Fatal("dispatchPalette must close the palette")
	}
	if cmd != nil {
		t.Fatal("dispatchPalette(register_project) with nil client must return a nil cmd (registerProject's own no-op)")
	}
}
