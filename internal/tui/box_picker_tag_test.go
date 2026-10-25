package tui

// box_picker_tag_test.go — TDD coverage for the Tag-Picker (`g`, E3 Task 2,
// bean bt-8v69): usage-counted toggle-multi-select + free-text new-tag
// entry + Pending-Diff (Port beans-src blockingpicker.go's Enter=confirm/
// esc=cancel convention -- NOT box_filter_facets.go's enter=close-without-
// mutating, since this overlay actually mutates). Reuses fixtureBeans/
// fixtureModel/step/keyMsg/runeMsg/focusBean from update_test.go/
// box_menu_value_test.go (same package).

import (
	"github.com/rierek/lif/pkg/item"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/data"
)

// fixtureBeansTagged layers tags onto fixtureBeans' fixed hierarchy so the
// tag-count/pending-diff tests have real cross-bean usage counts to work
// with: urgent appears on tk-1 AND tk-2 (count 2), backend and zeta each
// appear once (count 1, tied -- exercises collectTagCounts' alpha
// tie-break: backend < zeta).
func fixtureBeansTagged() []*item.Item {
	beans := fixtureBeans()
	for i := range beans {
		switch beans[i].ID {
		case "tk-1":
			beans[i].Tags = []string{"urgent", "backend"}
		case "tk-2":
			beans[i].Tags = []string{"urgent"}
		case "ep-1":
			beans[i].Tags = []string{"zeta"}
		}
	}
	return beans
}

// tagPickerCursorTo moves m.tagInputSuggestCursor onto the row whose tag ==
// tag WITHIN the currently live-filtered m.tagInputFiltered (bean bt-9ipw,
// D01: the Haupt-Picker's cursor moves over the filtered list, not the
// unfiltered tagItems, since D01 consolidated the search field into this
// overlay) -- failing the test if no such row exists -- shared setup step
// for every toggle-driving test below. Every call site below invokes this
// right after opening the picker with an untouched (empty) query, where
// tagInputFiltered == tagItems (filterTagItems("") is a full-list copy in
// the SAME order), so positions line up exactly as before this rename.
func tagPickerCursorTo(t *testing.T, m model, tag string) model {
	t.Helper()
	for i, it := range m.tagInputFiltered {
		if it.tag == tag {
			m.tagInputSuggestCursor = i
			return m
		}
	}
	t.Fatalf("tag %q not found in tagInputFiltered (%+v)", tag, m.tagInputFiltered)
	return m
}

// --- collectTagCounts ---

func TestCollectTagCountsSortedByCountDescThenAlpha(t *testing.T) {
	idx := data.NewIndex(fixtureBeansTagged())
	got := collectTagCounts(idx, nil)

	want := []tagCount{
		{tag: "urgent", count: 2},
		{tag: "backend", count: 1},
		{tag: "zeta", count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("collectTagCounts() = %+v, want len %d", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collectTagCounts()[%d] = %+v, want %+v (full: %+v)", i, got[i], want[i], got)
		}
	}
}

// --- collectTagCounts: D10 Suggest-Mode (T6, bean bt-pqq3, epic bt-362n) ---

// TestCollectTagCountsDefinedFirstThenCountDescThenAlpha is the bean
// bt-pqq3 body's own RED-Zitat verbatim: "defined" becomes the NEW PRIMARY
// sort key -- a registry-defined tag sorts before a free tag even when its
// usage count is far lower (rare-defined, count 1, beats popular-free,
// count 3).
func TestCollectTagCountsDefinedFirstThenCountDescThenAlpha(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "b1", Tags: []string{"popular-free"}},
		{ID: "b2", Tags: []string{"popular-free"}},
		{ID: "b3", Tags: []string{"popular-free"}},
		{ID: "b4", Tags: []string{"rare-defined"}},
	})
	defined := map[string]bool{"rare-defined": true}
	got := collectTagCounts(idx, defined)
	if len(got) != 2 || got[0].tag != "rare-defined" || !got[0].defined {
		t.Fatalf("want defined tag first despite lower count, got %+v", got)
	}
	if got[1].tag != "popular-free" || got[1].defined {
		t.Fatalf("want free tag second, got %+v", got)
	}
}

// TestCollectTagCountsIncludesUnusedDefinedTagAtCountZero guards the
// Suggest-Mode Union: a registry-defined tag with ZERO current usage must
// still appear (count 0, defined=true) -- mirrors D09's "auch mit Count 0"
// wording one layer up (view_tag_management.go's tagRegistryRows) and is
// the concrete behavior behind this task's own tmux-Smoke acceptance
// wording "unbenutzt-definierte sichtbar". "defined" stays the PRIMARY sort
// key even at count 0: ghost-defined (defined, count 0) still sorts before
// in-use-free (free, count 1).
func TestCollectTagCountsIncludesUnusedDefinedTagAtCountZero(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "b1", Tags: []string{"in-use-free"}},
	})
	defined := map[string]bool{"ghost-defined": true}
	got := collectTagCounts(idx, defined)
	if len(got) != 2 {
		t.Fatalf("want 2 rows (in-use-free + unused ghost-defined), got %+v", got)
	}
	if got[0].tag != "ghost-defined" || !got[0].defined || got[0].count != 0 {
		t.Fatalf("want ghost-defined first (defined, count 0), got %+v", got[0])
	}
	if got[1].tag != "in-use-free" || got[1].defined || got[1].count != 1 {
		t.Fatalf("want in-use-free second (free, count 1), got %+v", got[1])
	}
}

