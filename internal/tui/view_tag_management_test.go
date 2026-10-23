package tui

// view_tag_management_test.go — TDD coverage for T2 (bean bt-r92i, epic
// bt-362n E10, D05-D09): the Tag-Management page's read-only Grundgerüst --
// tagRegistryRows' Union/sort algebra (D09), the page's own full-capture key
// handler (D06/D08), and the Command-Center entry point (D05). Reuses
// fixtureBeans/fixtureModel/keyMsg/runeMsg (update_test.go), same package.

import (
	"errors"
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
)

// --- tagRegistryRows: D09 Union + sort algebra ---

// TestTagRegistryRowsDefinedFirstAlphaThenFreeByCountDesc is the bean's own
// named RED test (bt-r92i TDD section, quoted verbatim).
func TestTagRegistryRowsDefinedFirstAlphaThenFreeByCountDesc(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "b1", Tags: []string{"zzz-free"}},
		{ID: "b2", Tags: []string{"zzz-free"}},
		{ID: "b3", Tags: []string{"aaa-free"}},
	})
	rows := tagRegistryRows(idx, []string{"defined-b", "defined-a"})
	want := []string{"defined-a", "defined-b", "zzz-free", "aaa-free"}
	var got []string
	for _, r := range rows {
		got = append(got, r.name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

// TestTagRegistryRowsIncludesUnusedDefinedTagWithZeroCount is the bean's own
// named RED test (bt-r92i TDD section, quoted verbatim).
func TestTagRegistryRowsIncludesUnusedDefinedTagWithZeroCount(t *testing.T) {
	idx := data.NewIndex(nil)
	rows := tagRegistryRows(idx, []string{"unused"})
	if len(rows) != 1 || rows[0].count != 0 || !rows[0].defined {
		t.Fatalf("want one zero-count defined row, got %+v", rows)
	}
}

// TestTagRegistryRowsDefinedTagKeepsRealUsageCount guards D09's Union
// semantics against a naive "defined XOR free" split: a tag that is BOTH
// registry-defined AND currently in use on beans must appear exactly ONCE
// (in the Defined group), carrying its REAL usage count -- not 0, and not a
// second, duplicate row in the Free group.
func TestTagRegistryRowsDefinedTagKeepsRealUsageCount(t *testing.T) {
	idx := data.NewIndex([]*item.Item{
		{ID: "b1", Tags: []string{"shared-tag"}},
		{ID: "b2", Tags: []string{"shared-tag"}},
	})
	rows := tagRegistryRows(idx, []string{"shared-tag"})
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 row (no duplicate), got %d: %+v", len(rows), rows)
	}
	if !rows[0].defined || rows[0].count != 2 {
		t.Fatalf("want defined=true count=2, got %+v", rows[0])
	}
}

// TestTagRegistryRowsDedupesDuplicateDefNames guards against a registry
// slice containing an accidental duplicate (should never happen post-T1's
// own sortDedupTagNames, but tagRegistryRows must not crash or double-list
// if it ever does -- defensive, mirrors LoadTagDefs' own "never trust the
// file blindly" philosophy).
func TestTagRegistryRowsDedupesDuplicateDefNames(t *testing.T) {
	rows := tagRegistryRows(data.NewIndex(nil), []string{"dup", "dup"})
	if len(rows) != 1 {
		t.Fatalf("want exactly 1 deduped row, got %d: %+v", len(rows), rows)
	}
}

// TestTagRegistryRowsEmptyEverywhereReturnsEmpty guards the pre-load /
// empty-repo floor: no defs, no beans -> an empty (nil-or-zero-length) row
// list, never a panic.
func TestTagRegistryRowsEmptyEverywhereReturnsEmpty(t *testing.T) {
	rows := tagRegistryRows(data.NewIndex(nil), nil)
	if len(rows) != 0 {
		t.Fatalf("want 0 rows, got %d: %+v", len(rows), rows)
	}
}

// TestTagRegistryRowsNilIndexNoPanic guards the pre-load state (m.idx == nil
// before the first beansLoadedMsg) -- must never panic, defs alone still
// render as zero-count defined rows.
func TestTagRegistryRowsNilIndexNoPanic(t *testing.T) {
	rows := tagRegistryRows(nil, []string{"solo"})
	if len(rows) != 1 || !rows[0].defined || rows[0].count != 0 {
		t.Fatalf("want one zero-count defined row, got %+v", rows)
	}
}

// --- keyTagManagement: nav / esc / enter no-op (D06/D08) ---

// TestKeyTagManagementEscReturnsToBrowse is the bean's own named RED test
// (bt-r92i TDD section, quoted verbatim).
func TestKeyTagManagementEscReturnsToBrowse(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if nm.(model).view != viewBrowseRepo {
		t.Fatalf("want viewBrowseRepo, got %v", nm.(model).view)
	}
}

// TestHandleKeyOnTagManagementViewDoesNotLeakToNodeAction is the D06
// regression guard (bean bt-r92i TDD section; REWRITTEN in Fix-Runde 1,
// T2-Review F01, medium): the bean's original verbatim sketch used
// `newModel(nil, "")` -- a model with NO focused bean -- but keyNodeAction's
// branch (update.go) is a silent no-op when focusedBean()==nil anyway, so
// that version stayed GREEN even with the D06 full-capture guard
// (update.go, `if m.view == viewTagManagement`) removed entirely
// (reviewer-verified by mutation). The whole POINT of D06 is that
// focusedBean()'s `default` branch falls through to the TREE cursor while
// viewTagManagement is active -- so the test must set up exactly that leak
// surface: a real index + a real tree cursor on a real bean (focusBean,
// box_menu_value_test.go). With the guard removed, `d` now reaches
// keyNodeAction, resolves tk-2 as the (stale, unrelated) target and opens
// the Delete-Confirm (overlay=overlayDeleteConfirm) -- verified RED under
// exactly that mutation during Fix-Runde 1, quoted in the bean.
func TestHandleKeyOnTagManagementViewDoesNotLeakToNodeAction(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m.view = viewTagManagement

	// Sanity: the leak surface this test guards must actually exist --
	// focusedBean() resolves the tree cursor via its default branch even
	// while viewTagManagement is active. If this ever returns nil, the test
	// has silently degraded back to the F01 no-op version.
	if m.focusedBean() == nil {
		t.Fatal("test setup invalid: focusedBean() == nil -- the D06 leak this test guards would be impossible to trigger")
	}

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm := nm.(model)
	if mm.overlay != overlayNone {
		t.Fatalf("want no overlay opened, got %v", mm.overlay)
	}
	if mm.view != viewTagManagement {
		t.Fatalf("want view to stay viewTagManagement, got %v", mm.view)
	}
}

// TestHandleKeyOnTagManagementViewDoesNotLeakHelpOrPalette extends the D06
// regression guard to the Command-Center/Help bare-match checks (`?`/
// ctrl+k), which sit AFTER the viewTagManagement full-capture checkpoint in
// handleKey -- mirrors viewLobby's own precedent (view_lobby.go doc-stamp:
// "ctrl+k/`?` are DELIBERATELY not special-cased to still reach the
// Lobby").
func TestHandleKeyOnTagManagementViewDoesNotLeakHelpOrPalette(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement

	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	mm := nm.(model)
	if mm.helpOpen {
		t.Fatalf("want helpOpen=false (full-capture), got true")
	}
	if mm.view != viewTagManagement {
		t.Fatalf("want view to stay viewTagManagement, got %v", mm.view)
	}

	nm2, _ := m.Update(runeMsg('K'))
	mm2 := nm2.(model)
	if mm2.paletteOpen {
		t.Fatalf("want paletteOpen=false (full-capture), got true")
	}
}

func TestKeyTagManagementUpDownMovesCursor(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "a"}, {name: "b"}, {name: "c"}}
	m.tagMgmtCursor.setLen(len(m.tagMgmtRows))

	nm, _ := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyDown})
	mm := nm.(model)
	if mm.tagMgmtCursor.cursor != 1 {
		t.Fatalf("cursor after down = %d, want 1", mm.tagMgmtCursor.cursor)
	}
	nm2, _ := mm.keyTagManagement(tea.KeyMsg{Type: tea.KeyUp})
	if nm2.(model).tagMgmtCursor.cursor != 0 {
		t.Fatalf("cursor after up = %d, want 0", nm2.(model).tagMgmtCursor.cursor)
	}
}

// TestKeyTagManagementEnterIsHandledNoOp guards D08: enter must be a
// HANDLED no-op (view/cursor/overlay all unchanged), not an unhandled
// fallthrough.
func TestKeyTagManagementEnterIsHandledNoOp(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "a"}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyEnter})
	mm := nm.(model)
	if mm.view != viewTagManagement || mm.overlay != overlayNone || cmd != nil {
		t.Fatalf("enter must be a handled no-op, got view=%v overlay=%v cmd=%v", mm.view, mm.overlay, cmd)
	}
}

// --- openTagManagementPage: D03 fresh-load + D02 tolerant-missing ---

// TestOpenTagManagementPageNilClientBuildsFromIdxOnly guards D02's
// tolerant-missing contract at the model layer: no live client (pre-load /
// test fixture, e.g. fixtureModel's own newModel(nil, ...) convention) must
// never panic -- an empty registry applies, rows still build from whatever
// tags already sit on m.idx's beans.
func TestOpenTagManagementPageNilClientBuildsFromIdxOnly(t *testing.T) {
	m := fixtureModel(t, []*item.Item{
		{ID: "tk-1", Title: "T", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"loose"}},
	})
	nm, cmd := m.openTagManagementPage()
	mm := nm.(model)
	if mm.view != viewTagManagement {
		t.Fatalf("view = %v, want viewTagManagement", mm.view)
	}
	if cmd != nil {
		t.Fatalf("want nil Cmd (synchronous open, D02/D03), got %v", cmd)
	}
	if len(mm.tagMgmtRows) != 1 || mm.tagMgmtRows[0].name != "loose" || mm.tagMgmtRows[0].defined {
		t.Fatalf("want one free 'loose' row, got %+v", mm.tagMgmtRows)
	}
	if mm.tagMgmtCursor.length != 1 {
		t.Fatalf("tagMgmtCursor.length = %d, want 1", mm.tagMgmtCursor.length)
	}
}

// TestOpenTagManagementPageResetsStaleCursor guards that a stale cursor from
// a PREVIOUS page visit (more rows then, cursor deep in the list) never
// survives into a re-open with fewer rows.
func TestOpenTagManagementPageResetsStaleCursor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.tagMgmtCursor.setLen(50)
	m.tagMgmtCursor.cursor = 40

	nm, _ := m.openTagManagementPage()
	mm := nm.(model)
	if mm.tagMgmtCursor.cursor >= mm.tagMgmtCursor.length && mm.tagMgmtCursor.length > 0 {
		t.Fatalf("cursor %d out of bounds for length %d", mm.tagMgmtCursor.cursor, mm.tagMgmtCursor.length)
	}
}

// --- tagManagementChrome: D07 empty GlobalHint ---

// TestTagManagementChromeGlobalHintEmpty guards D07: the breadcrumb's right
// side must be empty (no renderBindings(globalBindings()) promise for keys
// that don't work during full-capture) -- checked by asserting the header
// line does not contain any of the 4 global binding key glyphs' rendered
// text.
func TestTagManagementChromeGlobalHintEmpty(t *testing.T) {
	m := newModel(nil, "/tmp/bt-fixture-repo")
	head, _ := m.tagManagementChrome(80)
	for _, glyph := range []string{"commands", "repos", "help", "quit"} {
		if strings.Contains(head, glyph) {
			t.Fatalf("tagManagementChrome head contains global hint text %q (D07 requires an empty GlobalHint): %q", glyph, head)
		}
	}
}

// TestTagManagementChromeFooterListsUpDownBack guards that the page's own
// Footer Zone 3 (T2 vorerst: Up/Down/Back) is actually rendered.
func TestTagManagementChromeFooterListsUpDownBack(t *testing.T) {
	_, localKeys := (model{}).tagManagementChrome(80)
	plain := stripHint(localKeys)
	if !strings.Contains(plain, "back") {
		t.Fatalf("tagManagementChrome footer missing 'back' hint: %q", plain)
	}
}

