package tui

// form_edit_title_test.go — TDD coverage for the Title-Edit-Form (E3 Task 5,
// bean bt-sl45, design decision h): a single-field huh form, prefilled with
// the bean's current title, nonEmpty-validated, that fires
// mutateCmd(SetTitle) DIRECTLY on completion -- no Confirm-Gate (Port devd
// isCreateKind's own exclusion: only "create"-kind forms get gated). Once
// reachable via "e"; D01 (design-spec.md §15 PF-17, bean bt-z4b1) moved "e"
// to the whole-bean $EDITOR exclusively -- this form is now reachable ONLY
// via enter on the title: field (activateDetailField, update.go).

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/rierek/lif/internal/tui/data"
)

// TestEditTitleFormPrefilledAndNonEmptyValidated guards both halves of
// buildEditTitleForm: the current title is bound as the field's initial
// value (no round-trip needed to observe it), and nonEmpty blocks an empty
// title from ever reaching huh.StateCompleted (Port devd forms_shared.go
// nonEmpty).
func TestEditTitleFormPrefilledAndNonEmptyValidated(t *testing.T) {
	f := buildEditTitleForm("Original Title")
	// GetString only reflects huh's f.results after a commit round-trip
	// (driveFormBudget's own doc-stamp, form_create_bean_test.go) -- settle
	// the single field WITHOUT editing it to observe the prefill survived.
	f = driveForm(f, enterMsg())
	if got := f.GetString("title"); got != "Original Title" {
		t.Fatalf("GetString(title) after settling unedited = %q, want prefill %q", got, "Original Title")
	}

	empty := buildEditTitleForm("")
	empty = driveForm(empty, enterMsg())
	if empty.State == huh.StateCompleted {
		t.Fatal("an empty title reached StateCompleted, want the nonEmpty validator to block it")
	}
}

// TestEditTitleSubmitFiresSetTitleDirectlyNoConfirm guards design decision
// h's core contract: completing the single field fires mutateCmd(SetTitle)
// IMMEDIATELY via submitForm's "editTitle" case -- no overlayCreateConfirm
// detour. Drives the underlying huh.Form to StateCompleted directly (Port
// devd's own single-field editField precedent) rather than through
// model.Update, so submitForm's returned Cmd can be inspected BEFORE it
// fires (mirrors box_picker_parent_test.go/box_confirm_create_test.go's
// nonexistent-RepoDir dispatch-proof pattern) -- the model-level chase
// (driveModel) would otherwise auto-execute the mutateCmd AND its
// applyMutationResult reload tail, whose LATER error would overwrite m.err
// and mask the one under test.
func TestEditTitleSubmitFiresSetTitleDirectlyNoConfirm(t *testing.T) {
	f := buildEditTitleForm("Original Title")
	f = driveForm(f, enterMsg())
	if f.State != huh.StateCompleted {
		t.Fatalf("setup: form.State = %v, want StateCompleted after the single field's enter", f.State)
	}

	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: "/nonexistent-bt-e3-t5-scratch-dir"}
	m.form = f
	m.formKind = "editTitle"
	m.mutTarget = "tk-1"

	tm, cmd := m.submitForm()
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("submitForm() did not return a model, got %T", tm)
	}
	if nm.form != nil {
		t.Fatal("form still open after submitForm, want nil (editTitle: no Confirm-Gate)")
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (design decision h: no Confirm-Gate for edits)", nm.overlay)
	}
	if cmd == nil {
		t.Fatal("submitForm(editTitle) must fire a Cmd (mutateCmd(SetTitle), no Confirm parking)")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif update") {
		t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves SetTitle dispatched)", mdm.err, "lif update")
	}
}