// --- openTagPicker ---

func TestOpenTagPickerSeedsPendingFromFocusedBean(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // tags: urgent

	m = step(t, m, runeMsg('g'))

	if m.overlay != overlayTagPicker {
		t.Fatalf("overlay = %v, want overlayTagPicker", m.overlay)
	}
	if m.mutTarget != "tk-2" {
		t.Fatalf("mutTarget = %q, want tk-2", m.mutTarget)
	}
	if len(m.tagOriginal) != 1 || !m.tagOriginal["urgent"] {
		t.Fatalf("tagOriginal = %v, want {urgent:true}", m.tagOriginal)
	}
	if len(m.tagPending) != 1 || !m.tagPending["urgent"] {
		t.Fatalf("tagPending = %v, want {urgent:true}", m.tagPending)
	}
	if len(m.tagItems) != 3 {
		t.Fatalf("tagItems len = %d, want 3 (urgent/backend/zeta)", len(m.tagItems))
	}

	// tagOriginal/tagPending must be two INDEPENDENT maps, not one aliasing
	// the other (design decision, plan »Task 2«: "zwei UNABHÄNGIGE Maps").
	m.tagPending["scratch-independence-probe"] = true
	if m.tagOriginal["scratch-independence-probe"] {
		t.Fatal("tagOriginal aliases tagPending's backing map -- must be independent")
	}
}

// --- openTagPicker: D03 fresh Tag-Registry load + D10 Suggest-Mode marking
// (T6, bean bt-pqq3, epic bt-362n) ---

// TestOpenTagPickerLoadsRegistryFreshMarksDefinedIncludingUnused writes a
// real .lif-tags.yml (T1, internal/data/tagdefs.go) to a temp repo dir,
// points m.store at it, and verifies openTagPicker (D03: loads the
// registry FRESH on every open) marks tagItems[].defined correctly AND
// surfaces a registry-defined tag that is not currently used on ANY bean
// (ghost-defined, count 0) -- the concrete behavior behind this task's own
// tmux-Smoke wording "unbenutzt-definierte sichtbar".
func TestOpenTagPickerLoadsRegistryFreshMarksDefinedIncludingUnused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif-tags.yml"), []byte("tags:\n  - urgent\n  - ghost-defined\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := fixtureModel(t, fixtureBeansTagged())
	m.store = &data.Store{RepoDir: dir}
	m = focusBean(m, "tk-2") // tags: urgent

	m = step(t, m, runeMsg('g'))

	want := []tagCount{
		{tag: "urgent", count: 2, defined: true},
		{tag: "ghost-defined", count: 0, defined: true},
		{tag: "backend", count: 1, defined: false},
		{tag: "zeta", count: 1, defined: false},
	}
	if len(m.tagItems) != len(want) {
		t.Fatalf("tagItems = %+v, want len %d", m.tagItems, len(want))
	}
	for i := range want {
		if m.tagItems[i] != want[i] {
			t.Fatalf("tagItems[%d] = %+v, want %+v (full: %+v)", i, m.tagItems[i], want[i], m.tagItems)
		}
	}
}

// TestOpenTagPickerNilClientDegradesToNoDefinedTags mirrors D02's
// tolerant-missing philosophy one layer up (same precedent as
// TestOpenTagManagementPageNilClientBuildsFromIdxOnly, view_tag_management_test.go,
// T2): a nil m.store (pre-load/test fixture -- fixtureModel's own default)
// must never panic on m.store.LoadTagDefs() and must degrade to an empty
// registry, i.e. every row free (defined=false).
func TestOpenTagPickerNilClientDegradesToNoDefinedTags(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")

	m = step(t, m, runeMsg('g')) // must not panic on a nil m.store

	for _, it := range m.tagItems {
		if it.defined {
			t.Fatalf("nil client must degrade to an empty registry -- no tag should be marked defined, got %+v", it)
		}
	}
}