// --- viewTagManagement: render-behavior (union rows, D09 order visible) ---

// TestViewTagManagementRendersDefinedAndFreeRows is a render-grounded test:
// after openTagManagementPage, viewTagManagement's output must contain both
// a registry-defined tag name AND a free/in-use tag name.
func TestViewTagManagementRendersDefinedAndFreeRows(t *testing.T) {
	m := fixtureModel(t, []*item.Item{
		{ID: "tk-1", Title: "T", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"free-tag"}},
	})
	m.width, m.height = 100, 30
	nm, _ := m.openTagManagementPage()
	mm := nm.(model)
	mm.tagMgmtRows = append(mm.tagMgmtRows, tagRegistryRow{name: "def-tag", count: 0, defined: true})

	out := mm.viewTagManagement()
	if !strings.Contains(out, "free-tag") {
		t.Errorf("viewTagManagement() output missing free-tag:\n%s", out)
	}
	if !strings.Contains(out, "def-tag") {
		t.Errorf("viewTagManagement() output missing def-tag:\n%s", out)
	}
}

// TestViewTagManagementNoNewRegistryFileOnRender guards the bean's own
// tmux-smoke acceptance wording ("T2 ist rein lesend") at the render layer:
// rendering the page must never touch the filesystem (viewTagManagement
// itself does no I/O at all -- LoadTagDefs only runs in
// openTagManagementPage). A weak but cheap guard: viewTagManagement must not
// panic/error against a client pointed at a real temp repo dir with no
// registry file, and must not create one as a side effect of rendering.
func TestViewTagManagementNoNewRegistryFileOnRender(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 100, 30
	_ = m.viewTagManagement() // must not panic
}

// TestViewTagManagementLinesFitExactlyNoWrap is a regression guard (found
// live during this task's OWN tmux-smoke run, same "B01" bug class as
// view_lobby.go's own doc-stamp / view_browse_repo.go's single-pane precedent
// paneW note): a single full-width pane's renderPane `w` argument is a
// CONTENT width (its own RoundedBorder adds +2 more) -- passing innerW
// unadjusted (instead of innerW-2) double-counts the border, and
// outerBorder's Render silently WORD-WRAPS any resulting over-width line
// instead of erroring, turning the width bug into extra, unbudgeted lines.
// Pins the fixed contract at BOTH tmux-smoke widths (120/80): every line is
// EXACTLY m.width cells wide, and the total line count is EXACTLY m.height
// -- a wrapped border line would inflate line count beyond height and break
// the per-line width check simultaneously.
func TestViewTagManagementLinesFitExactlyNoWrap(t *testing.T) {
	for _, width := range []int{120, 80} {
		m := fixtureModel(t, []*item.Item{
			{ID: "tk-1", Title: "T", Status: "todo", Type: "task", Priority: "normal", Tags: []string{"a-fairly-long-free-tag-name"}},
		})
		m.width, m.height = width, 40
		nm, _ := m.openTagManagementPage()
		mm := nm.(model)

		out := mm.viewTagManagement()
		lines := strings.Split(out, "\n")
		if len(lines) != m.height {
			t.Fatalf("width=%d: viewTagManagement() produced %d lines, want exactly %d (height)", width, len(lines), m.height)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w != m.width {
				t.Fatalf("width=%d: line %d width = %d, want exactly %d: %q", width, i, w, m.width, l)
			}
		}
	}
}

// --- View() dispatcher (D05 entry wiring) ---

func TestViewDispatcherRoutesTagManagementView(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 100, 30
	nm, _ := m.openTagManagementPage()
	mm := nm.(model)

	out := mm.View()
	// viewBrowseRepo's own breadcrumb title is "Browse" -- viewTagManagement's
	// is "Tags" (tagManagementChrome). A dispatcher bug (falling through to
	// the `default: viewBrowseRepo()` case) would render "Browse" instead.
	if strings.Contains(out, "Browse") {
		t.Errorf("View() while viewTagManagement is active still rendered the Browse breadcrumb (dispatcher not wired):\n%s", out)
	}
}

// --- Command-Center entry point (D05) ---

// TestPaletteActionsIncludesGoToTagsAndSettings guards D05's wiring:
// "view: tags" is a global action (no focused-bean requirement, mirrors
// "view: settings"). Order is pinned by TestPaletteActionsGroupedByVerb.
func TestPaletteActionsIncludesGoToTagsAndSettings(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	items := paletteActions(m)

	var gotIdx, settingsIdx = -1, -1
	for i, it := range items {
		switch it.actionID {
		case "go_tags":
			gotIdx = i
			if it.label != "view: tags" {
				t.Fatalf(`go_tags label = %q, want "view: tags"`, it.label)
			}
		case "settings":
			settingsIdx = i
		}
	}
	if gotIdx == -1 {
		t.Fatal(`paletteActions missing "go_tags"`)
	}
	if settingsIdx == -1 {
		t.Fatal(`paletteActions missing "settings"`)
	}
}

// TestDispatchPaletteGoTagsOpensTagManagementPage guards the Command-Center
// entry point (D05: NO dedicated keybinding, ONLY reachable via
// overlay_palette.go's "go to tags" action) -- mirrors
// TestDispatchPaletteSettingsOpensForm's own precedent
// (box_form_settings_test.go).
func TestDispatchPaletteGoTagsOpensTagManagementPage(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "go_tags", label: "view: tags"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	if mm.view != viewTagManagement {
		t.Fatalf("dispatchPalette(go_tags) view = %v, want viewTagManagement", mm.view)
	}
	if mm.paletteOpen {
		t.Fatal("dispatchPalette(go_tags) left paletteOpen=true, want closed")
	}
}

// --- T3 (bean bt-604w, epic bt-362n D11/D14): Create + shared free-text
// input sub-mode ---

// TestKeyTagMgmtInputRejectsInvalidName is the bean's own named RED test
// (bt-604w TDD section) -- quoted almost verbatim. ERRATUM vs. the bean's
// literal sketch: `m, _ = m.openTagMgmtInput(...)` does not compile
// (openTagMgmtInput returns tea.Model, an interface, into `m` which is the
// concrete `model` type -- the SAME assignability rule every other
// tea.Model-returning method in this codebase already requires an explicit
// type assertion for, e.g. TestOpenTagManagementPageNilClientBuildsFromIdxOnly
// above: `nm, cmd := m.openTagManagementPage(); mm := nm.(model)`). Fixed by
// routing through a fresh `nm`/type-assertion, same shape as every other
// test in this file -- assertions themselves are UNCHANGED from the bean's
// own wording (mirrors bt-pqq3's own precedent of fixing a test-sketch bug
// while leaving the asserted contract untouched, documented in ERRATA).
func TestKeyTagMgmtInputRejectsInvalidName(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	nm, _ := m.openTagMgmtInput("create", "")
	m = nm.(model)
	m.tagMgmtInput.SetValue("Not Valid!")
	nm2, _ := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	got := nm2.(model)
	if !got.tagMgmtInputActive || got.tagMgmtInputErr == "" {
		t.Fatalf("want input to stay open with an error, got active=%v err=%q",
			got.tagMgmtInputActive, got.tagMgmtInputErr)
	}
}

// TestKeyTagMgmtInputRejectsDuplicateAgainstExistingRows is the bean's own
// named RED test (bt-604w TDD section) -- same ERRATUM fix as
// TestKeyTagMgmtInputRejectsInvalidName above (fresh var + type assertion).
func TestKeyTagMgmtInputRejectsDuplicateAgainstExistingRows(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "already-there", defined: true}}
	nm, _ := m.openTagMgmtInput("create", "")
	m = nm.(model)
	m.tagMgmtInput.SetValue("already-there")
	nm2, _ := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	// Exact wording pinned by T5-F01 (bean bt-sohl): neutral "name already in
	// use" -- the dedupe set spans defined AND free rows, so the old "tag
	// already defined" text was factually wrong for free-row collisions.
	if got := nm2.(model); !got.tagMgmtInputActive || got.tagMgmtInputErr != "name already in use: already-there" {
		t.Fatalf("want rejected duplicate with neutral error text, got active=%v err=%q",
			got.tagMgmtInputActive, got.tagMgmtInputErr)
	}
}

// TestKeyTagMgmtInputRejectsDuplicateAgainstFreeRowToo pins the bean body's
// own explicit wording ("Dedupe-Check gegen die aktuelle m.tagMgmtRows-
// Namensmenge ... Dedupe prüft also gegen ALLE vorhandenen Namen" -- ALL
// existing names, not just registry-defined ones): typing the exact name of
// an already-visible FREE (undefined) row is ALSO rejected as a duplicate,
// not silently treated as a "promote this free tag" shortcut (D09's own
// promotion narrative is a documented, not-yet-built fast-follow, epic
// bt-362n Q-section -- retyping an existing free tag's name here is not that
// mechanism).
func TestKeyTagMgmtInputRejectsDuplicateAgainstFreeRowToo(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-tag", count: 3, defined: false}}
	nm, _ := m.openTagMgmtInput("create", "")
	m = nm.(model)
	m.tagMgmtInput.SetValue("free-tag")
	nm2, _ := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	// T5-F01 (bean bt-sohl): THIS is the case the old "tag already defined:"
	// text got factually wrong -- "free-tag" is NOT defined, only in use.
	// Exact neutral wording pinned.
	if got := nm2.(model); !got.tagMgmtInputActive || got.tagMgmtInputErr != "name already in use: free-tag" {
		t.Fatalf("want rejected duplicate against a FREE row with neutral error text, got active=%v err=%q",
			got.tagMgmtInputActive, got.tagMgmtInputErr)
	}
}

// TestKeyTagManagementNewTagOpensInput guards D14's entry point: `n`
// (keys.NewTag, reused verbatim from the Tag-Picker, same binding/meaning)
// opens the shared free-text input sub-mode in "create" mode, with an EMPTY
// target (D11: Create never prefills/targets an old name -- only T5/Rename
// will).
func TestKeyTagManagementNewTagOpensInput(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	mm := nm.(model)
	if !mm.tagMgmtInputActive {
		t.Fatal("want tagMgmtInputActive=true after 'n'")
	}
	if mm.tagMgmtInputMode != "create" {
		t.Fatalf("tagMgmtInputMode = %q, want %q", mm.tagMgmtInputMode, "create")
	}
	if mm.tagMgmtInputTarget != "" {
		t.Fatalf("tagMgmtInputTarget = %q, want empty (D11: Create never targets an old name)", mm.tagMgmtInputTarget)
	}
	if !mm.tagMgmtInput.Focused() {
		t.Fatal("want tagMgmtInput focused after opening")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (textinput.Blink), mirrors openTagInput")
	}
}

// TestKeyTagManagementNewTagOnFreeRowAdoptsDirectlyNoInput guards bt-idm1's
// Adopt path (E11 Item 6, blocked_by bt-ct3k): cursor on a FREE row + 'n'
// registers that row's OWN name directly via saveTagDefsCmd -- NO input
// sub-mode opens (Planner-Entscheidung final: "ein Tastendruck ohne
// Zwischenstopp", Registrieren ist nicht-destruktiv, kein Confirm-Bedarf).
// The dispatched Cmd carries the row's name both as the new Definition
// (data.AddTagDefName) and as refindName, plus a non-empty successToast
// (bt-ct3k Toast-Konsistenz: kein neuer stiller Erfolgspfad).
func TestKeyTagManagementNewTagOnFreeRowAdoptsDirectlyNoInput(t *testing.T) {
	dir := t.TempDir()
	m := newModel(&data.Store{RepoDir: dir}, dir)
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "old-defined", defined: true}, {name: "free-tag", defined: false, count: 2}}
	m.tagMgmtCursor.setLen(2)
	m.tagMgmtCursor.cursor = 1 // "free-tag"

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	mm := nm.(model)
	if mm.tagMgmtInputActive {
		t.Fatal("Adopt must NOT open the input sub-mode -- the name is already fixed")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (saveTagDefsCmd)")
	}
	msg := cmd()
	tdm, ok := msg.(tagDefsSavedMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want tagDefsSavedMsg", msg)
	}
	if tdm.err != nil {
		t.Fatalf("SaveTagDefs against a real t.TempDir() client failed: %v", tdm.err)
	}
	if tdm.refindName != "free-tag" {
		t.Fatalf("refindName = %q, want %q", tdm.refindName, "free-tag")
	}
	if tdm.successToast == "" {
		t.Fatal("want a non-empty successToast (Adopt's own success feedback, bt-ct3k Toast-Konsistenz)")
	}

	got, err := (&data.Store{RepoDir: dir}).LoadTagDefs()
	if err != nil {
		t.Fatalf("LoadTagDefs after save: %v", err)
	}
	want := []string{"free-tag", "old-defined"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on-disk registry = %v, want %v (free-tag adopted alongside the existing Definition)", got, want)
	}
}

