package tui

// form_create_bean.go — the Create-Form's field set (`c`, E3 Task 4, bean
// bt-y4ly, design decision e): title/type/priority/status/parent/tags/body,
// keyed huh v1.0.0 fields, values read via GetString after
// huh.StateCompleted (no pointer-binding over model copies). Port devd
// form_create_issue.go, restricted to beans-tui's own field set (no
// PO-Notes/user-stories/tag-multiselect -- those are devd Issue concepts).

import (
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/tui/data"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// beanDraft survives a form rebuild -- the Confirm-Gate's n/esc return
// reopens the Create-Form pre-filled from the last submitted values
// (Draft-Erhalt, Port DD2-190, box_confirm_create.go/keyCreateConfirm).
// Zero-value = an empty form (openCreateForm's first open with no prior
// draft).
type beanDraft struct {
	title, typ, priority, status, parent, tags, body string
}

// parseTagsField splits a whitespace/comma-separated tags field into
// individual tag names, validating each against data.ValidTagName (design
// decision e). Returns the FIRST invalid token's error -- huh surfaces it as
// the field's own validation message, so the PO sees exactly which token
// failed.
func parseTagsField(s string) ([]string, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	tags := make([]string, 0, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		if !data.ValidTagName(f) {
			return nil, fmt.Errorf("invalid tag %q (lowercase, hyphen-separated)", f)
		}
		tags = append(tags, f)
	}
	return tags, nil
}

// tagsFieldValidator wraps parseTagsField as a huh field validator -- huh
// only wants the error, not the parsed slice.
func tagsFieldValidator(s string) error {
	_, err := parseTagsField(s)
	return err
}

// parentFieldValidator accepts an empty parent (no parent set) or any ID
// present in idx.ByID -- design decision e: the parent field is ONLY
// existence-validated here. Type-hierarchy errors (e.g. a task as an epic's
// parent) deliberately run through the CLI's own VALIDATION_ERROR path
// (classifyError, mutations.go) -> the status line, no client-side
// duplication of the server rule (Parent-Picker's data.EligibleParents,
// box_picker_parent.go/hierarchy.go, is a DIFFERENT E3 Task 3 concern: this
// field is free-text, not a picker, per design decision e).
func parentFieldValidator(idx *data.Index) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		if idx == nil {
			return fmt.Errorf("unknown parent id %q", s)
		}
		if _, ok := idx.ByID[s]; !ok {
			return fmt.Errorf("unknown parent id %q", s)
		}
		return nil
	}
}

// selectOptions builds huh Select options from an ordered value slice
// (label == value, mirrors box_filter_facets.go's buildFilterItems lowercase
// convention -- no separate display casing).
func selectOptions(vals []string) []huh.Option[string] {
	opts := make([]huh.Option[string], len(vals))
	for i, v := range vals {
		opts[i] = huh.NewOption(v, v)
	}
	return opts
}

// typeSelectOptions/prioritySelectOptions/statusSelectOptions build the
// Create-Form's three Selects from the E3 enum single source (design
// decision b, data.TypeValues/PriorityValues/StatusValues) -- never a second
// hardcoded copy of the enum values.
func typeSelectOptions() []huh.Option[string]     { return selectOptions(data.TypeValues()) }
func prioritySelectOptions() []huh.Option[string] { return selectOptions(data.PriorityValues()) }
func statusSelectOptions() []huh.Option[string]   { return selectOptions(data.StatusValues()) }

