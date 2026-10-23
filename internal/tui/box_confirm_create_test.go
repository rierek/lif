package tui

// box_confirm_create_test.go — TDD coverage for the Create-Form's
// Confirm-Gate (E3 Task 4, bean bt-y4ly, design decision e): submitForm's
// park-and-open, enter-fires/n-esc-returns-filled Confirm-Gate semantics
// (Draft-Erhalt, Port DD2-190), and createDoneMsg's cursor-jump +
// applyMutationResult fallback (update.go).

import (
	"fmt"
	"github.com/rierek/lif/pkg/item"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// skipSlowHuhDriveInShortMode guards the handful of tests in this file that
// drive the 7-field Create-Form through openFilledCreateConfirm/typeIntoModel
// -- a real model-level tea.Update round-trip per keystroke/field-advance,
// which repeatedly re-arms huh's own self-perpetuating textinput-blink Cmd
// chain (driveModelBudget iterations each). Measured cost (E3 Task 6, bean
// bt-ppzb, "Kosten-Finding"): ~16-19s PER TEST, seven of them, ~119s of the
// internal/tui package's ~121s total -- by far the most expensive tests in
// the whole suite, an order of magnitude above anything else. `go test
// ./... -short` skips exactly these seven (and no others -- every other test
// in this file, and everywhere else, is already sub-second) for a fast local
// loop; the full (non -short) run still exercises them, and CI/pre-commit
// must keep running without -short. See CLAUDE.md's "schneller Lauf" note.
func skipSlowHuhDriveInShortMode(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping slow huh-drive Create-Form test in -short mode (~16-19s, see skipSlowHuhDriveInShortMode doc comment)")
	}
}

// driveModel is the model-level analogue of driveForm
// (form_create_bean_test.go): step() (update_test.go) discards every
// returned tea.Cmd, so it can never drive an open huh Form to
// huh.StateCompleted on its own -- keyForm/updateForm's returned Cmd is the
// SAME unresolved field-advance chain (see driveForm's doc comment for the
// full rationale, including the bounded driveFormBudget against huh's
// self-perpetuating textinput-blink timer).
func driveModel(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	nm, _ := driveModelStep(t, m, msg, driveFormBudget)
	return nm
}

func driveModelStep(t *testing.T, m model, msg tea.Msg, budget int) (model, int) {
	t.Helper()
	if budget <= 0 {
		return m, 0
	}
	budget--
	tm, cmd := m.Update(msg)
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(%T) did not return a model, got %T", msg, tm)
	}
	return driveModelCmd(t, nm, cmd, budget)
}

func driveModelCmd(t *testing.T, m model, cmd tea.Cmd, budget int) (model, int) {
	t.Helper()
	if cmd == nil || budget <= 0 {
		return m, budget
	}
	budget--
	msg := cmd()
	if msg == nil {
		return m, budget
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if budget <= 0 {
				break
			}
			m, budget = driveModelCmd(t, m, c, budget)
		}
		return m, budget
	}
	return driveModelStep(t, m, msg, budget)
}

// typeIntoModel sends s as a sequence of single-rune KeyMsgs -- mirrors a PO
// actually typing text into the focused field, consistent with this suite's
// own single-rune runeMsg convention (update_test.go).
func typeIntoModel(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = driveModel(t, m, runeMsg(r))
	}
	return m
}

// advanceFieldsModel drives m forward n fields (n calls to enterMsg) -- the
// model-level counterpart of advanceFields (form_create_bean_test.go).
func advanceFieldsModel(t *testing.T, m model, n int) model {
	t.Helper()
	for i := 0; i < n; i++ {
		m = driveModel(t, m, enterMsg())
	}
	return m
}