// TestEditTitleSubmitTargetVanishedShowsToast guards submitForm's "editTitle"
// vanished-target guard (box_confirm_create.go:66, bt-81f0's Inventar) --
// the ONE of the seven ex-silent m.err sites this file owns. m.mutTarget
// points at an id no longer in m.idx.ByID (beanETag's own ok=false
// contract), mirroring every other overlay's vanished-target guard
// (box_confirm_delete_test.go/box_picker_parent_test.go/etc.'s own
// "TargetVanishedClosesGracefully" tests) -- m.err alone used to be the
// ONLY feedback; post-bt-81f0 it renders nowhere, so a Toast must carry it.
func TestEditTitleSubmitTargetVanishedShowsToast(t *testing.T) {
	f := buildEditTitleForm("Original Title")
	f = driveForm(f, enterMsg())
	if f.State != huh.StateCompleted {
		t.Fatalf("setup: form.State = %v, want StateCompleted after the single field's enter", f.State)
	}

	m := fixtureModel(t, fixtureBeans())
	m.form = f
	m.formKind = "editTitle"
	m.mutTarget = "nonexistent-bt-81f0-target" // not in m.idx.ByID -> beanETag ok=false

	tm, cmd := m.submitForm()
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("submitForm() did not return a model, got %T", tm)
	}
	if nm.form != nil {
		t.Fatal("form still open after submitForm, want nil")
	}
	if nm.err == "" {
		t.Fatal("submitForm(editTitle) on a vanished target must set a status-line note (m.err)")
	}
	// bt-81f0: cmd is no longer nil -- it is now the Toast's own
	// auto-dismiss tick (non-sticky), NOT a doomed mutation (structurally
	// guaranteed: this branch returns before any mutateCmd(...) is built).
	// Not invoked here -- toastError's own 8s duration would block the test.
	if cmd == nil {
		t.Fatal("submitForm(editTitle) on a vanished target must still fire a Cmd (the Toast's own auto-dismiss tick, bt-81f0)")
	}
	if nm.toast == nil {
		t.Fatal("submitForm(editTitle) on a vanished target must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if nm.toast.kind != toastError {
		t.Errorf("toast.kind = %v, want toastError (bt-81f0 bindender Rahmen)", nm.toast.kind)
	}
}

// TestBuildEditTitleFormUsesMultiLineText guards B03's core swap (design-
// spec.md §15 PF-17, bean bt-2v38): the title field itself must now be a
// *huh.Text (multi-line textarea, huh v1.0.0 field_text.go), not the
// single-line *huh.Input E3 Task 5 originally wired. What this test pins
// is the FIELD TYPE plus a MINIMUM height ("multi-line at all": strictly
// more rendered rows than an equivalent *huh.Input's fixed single row for
// the SAME title/width, via bubbles textarea.go:1182-1199's documented
// "Always show at least `m.Height` lines" render contract) -- NOT the
// exact .Lines(3) value. .Lines(3) is a Planner estimate, not a design
// contract (bean bt-2v38 "Architektur-Vorgabe"), so a drive-by change to
// e.g. .Lines(4), or even dropping .Lines() entirely (huh's textarea
// default height is 6), deliberately does NOT fail here; only regressing
// back to a single-line field does (Review F03, Fix-Runde 1).
func TestBuildEditTitleFormUsesMultiLineText(t *testing.T) {
	f := buildEditTitleForm("Original Title")

	field := f.GetFocusedField()
	textField, ok := field.(*huh.Text)
	if !ok {
		t.Fatalf("field type = %T, want *huh.Text (B03: huh.NewInput -> huh.NewText)", field)
	}
	textField.WithWidth(40)
	textRows := strings.Count(textField.View(), "\n") + 1

	v := "Original Title"
	inputField := huh.NewInput().Key("title").Title("Title").Value(&v).Validate(nonEmpty)
	inputField.WithWidth(40)
	inputRows := strings.Count(inputField.View(), "\n") + 1

	if textRows <= inputRows {
		t.Fatalf("multi-line field rendered %d row(s), want MORE than the single-line Input's %d row(s) for the same title/width (Lines(3) must reserve extra height)", textRows, inputRows)
	}
}

