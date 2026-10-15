package tui

// form_create_bean_test.go — TDD coverage for the Create-Form field set
// (`c`, E3 Task 4, bean bt-y4ly, design decision e): title/type/priority/
// status/parent/tags/body, keyed huh v1.0.0 fields, parent existence-only
// validation (type-hierarchy stays the CLI's own VALIDATION_ERROR path).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/rierek/lif/internal/tui/data"
)

// driveFormBudget bounds how many Cmd rounds driveForm/driveFormCmd will
// chase per top-level call. huh's own field-advance chain (blur cmd + focus
// cmd, plus the nextFieldMsg round-trip that commits a field's value into
// f.results, huh form.go:565-568/615) depends on the SAME tea.BatchMsg
// fan-out a real tea.Program performs before calling Update() again -- a
// bare f.Update() call never does this on its own, hence driving it
// recursively here. BUT: focusing an Input field (huh field_input.go's
// Focus -> bubbles textinput.Focus) starts textinput's own cursor-blink
// timer (tea.Tick-based, bubbletea commands.go), which is SELF-PERPETUATING
// -- chasing it unbounded would hang forever (empirically confirmed: an
// earlier unbounded version of this helper hung the test binary). The
// legitimate field-advance chain settles in well under this budget; a blink
// timer just burns through it harmlessly (blink touches neither f.results
// nor field selection) and the helper returns once exhausted.
//
// I01 (E3-T4-Review PFLICHT, closed in T5, bean bt-sl45): the remaining
// budget SATURATES at exactly 0 (driveFormStep/driveFormCmd's own `if
// budget <= 0 { return ..., 0 }` guards), never negative -- and whether 6
// rounds is ENOUGH for the legitimate chain to settle is NOT a hard
// contract, it depends on huh's internal Cmd ORDERING (field-advance Cmds
// firing before/interleaved with the blink timer's own re-arm). That
// ordering is empirically stable against huh v1.0.0, not something this
// package controls -- a huh upgrade could shift it. Budget hitting 0 is
// therefore NOT on its own proof of a problem (a normal, fully-settled call
// commonly burns the rest of the budget on harmless blink-timer chasing
// too) -- but combined with a caller's OWN postcondition still failing
// afterwards, it narrows "did the chase simply run out of rounds" from "real
// logic bug" instead of leaving that ambiguity to a bare want/got diff.
// advanceFieldsExhausted/requireFieldValue below expose that signal for
// callers that want the loud-fail distinction (advanceFields/driveForm/
// driveModel themselves stay unchanged -- most callers don't need it).
const driveFormBudget = 6

func driveForm(f *huh.Form, msg tea.Msg) *huh.Form {
	f, _ = driveFormStep(f, msg, driveFormBudget)
	return f
}

func driveFormStep(f *huh.Form, msg tea.Msg, budget int) (*huh.Form, int) {
	if budget <= 0 {
		return f, 0
	}
	budget--
	m, cmd := f.Update(msg)
	nf, ok := m.(*huh.Form)
	if !ok {
		return f, budget
	}
	return driveFormCmd(nf, cmd, budget)
}

// driveFormCmd recursively executes cmd, unwrapping tea.BatchMsg (the only
// EXPORTED multi-cmd wrapper bubbletea produces here -- huh's own
// nextField/prevField helpers return single, unbatched Cmds) and feeding
// every resulting Msg back through driveFormStep, all against the SAME
// shared budget (driveFormBudget doc comment above).
func driveFormCmd(f *huh.Form, cmd tea.Cmd, budget int) (*huh.Form, int) {
	if cmd == nil || budget <= 0 {
		return f, budget
	}
	budget--
	msg := cmd()
	if msg == nil {
		return f, budget
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if budget <= 0 {
				break
			}
			f, budget = driveFormCmd(f, c, budget)
		}
		return f, budget
	}
	return driveFormStep(f, msg, budget)
}

// enterMsg advances the currently focused field -- Input/Select/Text all
// bind their Next/Submit keymap to plain "enter" (huh v1.0.0 keymap.go
// NewDefaultKeyMap), so one shared helper drives every field type in
// buildCreateBeanForm's group.
func enterMsg() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

// advanceFields drives f forward n fields (n calls to enterMsg) -- the
// caller counts how many of buildCreateBeanForm's 7 keyed fields
// (title/type/priority/status/parent/tags/body, in that order) it needs to
// walk past.
func advanceFields(f *huh.Form, n int) *huh.Form {
	for i := 0; i < n; i++ {
		f = driveForm(f, enterMsg())
	}
	return f
}

// advanceFieldsExhausted is advanceFields' loud-fail-capable counterpart
// (I01, E3-T4-Review PFLICHT, closed in T5, bean bt-sl45): same n-field
// advance, but also reports whether the LAST field-advance call's chase
// budget saturated at exactly 0 (driveFormBudget's own doc-stamp on what
// that does and doesn't prove). Pair with requireFieldValue below.
func advanceFieldsExhausted(f *huh.Form, n int) (*huh.Form, bool) {
	exhausted := false
	for i := 0; i < n; i++ {
		var remaining int
		f, remaining = driveFormStep(f, enterMsg(), driveFormBudget)
		exhausted = remaining == 0
	}
	return f, exhausted
}