// TestOpenTagPickerFreeTagRemainsTogglableAndSavable is the Akzeptanz-
// Checkliste's explicit "kein strict mode" regression: with a REAL registry
// loaded (urgent is defined), a NOT-defined tag (backend) must still be
// togglable AND fire a real save mutation on enter -- Suggest-Mode only
// affects sort/display, never selectability.
func TestOpenTagPickerFreeTagRemainsTogglableAndSavable(t *testing.T) {
	store := newTestStore(t, fixtureBeansTagged()...)
	if err := os.WriteFile(filepath.Join(store.RepoDir, ".lif-tags.yml"), []byte("tags:\n  - urgent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := fixtureModel(t, fixtureBeansTagged())
	m.store = store
	m = focusBean(m, "tk-2") // tags: urgent
	m = step(t, m, runeMsg('g'))

	m = tagPickerCursorTo(t, m, "backend") // NOT in the registry -- free tag
	m = step(t, m, runeMsg(' '))           // toggle on
	if !m.tagPending["backend"] {
		t.Fatal("a free (non-defined) tag must still be togglable in Suggest-Mode")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter must close the picker")
	}
	if cmd == nil {
		t.Fatal("a free tag toggle must still fire a save mutation (no strict mode)")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok || mdm.err != nil {
		t.Fatalf("cmd() = %+v, want a successful mutationDoneMsg -- free tag save must dispatch a real SetTags call", msg)
	}
	if !storeItem(t, store, "tk-2").HasTag("backend") {
		t.Fatal("the free tag did not land on tk-2 on disk")
	}
}

// --- keyTagPicker: toggle ---

func TestTagPickerToggleFlipsPendingOnly(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // tags: urgent
	m = step(t, m, runeMsg('g'))

	m = tagPickerCursorTo(t, m, "urgent")
	m = step(t, m, runeMsg(' '))
	if m.tagPending["urgent"] {
		t.Fatal("space did not toggle urgent OFF in tagPending")
	}
	if !m.tagOriginal["urgent"] {
		t.Fatal("tagOriginal must stay unchanged by a pending toggle")
	}

	// ERRATUM/D01-Nachtrag (Review-R1 B01): "x" no longer toggles here --
	// it is a literal, typeable search character (keys.TagToggle is
	// space-only). The second toggle exercises space again on a different
	// row; "x"'s literal-typing guarantee has its own dedicated test
	// (TestTagPickerTypedXStaysLiteralNotToggle).
	m = tagPickerCursorTo(t, m, "backend")
	m = step(t, m, runeMsg(' '))
	if !m.tagPending["backend"] {
		t.Fatal("space did not toggle backend ON in tagPending")
	}
	if m.tagOriginal["backend"] {
		t.Fatal("tagOriginal must stay unchanged (backend was never original)")
	}
}

// --- keyTagPicker: enter diffs via ONE SetTags call ---

// TestTagPickerEnterDiffsFireOneSetTagsCall guards the E3 Task 2 design
// decision (ERRATUM vs. the plan's own earlier sketch, settled in the
// "Design-Nachtrag" of epic-E3-plan.md »Task 2«): the diff (one add, one
// remove) fires as ONE combined data.SetTags call, not two separate
// AddTag/RemoveTag mutateCmds batched via tea.Batch -- N sequential
// mutations against ONE etag would be a conflict cascade. Verified two
// ways: cmd() resolves DIRECTLY to a mutationDoneMsg (a tea.Batch would
// instead resolve to a tea.BatchMsg), and the item on disk (real temp store)
// carries the tag diff, proving SetTags actually dispatched.
func TestTagPickerEnterDiffsFireOneSetTagsCall(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	store := newTestStore(t, fixtureBeansTagged()...)
	m.store = store
	m = focusBean(m, "tk-2") // tags: urgent
	m = step(t, m, runeMsg('g'))

	m = tagPickerCursorTo(t, m, "urgent") // originally present -> toggle off = remove
	m = step(t, m, runeMsg(' '))
	m = tagPickerCursorTo(t, m, "backend") // not originally present -> toggle on = add
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
		t.Fatalf("cmd() = %T, want mutationDoneMsg (ONE combined SetTags call, not tea.Batch)", msg)
	}
	if mdm.err != nil {
		t.Fatalf("mutationDoneMsg.err = %v, want nil", mdm.err)
	}
	if got := storeItem(t, store, "tk-2"); !got.HasTag("backend") || got.HasTag("urgent") {
		t.Fatalf("tk-2 tags on disk = %v, want urgent replaced by backend (proves SetTags dispatched)", got.Tags)
	}
}

func TestTagPickerEnterNoChangesNoMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter must close the picker even with no changes")
	}
	if cmd != nil {
		t.Fatal("enter with no pending changes must not fire a mutation Cmd")
	}
}

// --- keyTagPicker: esc discards ---