// TestKeyTagManagementNewTagOnDefinedRowStillOpensBlankCreate guards the
// Randfall the bean body's own Akzeptanzkriterium names explicitly: cursor on
// an ALREADY-defined row + 'n' must fall back to the unchanged Blank-Create
// (Registrieren ergibt hier keinen Sinn, Name existiert schon) -- NOT Adopt.
func TestKeyTagManagementNewTagOnDefinedRowStillOpensBlankCreate(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "already-defined", defined: true}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	mm := nm.(model)
	if !mm.tagMgmtInputActive || mm.tagMgmtInputMode != "create" {
		t.Fatalf("cursor on a DEFINED row must fall back to the unchanged Blank-Create, got active=%v mode=%q", mm.tagMgmtInputActive, mm.tagMgmtInputMode)
	}
	if mm.tagMgmtInputTarget != "" {
		t.Fatalf("tagMgmtInputTarget = %q, want empty (Blank-Create never targets an old name)", mm.tagMgmtInputTarget)
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (textinput.Blink)")
	}
}

// TestKeyTagManagementNewTagOnFreeRowInvalidNameWarnsNoSave guards Adopt's
// defensive ValidTagName gate (Review-Finding, Fix-Runde zu bt-idm1): a free
// row's name comes from a bean's ACTUAL tags on disk -- a hand-edited bean
// file can carry a name the tag grammar rejects (uppercase, underscore, ...).
// Adopting it must mirror the Create path's own pre-dispatch validation
// (keyTagMgmtInput's data.ValidTagName check): warn Toast, NO Registry write,
// and NO fallback to the Blank-Create input either (the PO pressed n ON this
// row -- silently opening an empty input would misread the intent).
func TestKeyTagManagementNewTagOnFreeRowInvalidNameWarnsNoSave(t *testing.T) {
	dir := t.TempDir()
	m := newModel(&data.Store{RepoDir: dir}, dir)
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "Bad_Tag", defined: false, count: 1}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	mm := nm.(model)
	if mm.tagMgmtInputActive {
		t.Fatal("an invalid free name must NOT fall back to the Blank-Create input")
	}
	if mm.toast == nil || mm.toast.kind != toastWarn {
		t.Fatalf("want a toastWarn Toast for an invalid free name, got %+v", mm.toast)
	}
	if cmd != nil {
		if _, isSave := cmd().(tagDefsSavedMsg); isSave {
			t.Fatal("an invalid free name must never dispatch saveTagDefsCmd (no Registry write)")
		}
	}
	if got, _ := (&data.Store{RepoDir: dir}).LoadTagDefs(); len(got) != 0 {
		t.Fatalf("on-disk registry must stay empty, got %v", got)
	}
}

// TestKeyTagMgmtInputEscDiscardsOnlySubmode guards D14/D06: esc while the
// input is active closes ONLY the input sub-mode -- the Page itself
// (m.view) and its row list stay completely untouched, mirrors keyTagInput's
// own esc contract (box_picker_tag.go doc-stamp: "only the input sub-mode
// itself is discarded").
func TestKeyTagMgmtInputEscDiscardsOnlySubmode(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "existing", defined: true}}
	m = step(t, m, runeMsg('n'))
	if !m.tagMgmtInputActive {
		t.Fatal("setup: want input active after 'n'")
	}
	m = step(t, m, runeMsg('x'))
	if m.tagMgmtInput.Value() != "x" {
		t.Fatalf("setup: want 'x' typed into the input, got %q", m.tagMgmtInput.Value())
	}

	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.tagMgmtInputActive {
		t.Fatal("esc must close the input sub-mode")
	}
	if m.view != viewTagManagement {
		t.Fatalf("esc in the input sub-mode must stay on the Page, view = %v", m.view)
	}
	if len(m.tagMgmtRows) != 1 || m.tagMgmtRows[0].name != "existing" {
		t.Fatalf("esc must not touch tagMgmtRows, got %+v", m.tagMgmtRows)
	}
}

// TestKeyTagMgmtInputCapturesEveryKeyNoLeak is the Full-Capture-Disziplin
// regression this task's own harness brief explicitly demands: while the
// input sub-mode is active, EVERY other key (including the global node-
// action set AND `?`/ctrl+k) belongs to the textinput, not to any outer
// handler -- mirrors TestHandleKeyOnTagManagementViewDoesNotLeakToNodeAction's
// D06 guard one layer up, here for D14's own nested capture state.
func TestKeyTagMgmtInputCapturesEveryKeyNoLeak(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m.view = viewTagManagement
	// I02 (T4-Review Runde 1): a DEFINED row must sit under the cursor,
	// otherwise the tagMgmtDeleteConfirm assertion below is dead --
	// openTagMgmtDeleteConfirm no-ops on an empty row list, so a leaked 'd'
	// could never flip the bool and the assertion would stay green even with
	// the input-capture guard removed (the exact dead-assertion trap the T2
	// review's own F01 already documented for the focusedBean()==nil case).
	m.tagMgmtRows = []tagRegistryRow{{name: "leak-probe", defined: true, count: 1}}
	m.tagMgmtCursor.setLen(1)
	m = step(t, m, runeMsg('n'))
	if !m.tagMgmtInputActive {
		t.Fatal("setup: want input active after 'n'")
	}

	// bt-mx4k: `K` replaces the retired ctrl+k as the palette probe -- it is
	// also an ordinary character, so it must land in the input like any
	// other captured key.
	for _, r := range "d?K" {
		m = step(t, m, runeMsg(r))
	}

	// I02 (T4-Review Runde 1): the Delete-Confirm is D15's page-local BOOL
	// (tagMgmtDeleteConfirm), never an overlayID case -- the former
	// m.overlay assertion here was dead (mutation-probe-verified: it stayed
	// green even with the input-capture guard removed, since keyTagManagement's
	// Delete path never touches m.overlay). The live check is the bool
	// itself; m.overlay is still asserted as a general no-overlay-opens
	// guard (e.g. against a future regression routing 'd' to the GLOBAL
	// keyNodeAction Delete-Confirm, which DOES use overlayDeleteConfirm).
	if m.tagMgmtDeleteConfirm {
		t.Fatal("'d' while typing must not open the page-local Delete-Confirm (D15 bool)")
	}
	if m.overlay != overlayNone {
		t.Fatalf("no overlay may open while typing, overlay = %v", m.overlay)
	}
	if m.helpOpen {
		t.Fatal("'?' while typing must not open Help")
	}
	if m.paletteOpen {
		t.Fatal("`K` while typing must not open the Command-Center")
	}
	if !m.tagMgmtInputActive || m.view != viewTagManagement {
		t.Fatalf("want to stay in the input sub-mode on the Page, active=%v view=%v", m.tagMgmtInputActive, m.view)
	}
	if got, want := m.tagMgmtInput.Value(), "d?K"; got != want {
		t.Fatalf("want every captured key typed into the input, got %q, want %q", got, want)
	}
}

// TestKeyTagMgmtInputRetypingNDoesNotReopen guards the 'n'-as-a-literal-
// -character edge case: keys.NewTag's own binding key ('n') must be
// swallowed by keyTagMgmtInput's OWN default branch (textinput.Update) once
// the sub-mode is already active, not re-dispatched to
// keyTagManagement's outer `case keybind.Matches(msg, keys.NewTag)` (which
// only exists OUTSIDE the `if m.tagMgmtInputActive` guard).
func TestKeyTagMgmtInputRetypingNDoesNotReopen(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m = step(t, m, runeMsg('n'))
	m = step(t, m, runeMsg('n')) // literal 'n' as part of a tag name, e.g. "new-ish"

	if m.tagMgmtInput.Value() != "n" {
		t.Fatalf("want the second 'n' typed into the input, got value=%q", m.tagMgmtInput.Value())
	}
}

// TestKeyTagMgmtInputValidSubmitFiresSaveTagDefsCmdWithAddedName guards D11's
// Create dispatch: a valid, non-duplicate name fires saveTagDefsCmd with the
// registry's CURRENT defined names (extracted from tagMgmtRows) plus the new
// one (data.AddTagDefName) -- proven by intercepting the returned Cmd
// (mirrors TestEditTitleSubmitFiresSetTitleDirectlyNoConfirm's own
// "inspect before it fires" pattern, form_edit_title_test.go) against a REAL
// t.TempDir()-backed *data.Store, doubling as the Persistenz-Roundtrip
// requirement: after cmd() runs, the on-disk .lif-tags.yml actually
// contains the new name.
func TestKeyTagMgmtInputValidSubmitFiresSaveTagDefsCmdWithAddedName(t *testing.T) {
	dir := t.TempDir()
	m := newModel(&data.Store{RepoDir: dir}, dir)
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "old-defined", defined: true}, {name: "free-one", defined: false}}

	nm, _ := m.openTagMgmtInput("create", "")
	m = nm.(model)
	m.tagMgmtInput.SetValue("brand-new")

	tm, cmd := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	mm := tm.(model)
	if mm.tagMgmtInputErr != "" {
		t.Fatalf("want no validation error, got %q", mm.tagMgmtInputErr)
	}
	if cmd == nil {
		t.Fatal("valid submit must return a non-nil Cmd (saveTagDefsCmd)")
	}

	msg := cmd()
	tdm, ok := msg.(tagDefsSavedMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want tagDefsSavedMsg", msg)
	}
	if tdm.err != nil {
		t.Fatalf("SaveTagDefs against a real t.TempDir() client failed: %v", tdm.err)
	}

	got, err := (&data.Store{RepoDir: dir}).LoadTagDefs()
	if err != nil {
		t.Fatalf("LoadTagDefs after save: %v", err)
	}
	want := []string{"brand-new", "old-defined"} // AddTagDefName, sorted -- "free-one" was never a registry entry
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on-disk registry = %v, want %v (persistence round trip)", got, want)
	}
}

