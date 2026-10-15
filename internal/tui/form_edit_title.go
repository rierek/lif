package tui

// form_edit_title.go — the single-field Title-Edit-Form (`e`, E3 Task 5,
// bean bt-sl45, design decision h): the SAME huh-Form-Hosting infra Task 4
// built (styleForm/formChrome), but with exactly ONE field and NO
// Confirm-Gate -- edits fire mutateCmd(SetTitle) DIRECTLY on
// huh.StateCompleted (submitForm's "editTitle" case, box_confirm_create.go
// -- Port devd isCreateKind's own exclusion: only "create"-kind forms get
// gated, edits never do). Port devd forms_shared.go:332-360
// (buildEditFieldForm), reduced to the one field this task needs (Title,
// Input, nonEmpty-required) -- beans-tui has no generic multi-field
// editField concept yet.

import (
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/tui/data"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// singleLineTitle is the Title-Edit-Form's validator (B03 Fix-Runde 1,
// Review F02, Supervisor-Entscheid: reject, do NOT silently normalize):
// nonEmpty (forms_shared.go) PLUS a newline rejection. The Input->Text swap
// made it possible to type a real "\n" into the title via huh's NewLine
// binding (alt+enter/ctrl+j, huh keymap.go) -- structurally impossible with
// huh.Input -- and that newline survived GetString -> SetTitle ->
// `beans update --title`, which is undefined for the YAML single-line
// title field. Scoped to THIS form on purpose: the Create-Form's title
// field is still a huh.Input (form_create_bean.go) and cannot contain one.
func singleLineTitle(s string) error {
	if err := nonEmpty(s); err != nil {
		return err
	}
	if strings.Contains(s, "\n") {
		return fmt.Errorf("title must be single-line")
	}
	return nil
}

// buildEditTitleForm constructs the keyed single-field form, pre-filled with
// the bean's current title (huh.Text.Value(&v), field_text.go),
// singleLineTitle-validated (nonEmpty + newline rejection, F02 above). B03
// (design-spec.md §15 PF-17, bean bt-2v38): swapped from huh.NewInput()
// (single-line, horizontal scroll on long titles) to huh.NewText().Lines(3)
// (multi-line, wraps instead). .ExternalEditor(false) is MANDATORY, not
// cosmetic: huh.Text brings its OWN ctrl+e editor-suspend mechanism
// (field_text.go default true) that would collide with D01's app-wide
// e/ctrl+e whole-bean $EDITOR (bt-z4b1) -- disabled so keyForm's own
// keyboard semantics (forms_shared.go) stay the only ctrl+e in play here.
// .Lines(3) is a Planner estimate (PO gave no line count); GetValue()/the
// Value(&v) binding still returns a plain string (verified against
// field_text.go), so submitForm's "editTitle" case
// (m.form.GetString("title"), box_confirm_create.go) is UNCHANGED by this
// swap.
func buildEditTitleForm(title string) *huh.Form {
	v := title
	field := huh.NewText().Key("title").Title("Title").Lines(3).
		ExternalEditor(false).Value(&v).Validate(singleLineTitle)
	form := huh.NewForm(huh.NewGroup(field))
	// F01 (Fix-Runde 1, bean bt-2v38 Review-Findings): the
	// .ExternalEditor(false) flag above does NOTHING by itself -- huh only
	// translates it into a disabled ctrl+e binding inside KeyBinds()
	// (field_text.go:249-252, `t.keymap.Editor.SetEnabled(t.externalEditor)`),
	// and huh.NewForm's own WithKeyMap has just REPLACED the field's keymap
	// with a fresh default whose Editor binding is enabled (form.go:119,
	// field_text.go:453-457). Without this explicit call the disable would
	// only ever happen as an ACCIDENTAL side effect of styleForm's
	// WithHeight()-before-WithShowHelp(false) call order triggering a
	// help-footer render (reviewer-proven: flag removed -> whole suite stays
	// green; styleForm order swapped -> huh's editor fires). Calling
	// KeyBinds() here, AFTER NewForm's keymap assignment, pins the disable
	// deterministically at build time; later KeyBinds() runs (help-footer
	// renders) re-derive it from the same flag and keep it disabled.
	field.KeyBinds()
	return form
}

// openEditTitleForm opens the Title-Edit-Form on b (`e`, keyNodeAction
// dispatch, update.go -- the focusedBean()!=nil guard lives in the caller).
// m.mutTarget captures WHICH bean the form acts on, reusing the value-menu/
// picker convention (one node-action target at a time; forms and overlays
// are mutually exclusive capture states, design decision a2, so there is no
// collision) -- the ETag is deliberately NOT captured here (design decision
// d): submitForm's "editTitle" case re-reads it fresh via m.beanETag.
func (m model) openEditTitleForm(b *data.Bean) (tea.Model, tea.Cmd) {
	m.mutTarget = b.ID
	m.formKind = "editTitle"
	m.form = m.styleForm(buildEditTitleForm(b.Title))
	return m, m.form.Init()
}
