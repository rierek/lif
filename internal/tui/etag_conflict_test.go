package tui

// etag_conflict_test.go — the ETag-Konflikt-Regression-Sweep (E3 Task 6,
// bean bt-ppzb): a stale-etag CONFLICT simulated for EVERY mutation site
// built across T1-T5 (Status/Type/Priority value menu, Tag-Picker, Parent-
// Picker Set/RemoveParent, Blocking-Picker, Title-Edit-Form, whole-bean
// $EDITOR/UpdateWhole -- D01, bean bt-z4b1, updated the last site's own
// dispatch from a Body-only SetBody to a combined UpdateWhole call),
// asserting the ONE shared contract every site funnels through
// (applyMutationResult, update.go, design decision d): a data.ErrConflict-
// classified mutationDoneMsg -> a status line containing "Conflict", no
// panic, no silent drop. applyMutationResult's OWN Konflikt-branch already
// has direct unit coverage (TestApplyMutationResultConflictSetsStatusLineAnd
// Reloads, box_menu_value_test.go) -- this sweep's value is proving each
// SITE's own dispatch (its Client method + args) actually reaches that one
// shared tail end-to-end, via the SAME fake-"beans"-binary technique
// editor_test.go's F2 tests already established (fakeBeansConflict/
// fakeBeansEchoIfMatch, editor_test.go, same package).
//
// Create needs no subtest here (no etag at all, a fresh bean has none to go
// stale) and neither does Delete (mutations.go Delete's own doc-stamp: the
// beans CLI's `delete` subcommand takes no --if-match at all) -- both are
// noted in the epic plan's own Sites list for exactly this reason.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

// assertConflictSweep is the ONE shared assertion every subtest below drives
// its site to: cmd must be non-nil, its resolved Msg must be a
// mutationDoneMsg wrapping data.ErrConflict, and running it back through
// Update (step) must land a "Conflict" note in the status line.
func assertConflictSweep(t *testing.T, tm tea.Model, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a mutation Cmd, got nil")
	}
	m, ok := tm.(model)
	if !ok {
		t.Fatalf("expected a model, got %T", tm)
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if !errors.Is(mdm.err, data.ErrConflict) {
		t.Fatalf("mutationDoneMsg.err = %v, want errors.Is(_, data.ErrConflict)", mdm.err)
	}
	nm := step(t, m, mdm)
	if !strings.Contains(nm.err, "Conflict") {
		t.Fatalf("status line = %q, want it to mention the conflict", nm.err)
	}
}