func TestTagPickerEscDiscardsPending(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	m = tagPickerCursorTo(t, m, "backend")
	m = step(t, m, runeMsg(' ')) // toggle backend on in pending

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("esc did not close the tag picker")
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// --- keyTagPicker/tagPickerEnter: create-path on no substring match (bean bt-9ipw, D01 consolidation) ---

// TestTagPickerNewTagValidatesRegex types DIRECTLY into the Haupt-Picker's
// always-focused search field (no `n` gate anymore, D01) -- an invalid tag
// name with zero substring matches must set tagInputErr and keep the picker
// open for a retry.
func TestTagPickerNewTagValidatesRegex(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	before := len(m.tagItems)

	for _, r := range "Über!" {
		m = step(t, m, runeMsg(r))
	}
	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.tagInputErr == "" {
		t.Fatal("invalid new tag name must set tagInputErr")
	}
	if m.overlay != overlayTagPicker {
		t.Fatal("invalid submit must keep the picker open for a retry, not close it")
	}
	if len(m.tagItems) != before {
		t.Fatalf("tagItems len changed on invalid submit: %d -> %d, want no change", before, len(m.tagItems))
	}
}

// TestTagPickerNewTagAddsPendingItem types a valid, non-matching tag name
// directly into the Haupt-Picker's search field and confirms with enter:
// the new tag is created+pending immediately, and (D01's own "keep going"
// design, unlike the old sub-mode which closed) the query clears back to the
// full list rather than closing the whole picker.
func TestTagPickerNewTagAddsPendingItem(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	before := len(m.tagItems)

	for _, r := range "greenfield" {
		m = step(t, m, runeMsg(r))
	}
	m = step(t, m, keyMsg(tea.KeyEnter))

	if m.tagInputErr != "" {
		t.Fatalf("valid new tag name must not set tagInputErr, got %q", m.tagInputErr)
	}
	if m.overlay != overlayTagPicker {
		t.Fatal("a create-on-no-match submit must NOT close the picker (D01: keep going)")
	}
	if m.tagInput.Value() != "" {
		t.Fatalf("tagInput.Value() = %q, want cleared back to \"\" after create", m.tagInput.Value())
	}
	if len(m.tagItems) != before+1 {
		t.Fatalf("tagItems len = %d, want %d (one new row)", len(m.tagItems), before+1)
	}
	if !m.tagPending["greenfield"] {
		t.Fatal("a freshly created tag must be pending=true immediately")
	}
	found := false
	for _, it := range m.tagItems {
		if it.tag == "greenfield" {
			found = true
		}
	}
	if !found {
		t.Fatal("greenfield missing from tagItems after a valid submit")
	}
}

// --- keyTagPicker: consolidated search field (bean bt-9ipw, US-07-Reopen 2026-07-17, D01) ---

// TestOpenTagPickerSeedsFilteredWithFullList is the consolidated search
// field's initial state (D01, epic-E12-plan.md »Item 1«): `t` alone (no
// second gate) must seed tagInputFiltered with every existing tagItems row
// (an empty substring matches everything) and park the cursor at 0 --
// mirrors filteredRepos()'s own "empty query -> full list" contract
// (view_lobby.go).
func TestOpenTagPickerSeedsFilteredWithFullList(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))

	if len(m.tagInputFiltered) != len(m.tagItems) {
		t.Fatalf("tagInputFiltered len = %d, want %d (full tagItems on empty query)", len(m.tagInputFiltered), len(m.tagItems))
	}
	if m.tagInputSuggestCursor != 0 {
		t.Fatalf("tagInputSuggestCursor = %d, want 0 on open", m.tagInputSuggestCursor)
	}
	if !m.tagInput.Focused() {
		t.Fatal("t alone must land on an already-focused search field (D01: no second gate)")
	}
}

// TestTagPickerFiltersLiveBySubstringCaseInsensitive is the PO-Review
// 2026-07-17 (US-07 REJECTED) fix itself: typing DIRECTLY in the
// Haupt-Picker (no `n` gate) narrows tagInputFiltered to rows whose tag
// contains the typed text as a case-insensitive substring (strings.Contains,
// no fuzzy scoring -- YAGNI per the bean's own "Nicht jetzt" section).
func TestTagPickerFiltersLiveBySubstringCaseInsensitive(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // tagItems: urgent(2), backend(1), zeta(1)
	m = step(t, m, runeMsg('g'))

	for _, r := range "UR" { // mixed-case substring of "urgent"
		m = step(t, m, runeMsg(r))
	}

	if len(m.tagInputFiltered) != 1 || m.tagInputFiltered[0].tag != "urgent" {
		t.Fatalf("tagInputFiltered = %+v, want exactly [urgent]", m.tagInputFiltered)
	}
	if got := m.tagInput.Value(); got != "UR" {
		t.Fatalf("tagInput.Value() = %q, want %q -- the PO-Review's own complaint: typed text must be visible", got, "UR")
	}
}

// TestTagPickerNavigationMovesCursorClampedToFilteredBounds: up/down (real
// arrow KeyType, NOT the vim-style i/k/j/l rune aliases navKey binds
// elsewhere -- those must stay literal, typeable characters here, see this
// test's sibling below) move tagInputSuggestCursor over tagInputFiltered,
// clamped to [0, len-1], DIRECTLY in the Haupt-Picker.
func TestTagPickerNavigationMovesCursorClampedToFilteredBounds(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g')) // empty query -> all 3 rows

	if m.tagInputSuggestCursor != 0 {
		t.Fatalf("setup: cursor = %d, want 0", m.tagInputSuggestCursor)
	}
	m = step(t, m, keyMsg(tea.KeyUp)) // already at 0 -- must clamp, not go negative
	if m.tagInputSuggestCursor != 0 {
		t.Fatalf("up at top clamped to %d, want 0", m.tagInputSuggestCursor)
	}

	m = step(t, m, keyMsg(tea.KeyDown))
	m = step(t, m, keyMsg(tea.KeyDown))
	if m.tagInputSuggestCursor != 2 {
		t.Fatalf("cursor after two downs = %d, want 2", m.tagInputSuggestCursor)
	}
	m = step(t, m, keyMsg(tea.KeyDown)) // already at bottom -- must clamp
	if m.tagInputSuggestCursor != 2 {
		t.Fatalf("down at bottom clamped to %d, want 2", m.tagInputSuggestCursor)
	}
}