// openFilledCreateConfirm drives a fresh Create-Form (`c`, optionally
// focused on parentID first for the Parent-Prefill) all the way through its
// 7 keyed fields (title/type/priority/status/parent/tags/body) to a filled
// Confirm-Gate -- shared setup for the Confirm-Gate tests below.
func openFilledCreateConfirm(t *testing.T, m model, parentID, title string) model {
	t.Helper()
	if parentID != "" {
		m = focusBean(m, parentID)
	}
	m = step(t, m, runeMsg('c'))
	if m.form == nil || m.formKind != "create" {
		t.Fatalf("setup: c did not open the create form (form=%v formKind=%q)", m.form, m.formKind)
	}
	m = typeIntoModel(t, m, title)
	m = advanceFieldsModel(t, m, 7) // title, type, priority, status, parent, tags, body -> submit
	if m.overlay != overlayCreateConfirm {
		t.Fatalf("setup: overlay = %v, want overlayCreateConfirm (form=%v)", m.overlay, m.form)
	}
	return m
}

// TestSubmitFormParksCmdAndOpensConfirm guards submitForm's own park-and-open
// contract (box_confirm_create.go): completing the create form nulls it,
// opens overlayCreateConfirm, parks a non-nil createCmd, and captures the
// draft (DD2-190) BEFORE the confirm.
func TestSubmitFormParksCmdAndOpensConfirm(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	if m.form != nil {
		t.Fatalf("form still open after the last field, want nil (submitForm must null it)")
	}
	if m.pendingCreate == nil {
		t.Fatal("pendingCreate is nil, want the parked createCmd")
	}
	if !strings.Contains(m.createLabel, "New Task") {
		t.Fatalf("createLabel = %q, want it to mention the title", m.createLabel)
	}
	if m.createDraft == nil || m.createDraft.title != "New Task" {
		t.Fatalf("createDraft = %+v, want title %q captured (DD2-190)", m.createDraft, "New Task")
	}
	if m.createDraft.parent != "ep-1" {
		t.Fatalf("createDraft.parent = %q, want ep-1 (Parent-Prefill survived to submit)", m.createDraft.parent)
	}
}

// TestCreateConfirmEnterFiresPendingCmd guards the enter branch of
// keyCreateConfirm: closes the overlay and returns the createCmd that
// actually dispatches `beans create` (verified via a real data.Store
// pointed at a nonexistent repo dir -- same no-binary-required pattern
// box_picker_parent_test.go/box_picker_tag_test.go already use).
//
// F1 (Review-Runde 2, Async-Gap-Clobbering): pendingCreate is deliberately
// NOT cleared here anymore (a behavior change vs. the original T4 version of
// this test) -- it now doubles as the in-flight guard for the WHOLE async
// gap between this enter and createDoneMsg arriving (types.go doc-stamp),
// so keyNodeAction's Create case can refuse a second `c` during that window
// (TestKeyNodeActionCIgnoredWhileCreateInFlight below). Only applyCreateDone
// (update.go) clears it, once createDoneMsg actually resolves.
func TestCreateConfirmEnterFiresPendingCmd(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	store := fixtureStore(t)
	m.store = store
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
	}
	if nm.pendingCreate == nil {
		t.Fatal("pendingCreate must stay non-nil after enter (F1 in-flight guard) -- only createDoneMsg clears it")
	}
	// B01 (E3-T4-Review PFLICHT, closed in T5, bean bt-sl45): createDraft
	// must SURVIVE the Confirm-Gate's own enter now -- only createDoneMsg
	// (applyCreateDone, update.go) resolves it, on either outcome (see
	// TestCreateConfirmEnterErrorPreservesDraftAndReopensForm/
	// TestCreateDoneSuccessConsumesDraft below for both halves).
	if nm.createDraft == nil {
		t.Fatal("createDraft must survive the Confirm-Gate's enter (B01 fix) -- nulled only once createDoneMsg resolves")
	}
	if cmd == nil {
		t.Fatal("enter must fire the parked createCmd")
	}
	msg := cmd()
	cdm, ok := msg.(createDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want createDoneMsg", msg)
	}
	if cdm.err != nil || cdm.bean == nil {
		t.Fatalf("createDoneMsg = %+v, want a created item (proves Create dispatched)", cdm)
	}
	if got := storeItem(t, store, cdm.bean.ID); got.Title != "New Task" || got.Parent != "ep-1" {
		t.Fatalf("created item on disk = %+v, want title New Task under ep-1", got)
	}

	// F1: once createDoneMsg resolves (error path here), pendingCreate must
	// finally clear -- the in-flight window is over.
	fm := step(t, nm, cdm)
	if fm.pendingCreate != nil {
		t.Fatal("pendingCreate must be cleared once createDoneMsg resolves")
	}
}