// TestApplyTagDefsSavedSuccessRefreshesRowsAndMovesCursor guards
// applyTagDefsSaved's success tail: the input sub-mode closes, tagMgmtRows
// is rebuilt from a FRESH LoadTagDefs (D03) via tagRegistryRows, and the
// cursor lands on the newly created row (mirrors applyLoaded's own
// cursor-refinding-by-ID pattern one layer up).
func TestApplyTagDefsSavedSuccessRefreshesRowsAndMovesCursor(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	if err := client.SaveTagDefs([]string{"old-defined", "brand-new"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}

	m := newModel(client, dir)
	m.view = viewTagManagement
	m.tagMgmtInputActive = true
	m.tagMgmtInputMode = "create"
	m.tagMgmtInput.SetValue("brand-new")
	m.tagMgmtInput.Focus()
	m.tagMgmtRows = []tagRegistryRow{{name: "old-defined", defined: true}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.applyTagDefsSaved(tagDefsSavedMsg{err: nil})
	mm := nm.(model)
	if cmd != nil {
		t.Fatalf("success must not return a Cmd (no unconditional reload, only the Registry changed), got %v", cmd)
	}
	if mm.tagMgmtInputActive {
		t.Fatal("success must close the input sub-mode")
	}
	if mm.tagMgmtInput.Focused() {
		t.Fatal("success must blur the input")
	}

	wantNames := []string{"brand-new", "old-defined"}
	var gotNames []string
	for _, r := range mm.tagMgmtRows {
		gotNames = append(gotNames, r.name)
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("tagMgmtRows names = %v, want %v", gotNames, wantNames)
	}
	if mm.tagMgmtCursor.cursor < 0 || mm.tagMgmtCursor.cursor >= len(mm.tagMgmtRows) || mm.tagMgmtRows[mm.tagMgmtCursor.cursor].name != "brand-new" {
		t.Fatalf("cursor must land on the new row, cursor=%d rows=%+v", mm.tagMgmtCursor.cursor, mm.tagMgmtRows)
	}
}

// TestApplyTagDefsSavedErrorKeepsInputOpenShowsToast guards the error tail
// (mirrors applyMutationResult's error branch, WITHOUT its unconditional
// loadCmd reload -- "hier gibt es kein m.idx zu invalidieren, nur die
// Registry", bean bt-604w wording): the input sub-mode stays open (so the
// PO can retry the same name without retyping it) and a Toast surfaces the
// error.
func TestApplyTagDefsSavedErrorKeepsInputOpenShowsToast(t *testing.T) {
	m := newModel(newTestStore(t), "/nonexistent-bt-604w-scratch-dir")
	m.view = viewTagManagement
	m.tagMgmtInputActive = true
	m.tagMgmtInputMode = "create"
	m.tagMgmtInput.SetValue("brand-new")

	nm, cmd := m.applyTagDefsSaved(tagDefsSavedMsg{err: fmt.Errorf("write .lif-tags.yml: no such file or directory")})
	mm := nm.(model)
	if !mm.tagMgmtInputActive {
		t.Fatal("an I/O failure must keep the input sub-mode open for a retry")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the error Toast)")
	}
	if mm.toast == nil || mm.toast.kind != toastError {
		t.Fatalf("want an error Toast, got %+v", mm.toast)
	}
}

// TestApplyTagDefsSavedSuccessToastShowsInfoToastWhenSuccessToastSet guards
// bt-idm1's own success-Toast tail (E11 Item 6): when the dispatching Cmd
// carried a non-empty successToast (Adopt's own dispatch, unlike Create/
// Rename/Delete which all pass "" unchanged), the success path shows a
// toastInfo Toast with that exact text -- Create/Rename/Delete stay silent on
// success (TestApplyTagDefsSavedSuccessRefreshesRowsAndMovesCursor above
// already guards the cmd==nil case for an empty successToast).
func TestApplyTagDefsSavedSuccessToastShowsInfoToastWhenSuccessToastSet(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	if err := client.SaveTagDefs([]string{"free-tag"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m := newModel(client, dir)
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-tag", defined: false, count: 1}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.applyTagDefsSaved(tagDefsSavedMsg{err: nil, refindName: "free-tag", successToast: "tag 'free-tag' registered"})
	mm := nm.(model)
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the success Toast's auto-dismiss timeout) when successToast is set")
	}
	if mm.toast == nil || mm.toast.kind != toastInfo {
		t.Fatalf("want a toastInfo Toast, got %+v", mm.toast)
	}
	if mm.toast.title != "tag 'free-tag' registered" {
		t.Fatalf("toast.title = %q, want %q", mm.toast.title, "tag 'free-tag' registered")
	}
}

// TestFullCreateFlowRefreshesPageAndTouchesNoBean is the end-to-end
// behavioral test the harness brief demands ("Page-Refresh nach Create") --
// drives the ENTIRE wiring through m.Update() (handleKey -> keyTagManagement
// -> keyTagMgmtInput -> saveTagDefsCmd -> cmd() -> Update(tagDefsSavedMsg)
// -> applyTagDefsSaved), then asserts D11's own regression requirement: a
// Create never touches m.idx (no Bean is mutated, only the Registry).
func TestFullCreateFlowRefreshesPageAndTouchesNoBean(t *testing.T) {
	dir := t.TempDir()
	m := fixtureModel(t, fixtureBeans())
	m.store = &data.Store{RepoDir: dir}
	m.repoDir = dir
	m.view = viewTagManagement
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	idxBefore := m.idx

	m = step(t, m, runeMsg('n'))
	for _, r := range "released" {
		m = step(t, m, runeMsg(r))
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	m = tm.(model)
	if cmd == nil {
		t.Fatal("enter on a valid name must return the saveTagDefsCmd Cmd")
	}
	msg := cmd()

	tm2, _ := m.Update(msg)
	m = tm2.(model)

	if m.tagMgmtInputActive {
		t.Fatal("Page must exit the input sub-mode after a successful create")
	}
	found := false
	for _, r := range m.tagMgmtRows {
		if r.name == "released" && r.defined {
			found = true
		}
	}
	if !found {
		t.Fatalf("want 'released' to appear as a defined row after Create, got %+v", m.tagMgmtRows)
	}
	if m.idx != idxBefore {
		t.Fatal("D11: Create must not touch m.idx (no Bean mutation, Registry-only)")
	}
	for _, b := range m.idx.ByID {
		for _, tag := range b.Tags {
			if tag == "released" {
				t.Fatalf("D11: Create must not touch any Bean, but %s now carries 'released'", b.ID)
			}
		}
	}
}

// TestFullAdoptFlowRegistersFreeRowShowsToastCursorFollows is the end-to-end
// behavioral test for bt-idm1 (E11 Item 6): drives the ENTIRE wiring through
// m.Update() (handleKey -> keyTagManagement -> openTagMgmtAdopt ->
// saveTagDefsCmd -> cmd() -> Update(tagDefsSavedMsg) -> applyTagDefsSaved)
// with the cursor on a FREE row -- asserts the row becomes defined=true, the
// cursor follows it (mirrors Delete/Rename's own refindName convention), and
// a toastInfo success Toast appears (no new silent success path, bt-ct3k
// Toast-Konsistenz).
func TestFullAdoptFlowRegistersFreeRowShowsToastCursorFollows(t *testing.T) {
	dir := t.TempDir()
	m := fixtureModel(t, fixtureBeans())
	m.store = &data.Store{RepoDir: dir}
	m.repoDir = dir
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "old-defined", defined: true}, {name: "free-tag", defined: false, count: 3}}
	m.tagMgmtCursor.setLen(2)
	m.tagMgmtCursor.cursor = 1

	tm, cmd := m.Update(runeMsg('n'))
	m = tm.(model)
	if m.tagMgmtInputActive {
		t.Fatal("Adopt must not open the input sub-mode")
	}
	if cmd == nil {
		t.Fatal("'n' on a free row must return the saveTagDefsCmd Cmd")
	}
	msg := cmd()

	tm2, _ := m.Update(msg)
	m = tm2.(model)

	found := false
	for i, r := range m.tagMgmtRows {
		if r.name == "free-tag" {
			found = true
			if !r.defined {
				t.Fatalf("want 'free-tag' now defined=true after Adopt, got %+v", r)
			}
			if m.tagMgmtCursor.cursor != i {
				t.Fatalf("want cursor to follow 'free-tag' to index %d, cursor=%d", i, m.tagMgmtCursor.cursor)
			}
		}
	}
	if !found {
		t.Fatalf("want 'free-tag' still present as a row, got %+v", m.tagMgmtRows)
	}
	if m.toast == nil || m.toast.kind != toastInfo {
		t.Fatalf("want a toastInfo success Toast after Adopt, got %+v", m.toast)
	}
}

// TestTagManagementLocalBindingsIncludesNewTag guards the Footer Zone 3
// wiring: tagManagementLocalBindings (shared function body, T2's own
// "EIN gemeinsamer Funktionsrumpf" contract) grows keys.NewTag alongside
// Up/Down/Back.
func TestTagManagementLocalBindingsIncludesNewTag(t *testing.T) {
	bindings := tagManagementLocalBindings()
	found := false
	for _, b := range bindings {
		if b.Help().Key == keys.NewTag.Help().Key {
			found = true
		}
	}
	if !found {
		t.Fatalf("tagManagementLocalBindings missing keys.NewTag, got %+v", bindings)
	}
}

// TestViewTagManagementRendersInputSubmode guards the (bean-implicit, but
// tmux-smoke-mandatory) render contract: while the input sub-mode is active,
// viewTagManagement's output shows the live input value AND an inline error
// (when set) INSIDE the SAME full-capture page (D06/D14 -- "lebt INNERHALB
// der Full-Capture-Page"), not a floating overlay/modalPanel like the
// Tag-Picker's own tagInputBox.
func TestViewTagManagementRendersInputSubmode(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 100, 30
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	nm2, _ := m.openTagMgmtInput("create", "")
	m = nm2.(model)
	m.tagMgmtInput.SetValue("typed-so-far")
	m.tagMgmtInputErr = "invalid tag name (a-z0-9, hyphen-separated, lowercase)"

	out := m.viewTagManagement()
	if !strings.Contains(out, "typed-so-far") {
		t.Errorf("viewTagManagement() while input active missing the typed value:\n%s", out)
	}
	if !strings.Contains(out, "invalid tag name") {
		t.Errorf("viewTagManagement() while input active missing the inline error:\n%s", out)
	}
}

// TestViewTagManagementInputSubmodeNoWrapAt80 pins the tmux-smoke acceptance
// wording verbatim ("Bei 80 Spalten: Inline-Fehlertext darf nicht truncaten
// ohne …") -- an oversized error message at width=80 must be cut with an
// ellipsis (renderPane's own per-row truncate(w-2) budget, view.go's
// truncate helper), never silently wrapped/overflowed (same B01 bug class
// TestViewTagManagementLinesFitExactlyNoWrap already pins for the normal row
// list).
func TestViewTagManagementInputSubmodeNoWrapAt80(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 80, 40
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	nm2, _ := m.openTagMgmtInput("create", "")
	m = nm2.(model)
	m.tagMgmtInputErr = strings.Repeat("a very long inline validation error message ", 5)

	out := m.viewTagManagement()
	lines := strings.Split(out, "\n")
	if len(lines) != m.height {
		t.Fatalf("got %d lines, want exactly %d (height) -- an unbudgeted error line broke the pane's line count", len(lines), m.height)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w != m.width {
			t.Fatalf("line %d width = %d, want exactly %d: %q", i, w, m.width, l)
		}
	}
}

// --- openTagMgmtDeleteConfirm / keyTagMgmtDeleteConfirm: D12/D15 Delete ---

// TestOpenTagMgmtDeleteConfirmNoOpOnFreeTag is the bean's own named RED test
// (bt-1lsu TDD section, quoted verbatim): `d` on a FREE (undefined) row has
// nothing to delete -- a free tag has no Registry Definition to remove --
// so opening the Confirm is a silent no-op, mirrors focusedBean()==nil's
// no-op convention quer durchs Repo (bean body wording).
func TestOpenTagMgmtDeleteConfirmNoOpOnFreeTag(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-tag", defined: false, count: 3}}
	nm, cmd := m.openTagMgmtDeleteConfirm()
	mm := nm.(model)
	if mm.tagMgmtDeleteConfirm {
		t.Fatal("want no-op for a free (undefined) tag row -- no confirm opens, no Registry write")
	}
	// bt-ct3k (E11 Item 5): the no-op must no longer be SILENT -- a warn Toast
	// replaces the former "nothing visibly happens" behavior (PO-Nebenbefund,
	// US-Review Runde 7: felt like a broken keybind).
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the warn Toast's auto-dismiss timeout) instead of a silent no-op")
	}
	if mm.toast == nil || mm.toast.kind != toastWarn {
		t.Fatalf("want a toastWarn Toast, got %+v", mm.toast)
	}
	if !strings.Contains(mm.toast.title, "Unregistered tag") {
		t.Fatalf("toast.title = %q, want it to mention 'Unregistered tag'", mm.toast.title)
	}
}