// TestEtagConflictSweep simulates a stale-etag conflict for EVERY mutation
// site built in T1-T5 and asserts the ONE shared contract (design decision
// d): mutationDoneMsg{ErrConflict} -> status line contains "Conflict",
// reload cmd fired, no panic, overlay/form state sane (each subtest's own
// setup already closes its overlay/form as part of firing the mutation --
// see e.g. applyValueMenuSelection/applyTagPickerDiff/keyDeleteConfirm's own
// `m.overlay = overlayNone` before returning the Cmd).
func TestEtagConflictSweep(t *testing.T) {
	t.Run("value-menu status", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeans())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBean(m, "tk-2")
		m = step(t, m, runeMsg('s'))
		if m.overlay != overlayValueMenu {
			t.Fatal("setup: s did not open the value menu")
		}
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("value-menu type", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeans())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBean(m, "tk-2")
		m = step(t, m, runeMsg('s'))
		m.menu.cursor = valueMenuCursorFor(m.menuItems, "type", "bug")
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("value-menu priority", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeans())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBean(m, "tk-2")
		m = step(t, m, runeMsg('s'))
		m.menu.cursor = valueMenuCursorFor(m.menuItems, "priority", "high")
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("tag-picker SetTags", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeansTagged())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBean(m, "tk-2")
		m = step(t, m, runeMsg('t'))
		if m.overlay != overlayTagPicker {
			t.Fatal("setup: t did not open the tag picker")
		}
		m = tagPickerCursorTo(t, m, "backend")
		m = step(t, m, runeMsg(' ')) // toggle -> a pending diff exists, enter must fire
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("parent-picker SetParent", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeansForParentPicker())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBeanFull(m, "ep-1")
		m = step(t, m, runeMsg('a'))
		for i, it := range m.parentItems {
			if it.id == "ms-1" {
				m.menu.cursor = i
			}
		}
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("parent-picker RemoveParent", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeansForParentPicker())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBeanFull(m, "ep-1")
		m = step(t, m, runeMsg('a'))
		m.menu.cursor = 0 // "(No parent)" clear row
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("blocking-picker SetBlocking", func(t *testing.T) {
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeansWithBlocking())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m = focusBeanFull(m, "bean-a")
		m = step(t, m, runeMsg('r'))
		if m.overlay != overlayBlockingPicker {
			t.Fatal("setup: r did not open the blocking picker")
		}
		for i, it := range m.blockItems {
			if it.id == "ep-1" {
				m.menu.cursor = i
			}
		}
		m = step(t, m, runeMsg(' ')) // toggle -> a pending diff exists, enter must fire
		tm, cmd := m.Update(keyMsg(tea.KeyEnter))
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("edit-title SetTitle", func(t *testing.T) {
		// Uses the FORM-level chase (driveForm on a bare *huh.Form), not the
		// model-level keystroke drive (driveModel/typeIntoModel) -- the same
		// deliberate choice form_edit_title_test.go's own
		// TestEditTitleSubmitFiresSetTitleDirectlyNoConfirm already makes:
		// driveModel's real Update round-trips fire huh's self-perpetuating
		// blink-tick Cmds (the exact cost box_confirm_create_test.go's
		// 7-field Create-Form drive pays, ~16-19s each, see this task's
		// testing.Short() guards there) -- a single-field settle via
		// driveForm avoids reintroducing that cost for a sweep subtest that
		// only needs submitForm's OWN dispatch, not a full keystroke replay.
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeans())
		m.client = &data.Client{RepoDir: t.TempDir()}
		f := buildEditTitleForm("Task Two")
		f = driveForm(f, enterMsg())
		m.form = f
		m.formKind = "editTitle"
		m.mutTarget = "tk-2"
		tm, cmd := m.submitForm()
		assertConflictSweep(t, tm, cmd)
	})

	t.Run("editor UpdateWhole", func(t *testing.T) {
		// KORRIGIERT (Review-Runde 2, F2, bean bt-sl45; UPDATED for D01, bean
		// bt-z4b1, design-spec.md §15 PF-17): this site's etag (AND full
		// snapshot, D01's extension) is frozen at $EDITOR-open
		// (m.editorETag/m.editorSnapshot), never re-read fresh at submit
		// (applyEditorFinished's own doc-stamp, update.go) -- the specific
		// "freeze, don't re-read" nuance already has its OWN dedicated
		// regression test
		// (TestEditorFinishedUsesEtagCapturedAtOpenNotFreshIndexRead,
		// editor_test.go); this subtest only proves the SAME shared conflict
		// contract (Konflikt statusline + reload) every other site in this
		// sweep is checked against, driving applyEditorFinished directly via
		// editorFinishedMsg with a VALID raw-bean content (parseRawBean must
		// succeed for this to reach UpdateWhole rather than the parse-error
		// recovery path) -- mirrors
		// TestEditorFinishedConflictWritesRecoveryTempFileAndSurfacesPath's
		// own setup shape, editor_test.go).
		fakeBeansConflict(t)
		m := fixtureModel(t, fixtureBeans())
		m.client = &data.Client{RepoDir: t.TempDir()}
		m.editorTarget = "tk-2"
		m.editorETag = "captured-at-open-etag"
		m.editorSnapshot = &data.Bean{ID: "tk-2", Title: "Task Two", Status: "todo", Type: "task", Priority: "normal"}

		fm := rawBeanFrontmatter{Title: "Task Two Edited", Status: "todo", Type: "task", Priority: "normal"}
		raw := rawEditorText(t, fm, "edited body")

		tm, cmd := m.Update(editorFinishedMsg{content: raw, changed: true})
		assertConflictSweep(t, tm, cmd)
	})
}