// requireFieldValue is I01's loud-fail assertion: a bare want/got mismatch
// cannot distinguish a real logic bug from the chase budget running out
// before huh's Cmd chain settled (driveFormBudget's own doc-stamp) --
// exhausted (advanceFieldsExhausted's own signal) narrows a MISMATCH
// specifically to the second, more actionable cause instead of leaving that
// ambiguity to an opaque diff.
func requireFieldValue(t *testing.T, exhausted bool, what, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	if exhausted {
		t.Fatalf("%s = %q, want %q -- AND the chase budget (driveFormBudget=%d) was exhausted reaching it: bump the budget before assuming a logic bug", what, got, want, driveFormBudget)
	}
	t.Fatalf("%s = %q, want %q", what, got, want)
}

func idxWithMS1() *data.Index {
	return data.NewIndex([]data.Bean{
		{ID: "ms-1", Title: "Milestone One", Status: "todo", Type: "milestone", Priority: "normal"},
	})
}

// TestBuildCreateBeanFormPrefillsParentFromCursor guards the draft-prefill
// contract (design decision e "vorbelegt aus Tree-Cursor-Kontext"): a
// non-empty beanDraft.parent is bound into the Input's accessor immediately
// at construction (huh.Input.Value(&parent), field_input.go) -- advancing
// PAST the parent field (5 fields: title/type/priority/status/parent)
// without ever typing anything new must leave GetString("parent") at the
// prefill.
func TestBuildCreateBeanFormPrefillsParentFromCursor(t *testing.T) {
	idx := idxWithMS1()
	f := buildCreateBeanForm(idx, beanDraft{title: "New Task", parent: "ms-1"})
	f, exhausted := advanceFieldsExhausted(f, 5) // title, type, priority, status, parent

	requireFieldValue(t, exhausted, "GetString(parent)", f.GetString("parent"), "ms-1")
}

// TestParseTagsFieldSplitsAndValidates covers both the split (whitespace AND
// comma) and the per-token data.ValidTagName validation (design decision e).
func TestParseTagsFieldSplitsAndValidates(t *testing.T) {
	tags, err := parseTagsField("a b,c")
	if err != nil {
		t.Fatalf("parseTagsField(%q) error = %v, want nil", "a b,c", err)
	}
	want := []string{"a", "b", "c"}
	if len(tags) != len(want) {
		t.Fatalf("parseTagsField(%q) = %v, want %v", "a b,c", tags, want)
	}
	for i, w := range want {
		if tags[i] != w {
			t.Fatalf("parseTagsField(%q)[%d] = %q, want %q", "a b,c", i, tags[i], w)
		}
	}

	if _, err := parseTagsField("Ü!"); err == nil {
		t.Fatal("parseTagsField(\"Ü!\") error = nil, want a validation error")
	}
}

// TestParentFieldValidatorAcceptsEmptyAndExistingID guards design decision
// e's own scope-cut: empty (no parent) and any existing ID both pass.
func TestParentFieldValidatorAcceptsEmptyAndExistingID(t *testing.T) {
	v := parentFieldValidator(idxWithMS1())
	if err := v(""); err != nil {
		t.Fatalf("parentFieldValidator(\"\") = %v, want nil (optional field)", err)
	}
	if err := v("ms-1"); err != nil {
		t.Fatalf("parentFieldValidator(\"ms-1\") = %v, want nil (existing id)", err)
	}
}

// TestParentFieldValidatorRejectsUnknownID guards the existence-only rule --
// no type-hierarchy duplication of the CLI's own server-side rule (design
// decision e).
func TestParentFieldValidatorRejectsUnknownID(t *testing.T) {
	v := parentFieldValidator(idxWithMS1())
	if err := v("does-not-exist"); err == nil {
		t.Fatal("parentFieldValidator(\"does-not-exist\") = nil, want an error")
	}
}

// TestCreateOptsFromDraftMapsAllFields guards the beanDraft->CreateOpts
// mapping submitForm relies on (box_confirm_create.go): every field lands in
// its CreateOpts counterpart, tags parsed via parseTagsField.
func TestCreateOptsFromDraftMapsAllFields(t *testing.T) {
	d := beanDraft{
		title:    "  New Task  ",
		typ:      "task",
		priority: "high",
		status:   "in-progress",
		parent:   "ms-1",
		tags:     "a b",
		body:     "some body",
	}
	opts := createOptsFromDraft(d)
	if opts.Title != "New Task" {
		t.Errorf("Title = %q, want %q (trimmed)", opts.Title, "New Task")
	}
	if opts.Type != "task" {
		t.Errorf("Type = %q, want task", opts.Type)
	}
	if opts.Priority != "high" {
		t.Errorf("Priority = %q, want high", opts.Priority)
	}
	if opts.Status != "in-progress" {
		t.Errorf("Status = %q, want in-progress", opts.Status)
	}
	if opts.Parent != "ms-1" {
		t.Errorf("Parent = %q, want ms-1", opts.Parent)
	}
	if strings.Join(opts.Tags, ",") != "a,b" {
		t.Errorf("Tags = %v, want [a b]", opts.Tags)
	}
	if opts.Body != "some body" {
		t.Errorf("Body = %q, want %q", opts.Body, "some body")
	}
}