// TestOpenEditTitleFormRoundTripsLongTitleUnchanged guards B03's data-
// integrity contract: a title long enough to need the new multi-line wrap
// (.Lines(3), > 60 chars) must survive an UNEDITED submit byte-for-byte --
// no truncation, no embedded newline snuck in by the Input->Text field-type
// swap. Two halves, both a REAL huh.Form/model tea.Update round-trip (Port
// devd-style, ANALOG TestEditTitleSubmitFiresSetTitleDirectlyNoConfirm
// above): (1) the exact string huh hands back via GetString("title") -- the
// SAME untransformed read submitForm's "editTitle" case performs
// (box_confirm_create.go, `title := m.form.GetString("title")`), checked
// directly since this suite has no PATH-stubbed "beans" binary to intercept
// the mutation's real CLI args (every sibling test in this package settles
// for the same "lif update" dispatch-proof instead); (2) submitForm
// itself still fires the SAME mutateCmd with no Confirm-Gate detour.
func TestOpenEditTitleFormRoundTripsLongTitleUnchanged(t *testing.T) {
	long := "A bean title that runs well past sixty characters so the new multi-line Text field must wrap it instead of truncating or scrolling"
	if len(long) <= 60 {
		t.Fatalf("setup: fixture title is %d chars, want > 60", len(long))
	}

	f := buildEditTitleForm(long)
	f = driveForm(f, enterMsg())
	if f.State != huh.StateCompleted {
		t.Fatalf("setup: form.State = %v, want StateCompleted after the single field's enter", f.State)
	}
	if got := f.GetString("title"); got != long {
		t.Fatalf("GetString(title) after an unedited submit = %q (%d chars), want the full unchanged title (%d chars) -- B03 must not truncate", got, len(got), len(long))
	}
	if strings.Contains(f.GetString("title"), "\n") {
		t.Fatal("GetString(title) contains a newline -- B03's multi-line Text field must not leak its internal line-wrap into the stored value")
	}

	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: "/nonexistent-bt-2v38-scratch-dir"}
	m.form = f
	m.formKind = "editTitle"
	m.mutTarget = "tk-1"

	tm, cmd := m.submitForm()
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("submitForm() did not return a model, got %T", tm)
	}
	if nm.form != nil {
		t.Fatal("form still open after submitForm, want nil (editTitle: no Confirm-Gate)")
	}
	if cmd == nil {
		t.Fatal("submitForm(editTitle) must fire a Cmd (mutateCmd(SetTitle))")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif update") {
		t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves SetTitle dispatched, same GetString(title) verified full/unchanged above)", mdm.err, "lif update")
	}
}