// TestOpenTagMgmtDeleteConfirmOnDefinedRowSetsConfirmAndTarget guards the
// success path: `d` on a DEFINED row sets tagMgmtDeleteConfirm=true and
// tagMgmtDeleteTarget to that row's name (D15's page-local Bool+Ziel-Paar).
func TestOpenTagMgmtDeleteConfirmOnDefinedRowSetsConfirmAndTarget(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 5}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.openTagMgmtDeleteConfirm()
	mm := nm.(model)
	if !mm.tagMgmtDeleteConfirm {
		t.Fatal("want tagMgmtDeleteConfirm=true for a defined row")
	}
	if mm.tagMgmtDeleteTarget != "to-review" {
		t.Fatalf("tagMgmtDeleteTarget = %q, want %q", mm.tagMgmtDeleteTarget, "to-review")
	}
	if cmd != nil {
		t.Fatalf("want nil Cmd (no save yet, only the confirm opens), got %v", cmd)
	}
}

// TestOpenTagMgmtDeleteConfirmNoOpWhenCursorOutOfRange guards the Randfall
// an empty/degenerate row list -- must never panic or open a confirm with a
// bogus target.
func TestOpenTagMgmtDeleteConfirmNoOpWhenCursorOutOfRange(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = nil
	m.tagMgmtCursor = listState{}

	nm, _ := m.openTagMgmtDeleteConfirm()
	if nm.(model).tagMgmtDeleteConfirm {
		t.Fatal("want no-op when the cursor has no row to target")
	}
}

// TestKeyTagManagementDeleteDispatchesToOpenConfirm guards keyTagManagement's
// own D06 dispatch wiring: keys.Delete ("d") on a defined row opens the
// Confirm via the SAME path a direct openTagMgmtDeleteConfirm() call takes.
func TestKeyTagManagementDeleteDispatchesToOpenConfirm(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 2}}
	m.tagMgmtCursor.setLen(1)

	nm, _ := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm := nm.(model)
	if !mm.tagMgmtDeleteConfirm || mm.tagMgmtDeleteTarget != "to-review" {
		t.Fatalf("want confirm opened targeting 'to-review', got confirm=%v target=%q", mm.tagMgmtDeleteConfirm, mm.tagMgmtDeleteTarget)
	}
}

// TestKeyTagManagementDeleteNoOpOnFreeRowViaFullDispatch guards the SAME
// no-op through the full keyTagManagement dispatch (not just the direct
// openTagMgmtDeleteConfirm() call above).
func TestKeyTagManagementDeleteNoOpOnFreeRowViaFullDispatch(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-one", defined: false, count: 4}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	mm := nm.(model)
	if mm.tagMgmtDeleteConfirm {
		t.Fatal("want no confirm opened for a free row")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the warn Toast), not a silent no-op")
	}
	if mm.toast == nil || mm.toast.kind != toastWarn {
		t.Fatalf("want a toastWarn Toast via the full dispatch, got %+v", mm.toast)
	}
}

// TestKeyTagMgmtDeleteConfirmEscCancelsWithoutSaving is the bean's own named
// RED test (bt-1lsu TDD section, quoted verbatim).
func TestKeyTagMgmtDeleteConfirmEscCancelsWithoutSaving(t *testing.T) {
	m := newModel(nil, "")
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"
	nm, cmd := m.keyTagMgmtDeleteConfirm(tea.KeyMsg{Type: tea.KeyEsc})
	if nm.(model).tagMgmtDeleteConfirm || cmd != nil {
		t.Fatalf("want cancel with no Cmd, got confirm=%v cmd=%v",
			nm.(model).tagMgmtDeleteConfirm, cmd)
	}
}

// TestKeyTagMgmtDeleteConfirmNCancelsWithoutSaving extends the esc-cancel
// RED test above to the literal 'n' key (deleteBox's own esc/n-cancel dual,
// box_confirm_delete.go's keyDeleteConfirm precedent).
func TestKeyTagMgmtDeleteConfirmNCancelsWithoutSaving(t *testing.T) {
	m := newModel(nil, "")
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"
	nm, cmd := m.keyTagMgmtDeleteConfirm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if nm.(model).tagMgmtDeleteConfirm || cmd != nil {
		t.Fatalf("want cancel with no Cmd, got confirm=%v cmd=%v",
			nm.(model).tagMgmtDeleteConfirm, cmd)
	}
	if nm.(model).tagMgmtDeleteTarget != "" {
		t.Fatalf("want tagMgmtDeleteTarget cleared after cancel, got %q", nm.(model).tagMgmtDeleteTarget)
	}
}

// TestKeyTagMgmtDeleteConfirmOtherKeysDuringConfirmAreNoOp is the
// Full-Capture-Disziplin guard this task's harness brief explicitly demands:
// any key OTHER than enter/esc/n while the Confirm is open must be silently
// swallowed (no leak to any global/node-action handler, mirrors
// TestKeyTagMgmtInputCapturesEveryKeyNoLeak's own precedent one layer up).
func TestKeyTagMgmtDeleteConfirmOtherKeysDuringConfirmAreNoOp(t *testing.T) {
	m := newModel(nil, "")
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("x")},
		{Type: tea.KeyUp},
		{Type: tea.KeyDown},
		{Type: tea.KeyRunes, Runes: []rune("?")},
	} {
		nm, cmd := m.keyTagMgmtDeleteConfirm(msg)
		mm := nm.(model)
		if !mm.tagMgmtDeleteConfirm || mm.tagMgmtDeleteTarget != "to-review" || cmd != nil {
			t.Fatalf("key %v must be a no-op while Confirm is open, got confirm=%v target=%q cmd=%v", msg, mm.tagMgmtDeleteConfirm, mm.tagMgmtDeleteTarget, cmd)
		}
	}
}

// TestKeyTagManagementDeleteConfirmCapturesFullDispatchNoLeak extends the
// no-leak guard through the FULL keyTagManagement dispatch (not just the
// isolated keyTagMgmtDeleteConfirm call above) -- pressing 'd'/'?'/ctrl+k
// while the Confirm is open must not open a SECOND confirm, Help, or the
// Command-Center (mirrors TestHandleKeyOnTagManagementViewDoesNotLeakToNodeAction/
// TestKeyTagMgmtInputCapturesEveryKeyNoLeak's own D06/full-capture
// precedent).
func TestKeyTagManagementDeleteConfirmCapturesFullDispatchNoLeak(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 1}, {name: "other-tag", defined: true, count: 0}}
	m.tagMgmtCursor.setLen(2)
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"

	m = step(t, m, runeMsg('d'))
	m = step(t, m, runeMsg('?'))
	m = step(t, m, runeMsg('K')) // bt-mx4k: `K` replaces the retired ctrl+k

	if m.helpOpen {
		t.Fatal("'?' while Confirm is open must not open Help")
	}
	if m.paletteOpen {
		t.Fatal("`K` while Confirm is open must not open the Command-Center")
	}
	if !m.tagMgmtDeleteConfirm || m.tagMgmtDeleteTarget != "to-review" {
		t.Fatalf("want Confirm to stay open and unchanged, got confirm=%v target=%q", m.tagMgmtDeleteConfirm, m.tagMgmtDeleteTarget)
	}
}

// TestKeyTagMgmtDeleteConfirmEnterFiresSaveTagDefsCmdWithTargetRemoved
// guards D12's core dispatch: enter fires saveTagDefsCmd EXACTLY ONCE with
// the target removed from the registry's current defined names
// (data.RemoveTagDefName, reused from T1 per bean bt-604w's own "Notes for
// T4+T5" pointer) -- proven against a REAL t.TempDir()-backed *data.Store,
// doubling as the Registry-Roundtrip requirement (tag gone from the file).
func TestKeyTagMgmtDeleteConfirmEnterFiresSaveTagDefsCmdWithTargetRemoved(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	if err := client.SaveTagDefs([]string{"keep-me", "to-review"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}

	m := newModel(client, dir)
	m.tagMgmtRows = []tagRegistryRow{{name: "keep-me", defined: true}, {name: "to-review", defined: true, count: 3}}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"

	nm, cmd := m.keyTagMgmtDeleteConfirm(tea.KeyMsg{Type: tea.KeyEnter})
	mm := nm.(model)
	if mm.tagMgmtDeleteConfirm {
		t.Fatal("enter must close the Confirm immediately (mirrors keyDeleteConfirm)")
	}
	if cmd == nil {
		t.Fatal("enter must return a non-nil Cmd (saveTagDefsCmd)")
	}

	msg := cmd()
	tdm, ok := msg.(tagDefsSavedMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want tagDefsSavedMsg", msg)
	}
	if tdm.err != nil {
		t.Fatalf("SaveTagDefs against a real t.TempDir() client failed: %v", tdm.err)
	}

	got, err := (&data.Store{RepoDir: dir}).LoadTagDefs()
	if err != nil {
		t.Fatalf("LoadTagDefs after delete: %v", err)
	}
	want := []string{"keep-me"} // "to-review" removed, "keep-me" untouched
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("on-disk registry = %v, want %v (Registry-Roundtrip)", got, want)
	}
}