// TestTagPickerArrowKeysDoNotLeakIntoTypedText guards the plan's own
// explicit promise ("keine Kollision mit textinput.Model") the OTHER
// direction: the navigation intercept must key off the raw
// tea.KeyUp/tea.KeyDown KeyType, NOT navKey's letter-alias table (keys.Up
// binds "i", keys.Down binds "k") -- otherwise a tag name containing "i" or
// "k" (e.g. "risk") could never be typed. Typing the literal runes must land
// in the input value untouched.
func TestTagPickerArrowKeysDoNotLeakIntoTypedText(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))

	for _, r := range "risk" {
		m = step(t, m, runeMsg(r))
	}
	if got := m.tagInput.Value(); got != "risk" {
		t.Fatalf("tagInput.Value() = %q, want %q -- i/k must stay literal, not be swallowed as up/down", got, "risk")
	}
}

// TestTagPickerToggleTogglesCursoredFilteredSuggestion is D01's central
// UX change vs. the pre-reopen `n`-submode: selecting an existing,
// substring-narrowed tag is now done via space/x (Toggle) at the cursor
// position -- multi-select stays available WHILE the search field is
// focused/filtering (the explicit acceptance criterion this bean was
// reopened for), NOT a single enter-selects-and-closes shortcut.
func TestTagPickerToggleTogglesCursoredFilteredSuggestion(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // original tags: urgent
	m = step(t, m, runeMsg('g'))
	before := len(m.tagItems)

	for _, r := range "back" { // substring of "backend", exactly one match
		m = step(t, m, runeMsg(r))
	}
	if len(m.tagInputFiltered) != 1 || m.tagInputFiltered[0].tag != "backend" {
		t.Fatalf("setup: tagInputFiltered = %+v, want exactly [backend]", m.tagInputFiltered)
	}

	m = step(t, m, runeMsg(' '))

	if m.overlay != overlayTagPicker {
		t.Fatal("toggling a filtered suggestion must NOT close the picker")
	}
	if !m.tagPending["backend"] {
		t.Fatal("space on the 'backend' suggestion must set tagPending[backend]=true")
	}
	if len(m.tagItems) != before {
		t.Fatalf("tagItems len = %d, want %d unchanged -- 'backend' already existed, no duplicate row", len(m.tagItems), before)
	}
}

// TestTagPickerToggleTogglesCursoredSuggestionNotFirst pins that toggle
// flips the tag AT THE CURSOR, not blindly tagInputFiltered[0] (mirrors the
// bt-9ipw Fix-Runde's Review-Finding 1, re-targeted at the Toggle path now
// that enter no longer does per-suggestion selection, D01). The filter
// keeps >1 rows, the cursor is moved OFF index 0 via Down, and space must
// toggle the cursored tag -- and NOT the index-0 tag.
func TestTagPickerToggleTogglesCursoredSuggestionNotFirst(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")     // original tags: urgent
	m = step(t, m, runeMsg('g')) // empty query -> all 3 rows: urgent, backend, zeta

	if len(m.tagInputFiltered) < 2 {
		t.Fatalf("setup: tagInputFiltered = %+v, want >1 rows", m.tagInputFiltered)
	}
	first := m.tagInputFiltered[0].tag

	m = step(t, m, keyMsg(tea.KeyDown)) // cursor -> index 1, off the trivial 0
	cursored := m.tagInputFiltered[m.tagInputSuggestCursor].tag
	if cursored == first {
		t.Fatalf("setup: cursored tag %q must differ from index-0 tag %q", cursored, first)
	}

	m = step(t, m, runeMsg(' '))

	if !m.tagPending[cursored] {
		t.Fatalf("space must toggle the CURSORED suggestion %q, tagPending = %v", cursored, m.tagPending)
	}
	if first != "urgent" && m.tagPending[first] {
		// index-0 tag must NOT have been toggled instead (urgent is exempt
		// from this assertion only because it is tk-2's ORIGINAL tag and
		// thus legitimately pending from the seed).
		t.Fatalf("space toggled index-0 tag %q instead of the cursored %q", first, cursored)
	}
}