// collectCmdMsgs executes cmd and returns every tea.Msg it produces,
// recursively unwrapping tea.BatchMsg -- the only EXPORTED multi-cmd
// wrapper bubbletea produces on this path (same scope note as
// driveFormCmd, form_create_bean_test.go). Unlike driveFormCmd it does NOT
// feed the msgs back into anything: callers want to INSPECT what a single
// Update round emitted (e.g. hunt for tea's unexported execMsg), not
// settle a form. budget bounds the walk against huh's self-perpetuating
// blink-timer chain (driveFormBudget doc comment).
func collectCmdMsgs(cmd tea.Cmd, budget int) []tea.Msg {
	if cmd == nil || budget <= 0 {
		return nil
	}
	msg := cmd()
	if msg == nil {
		return nil
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			budget--
			if budget <= 0 {
				break
			}
			out = append(out, collectCmdMsgs(c, budget)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// TestEditTitleFormCtrlEDoesNotFireHuhEditor guards F01 (Fix-Runde 1, bean
// bt-2v38 Review-Findings): huh.Text ships its OWN ctrl+e editor-suspend
// (field_text.go:321-330, tea.ExecProcess -> tea's unexported execMsg).
// .ExternalEditor(false) alone does NOT disable it -- the flag is only
// translated into a disabled key binding inside KeyBinds()
// (field_text.go:249-252), which huh.NewForm's own WithKeyMap first UNDOES
// (it replaces the field's keymap with a fresh default whose Editor
// binding is enabled, form.go:119/286-297, field_text.go:453-457). Before
// Fix-Runde 1 the disable only happened ACCIDENTALLY, as a side effect of
// styleForm's WithHeight()-before-WithShowHelp(false) call order
// triggering a help-footer render. This test therefore drives the REAL
// production flow (enter on the title: field -> openEditTitleForm ->
// styleForm) and proves a real ctrl+e KeyMsg through model.Update yields
// NO execMsg anywhere in the returned Cmd tree -- it goes red both if
// .ExternalEditor(false) is removed AND if the accidental-order safety net
// alone is what disables the binding (buildEditTitleForm's explicit
// KeyBinds() call is the deterministic fix).
func TestEditTitleFormCtrlEDoesNotFireHuhEditor(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, keyMsg(tea.KeyTab))
	m = step(t, m, keyMsg(tea.KeyEnter)) // field level, fieldCursor=0 (title)
	m = step(t, m, keyMsg(tea.KeyEnter)) // opens the Title-Edit-Form via styleForm
	if m.form == nil || m.formKind != "editTitle" {
		t.Fatalf("setup: form=%v formKind=%q, want an open editTitle form", m.form, m.formKind)
	}

	tm, cmd := m.Update(keyMsg(tea.KeyCtrlE))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(ctrl+e) did not return a model, got %T", tm)
	}
	if nm.form == nil {
		t.Fatal("ctrl+e closed the form, want it still open (ctrl+e must be inert inside the Title-Edit-Form)")
	}
	for _, msg := range collectCmdMsgs(cmd, 16) {
		if strings.Contains(fmt.Sprintf("%T", msg), "execMsg") {
			t.Fatalf("ctrl+e inside the Title-Edit-Form produced %T -- huh's own editor-suspend fired despite .ExternalEditor(false) (F01: the flag only acts via KeyBinds(), which must be invoked deterministically, not as a styleForm call-order accident)", msg)
		}
	}
}

// TestEditTitleSubmitRejectsEmbeddedNewline guards F02 (Fix-Runde 1, bean
// bt-2v38 Review-Findings, Supervisor-Entscheid: validator, NO silent
// normalization): the Input->Text swap made it possible to type a REAL
// newline into the title via huh's NewLine binding (alt+enter/ctrl+j,
// keymap.go:136) -- with huh.Input that was structurally impossible, and
// it survived all the way into `beans update --title` (undefined for the
// YAML single-line title field). The single-line validator must reject it
// with a clear message at submit; the counterpart half proves a normal
// title still completes unchanged through the SAME flow.
func TestEditTitleSubmitRejectsEmbeddedNewline(t *testing.T) {
	f := buildEditTitleForm("Original Title")
	// Init() focuses the single field synchronously (group.Init ->
	// Field.Focus, huh group.go:195-215) -- without focus the underlying
	// textarea drops every input key (bubbles textarea.go Update's !m.focus
	// guard), so ctrl+j could never insert its newline.
	f.Init()
	f = driveForm(f, keyMsg(tea.KeyCtrlJ))
	tf, ok := f.GetFocusedField().(*huh.Text)
	if !ok {
		t.Fatalf("focused field = %T, want *huh.Text", f.GetFocusedField())
	}
	if got, _ := tf.GetValue().(string); !strings.Contains(got, "\n") {
		t.Fatalf("setup: field value %q contains no newline after ctrl+j -- huh's NewLine binding did not insert one", got)
	}

	f = driveForm(f, enterMsg())
	if f.State == huh.StateCompleted {
		t.Fatal("a title with an embedded newline reached StateCompleted, want the single-line validator to block the submit (F02)")
	}
	if err := tf.Error(); err == nil || !strings.Contains(err.Error(), "single-line") {
		t.Fatalf("field error after blocked submit = %v, want a %q validation error (F02)", err, "title must be single-line")
	}

	// Counterpart: the SAME flow minus the ctrl+j still completes with the
	// value untouched -- the new validator must not reject normal titles.
	f2 := buildEditTitleForm("Original Title")
	f2.Init()
	f2 = driveForm(f2, enterMsg())
	if f2.State != huh.StateCompleted {
		t.Fatalf("form.State = %v after submitting a normal single-line title, want StateCompleted (the F02 validator must only reject titles containing a newline)", f2.State)
	}
	if got := f2.GetString("title"); got != "Original Title" {
		t.Fatalf("GetString(title) = %q, want %q unchanged", got, "Original Title")
	}
}

// TestKeyNodeActionEDoesNotOpenEditTitleFormAnymore is D01's regression
// guard for this file's own scope (design-spec.md §15 PF-17, bean bt-z4b1,
// SUPERSEDES design decision h's original "e opens Title-Edit-Form"
// contract, E3 Task 5): "e" now ALWAYS opens the whole-bean $EDITOR
// (TestKeyNodeActionEditorAlwaysOpensBeanEditor, update_test.go) and NEVER
// the Title-Edit-Form anymore -- that form is reachable ONLY via enter on
// the title: field now (TestKeyDetailFocusEnterOnTitleFieldOpensEditTitleForm,
// update_test.go, activateDetailField's "title" case, unchanged by D01).
func TestKeyNodeActionEDoesNotOpenEditTitleFormAnymore(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")

	m = step(t, m, runeMsg('e'))
	if m.form != nil {
		t.Fatalf("e opened a form (form=%v formKind=%q), want NO form -- e now ALWAYS opens the whole-bean $EDITOR (D01)", m.form, m.formKind)
	}
	if m.editorTarget != "tk-1" {
		t.Fatalf("editorTarget = %q, want tk-1 (e must open the whole-bean $EDITOR instead)", m.editorTarget)
	}
}
