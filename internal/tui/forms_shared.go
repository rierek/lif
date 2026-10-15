package tui

// forms_shared.go — the huh-Form-Hosting infrastructure (E3 Task 4, bean
// bt-y4ly, Port devd forms_shared.go:42-97/125-172/177-225/289-307): forms
// run as an EMBEDDED sub-model inside the running Bubble Tea loop (no
// separate form.Run() one-shot). Values are NOT bound over pointers into
// model (would break against model's value-copy Elm semantics) -- they are
// read keyed via GetString AFTER huh.StateCompleted (devd's own
// "kein Pointer-Binding über Model-Copies" convention). Kind-specific
// builders live in their own file (form_create_bean.go for "create"; T5
// adds form_edit_title.go for "editTitle") -- this file is the shared
// gerüst only: styling, chrome, and the Update-side plumbing
// (updateForm/keyForm).

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// nonEmpty is the shared required-field validator (title etc.) -- Port devd
// forms_shared.go:42-47.
func nonEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("must not be empty")
	}
	return nil
}

// defaultFormWidth is the Create-/Edit-Form modal's desired width (devd's
// defaultModalWidth, scoped here to forms only -- beans-tui's other modals
// each size individually via direct clampModalWidth calls,
// box_picker_parent.go/box_picker_blocking.go/box_confirm_create.go).
// clampModalWidth itself already exists (box_filter_facets.go, E2 Task 4) --
// not redefined here.
var defaultFormWidth = 64

// formBoxWidth clamps the form modal's desired width to the terminal
// (mirrors devd modalBoxWidth, box_filter_facets.go's clampModalWidth as the
// underlying primitive).
func formBoxWidth(termW int) int { return clampModalWidth(defaultFormWidth, termW) }

// formInnerWidth is the huh Form's own width WITHIN the box (Rahmen +
// Padding budget), Port devd forms_shared.go:100-106.
func formInnerWidth(termW int) int {
	w := formBoxWidth(termW) - 6
	if w < 18 {
		w = 18
	}
	return w
}

// formInnerHeight caps the huh Form's height to the terminal (huh scrolls
// internally rather than overflowing past the modal), Port devd
// forms_shared.go:108-123. termH<=0 (init/tests, unknown height) -> a
// conservative fallback, reapplied on the next real WindowSizeMsg.
func formInnerHeight(termH int) int {
	if termH <= 0 {
		return 16
	}
	h := termH - 7 // Box-Chrome (Titel+Leerzeile+Rahmen ~5) + 2 Zeilen Rand
	if h > 20 {
		h = 20
	}
	if h < 5 {
		h = 5
	}
	return h
}

// styleForm finalizes an embedded huh Form uniformly: Theme + terminal
// dimensions + huh's own help switched off (the footer hint comes from
// formChrome instead). Single source for every form-open path (T4's
// openCreateFormWithDraft, T5's editTitle equivalent). Port devd
// forms_shared.go:167-172.
func (m model) styleForm(f *huh.Form) *huh.Form {
	return f.WithWidth(formInnerWidth(m.width)).
		WithHeight(formInnerHeight(m.height)).
		WithShowHelp(false).
		WithTheme(paletteFormTheme())
}

// paletteFormTheme mirrors the app's own modal styling onto a huh.Theme --
// Theme-Tokens only (theme.Mauve/Yellow/Base/Red/Text/Hint), no hex here.
// Mauve = Akzent (Titel, Selector, Indikatoren, aktive Option), Yellow =
// Signalbalken am aktiven Feld (Orientierung), Base = BG, Red = Fehler.
// Port devd forms_shared.go:177-225.
func paletteFormTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(theme.Yellow).Background(theme.Base)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(theme.Mauve).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(theme.Hint)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(theme.Red)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(theme.Red)

	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(theme.Mauve).SetString("▸ ")
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(theme.Mauve)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(theme.Mauve)
	t.Focused.Option = t.Focused.Option.Foreground(theme.Text)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(theme.Mauve)

	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(theme.Mauve).SetString("▸ ")
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(theme.Mauve).SetString("[•] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(theme.Hint).SetString("[ ] ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(theme.Text)

	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(theme.Base).Background(theme.Mauve)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(theme.Text).Background(theme.Surface)
	t.Focused.Next = t.Focused.FocusedButton

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(theme.Mauve)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(theme.Hint)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(theme.Mauve).SetString("")
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(theme.Text)

	t.Form.Base = t.Form.Base.Background(theme.Base)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderForeground(theme.Hint).Background(theme.Base)
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	return t
}

// formTitle labels the open form's modal (m.formKind-keyed, mirrors devd
// forms_shared.go's own formTitle switch).
func (m model) formTitle() string {
	switch m.formKind {
	case "create":
		return "New item"
	case "editTitle":
		return "Edit title"
	case "settings":
		return "Settings"
	}
	return ""
}

// formChrome frames the open huh Form like every other modal (Header-Titel,
// Dim-Separator + Dim-Footer-Hint), wrapped via modalPanel (modal.go).
// Single source for every form kind. Port devd forms_shared.go:289-307.
func (m model) formChrome() string {
	innerW := formInnerWidth(m.width)
	body := theme.Dim.Render(strings.Repeat("─", innerW)) + "\n" + m.form.View()
	hint := "enter next/save · esc cancel"
	return modalPanel(m.formTitle(), body, hint, formBoxWidth(m.width), theme.Mauve)
}

// keyForm drives an open huh Form (m.form != nil, handleKey capture order
// doc-stamp, update.go): esc aborts the form OUTRIGHT (Formular verwerfen)
// regardless of formKind -- Draft-Erhalt exists ONLY on the Confirm-Gate's
// own n/esc path (box_confirm_create.go), never here (devd-Parität, Port
// devd updateForm's own unconditional KeyEsc intercept, update.go:993-1002).
// Every other key delegates to updateForm, which forwards it to huh.
func (m model) keyForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.form = nil
		m.formKind = ""
		return m, nil
	}
	return m.updateForm(msg)
}

// updateForm is the central NON-key router for an open huh Form (E3 Task 4):
// Update's own type switch falls back here for every Msg it doesn't
// otherwise recognize while m.form != nil (huh-internal Msgs -- cursor
// blink, viewport ticks, its own nextField/nextGroup chain -- keep firing
// for as long as the form is open). Key-Msgs reach here too, via keyForm
// above (every key except esc). Delegates to m.form.Update, then reacts to a
// State transition: huh.StateCompleted -> submitForm (box_confirm_create.go)
// hands off to the Confirm-Gate; huh.StateAborted (huh's OWN Quit binding,
// ctrl+c by default, huh keymap.go) discards the form the same way keyForm's
// esc-intercept does.
func (m model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	f, ok := form.(*huh.Form)
	if !ok {
		return m, cmd
	}
	m.form = f
	switch m.form.State {
	case huh.StateCompleted:
		return m.submitForm()
	case huh.StateAborted:
		m.form = nil
		m.formKind = ""
		return m, nil
	}
	return m, cmd
}