// TestCreateConfirmEnterErrorPreservesDraftAndReopensForm guards B01 (E3-T4-
// Review PFLICHT, Important, closed in T5, bean bt-sl45): a CLI-rejected
// create (e.g. VALIDATION_ERROR) must not lose the PO's filled-in draft --
// createDraft survives the Confirm-Gate's own enter (keyCreateConfirm no
// longer nulls it), so applyCreateDone's error branch reopens the FILLED
// Create-Form from it instead of routing through the draft-agnostic
// applyMutationResult tail (status line + reload, which would just discard
// the work).
func TestCreateConfirmEnterErrorPreservesDraftAndReopensForm(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m.store = newTestStore(t)
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if cmd == nil {
		t.Fatal("enter must fire the parked createCmd")
	}
	msg := cmd()
	cdm, ok := msg.(createDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want createDoneMsg", msg)
	}
	if cdm.err == nil {
		t.Fatal("setup: expected the bogus RepoDir to fail the create")
	}

	tm2, _ := nm.Update(cdm)
	fm, ok := tm2.(model)
	if !ok {
		t.Fatalf("Update(createDoneMsg) did not return a model, got %T", tm2)
	}
	if fm.form == nil {
		t.Fatal("a CLI-rejected create must reopen the Create-Form (B01, draft-loss fix)")
	}
	if fm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (Confirm-Gate must not linger)", fm.overlay)
	}
	if fm.err == "" {
		t.Fatal("the rejected create's error must still surface in the status line")
	}
	if fm.createDraft != nil {
		t.Fatal("createDraft must be consumed (nil) once reopened into the form, same as the esc/n path")
	}
	// B01 (E5-T1-Review Prelude, PFLICHT): the reopen-branch is the ONE
	// mutation-adjacent error path E5 Task 1's dual-write audit missed (it
	// enumerated update.go's OTHER 8 showToast sites but not this one,
	// since applyCreateDone's reopen branch returns through
	// openCreateFormWithDraft rather than applyMutationResult's shared
	// tail) -- same Kind/sticky convention as every other hard-error site
	// (toastError, non-sticky, title mirrors m.err).
	if fm.toast == nil || fm.toast.kind != toastError || fm.toast.title != fm.err {
		t.Fatalf("toast = %+v, want a non-nil toastError mirroring m.err %q (B01: reopen-branch dual-write)", fm.toast, fm.err)
	}

	fm = advanceFieldsModel(t, fm, 7)
	if !strings.Contains(fm.createLabel, "New Task") {
		t.Fatalf("createLabel after reopen+resubmit = %q, want it to still mention %q (B01: draft survived a REJECTED create too)", fm.createLabel, "New Task")
	}
}