// TestFullDeleteFlowUsedTagFallsBackToFreeGroupCountPreserved is the
// end-to-end D12 regression: deleting the Definition of a tag that is
// CURRENTLY USED by a bean must NOT touch that bean -- the tag simply falls
// back into the Free group with its usage count UNCHANGED. Drives the
// ENTIRE wiring through m.Update() (handleKey -> keyTagManagement ->
// keyTagMgmtDeleteConfirm -> saveTagDefsCmd -> cmd() ->
// Update(tagDefsSavedMsg) -> applyTagDefsSaved), mirrors T3's own
// TestFullCreateFlowRefreshesPageAndTouchesNoBean D11-regression precedent,
// here for D12.
func TestFullDeleteFlowUsedTagFallsBackToFreeGroupCountPreserved(t *testing.T) {
	dir := t.TempDir()
	beans := fixtureBeansTagged() // "urgent" used by tk-1+tk-2 (count 2)
	m := fixtureModel(t, beans)
	m.store = &data.Store{RepoDir: dir}
	m.repoDir = dir
	if err := m.store.SaveTagDefs([]string{"urgent"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m.view = viewTagManagement
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	idxBefore := m.idx

	found := false
	for i, r := range m.tagMgmtRows {
		if r.name == "urgent" && r.defined {
			m.tagMgmtCursor.cursor = i
			found = true
		}
	}
	if !found {
		t.Fatalf("setup: want 'urgent' as a defined row before delete, got %+v", m.tagMgmtRows)
	}

	m = step(t, m, runeMsg('d'))
	if !m.tagMgmtDeleteConfirm || m.tagMgmtDeleteTarget != "urgent" {
		t.Fatalf("want Confirm open targeting 'urgent', got confirm=%v target=%q", m.tagMgmtDeleteConfirm, m.tagMgmtDeleteTarget)
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	m = tm.(model)
	if cmd == nil {
		t.Fatal("enter on the Confirm must return the saveTagDefsCmd Cmd")
	}
	msg := cmd()
	tm2, _ := m.Update(msg)
	m = tm2.(model)

	if m.tagMgmtDeleteConfirm {
		t.Fatal("Confirm must be closed after the save round-trips")
	}

	var row *tagRegistryRow
	for i := range m.tagMgmtRows {
		if m.tagMgmtRows[i].name == "urgent" {
			row = &m.tagMgmtRows[i]
		}
	}
	if row == nil {
		t.Fatal("want 'urgent' to still appear as a row (now free) after delete")
	}
	if row.defined {
		t.Fatal("D12: 'urgent' must be FREE (undefined) after its Definition is deleted")
	}
	if row.count != 2 {
		t.Fatalf("D12: usage count must be preserved (tk-1+tk-2 still carry it), got %d, want 2", row.count)
	}

	if m.idx != idxBefore {
		t.Fatal("D12: Delete must not touch m.idx (no Bean mutation, Registry-only)")
	}
	tagged := 0
	for _, b := range m.idx.ByID {
		for _, tag := range b.Tags {
			if tag == "urgent" {
				tagged++
			}
		}
	}
	if tagged != 2 {
		t.Fatalf("D12: both beans must still carry 'urgent' after Delete, got %d beans tagged", tagged)
	}
}

// TestFullDeleteFlowUnusedTagDisappearsEntirely is D12's second acceptance
// clause ("unbenutztes Tag verschwindet ganz"): deleting the Definition of a
// tag with usage count 0 removes the row ENTIRELY (it was only visible
// because it was defined -- with the Definition gone and no bean carrying
// it, D09's Union has nothing left to list it under).
func TestFullDeleteFlowUnusedTagDisappearsEntirely(t *testing.T) {
	dir := t.TempDir()
	m := fixtureModel(t, fixtureBeans()) // no tags on any bean
	m.store = &data.Store{RepoDir: dir}
	m.repoDir = dir
	if err := m.store.SaveTagDefs([]string{"unused-tag"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m.view = viewTagManagement
	nm, _ := m.openTagManagementPage()
	m = nm.(model)

	found := false
	for i, r := range m.tagMgmtRows {
		if r.name == "unused-tag" {
			m.tagMgmtCursor.cursor = i
			found = true
		}
	}
	if !found {
		t.Fatalf("setup: want 'unused-tag' as a row before delete, got %+v", m.tagMgmtRows)
	}

	m = step(t, m, runeMsg('d'))
	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	m = tm.(model)
	msg := cmd()
	tm2, _ := m.Update(msg)
	m = tm2.(model)

	for _, r := range m.tagMgmtRows {
		if r.name == "unused-tag" {
			t.Fatalf("want 'unused-tag' to disappear entirely after delete, still present: %+v", r)
		}
	}
}

// TestFullDeleteFlowIgnoresStaleAbortedCreateInputText is the B01
// regression (T4-Review Runde 1, medium): applyTagDefsSaved used to re-find
// the cursor via strings.TrimSpace(m.tagMgmtInput.Value()) -- safe while
// Create was the ONLY saveTagDefsCmd caller, but T4's Delete path never
// touches the input field, and T3's esc-abort deliberately does NOT clear
// Value(). Repro (reviewer's own, verbatim shape): type a name into the
// Create input, abort with esc, then run an UNRELATED Delete -- the cursor
// must not jump onto the stale typed text's row. Fixed by tagDefsSavedMsg's
// explicit refindName field (both callers pass the name explicitly, no
// implicit input read).
func TestFullDeleteFlowIgnoresStaleAbortedCreateInputText(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	if err := client.SaveTagDefs([]string{"alpha", "bravo", "charlie"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m := fixtureModel(t, fixtureBeans()) // no tags on any bean
	m.store = client
	m.repoDir = dir
	m.view = viewTagManagement
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	// rows: alpha, bravo, charlie (all defined, alpha-sorted, all count 0)

	// Type "charlie" into the Create input, then abort with esc -- T3's esc
	// closes the sub-mode but deliberately does NOT clear the typed text.
	m = step(t, m, runeMsg('n'))
	for _, r := range "charlie" {
		m = step(t, m, runeMsg(r))
	}
	m = step(t, m, keyMsg(tea.KeyEsc))
	if m.tagMgmtInputActive {
		t.Fatal("setup: esc must close the input sub-mode")
	}

	// Cursor on alpha (index 0), delete it -- an operation completely
	// unrelated to the aborted Create above.
	m.tagMgmtCursor.cursor = 0
	if m.tagMgmtRows[0].name != "alpha" {
		t.Fatalf("setup: want row 0 = alpha, got %+v", m.tagMgmtRows)
	}
	m = step(t, m, runeMsg('d'))
	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	m = tm.(model)
	if cmd == nil {
		t.Fatal("enter on the Confirm must return the saveTagDefsCmd Cmd")
	}
	tm2, _ := m.Update(cmd())
	m = tm2.(model)

	// rows now: bravo, charlie. The cursor must NOT land on "charlie" just
	// because that text is still sitting in the aborted Create input.
	if got := m.tagMgmtRows[m.tagMgmtCursor.cursor].name; got == "charlie" {
		t.Fatalf("B01: cursor jumped to stale aborted-Create text %q after an independent Delete", got)
	}
	if m.tagMgmtCursor.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (deleted row vanished entirely, no refind target left)", m.tagMgmtCursor.cursor)
	}
}

// TestFullDeleteFlowCursorFollowsUsedTagIntoFreeGroup pins refindName's own
// positive Delete-side behavior (B01 fix, T4-Review Runde 1): Delete passes
// its TARGET as the refind name, so deleting a still-USED tag's Definition
// moves the cursor along with the row as it falls into the Free group --
// instead of silently pointing at whatever row inherited the old index.
func TestFullDeleteFlowCursorFollowsUsedTagIntoFreeGroup(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	// "urgent" is used by tk-1+tk-2 (fixtureBeansTagged). Registry layout is
	// picked so the deleted row's OLD index (0: defined group sorts urgent <
	// zzz-last) and its NEW free-group position (1: after the one remaining
	// defined row) genuinely differ -- a cursor that merely kept its old
	// numeric index would land on "zzz-last", not follow "urgent".
	if err := client.SaveTagDefs([]string{"urgent", "zzz-last"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m := fixtureModel(t, fixtureBeansTagged())
	m.store = client
	m.repoDir = dir
	m.view = viewTagManagement
	nm, _ := m.openTagManagementPage()
	m = nm.(model)

	found := false
	for i, r := range m.tagMgmtRows {
		if r.name == "urgent" && r.defined {
			m.tagMgmtCursor.cursor = i
			found = true
		}
	}
	if !found {
		t.Fatalf("setup: want 'urgent' as a defined row, got %+v", m.tagMgmtRows)
	}

	m = step(t, m, runeMsg('d'))
	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	m = tm.(model)
	tm2, _ := m.Update(cmd())
	m = tm2.(model)

	got := m.tagMgmtRows[m.tagMgmtCursor.cursor]
	if got.name != "urgent" || got.defined {
		t.Fatalf("want cursor to follow 'urgent' into the Free group, got %+v", got)
	}
}

// TestTagMgmtDeleteConfirmBoxShowsLiveCountAndName guards the Confirm
// modal's own render contract (D12's own "zeigt trotzdem den LIVE-
// Verwendungszähler VOR dem Löschen" wording): the box's text names the
// target and its CURRENT count, resolved live from m.tagMgmtRows at render
// time (mirrors deleteBox's own "typ resolves from the LIVE index at render
// time... since it is only needed for display, not for dispatch" precedent,
// box_confirm_delete.go doc-stamp).
func TestTagMgmtDeleteConfirmBoxShowsLiveCountAndName(t *testing.T) {
	m := newModel(nil, "")
	m.width = 80
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 7}}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"

	out := m.tagMgmtDeleteConfirmBox()
	if !strings.Contains(out, "to-review") {
		t.Errorf("Confirm box missing the target tag name:\n%s", out)
	}
	if !strings.Contains(out, "7") {
		t.Errorf("Confirm box missing the live usage count:\n%s", out)
	}
	if !strings.Contains(out, "enter") || !strings.Contains(out, "esc") {
		t.Errorf("Confirm box missing the enter/esc footer hint:\n%s", out)
	}
}

// TestTagMgmtDeleteConfirmBoxZeroCountShorterText pins D12's own explicit
// wording for the count==0 case ("Not currently used by any item" -- a
// shorter, non-contradictory text, not "Still used by 0 bean(s)").
func TestTagMgmtDeleteConfirmBoxZeroCountShorterText(t *testing.T) {
	m := newModel(nil, "")
	m.width = 80
	m.tagMgmtRows = []tagRegistryRow{{name: "unused-tag", defined: true, count: 0}}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "unused-tag"

	out := m.tagMgmtDeleteConfirmBox()
	if !strings.Contains(out, "Not currently used by any item") {
		t.Errorf("want the zero-count shorter text, got:\n%s", out)
	}
	if strings.Contains(out, "Still used by") {
		t.Errorf("zero-count text must not also render the 'Still used by' wording:\n%s", out)
	}
}

// TestTagMgmtDeleteConfirmBoxSingularOneBean pins the count==1 singular
// branch ("Still used by 1 bean — it keeps the tag ...") -- I01, T4-Review
// Runde 1: this bug class has repo precedent (box_confirm_delete.go's own
// I02 doc-stamp, "1 child(ren) lose" was grammatically wrong for count==1),
// so the singular branch gets its own dedicated test exactly like the
// count==0 and count==7 cases above/below it.
func TestTagMgmtDeleteConfirmBoxSingularOneBean(t *testing.T) {
	m := newModel(nil, "")
	m.width = 80
	m.tagMgmtRows = []tagRegistryRow{{name: "solo-tag", defined: true, count: 1}}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "solo-tag"

	out := m.tagMgmtDeleteConfirmBox()
	if !strings.Contains(out, "Still used by 1 item") {
		t.Errorf("want the singular 'Still used by 1 item' text, got:\n%s", out)
	}
	if strings.Contains(out, "beans") {
		t.Errorf("count==1 must not render the plural 'beans' wording:\n%s", out)
	}
	if strings.Contains(out, "Not currently used") {
		t.Errorf("count==1 must not render the zero-count text:\n%s", out)
	}
}

// TestViewTagManagementRendersDeleteConfirmCentered guards the D15/D06
// compositing wiring: viewTagManagement (via composeOverlays,
// view_browse_repo.go) paints the Confirm modal over the row list when
// tagMgmtDeleteConfirm is set -- mirrors confirmQuit's own composeOverlays
// branch (D15's explicit "exakt wie es bereits m.confirmQuit kennt"
// wording).
func TestViewTagManagementRendersDeleteConfirmCentered(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.width, m.height = 100, 30
	nm, _ := m.openTagManagementPage()
	m = nm.(model)
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 4}}
	m.tagMgmtDeleteConfirm = true
	m.tagMgmtDeleteTarget = "to-review"

	out := m.viewTagManagement()
	if !strings.Contains(out, "to-review") {
		t.Errorf("viewTagManagement() with Confirm open missing the target tag name:\n%s", out)
	}
	if !strings.Contains(out, "Still used by 4 items") {
		t.Errorf("viewTagManagement() with Confirm open missing the live count text:\n%s", out)
	}
}

// TestTagManagementLocalBindingsIncludesDelete guards the Footer Zone 3
// wiring: tagManagementLocalBindings (shared function body, T2's own "EIN
// gemeinsamer Funktionsrumpf" contract) grows keys.Delete alongside
// Up/Down/NewTag/Back.
func TestTagManagementLocalBindingsIncludesDelete(t *testing.T) {
	bindings := tagManagementLocalBindings()
	found := false
	for _, b := range bindings {
		if b.Help().Key == keys.Delete.Help().Key {
			found = true
		}
	}
	if !found {
		t.Fatalf("tagManagementLocalBindings missing keys.Delete, got %+v", bindings)
	}
}

// --- T5 (bean bt-y9my, epic bt-362n D13/D14): Rename + Bean-Sweep ---

// --- openTagMgmtRename ---

// TestOpenTagMgmtRenameNoOpOnFreeTag mirrors
// TestOpenTagMgmtDeleteConfirmNoOpOnFreeTag's own no-op shape (T4,
// bt-1lsu): a free (undefined) row has no Registry entry to rename.
func TestOpenTagMgmtRenameNoOpOnFreeTag(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-tag", defined: false, count: 3}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.openTagMgmtRename()
	mm := nm.(model)
	if mm.tagMgmtInputActive {
		t.Fatal("want no-op for a free (undefined) tag row -- no input opens, no Registry write")
	}
	// bt-ct3k (E11 Item 5): the no-op must no longer be SILENT -- a warn Toast
	// replaces the former "nothing visibly happens" behavior (PO-Nebenbefund,
	// US-Review Runde 7: felt like a broken keybind).
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the warn Toast's auto-dismiss timeout) instead of a silent no-op")
	}
	if mm.toast == nil || mm.toast.kind != toastWarn {
		t.Fatalf("want a toastWarn Toast, got %+v", mm.toast)
	}
	if !strings.Contains(mm.toast.title, "Unregistered tag") {
		t.Fatalf("toast.title = %q, want it to mention 'Unregistered tag'", mm.toast.title)
	}
}

// TestOpenTagMgmtRenameNoOpWhenCursorOutOfRange guards the degenerate/empty
// row-list Randfall -- must never panic or open the input with a bogus
// target.
func TestOpenTagMgmtRenameNoOpWhenCursorOutOfRange(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = nil
	m.tagMgmtCursor = listState{}

	nm, _ := m.openTagMgmtRename()
	if nm.(model).tagMgmtInputActive {
		t.Fatal("want no-op when the cursor has no row to target")
	}
}

// TestOpenTagMgmtRenameOnDefinedRowOpensPrefilledInput guards the success
// path: `e` on a DEFINED row opens the SHARED input sub-mode (D14) in
// "rename" mode, prefilled with the row's own name in BOTH the visible text
// AND tagMgmtInputTarget (T3's own "Notes for T5" pointer, bean bt-604w).
func TestOpenTagMgmtRenameOnDefinedRowOpensPrefilledInput(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 5}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.openTagMgmtRename()
	mm := nm.(model)
	if !mm.tagMgmtInputActive || mm.tagMgmtInputMode != "rename" {
		t.Fatalf("want rename input open, got active=%v mode=%q", mm.tagMgmtInputActive, mm.tagMgmtInputMode)
	}
	if mm.tagMgmtInputTarget != "to-review" {
		t.Fatalf("tagMgmtInputTarget = %q, want %q", mm.tagMgmtInputTarget, "to-review")
	}
	if mm.tagMgmtInput.Value() != "to-review" {
		t.Fatalf("input value = %q, want prefilled %q", mm.tagMgmtInput.Value(), "to-review")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (textinput.Blink)")
	}
}

// TestKeyTagManagementRenameDispatchesToOpenInput guards keyTagManagement's
// own D06 dispatch wiring: keys.RenameTag ("e") on a defined row opens the
// rename input via the SAME path a direct openTagMgmtRename() call takes.
func TestKeyTagManagementRenameDispatchesToOpenInput(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "to-review", defined: true, count: 2}}
	m.tagMgmtCursor.setLen(1)

	nm, _ := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm := nm.(model)
	if !mm.tagMgmtInputActive || mm.tagMgmtInputMode != "rename" || mm.tagMgmtInputTarget != "to-review" {
		t.Fatalf("want rename input opened targeting 'to-review', got active=%v mode=%q target=%q",
			mm.tagMgmtInputActive, mm.tagMgmtInputMode, mm.tagMgmtInputTarget)
	}
}