// TestTagPickerTypedXStaysLiteralNotToggle is Review-R1 B01 (bt-9ipw
// Fix-Runde, Supervisor-ERRATUM zu D01): the first consolidation round
// intercepted keys.Toggle (which binds BOTH " " and "x", keymap.go) ahead of
// the textinput -- so "x" could NEVER be typed into the search field, making
// tags like nginx/linux/unix/box silently unfilterable and uncreatable.
// D01-Nachtrag: toggle is deliberately narrowed to the SPACE character only
// (space is never part of a valid tag name, data.ValidTagName) -- "x" must
// fall through to the textinput as a literal, typeable character, same
// rationale as the i/k raw-KeyType intercept.
func TestTagPickerTypedXStaysLiteralNotToggle(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	pendingBefore := len(m.tagPending)

	for _, r := range "linux" {
		m = step(t, m, runeMsg(r))
	}

	if got := m.tagInput.Value(); got != "linux" {
		t.Fatalf("tagInput.Value() = %q, want %q -- 'x' must stay a literal, typeable character (B01)", got, "linux")
	}
	if len(m.tagPending) != pendingBefore {
		t.Fatalf("tagPending = %v, want unchanged -- typing 'x' must not toggle a row as a side effect", m.tagPending)
	}
}

// TestTagPickerBoxRendersSearchField is Review-R1 I01 (bt-9ipw Fix-Runde):
// the render layer was unguarded -- a reviewer mutation that deleted the
// m.tagInput.View() write in tagPickerBox survived the whole suite GREEN.
// This pins the always-visible search field at the RENDER level: the
// ANSI-stripped tagPickerBox output must contain the field (placeholder
// text while empty, the typed value once typing) -- exactly the PO-facing
// surface US-07 was reopened over.
func TestTagPickerBoxRendersSearchField(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))

	out := ansi.Strip(m.tagPickerBox())
	if !strings.Contains(out, "type to filter or create") {
		t.Fatalf("tagPickerBox() = %q, want the search field's placeholder rendered while the query is empty", out)
	}

	for _, r := range "rev" {
		m = step(t, m, runeMsg(r))
	}
	out = ansi.Strip(m.tagPickerBox())
	if !strings.Contains(out, "rev") {
		t.Fatalf("tagPickerBox() = %q, want the typed query %q visibly rendered in the search field", out, "rev")
	}
}

// TestTagPickerEnterWithFilteredMatchSavesAndClosesPicker guards D01's OTHER
// half of the enter-overload: as long as tagInputFiltered has at least one
// row -- INCLUDING the untouched empty-query "every tag" state -- enter
// keeps its PRE-Typeahead meaning (applyTagPickerDiff: save the pending diff
// and close), UNCHANGED by narrowing the list via typing. This is the
// explicit behavior change vs. the old `n`-submode (whose enter picked a
// SINGLE suggestion and closed) -- picking now happens via Toggle (see
// above), enter always means "commit and close" whenever there is a match.
func TestTagPickerEnterWithFilteredMatchSavesAndClosesPicker(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m.store = newTestStore(t)
	m = focusBean(m, "tk-2") // original tags: urgent
	m = step(t, m, runeMsg('g'))

	for _, r := range "back" { // narrows to exactly [backend], a real match
		m = step(t, m, runeMsg(r))
	}
	m = step(t, m, runeMsg(' ')) // toggle backend on while filtered

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after enter with a filtered match = %v, want overlayNone (save+close, D01 unchanged)", nm.overlay)
	}
	if cmd == nil {
		t.Fatal("enter with a pending change (even while filtered) must fire the save Cmd")
	}
}

// --- tagPickerBox: rendering the live-filtered row list (bean bt-9ipw, D01) ---

// TestTagPickerBoxRendersFilteredRowsWithCursorMarker is the Rendering Step
// (bean bt-9ipw, D01 consolidation): tagPickerBox must render the always-
// visible search field PLUS one line per tagInputFiltered row, the cursored
// row carrying the "▸" marker convention.
func TestTagPickerBoxRendersFilteredRowsWithCursorMarker(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	m = step(t, m, keyMsg(tea.KeyDown)) // cursor -> row index 1

	out := ansi.Strip(m.tagPickerBox())
	if !strings.Contains(out, "urgent") || !strings.Contains(out, "backend") || !strings.Contains(out, "zeta") {
		t.Fatalf("tagPickerBox() = %q, want all 3 rows rendered", out)
	}
	if !strings.Contains(out, "▸") {
		t.Fatalf("tagPickerBox() = %q, want a cursor marker", out)
	}
}

// --- vanished-target guard on enter (design decision d parity, T1) ---