// TestCreateDoneErrorWhileBusyRoutesToStatusLineFormUntouched guards F1
// (Review-Runde 2, Async-Gap-Clobbering, Finding 1): between the Confirm-
// Gate's enter (overlay -> None, form -> nil) and createDoneMsg actually
// arriving, overlay/form are back to their "nothing open" state -- the PO
// can open something ELSE in that window (here: the Status/Type/Priority
// value menu via `s` on a DIFFERENT bean than the one being created). A
// createDoneMsg carrying an error must NOT clobber whatever the PO is now
// doing by unconditionally reopening the stale Create-Form on top of it
// (applyCreateDone's OLD behavior) -- it must route through the draft-
// agnostic applyMutationResult tail instead (status line + reload), leaving
// the currently open overlay exactly as the PO left it.
func TestCreateDoneErrorWhileBusyRoutesToStatusLineFormUntouched(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m.store = newTestStore(t)
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	tm, cmd := m.Update(keyMsg(tea.KeyEnter)) // fires the createCmd, now in flight
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.pendingCreate == nil {
		t.Fatal("setup: pendingCreate must stay non-nil while the create is in flight (F1)")
	}

	// PO opens something else during the async gap.
	nm = focusBean(nm, "tk-2")
	nm = step(t, nm, runeMsg('s'))
	if nm.overlay != overlayValueMenu {
		t.Fatalf("setup: overlay = %v, want overlayValueMenu ('s' must open it)", nm.overlay)
	}

	msg := cmd()
	cdm, ok := msg.(createDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want createDoneMsg", msg)
	}
	if cdm.err == nil {
		t.Fatal("setup: expected the bogus RepoDir to fail the create")
	}

	tm2, _ := nm.Update(cdm)
	fm, ok := tm2.(model)
	if !ok {
		t.Fatalf("Update(createDoneMsg) did not return a model, got %T", tm2)
	}
	if fm.form != nil {
		t.Fatal("a busy PO's state must not be clobbered by a Create-Form reopen (F1)")
	}
	if fm.overlay != overlayValueMenu {
		t.Fatalf("overlay = %v, want overlayValueMenu (untouched by the failed create)", fm.overlay)
	}
	if fm.err == "" {
		t.Fatal("the failed create's error must still surface in the status line")
	}
	if fm.createDraft != nil {
		t.Fatal("createDraft must be dropped once busy (F1: PO abandoned the failed create by starting something new)")
	}
	if fm.pendingCreate != nil {
		t.Fatal("pendingCreate must be cleared once createDoneMsg resolves, in flight or not")
	}
}

// TestKeyNodeActionCIgnoredWhileCreateInFlight guards F1's in-flight guard
// (Finding 1b): pressing `c` again while an earlier create is still in
// flight (the same async gap as above, m.pendingCreate != nil) must NOT open
// a second Create-Form -- cross-contaminating the single createDraft/
// pendingCreate slots with a second create's state is exactly the
// clobbering F1 closes.
func TestKeyNodeActionCIgnoredWhileCreateInFlight(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m.store = newTestStore(t)
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.pendingCreate == nil {
		t.Fatal("setup: pendingCreate must stay non-nil while the create is in flight")
	}
	_ = cmd // the in-flight createCmd itself, not exercised by this test

	nm2 := step(t, nm, runeMsg('c'))
	if nm2.form != nil {
		t.Fatal("c while a create is in flight must not open a second Create-Form")
	}
	if nm2.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (c while busy must not open anything either)", nm2.overlay)
	}
	if nm2.err == "" {
		t.Fatal("c while a create is in flight should surface a brief status note")
	}
	if nm2.pendingCreate == nil {
		t.Fatal("pendingCreate must remain set -- the original in-flight create must not be forgotten")
	}
}

// TestSubmitFormCreateIgnoredWhilePendingCreateInFlight guards submitForm's
// OWN copy of the F1 in-flight guard (Finding 1b) -- a second layer of the
// SAME single-create invariant keyNodeAction's Create case already enforces
// above. Under normal UI flow keyNodeAction's guard prevents a second
// Create-Form from ever opening in the first place, so this test drives the
// scenario directly (form already open, pendingCreate set out-of-band) to
// exercise submitForm's own check rather than relying on it staying
// unreachable.
func TestSubmitFormCreateIgnoredWhilePendingCreateInFlight(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('c'))
	if m.form == nil || m.formKind != "create" {
		t.Fatalf("setup: c did not open the create form (form=%v formKind=%q)", m.form, m.formKind)
	}
	m.pendingCreate = func() tea.Msg { return nil } // simulate an earlier create already in flight

	m = typeIntoModel(t, m, "New Task")
	m = advanceFieldsModel(t, m, 7) // title, type, priority, status, parent, tags, body -> submit

	if m.form != nil {
		t.Fatal("submitForm must discard the second form while a create is in flight")
	}
	if m.overlay == overlayCreateConfirm {
		t.Fatal("submitForm must NOT open a second Confirm-Gate while a create is in flight")
	}
	if m.err == "" {
		t.Fatal("submitForm should surface a brief status note when it drops the second create")
	}
	// bt-81f0: this guard's m.err (box_confirm_create.go's OWN copy, distinct
	// from keyNodeAction's already-Toast-backed guard, update.go:747) lost
	// its rendering -- must not go silent. kind=toastWarn (bt-tm4a): the
	// in-flight guard is a hint ("please wait"), not a hard error -- all
	// three createInFlightNote guards now agree on toastWarn, matching
	// keyNodeAction's original choice (update.go:747).
	if m.toast == nil {
		t.Fatal("submitForm dropping the second create must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if m.toast.kind != toastWarn {
		t.Errorf("toast.kind = %v, want toastWarn (bt-tm4a: unified in-flight-guard severity)", m.toast.kind)
	}
}

