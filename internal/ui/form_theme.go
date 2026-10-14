package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// FormPalette is the set of colour roles a huh form theme is built from.
// An empty role paints nothing, so a caller can leave Background unset and
// keep the terminal's own background.
type FormPalette struct {
	Accent     lipgloss.Color // titles, selectors, indicators, selected option, cursor, prompt, focused button background
	Focus      lipgloss.Color // left border bar of the focused field
	Background lipgloss.Color // form and field background, focused button text
	Error      lipgloss.Color // error indicator and message
	Text       lipgloss.Color // options, input text, blurred button text
	Muted      lipgloss.Color // descriptions, placeholders, unselected prefix, blurred border, help line
	Surface    lipgloss.Color // blurred button background
}

// FlavourFormPalette maps one Catppuccin flavour to the form roles. Every
// role is a tone of t. Muted is overlay1 because the TUI's grey hint colour
// lies outside Catppuccin.
func FlavourFormPalette(t Theme) FormPalette {
	c := func(tone string) lipgloss.Color { return lipgloss.Color(t.Hex(tone)) }
	return FormPalette{
		Accent:     c("mauve"),
		Focus:      c("yellow"),
		Background: c("base"),
		Error:      c("red"),
		Text:       c("text"),
		Muted:      c("overlay1"),
		Surface:    c("surface1"),
	}
}

// FormTheme builds a fresh huh theme from p. It is the only huh theme
// construction in the code base: huh's ThemeBase supplies the layout, and
// every colour it carries (ANSI buttons and placeholder, bubbles help
// defaults) is replaced by a palette role.
func FormTheme(p FormPalette) *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(p.Focus).Background(p.Background)
	t.Focused.Card = t.Focused.Base
	t.Focused.Title = t.Focused.Title.Foreground(p.Accent).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(p.Muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(p.Error)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(p.Error)

	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(p.Accent).SetString("▸ ")
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(p.Accent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(p.Accent)
	t.Focused.Option = t.Focused.Option.Foreground(p.Text)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(p.Accent)

	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(p.Accent).SetString("▸ ")
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(p.Accent).SetString("[•] ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(p.Muted).SetString("[ ] ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(p.Text)

	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(p.Background).Background(p.Accent)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(p.Text).Background(p.Surface)
	t.Focused.Next = t.Focused.FocusedButton

	// The select filter line reads these text-input styles.
	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(p.Accent)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(p.Muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(p.Accent).SetString("")
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(p.Text)

	t.Form.Base = t.Form.Base.Background(p.Background)

	t.Help.Ellipsis = t.Help.Ellipsis.Foreground(p.Muted)
	t.Help.ShortKey = t.Help.ShortKey.Foreground(p.Muted)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(p.Muted)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(p.Muted)
	t.Help.FullKey = t.Help.FullKey.Foreground(p.Muted)
	t.Help.FullDesc = t.Help.FullDesc.Foreground(p.Muted)
	t.Help.FullSeparator = t.Help.FullSeparator.Foreground(p.Muted)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderForeground(p.Muted).Background(p.Background)
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	return t
}