func TestTagPickerEnterTargetVanishedClosesGracefully(t *testing.T) {
	beans := fixtureBeansTagged()
	m := fixtureModel(t, beans)
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	m = tagPickerCursorTo(t, m, "backend")
	m = step(t, m, runeMsg(' ')) // ensure a pending change exists

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

// --- inherited PFLICHT findings from the E3-T2 review (bt-8v69 body),
// closed out here as part of E3 Task 3 (bean bt-p1uz) per its own bean
// body's "Übernommene Findings" section ---

// TestTagPickerEscWhileTypingDiscardsAndClosesEntirePicker guards D01's
// consolidation of PFLICHT I1a (originally: esc from the separate free-text
// sub-mode kept the outer picker open) into the new single-mode contract:
// since there is no longer a separate sub-mode to "esc out of" while
// staying open, esc while actively typing a filter query now discards
// EVERYTHING (mirrors TestTagPickerEscDiscardsPending, exercised here with
// a non-empty search query in flight to prove typing state doesn't change
// esc's outcome).
func TestTagPickerEscWhileTypingDiscardsAndClosesEntirePicker(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // tags: urgent
	m = step(t, m, runeMsg('g'))
	m = tagPickerCursorTo(t, m, "backend")
	m = step(t, m, runeMsg(' ')) // toggle backend on in pending

	for _, r := range "urg" { // some in-flight filter text
		m = step(t, m, runeMsg(r))
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (esc always fully discards+closes, D01: no partial-escape sub-mode left)", nm.overlay)
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// TestTagPickerToggleOffThenOnYieldsEmptyDiffNoMutation is PFLICHT I1b: a
// tag toggled off and then back on nets out to the SAME state as
// tagOriginal -- applyTagPickerDiff's add/remove diff must come out empty,
// so enter closes the picker WITHOUT firing a Cmd, exactly like the
// no-changes-at-all case (TestTagPickerEnterNoChangesNoMutation).
func TestTagPickerToggleOffThenOnYieldsEmptyDiffNoMutation(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2") // tags: urgent
	m = step(t, m, runeMsg('g'))

	m = tagPickerCursorTo(t, m, "urgent")
	m = step(t, m, runeMsg(' ')) // off
	m = step(t, m, runeMsg(' ')) // back on -- net: unchanged vs. tagOriginal

	if !m.tagPending["urgent"] {
		t.Fatal("setup: urgent should be back in tagPending after toggling off then on")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter must close the picker even when the toggle nets out to no change")
	}
	if cmd != nil {
		t.Fatal("a toggle-off-then-on net-zero diff must not fire a mutation Cmd")
	}
}

// --- capture-order guard (cheap follow-up from the T1 review, bt-8v69 body) ---

// TestOverlayCaptureSwallowsQuitKeysWhileTagPickerOpen pins handleKey's
// capture order (design decision a2/the overlay precedent
// m.searchActive already established): q must never leak through to
// a quit-request while ANY node-action overlay is open. Exercised against
// the tag picker since that is what this task builds, but the guard itself
// (m.overlay != overlayNone routes to keyOverlay BEFORE the global
// ctrl+c/q switch, handleKey) is overlay-agnostic. Post-D01 (bean bt-9ipw):
// "q" is now ordinary filter text inside the always-focused search field --
// the guard this test cares about is narrower than "cmd==nil" (a benign
// textinput blink Cmd is expected and fine), it is specifically "never
// tea.Quit's Cmd".
func TestOverlayCaptureSwallowsQuitKeysWhileTagPickerOpen(t *testing.T) {
	m := fixtureModel(t, fixtureBeansTagged())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('g'))
	if m.overlay != overlayTagPicker {
		t.Fatal("setup: t did not open the tag picker")
	}

	assertQuitsAtOnce(t, m)

	tm, cmd := m.Update(runeMsg('q'))
	nm := tm.(model)
	// D01 (bean bt-9ipw): "q" now lands in the always-focused search field
	// like any other letter -- textinput.Update legitimately returns a
	// non-nil Cmd here (its own blink-restart tick), so a bare cmd!=nil
	// check would misfire. The actual capture-order guard is: whatever cmd
	// resolves to, it must NOT be tea.Quit's QuitMsg.
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("q while an overlay is open must not fire tea.Quit (capture-order guard)")
		}
	}
	if nm.overlay != overlayTagPicker {
		t.Fatal("q must not close the open overlay either")
	}
	if got := nm.tagInput.Value(); got != "q" {
		t.Fatalf("tagInput.Value() = %q, want %q -- q is captured as ordinary filter text, not a quit key, inside the picker", got, "q")
	}
}

// --- tagPickerBox: PF-12 reserved marker column (T6, bean bt-pqq3, D10) ---

// checkboxColStart returns the DISPLAY-COLUMN (lipgloss.Width, cell-based)
// at which line's "[" checkbox starts, or -1 if absent. Width-based, NOT a
// raw strings.Index byte offset (B-ERRATUM below): tagManagementMarkerGlyph
// ("✓", U+2713) and the modal's own "▸" cursor glyph are both 3-byte UTF-8
// sequences that occupy a SINGLE terminal cell, same as the ASCII blank
// they replace -- byte-length and cell-width diverge for any non-ASCII
// glyph, and PF-12's own literal wording (design-spec.md §15, quoted in the
// epic body bt-362n's D10) is a lipgloss.Width invariant, not a byte-offset
// one.
func checkboxColStart(line string) int {
	i := strings.Index(line, "[")
	if i < 0 {
		return -1
	}
	return lipgloss.Width(line[:i])
}