// TestCreateDoneSuccessConsumesDraft guards B01's other half (mirrors
// TestCreateConfirmEnterErrorPreservesDraftAndReopensForm's failure-path
// counterpart above): a SUCCESSFUL create clears createDraft -- no stale
// draft lingers for some later, unrelated reopen to accidentally pick up.
func TestCreateDoneSuccessConsumesDraft(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	d := beanDraft{title: "New Task"}
	m.createDraft = &d
	newBean := &item.Item{ID: "tk-9", Title: "New Task", Type: "task", Status: "todo", Priority: "normal"}

	nm := step(t, m, createDoneMsg{bean: newBean})
	if nm.createDraft != nil {
		t.Fatal("a successful create must consume (nil) the draft")
	}
}

// TestCreateConfirmEscReturnsToFilledForm guards the n/esc branch
// (Draft-Erhalt, Port DD2-190): the Confirm-Gate closes WITHOUT firing a
// Cmd, and the Create-Form reopens carrying the original draft -- proven by
// advancing straight through the reopened form (no retyping) and checking
// the resulting createLabel still names the original title.
func TestCreateConfirmEscReturnsToFilledForm(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	m := fixtureModel(t, fixtureBeans())
	m = openFilledCreateConfirm(t, m, "ep-1", "New Task")

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(esc) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after esc = %v, want overlayNone", nm.overlay)
	}
	if nm.form == nil {
		t.Fatal("esc did not reopen the Create-Form (Draft-Erhalt, DD2-190)")
	}
	// cmd is the reopened form's own m.form.Init() (huh's group/field
	// housekeeping, e.g. WindowSize/cursor setup) -- NOT a create dispatch;
	// pendingCreate/createLabel being cleared (below) is what proves no
	// creation actually fired.
	_ = cmd
	if nm.createDraft != nil {
		t.Fatal("createDraft must be consumed (nil) once reopened into the form")
	}
	if nm.pendingCreate != nil {
		t.Fatal("pendingCreate must be cleared, esc/n must not leave a stale create dispatch parked")
	}

	nm = advanceFieldsModel(t, nm, 7)
	if !strings.Contains(nm.createLabel, "New Task") {
		t.Fatalf("createLabel after reopen+resubmit = %q, want it to still mention %q (draft survived)", nm.createLabel, "New Task")
	}
}

