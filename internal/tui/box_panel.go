package tui

// box_panel.go — panelBox (jira-style experiment, design-spec.md D01/D04):
// a multi-line titled panel sibling to box_dropdown.go's dropdownBox. Same
// border composition (label in the TOP border, hotkey badge in the BOTTOM
// border, focus-colored frame), but the middle is N pre-wrapped content
// lines instead of a single value+▾ row — a panel is not a dropdown, so
// there is no ▾ arrow. ADDITIVE (S2c): used by detailBoxForm's Body/
// Relations/History sections, not wired into the live accordion view.

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/theme"
)

// panelBox frames pre-rendered multi-line content as a titled box (design-spec
// D01/D04): label in top border, hotkey (if any) in bottom border, content
// lines framed by │ … │. content is already wrapped to the inner width
// (= width-4). Used for Body/Relations/History in detailBoxForm.
func panelBox(label, content, hotkey string, width int, focused bool) string {
	return panelBoxWith(label, content, hotkey, width, focused, false)
}

// panelBoxTopHotkey is panelBox with the hotkey badge in the TOP border
// instead of the bottom one (bean bt-oox1, PO finding #4). Use it for a
// panel whose height follows its content and can therefore outgrow the pane
// -- the Body panel in detailBoxForm is the case that prompted it: a bean
// with a long body scrolled its own bottom border, and the (e) edit badge,
// off screen. See boxTopBorderHotkey (box_dropdown.go) for the full
// rationale.
func panelBoxTopHotkey(label, content, hotkey string, width int, focused bool) string {
	return panelBoxWith(label, content, hotkey, width, focused, true)
}

// panelBoxWith is the shared body of the two above -- ONE frame composition,
// so the variants can never drift on padding, clamping or focus color.
func panelBoxWith(label, content, hotkey string, width int, focused, hotkeyOnTop bool) string {
	if width < 8 {
		width = 8
	}
	borderColor := theme.Overlay
	if focused {
		borderColor = theme.Mauve
	}
	frame := lipgloss.NewStyle().Foreground(borderColor)

	top := boxTopBorder(label, width, frame)
	botKey := hotkey
	if hotkeyOnTop {
		top = boxTopBorderHotkey(label, hotkey, width, frame)
		botKey = ""
	}

	inner := width - 4
	rawLines := strings.Split(content, "\n")
	midLines := make([]string, len(rawLines))
	for i, l := range rawLines {
		l = clampVisible(l, inner)
		pad := inner - lipgloss.Width(l)
		if pad < 0 {
			pad = 0
		}
		midLines[i] = frame.Render("│") + " " + l + strings.Repeat(" ", pad) + " " + frame.Render("│")
	}
	mid := strings.Join(midLines, "\n")

	bot := boxBottomBorder(botKey, width, frame)

	return top + "\n" + mid + "\n" + bot
}