// TestTagPickerBoxReservesMarkerColumnRegardlessOfDefined is the bean
// bt-pqq3 body's own RED-Zitat, with two implementer-found test-bugs fixed
// (B-ERRATUM, documented in this task's Deviations section, mirrors bean
// bt-r92i's own B01 precedent -- found+fixed+regression-tested, no
// PO-visible behavior affected):
//
//  1. The original `strings.Contains(l, "a") || strings.Contains(l, "b ")`
//     filter also matched the modal's OWN title line ("Tags", contains "a")
//     and the hint line ("space/x:toggle ... enter:save", contains "a" —
//     and, at modalBox's width=40, wraps across TWO physical lines, neither
//     of which contains "["). Both non-tag lines sorted BEFORE the two real
//     tag rows in `lines`, so rowLines[0]/rowLines[1] were actually two
//     non-tag lines whose "[" search both return -1 -- a VACUOUS pass (-1
//     == -1) that verified nothing. Fixed: filter directly on `strings.Contains(l, "[")`,
//     the actual property under test (a checkbox row).
//  2. `strings.Index` is a BYTE offset; tagManagementMarkerGlyph ("✓") and
//     the pre-existing "▸" cursor glyph are 3-byte UTF-8 sequences that
//     occupy ONE terminal cell each, same as the ASCII space they replace --
//     so a defined+cursored row and a free+non-cursored row can have
//     IDENTICAL display columns but DIFFERENT byte offsets. Fixed: compare
//     via checkboxColStart (lipgloss.Width-based), matching PF-12's own
//     literal wording (design-spec.md §15, epic body bt-362n's D10 citation:
//     "lipgloss.Width ... identisch").
func TestTagPickerBoxReservesMarkerColumnRegardlessOfDefined(t *testing.T) {
	m := newModel(nil, "")
	m.tagItems = []tagCount{{tag: "a", defined: true}, {tag: "b", defined: false}}
	m.tagInputFiltered = m.tagItems
	out := m.tagPickerBox()
	lines := strings.Split(ansi.Strip(out), "\n")
	// Beide Tag-Zeilen müssen an identischer Spalte beginnen -- PF-12: kein
	// bedingtes Weglassen der Marker-Spalte.
	var rowLines []string
	for _, l := range lines {
		if strings.Contains(l, "[") {
			rowLines = append(rowLines, l)
		}
	}
	if len(rowLines) < 2 {
		t.Fatalf("want at least 2 tag rows rendered, got %v", rowLines)
	}
	// Spalte (lipgloss.Width) des '[' (Checkbox-Start) muss in beiden Zeilen
	// identisch sein, obwohl rowLines[0] (tag "a") den Cursor trägt und
	// rowLines[1] (tag "b") nicht -- PF-12 deckt BEIDE Achsen (Cursor- UND
	// Marker-Spalte) durch denselben "immer reserviert"-Vertrag ab.
	if c0, c1 := checkboxColStart(rowLines[0]), checkboxColStart(rowLines[1]); c0 != c1 {
		t.Fatalf("marker column shifted layout: col %d (%q) vs col %d (%q)", c0, rowLines[0], c1, rowLines[1])
	}
}

// TestTagPickerBoxMarkerColumnWidthStableAcrossNonCursorRows is PF-12's OWN
// literal test-obligation (design-spec.md §15, quoted verbatim in the epic
// body bt-362n's D10: "lipgloss.Width einer NICHT-aktiven Zeile identisch,
// unabhängig von einer anderen aktiven Zeile") applied to the marker column
// -- TWO non-cursor rows (one defined, one free) must render to the IDENTICAL
// lipgloss.Width, cursor-styling-independent, isolating the marker column's
// own contribution from the pre-existing "▸ "-cursor-prefix's own (unrelated,
// pre-T6) styling.
func TestTagPickerBoxMarkerColumnWidthStableAcrossNonCursorRows(t *testing.T) {
	m := newModel(nil, "")
	m.tagItems = []tagCount{
		{tag: "alpha-defined", defined: true},
		{tag: "middle-cursored", defined: false},
		{tag: "zzz-free", defined: false},
	}
	m.tagInputFiltered = m.tagItems
	m.tagInputSuggestCursor = 1 // neither row 0 nor row 2 carries the cursor

	lines := strings.Split(ansi.Strip(m.tagPickerBox()), "\n")
	var rowLines []string
	for _, l := range lines {
		if strings.Contains(l, "[") {
			rowLines = append(rowLines, l)
		}
	}
	if len(rowLines) != 3 {
		t.Fatalf("want 3 rendered tag rows, got %d (%v)", len(rowLines), rowLines)
	}
	definedRow, freeRow := rowLines[0], rowLines[2]
	if w1, w2 := lipgloss.Width(definedRow), lipgloss.Width(freeRow); w1 != w2 {
		t.Fatalf("marker column shifted layout between non-cursor rows: defined width=%d (%q), free width=%d (%q)", w1, definedRow, w2, freeRow)
	}
	if c0, c1 := checkboxColStart(definedRow), checkboxColStart(freeRow); c0 != c1 {
		t.Fatalf("marker column shifted the checkbox column between non-cursor rows: col %d (%q) vs col %d (%q)", c0, definedRow, c1, freeRow)
	}
}