// TestKeyTagManagementRenameNoOpOnFreeRowViaFullDispatch guards the SAME
// no-op through the full keyTagManagement dispatch (not just the isolated
// openTagMgmtRename() call above).
func TestKeyTagManagementRenameNoOpOnFreeRowViaFullDispatch(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "free-one", defined: false, count: 4}}
	m.tagMgmtCursor.setLen(1)

	nm, cmd := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm := nm.(model)
	if mm.tagMgmtInputActive {
		t.Fatal("want no input opened for a free row")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (the warn Toast), not a silent no-op")
	}
	if mm.toast == nil || mm.toast.kind != toastWarn {
		t.Fatalf("want a toastWarn Toast via the full dispatch, got %+v", mm.toast)
	}
}

// --- keyTagMgmtInput "rename" mode: dedupe, esc, submit ---

// TestKeyTagMgmtInputRenameDedupeAllowsOwnOldNameUnchanged is the bean's own
// explicitly-demanded Randfall test (bt-y9my TDD section): re-confirming
// the UNCHANGED old name (the prefilled value, untouched) must NOT be
// rejected as a duplicate of itself.
func TestKeyTagMgmtInputRenameDedupeAllowsOwnOldNameUnchanged(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "alpha", defined: true}, {name: "bravo", defined: true}}
	nm, _ := m.openTagMgmtInput("rename", "alpha")
	m = nm.(model)

	nm2, _ := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	got := nm2.(model)
	if got.tagMgmtInputErr != "" {
		t.Fatalf("want no dedupe error for the unchanged own old name, got %q", got.tagMgmtInputErr)
	}
}

// TestKeyTagMgmtInputRenameRejectsDuplicateAgainstOtherExistingName guards
// the OTHER side of the dedupe exclusion: renaming to a DIFFERENT existing
// name is still rejected, only the rename's own old name is excluded.
func TestKeyTagMgmtInputRenameRejectsDuplicateAgainstOtherExistingName(t *testing.T) {
	m := newModel(nil, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "alpha", defined: true}, {name: "bravo", defined: true}}
	nm, _ := m.openTagMgmtInput("rename", "alpha")
	m = nm.(model)
	m.tagMgmtInput.SetValue("bravo")

	nm2, _ := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	got := nm2.(model)
	// Exact wording pinned by T5-F01 (bean bt-sohl), same neutral text as the
	// Create-mode dedupe tests above.
	if !got.tagMgmtInputActive || got.tagMgmtInputErr != "name already in use: bravo" {
		t.Fatalf("want rejected duplicate against a DIFFERENT existing name with neutral error text, got active=%v err=%q",
			got.tagMgmtInputActive, got.tagMgmtInputErr)
	}
}

// TestKeyTagMgmtInputEscInRenameModeDiscardsOnlySubmodeNoSave mirrors T3's
// own esc-discards-only-submode contract (D14) explicitly for Rename: esc
// must fire NO Cmd (no Registry save, no sweep) and leave the Page itself
// open.
func TestKeyTagMgmtInputEscInRenameModeDiscardsOnlySubmodeNoSave(t *testing.T) {
	m := newModel(newTestStore(t), "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "alpha", defined: true}}
	nm, _ := m.openTagMgmtInput("rename", "alpha")
	m = nm.(model)
	m.tagMgmtInput.SetValue("zulu")

	nm2, cmd := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEsc})
	mm := nm2.(model)
	if mm.tagMgmtInputActive {
		t.Fatal("want esc to close the rename submode")
	}
	if cmd != nil {
		t.Fatalf("want nil Cmd on esc (no save, no sweep), got %v", cmd)
	}
	if mm.view != viewTagManagement {
		t.Fatal("want esc to leave the Page itself open (only the submode closes)")
	}
}

// TestKeyTagMgmtInputCapturesEveryKeyNoLeakInRenameMode is the Full-Capture-
// Disziplin guard, specifically for Rename mode (mirrors
// TestKeyTagMgmtInputCapturesEveryKeyNoLeak's own "create"-mode precedent
// one layer up -- this task's harness brief explicitly demands the SAME
// coverage for Rename).
func TestKeyTagMgmtInputCapturesEveryKeyNoLeakInRenameMode(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "leak-probe", defined: true, count: 1}}
	m.tagMgmtCursor.setLen(1)
	nm, _ := m.openTagMgmtInput("rename", "leak-probe")
	m = nm.(model)

	for _, r := range "d?K" { // bt-mx4k: `K` replaces the retired ctrl+k
		m = step(t, m, runeMsg(r))
	}

	if m.tagMgmtDeleteConfirm {
		t.Fatal("'d' while typing (rename mode) must not open the page-local Delete-Confirm")
	}
	if m.overlay != overlayNone {
		t.Fatalf("no overlay may open while typing, overlay = %v", m.overlay)
	}
	if m.helpOpen {
		t.Fatal("'?' while typing (rename mode) must not open Help")
	}
	if m.paletteOpen {
		t.Fatal("`K` while typing (rename mode) must not open the Command-Center")
	}
	if !m.tagMgmtInputActive || m.tagMgmtInputMode != "rename" {
		t.Fatal("want the rename submode to stay active and unchanged")
	}
}

// TestKeyTagMgmtInputRenameValidSubmitFiresBatchOfTwoCmds guards D13's core
// dispatch shape: a valid Rename submit fires EXACTLY TWO independent Cmds
// in ONE tea.Batch (Registry-save + Bean-sweep) -- proven by resolving the
// returned Cmd and asserting it decomposes into a tea.BatchMsg of length 2
// (no assertion against execution ORDER, D13's own "tea.Batch garantiert
// KEINE Reihenfolge" rationale, messages.go's tagRenameDoneMsg doc-stamp).
func TestKeyTagMgmtInputRenameValidSubmitFiresBatchOfTwoCmds(t *testing.T) {
	m := newModel(&data.Store{RepoDir: t.TempDir()}, "")
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "alpha", defined: true}}
	nm, _ := m.openTagMgmtInput("rename", "alpha")
	m = nm.(model)
	m.tagMgmtInput.SetValue("zulu")

	_, cmd := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("want a non-nil Cmd")
	}
	msg := cmd()
	bm, ok := msg.(tea.BatchMsg)
	if !ok || len(bm) != 2 {
		t.Fatalf("cmd() = %T (%v), want tea.BatchMsg of length 2", msg, msg)
	}
}

// TestKeyTagMgmtInputRenameRegistryPersistsIndependentlyOfSweepCmd is the
// bean's own explicitly-demanded "Registry-sofort-Semantik" test (D13:
// "Datei schon umbenannt BEVOR Sweep endet -- testbar über Cmd-Zerlegung"):
// executing ONLY the Registry-save half of the batch (never the sweep half)
// must already persist the rename to disk -- the two Cmds are genuinely
// independent, not a hidden sequential dependency.
func TestKeyTagMgmtInputRenameRegistryPersistsIndependentlyOfSweepCmd(t *testing.T) {
	dir := t.TempDir()
	client := &data.Store{RepoDir: dir}
	if err := client.SaveTagDefs([]string{"alpha", "bravo"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}
	m := newModel(client, dir)
	m.view = viewTagManagement
	m.tagMgmtRows = []tagRegistryRow{{name: "alpha", defined: true}, {name: "bravo", defined: true}}
	nm, _ := m.openTagMgmtInput("rename", "alpha")
	m = nm.(model)
	m.tagMgmtInput.SetValue("zulu")

	_, cmd := m.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	bm, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("want tea.BatchMsg, got %T", cmd())
	}

	var savedRan bool
	for _, c := range bm {
		if sm, ok := c().(tagDefsSavedMsg); ok {
			savedRan = true
			if sm.err != nil {
				t.Fatalf("tagDefsSavedMsg.err = %v, want nil", sm.err)
			}
			if sm.refindName != "zulu" {
				t.Fatalf("refindName = %q, want %q", sm.refindName, "zulu")
			}
		}
	}
	if !savedRan {
		t.Fatal("want the Registry-save Cmd present in the batch")
	}

	got, err := client.LoadTagDefs()
	if err != nil {
		t.Fatalf("LoadTagDefs: %v", err)
	}
	want := []string{"bravo", "zulu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry on disk = %v, want %v (renamed, independent of the sweep Cmd ever running)", got, want)
	}
}

// TestTagManagementLocalBindingsIncludesRenameTag mirrors
// TestTagManagementLocalBindingsIncludesDelete's own shape.
func TestTagManagementLocalBindingsIncludesRenameTag(t *testing.T) {
	bindings := tagManagementLocalBindings()
	found := false
	for _, b := range bindings {
		if b.Help().Key == keys.RenameTag.Help().Key {
			found = true
		}
	}
	if !found {
		t.Fatalf("tagManagementLocalBindings missing keys.RenameTag, got %+v", bindings)
	}
}

// --- renameTagCmd: the Bean-Sweep itself ---