// TestCreateDoneJumpsCursorAndExpandsParentPath guards applyCreateDone's
// success path (update.go): the new bean's immediate parent AND its own
// ancestors get expanded, the cursor jumps to the new bean's ID BEFORE the
// reload fires, and applyLoaded's existing "exact bean still present" path
// keeps the cursor there once the reload (a manually-fed beansLoadedMsg,
// mirroring fixtureModel's own convention) arrives.
func TestCreateDoneJumpsCursorAndExpandsParentPath(t *testing.T) {
	m := fixtureModel(t, fixtureBeans()) // ms-1 -> ep-1 -> tk-1, tk-2
	newBean := &item.Item{ID: "tk-9", Title: "New Task", Type: "task", Status: "todo", Priority: "normal", Parent: "ep-1"}

	tm, cmd := m.Update(createDoneMsg{bean: newBean})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(createDoneMsg) did not return a model, got %T", tm)
	}
	if !nm.expanded["ep-1"] {
		t.Fatalf("expanded[ep-1] = %v, want true (new bean's immediate parent must be expanded)", nm.expanded)
	}
	if !nm.expanded["ms-1"] {
		t.Fatalf("expanded[ms-1] = %v, want true (ep-1's own ancestor must be expanded too)", nm.expanded)
	}
	if nm.cursorID != "tk-9" {
		t.Fatalf("cursorID = %q, want tk-9 (jump to the new bean)", nm.cursorID)
	}
	if cmd == nil {
		t.Fatal("createDoneMsg (success) must trigger a reload Cmd")
	}

	nm = step(t, nm, beansLoadedMsg{beans: append(fixtureBeans(), newBean)})
	if nm.cursorID != "tk-9" {
		t.Fatalf("cursorID after reload = %q, want tk-9 (applyLoaded's exact-bean-present path)", nm.cursorID)
	}
}

// TestCreateDoneErrRoutesToApplyMutationResult guards the failure path: a
// createDoneMsg carrying an error must NOT touch the cursor -- it routes
// straight through the shared applyMutationResult tail (status line +
// unconditional reload), same as every other mutation.
func TestCreateDoneErrRoutesToApplyMutationResult(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	wantCursor := m.cursorID
	wantErr := fmt.Errorf("lif create: boom")

	tm, cmd := m.Update(createDoneMsg{err: wantErr})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(createDoneMsg) did not return a model, got %T", tm)
	}
	if nm.err != wantErr.Error() {
		t.Fatalf("m.err = %q, want %q", nm.err, wantErr.Error())
	}
	if nm.cursorID != wantCursor {
		t.Fatalf("cursorID = %q, want unchanged %q (a failed create must not jump the cursor)", nm.cursorID, wantCursor)
	}
	if cmd == nil {
		t.Fatal("createDoneMsg (error) must still trigger a reload Cmd (applyMutationResult)")
	}
}

// TestFormCapturesAllKeysWhileOpen guards the capture-order contract
// (handleKey doc-stamp): "q" while a Create-Form is open must type into the
// focused field, NOT request a quit -- same precedent as searchActive/
// filterOpen's full-capture behavior. ctrl+c (I02, E3-T4-Review PFLICHT,
// closed in T5, bean bt-sl45) is a DIFFERENT case: huh binds its OWN Quit
// keymap to ctrl+c (huh keymap.go's NewDefaultKeyMap), and handleKey's
// m.form != nil capture routes it to keyForm -> updateForm BEFORE bt's own
// ctrl+c/tea.Quit switch ever sees it -- so ctrl+c must abort the FORM
// (huh.StateAborted -> m.form nil, updateForm's own doc-stamp), never the
// whole app.
func TestFormCapturesAllKeysWhileOpen(t *testing.T) {
	t.Run("q types into the field", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m = step(t, m, runeMsg('c'))
		if m.form == nil {
			t.Fatal("setup: c did not open the create form")
		}

		nm := step(t, m, runeMsg('q'))
		if nm.confirmQuit {
			t.Fatal("q while the create form is open must type 'q', not request quit (searchActive-style capture precedent)")
		}
		if nm.form == nil {
			t.Fatal("form must still be open after typing q into it")
		}
		if nm.overlay != overlayNone {
			t.Fatalf("overlay = %v, want overlayNone (q must not open anything either)", nm.overlay)
		}
	})

	t.Run("ctrl+c quits the app at once", func(t *testing.T) {
		m := fixtureModel(t, fixtureBeans())
		m = step(t, m, runeMsg('c'))
		if m.form == nil {
			t.Fatal("setup: c did not open the create form")
		}
		assertQuitsAtOnce(t, m)
	})
}