// TestPlainConflictSetsNonEmptyToastCtx guards bt-0xrb (D04, Plan-
// Konkretisierung E13 Item 1, Vorgehen #4): applyMutationResult's
// plain-ErrConflict branch (no *conflictWithRecovery match -- the ordinary
// value-menu/picker conflict path, NOT the Editor-recovery path) must
// pre-fill toastCtx from err.Error() instead of leaving it "" -- err already
// carries the bean-ID + beans-CLI detail (internal/data/mutations.go:63/75,
// classifyError) that used to be silently dropped, leaving the PO with no
// handlungsleitende detail beyond the generic "Conflict: bean changed
// externally" title.
func TestPlainConflictSetsNonEmptyToastCtx(t *testing.T) {
	fakeBeansConflict(t)
	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: t.TempDir()}
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('s'))
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: s did not open the value menu")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("setup: expected a mutation Cmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if !errors.Is(mdm.err, data.ErrConflict) {
		t.Fatalf("setup: mutationDoneMsg.err = %v, want a data.ErrConflict-classified error", mdm.err)
	}

	nm2 := step(t, nm, mdm)
	if nm2.toast == nil {
		t.Fatal("expected a toast after a plain ErrConflict mutation result")
	}
	if nm2.toast.context == "" {
		t.Fatal("toast.context is empty, want it pre-filled from err.Error() (bean-ID + CLI detail, D04)")
	}
	if !strings.Contains(nm2.toast.context, "tk-2") {
		t.Errorf("toast.context = %q, want it to mention the conflicting bean ID (tk-2), mirroring err.Error()", nm2.toast.context)
	}
}

// TestConflictAfterWatchReloadUsesFreshETagNoConflict is the positive
// counter-probe to design decision d: a watch-reload landing BETWEEN an
// overlay's open and its enter must dispatch the FRESH (reloaded) etag, not
// the stale open-time one -- so a reload itself never manufactures a false
// conflict, only a genuine Submit<->Disk race does. Does NOT apply to the
// editor UpdateWhole site (m.editorETag is deliberately FROZEN at open, F2
// Review-Runde 2 -- see editor_test.go's own
// TestEditorFinishedUsesEtagCapturedAtOpenNotFreshIndexRead, which proves the
// OPPOSITE direction for that one site on purpose). Verified the same way
// that test proves the editor path's frozen etag: a fake "beans" binary that
// echoes the ACTUAL --if-match value it received into its (still-failing,
// for observability only) error text.
func TestConflictAfterWatchReloadUsesFreshETagNoConflict(t *testing.T) {
	fakeBeansEchoIfMatch(t)

	beans := fixtureBeans()
	for i := range beans {
		if beans[i].ID == "tk-2" {
			beans[i].ETag = "etag-at-open"
		}
	}
	m := fixtureModel(t, beans)
	m.client = &data.Client{RepoDir: t.TempDir()}
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('s')) // opens the value menu, mutTarget=tk-2, NO etag captured (design d)
	if m.overlay != overlayValueMenu {
		t.Fatal("setup: s did not open the value menu")
	}

	reloaded := fixtureBeans()
	for i := range reloaded {
		if reloaded[i].ID == "tk-2" {
			reloaded[i].ETag = "etag-after-reload"
		}
	}
	m = step(t, m, beansLoadedMsg{beans: reloaded}) // watch-reload lands while the menu is still open

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	if _, ok := tm.(model); !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("enter must fire a mutation Cmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err == nil {
		t.Fatal("setup: the fake lif script always exits 1")
	}
	if !strings.Contains(mdm.err.Error(), "received-if-match=etag-after-reload") {
		t.Fatalf("dispatched --if-match: %v, want it to contain the FRESH (reloaded) etag %q", mdm.err, "etag-after-reload")
	}
	if strings.Contains(mdm.err.Error(), "etag-at-open") {
		t.Fatalf("dispatched the STALE (open-time) etag: %v, want the fresh one (design decision d)", mdm.err)
	}
}