// TestRenameTagCmdContinuesPastOneFailure is the bean's own named RED test
// (bt-y9my TDD section), adapted to this codebase's REAL test pattern for
// driving data.Store (a concrete struct with no interface seam, always
// exercised via a fake "beans" binary on PATH -- see fakeBeansConflict/
// fakeBeansEchoIfMatch, editor_test.go -- NOT a fakeSetTagsClient mock,
// which cannot satisfy renameTagCmd's *data.Store parameter type; the
// bean's own literal sketch has the same "test-sketch compiles against a
// fake type" issue T3/T4 already documented as ERRATA and fixed the same
// way). fakeBeansFailForIDLogCalls fails ONLY b2's call (a genuine CONFLICT)
// and additionally logs every ATTEMPTED id, so this test can prove b1 AND
// b3 were still attempted despite b2's failure -- not just infer it from
// the final count.
func TestRenameTagCmdContinuesPastOneFailure(t *testing.T) {
	store := newTestStore(t, taggedItem("b1", "old"), taggedItem("b2", "old"), taggedItem("b3", "old"))
	idx := data.NewIndex(store.Items())
	// b2 changes on disk after the index was built: its etag is stale, a
	// genuine conflict for exactly this one item.
	b2 := storeItem(t, store, "b2")
	if err := store.SetPriority("b2", "high", b2.ETag()); err != nil {
		t.Fatalf("setup: %v", err)
	}

	msg := renameTagCmd(store, idx, "old", "new")().(tagRenameDoneMsg)
	if msg.renamed != 2 || len(msg.failed) != 1 || msg.failed[0].id != "b2" {
		t.Fatalf("want 2 renamed, 1 failed(b2), got %+v", msg)
	}
	for _, id := range []string{"b1", "b3"} {
		if it := storeItem(t, store, id); !it.HasTag("new") || it.HasTag("old") {
			t.Fatalf("%s tags = %v, want old replaced by new (the sweep must not abort after b2's failure)", id, it.Tags)
		}
	}
	if it := storeItem(t, store, "b2"); !it.HasTag("old") || it.HasTag("new") {
		t.Fatalf("b2 tags = %v, want untouched (its rename conflicted)", it.Tags)
	}
}

// TestRenameTagCmdDispatchesTagAndRemoveTagFlagsPerBean guards the exact
// per-bean CLI call shape the harness brief demands: "je Bean EIN
// CLI-Aufruf --remove-tag old --tag new" -- fakeBeansEchoArgs
// (editor_test.go, existing precedent) always fails and echoes every
// argument to stderr, letting this test assert on the ACTUAL flags
// dispatched.
func TestRenameTagCmdDispatchesTagAndRemoveTagFlagsPerBean(t *testing.T) {
	store := newTestStore(t, taggedItem("solo", "old", "keep"))
	idx := data.NewIndex(store.Items())

	msg := renameTagCmd(store, idx, "old", "new")().(tagRenameDoneMsg)
	if msg.renamed != 1 || len(msg.failed) != 0 {
		t.Fatalf("want 1 renamed, got %+v", msg)
	}
	if it := storeItem(t, store, "solo"); !it.HasTag("new") || it.HasTag("old") || !it.HasTag("keep") {
		t.Fatalf("tags = %v, want old replaced by new, keep untouched", it.Tags)
	}
}

// TestRenameTagCmdNoBeansWithOldTagReturnsZeroRenamedNoFailures guards the
// empty-sweep floor -- no bean currently carries oldTag, so the sweep is a
// clean zero-value no-op, never a panic or a spurious CLI call.
func TestRenameTagCmdNoBeansWithOldTagReturnsZeroRenamedNoFailures(t *testing.T) {
	store := newTestStore(t, taggedItem("b1", "other"))
	idx := data.NewIndex(store.Items())

	msg := renameTagCmd(store, idx, "old", "new")().(tagRenameDoneMsg)
	if msg.renamed != 0 || len(msg.failed) != 0 {
		t.Fatalf("want a zero-value no-op sweep, got %+v", msg)
	}
}

// TestRenameTagCmdNilIndexNoPanicZeroRenamed mirrors collectTagCounts' own
// "if idx != nil" defensive guard (box_picker_tag.go) -- a pre-load/
// test-fixture model (m.idx unset) must never panic here.
func TestRenameTagCmdNilIndexNoPanicZeroRenamed(t *testing.T) {
	msg := renameTagCmd(newTestStore(t), nil, "old", "new")().(tagRenameDoneMsg)
	if msg.renamed != 0 || len(msg.failed) != 0 {
		t.Fatalf("want a zero-value sweep result for a nil idx, got %+v", msg)
	}
}

// TestRenameTagCmdSameOldNewNameSkipsSweepNoDestructiveSetTags guards the
// self-rename data-loss guard renameTagCmd's own doc-stamp explains
// (messages.go): a resubmitted, UNCHANGED name (D14's dedupe exclusion
// deliberately lets this happen) must NEVER call SetTags -- doing so would
// hit SetTags' own documented "same tag in both add and remove -> remove
// wins" resolver and silently STRIP the tag from every bean. The fake here
// FAILS EVERY call it receives, so any (bug-triggered) SetTags call at all
// would flip this test red.
func TestRenameTagCmdSameOldNewNameSkipsSweepNoDestructiveSetTags(t *testing.T) {
	store := newTestStore(t, taggedItem("b1", "same"), taggedItem("b2", "same"))
	idx := data.NewIndex(store.Items())

	msg := renameTagCmd(store, idx, "same", "same")().(tagRenameDoneMsg)
	if msg.renamed != 2 || len(msg.failed) != 0 {
		t.Fatalf("want renamed=2 failed=0 (self-rename no-op, no SetTags call ever fired), got %+v", msg)
	}
	for _, id := range []string{"b1", "b2"} {
		if !storeItem(t, store, id).HasTag("same") {
			t.Fatalf("%s lost its tag: a self-rename must never strip it", id)
		}
	}
}

// --- applyTagRenameDone ---

// TestApplyTagRenameDoneAlwaysReloads is the bean's own named RED test
// (bt-y9my TDD section, quoted verbatim).
func TestApplyTagRenameDoneAlwaysReloads(t *testing.T) {
	m := newModel(nil, "")
	_, cmd := m.applyTagRenameDone(tagRenameDoneMsg{oldTag: "old", newTag: "new", renamed: 1})
	if cmd == nil {
		t.Fatal("want a non-nil Cmd (toast + loadCmd batch)")
	}
}

// TestApplyTagRenameDoneSuccessTextAndToastKind guards the Toast's success
// shape: toastInfo (overlay_show_toast.go's own "Blue/Mauve —
// Hinweis/Erfolg" doc-stamp), title mentions old/new/count, no "failed"
// wording.
func TestApplyTagRenameDoneSuccessTextAndToastKind(t *testing.T) {
	m := newModel(nil, "")
	nm, _ := m.applyTagRenameDone(tagRenameDoneMsg{oldTag: "old", newTag: "new", renamed: 3})
	mm := nm.(model)
	if mm.toast == nil {
		t.Fatal("want a toast set")
	}
	if mm.toast.kind != toastInfo {
		t.Fatalf("toast.kind = %v, want toastInfo (success)", mm.toast.kind)
	}
	if !strings.Contains(mm.toast.title, "old") || !strings.Contains(mm.toast.title, "new") || !strings.Contains(mm.toast.title, "3") {
		t.Fatalf("toast.title = %q, want it to mention old/new/3", mm.toast.title)
	}
	if strings.Contains(mm.toast.title, "failed") {
		t.Fatalf("toast.title = %q, must not mention 'failed' on a clean success", mm.toast.title)
	}
}

// TestApplyTagRenameDoneFailureTextAndToastKindError guards the Toast's
// continue-on-error shape: toastError, title mentions the failure count AND
// the first error's text (D13's own "erste Fehlermeldung" wording).
func TestApplyTagRenameDoneFailureTextAndToastKindError(t *testing.T) {
	m := newModel(nil, "")
	nm, _ := m.applyTagRenameDone(tagRenameDoneMsg{
		oldTag: "old", newTag: "new", renamed: 2,
		failed: []tagRenameFailure{{id: "b2", err: errors.New("boom")}},
	})
	mm := nm.(model)
	if mm.toast == nil || mm.toast.kind != toastError {
		t.Fatalf("want an error-kind toast, got %+v", mm.toast)
	}
	if !strings.Contains(mm.toast.title, "1 failed") || !strings.Contains(mm.toast.title, "boom") {
		t.Fatalf("toast.title = %q, want it to mention '1 failed' and the first error text", mm.toast.title)
	}
}

// TestUpdateDispatchesTagRenameDoneMsgToApplyTagRenameDone guards the
// central Update() switch wiring (mirrors how tagDefsSavedMsg/createDoneMsg
// are already wired there).
func TestUpdateDispatchesTagRenameDoneMsgToApplyTagRenameDone(t *testing.T) {
	m := newModel(nil, "")
	tm, cmd := m.Update(tagRenameDoneMsg{oldTag: "old", newTag: "new", renamed: 1})
	mm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update did not return a model, got %T", tm)
	}
	if mm.toast == nil {
		t.Fatal("want Update(tagRenameDoneMsg) to route through applyTagRenameDone (toast set)")
	}
	if cmd == nil {
		t.Fatal("want a non-nil Cmd")
	}
}

// --- Full end-to-end Rename flow (real *data.Store) ---

// TestFullRenameFlowRenamesRegistryAndSweepsBeansViaRealClient drives the
// COMPLETE Rename flow (open -> type -> submit -> resolve both halves of
// the batch) against a real *data.Store (temp-dir Registry AND real items
// for the Bean-sweep) -- proves the whole D13 chain end-to-end, not just its isolated
// pieces.
func TestFullRenameFlowRenamesRegistryAndSweepsBeansViaRealClient(t *testing.T) {
	client := newTestStore(t, taggedItem("b1", "alpha"), taggedItem("b2", "alpha"))
	dir := client.RepoDir
	if err := client.SaveTagDefs([]string{"alpha", "bravo"}); err != nil {
		t.Fatalf("setup SaveTagDefs: %v", err)
	}

	idx := data.NewIndex(client.Items())
	m := newModel(client, dir)
	m.idx = idx
	m.view = viewTagManagement
	m.tagMgmtRows = tagRegistryRows(idx, []string{"alpha", "bravo"})
	m.tagMgmtCursor.setLen(len(m.tagMgmtRows))
	// "alpha" sorts first in the Defined (alpha-sorted) group -- cursor
	// already points at it.

	nm, _ := m.keyTagManagement(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	mm := nm.(model)
	if !mm.tagMgmtInputActive || mm.tagMgmtInputMode != "rename" || mm.tagMgmtInput.Value() != "alpha" {
		t.Fatalf("want rename input open prefilled 'alpha', got active=%v mode=%q value=%q",
			mm.tagMgmtInputActive, mm.tagMgmtInputMode, mm.tagMgmtInput.Value())
	}

	mm.tagMgmtInput.SetValue("renamed-alpha")
	_, cmd := mm.keyTagMgmtInput(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("want a non-nil Cmd on valid rename submit")
	}

	bm, ok := cmd().(tea.BatchMsg)
	if !ok || len(bm) != 2 {
		t.Fatalf("want tea.BatchMsg of length 2, got %T", cmd())
	}

	var sawSaved, sawRenamed bool
	for _, c := range bm {
		switch rm := c().(type) {
		case tagDefsSavedMsg:
			sawSaved = true
			if rm.err != nil {
				t.Fatalf("tagDefsSavedMsg.err = %v, want nil", rm.err)
			}
			if rm.refindName != "renamed-alpha" {
				t.Fatalf("refindName = %q, want %q", rm.refindName, "renamed-alpha")
			}
		case tagRenameDoneMsg:
			sawRenamed = true
			if rm.oldTag != "alpha" || rm.newTag != "renamed-alpha" || rm.renamed != 2 || len(rm.failed) != 0 {
				t.Fatalf("tagRenameDoneMsg = %+v, want oldTag=alpha newTag=renamed-alpha renamed=2 failed=0", rm)
			}
		default:
			t.Fatalf("unexpected batch msg type %T", rm)
		}
	}
	if !sawSaved || !sawRenamed {
		t.Fatalf("want both tagDefsSavedMsg and tagRenameDoneMsg in the batch, sawSaved=%v sawRenamed=%v", sawSaved, sawRenamed)
	}

	got, err := client.LoadTagDefs()
	if err != nil {
		t.Fatalf("LoadTagDefs after rename: %v", err)
	}
	want := []string{"bravo", "renamed-alpha"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registry after rename = %v, want %v", got, want)
	}
}