// buildCreateBeanForm constructs the keyed Create-Form (vanilla huh embedded
// sub-model, design decision e), pre-filled from d (Draft-Erhalt/Confirm-Gate
// n/esc return, DD2-190) with type/priority/status defaulting to
// task/normal/todo when d carries no draft yet (first open, empty
// beanDraft{}). idx backs the parent-existence validator -- a nil idx
// (defensive; should not happen once the App-Shell has loaded) makes every
// non-empty parent invalid rather than panicking.
//
// The Body field switches huh's OWN ctrl+e $EDITOR launch off
// (ExternalEditor(false)) -- devd's DD2-234 trap: that built-in editor runs
// over $EDITOR and BROADCASTS its result to every Text field in the group.
// This form only has one Text field today, but the switch is set on
// principle (Port devd form_create_issue.go's own doc comment) since a
// second Text field would silently reintroduce the bug the moment one is
// added.
func buildCreateBeanForm(idx *data.Index, d beanDraft) *huh.Form {
	title, parent, tags, body := d.title, d.parent, d.tags, d.body
	typ := d.typ
	if typ == "" {
		typ = "task"
	}
	prio := d.priority
	if prio == "" {
		prio = "normal"
	}
	status := d.status
	if status == "" {
		status = "todo"
	}

	fields := []huh.Field{
		huh.NewInput().Key("title").Title("Title").Value(&title).Validate(nonEmpty),
		huh.NewSelect[string]().Key("type").Title("Type").Options(typeSelectOptions()...).Value(&typ),
		huh.NewSelect[string]().Key("priority").Title("Priority").Options(prioritySelectOptions()...).Value(&prio),
		huh.NewSelect[string]().Key("status").Title("Status").Options(statusSelectOptions()...).Value(&status),
		huh.NewInput().Key("parent").Title("Parent (optional)").Value(&parent).Validate(parentFieldValidator(idx)),
		huh.NewInput().Key("tags").Title("Tags (optional, space/comma-separated)").Value(&tags).Validate(tagsFieldValidator),
		huh.NewText().Key("body").Title("Body (optional)").Value(&body).ExternalEditor(false),
	}
	return huh.NewForm(huh.NewGroup(fields...))
}

// draftFromForm snapshots the CURRENT values of a form that just reached
// huh.StateCompleted -- submitForm's own DD2-190 draft capture
// (box_confirm_create.go), called BEFORE the form is nulled. Only valid once
// every field has been visited (huh's f.results is populated field-by-field
// via its own nextFieldMsg commit chain, form.go -- exactly what
// StateCompleted guarantees).
func draftFromForm(f *huh.Form) beanDraft {
	return beanDraft{
		title:    f.GetString("title"),
		typ:      f.GetString("type"),
		priority: f.GetString("priority"),
		status:   f.GetString("status"),
		parent:   f.GetString("parent"),
		tags:     f.GetString("tags"),
		body:     f.GetString("body"),
	}
}

// createOptsFromDraft maps a completed beanDraft onto data.CreateOpts.
// parseTagsField's error is ignored here (not "hoped away" -- submitForm
// only calls this AFTER huh's own tagsFieldValidator already accepted the
// tags field, so a second parse failure here cannot occur in practice; the
// zero-value fallback keeps the mapping total instead of adding a second,
// redundant error return no caller could ever observe firing).
func createOptsFromDraft(d beanDraft) data.CreateOpts {
	tags, _ := parseTagsField(d.tags)
	return data.CreateOpts{
		Title:    strings.TrimSpace(d.title),
		Type:     d.typ,
		Priority: d.priority,
		Status:   d.status,
		Parent:   strings.TrimSpace(d.parent),
		Tags:     tags,
		Body:     strings.TrimSpace(d.body),
	}
}

// openCreateForm opens a fresh Create-Form (`c`, keyNodeAction dispatch,
// update.go): works WITHOUT a focused bean (an empty repo can still be
// seeded, design decision e / plan Task 1 Step 10). Parent-Prefill = the
// focused bean's ID when one exists ("vorbelegt aus Tree-Cursor-Kontext",
// Task 4 Akzeptanz), empty otherwise.
func (m model) openCreateForm() (tea.Model, tea.Cmd) {
	d := beanDraft{}
	if b := m.focusedBean(); b != nil {
		d.parent = b.ID
	}
	return m.openCreateFormWithDraft(d)
}

// openCreateFormWithDraft (re)opens the Create-Form pre-filled from d --
// shared by openCreateForm's first open (empty/parent-prefilled draft) and
// keyCreateConfirm's n/esc Draft-Erhalt return (box_confirm_create.go,
// DD2-190).
func (m model) openCreateFormWithDraft(d beanDraft) (tea.Model, tea.Cmd) {
	m.formKind = "create"
	m.form = m.styleForm(buildCreateBeanForm(m.idx, d))
	return m, m.form.Init()
}
