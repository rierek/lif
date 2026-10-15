package tui

// modal.go — render-component layer for floating overlays (ported from devd
// DD2 I01, ~/dev/DeveloperDashboard/apps/cli-go/internal/tui/
// modal.go): single source against drift, new modals/menus become a handful
// of lines instead of ~20.
//   modalBox   — the box chrome (RoundedBorder, Base BG, Padding)
//   modalPanel — header title + body + optional dim footer, inside modalBox
//   menuList   — selection list with a ▸ cursor (Accent) before the active entry
//
// rebaseBg is ported alongside modalBox from devd's forms_shared.go (not a
// separate forms.go — forms land in E3, this is its only caller here) since
// modalBox depends on it for the Base-BG fix (devd B02/B03).

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/tui/theme"
)

// rebaseBg re-opens the modal's Base background color after every inner
// ESC[0m reset (devd B03): without this, inner resets (e.g. theme.Header/
// Accent.Render, later huh forms) leave cells falling back to the terminal's
// default background (usually black) instead of the modal's Base — visible
// e.g. as a selected menu entry rendered fg+bold with no bg. Profile-aware
// (TrueColor/256/16); under the Ascii profile the color the profile resolves
// to carries no real color escape (empty SGR), so no visible color leaks in
// even though the string is technically still rewritten.
func rebaseBg(s string) string {
	c := lipgloss.ColorProfile().Color(string(theme.Base))
	if c == nil {
		return s
	}
	open := termenv.CSI + c.Sequence(true) + "m"
	return strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+open)
}

// modalBox frames pre-rendered content as a floating overlay modal.
// border = theme.Mauve (default) or theme.Red (destructive dialogs).
func modalBox(inner string, width int, border lipgloss.Color) string {
	// B02: BorderBackground(theme.Base) — otherwise the border cells fall back
	// to the terminal's default background (black) instead of matching the
	// modal body's Base background.
	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		BorderBackground(theme.Base).
		Background(theme.Base).
		Padding(0, 1).
		Render(rebaseBg(inner))
}

// modalPanel builds a standard modal: header title + body + optional dim
// footer, wrapped by modalBox. The body brings its own inner line breaks
// (subtitle/separator/items). Single source for command-center/menu-style
// overlays (T8+).
func modalPanel(title, body, footer string, width int, border lipgloss.Color) string {
	var b strings.Builder
	b.WriteString(theme.Header.Render(title) + "\n")
	b.WriteString(body)
	if footer != "" {
		b.WriteString("\n" + theme.Dim.Render(footer))
	}
	return modalBox(b.String(), width, border)
}

// menuList renders a selection list with a ▸ cursor (Accent) before the
// active entry. render(i, selected) returns the entry text AFTER the cursor
// — the caller controls label/marker/highlight. Every line ends with "\n".
func menuList(n, cursor int, render func(i int, selected bool) string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		sel := i == cursor
		c := "  "
		if sel {
			c = theme.Accent.Render("▸ ")
		}
		b.WriteString(c + render(i, sel) + "\n")
	}
	return b.String()
}
