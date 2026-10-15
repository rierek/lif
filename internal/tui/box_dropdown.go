package tui

// box_dropdown.go — das eine wiederverwendete jira-Style Dropdown-Widget
// (../docs/topics/jira-style-experiment/design-spec.md, D01): Label im oberen
// Rahmen, Wert + ▾ innen, Hotkey unten-rechts im Rahmen. lipgloss kann keinen
// Text-im-Rahmen -> Rahmenzeilen manuell aus Box-Zeichen komponiert. Breiten-
// und ANSI-korrekt über lipgloss.Width. Reused von Detail-Pane (S2) +
// Filter-Leiste (S3).

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/tui/theme"
)

func clampVisible(s string, n int) string {
	if n < 0 {
		n = 0
	}
	return ansi.Truncate(s, n, "")
}

func borderDashes(n int) string {
	if n < 1 {
		return ""
	}
	return strings.Repeat("─", n)
}

// boxTopBorder builds the ╭─ label ─…─╮ row at exactly `width` cells: label
// in labelStyle, dashes/corners in `frame`. Shared by dropdownBox + panelBox
// (S2e Change 1, review B4 — was duplicated in both).
func boxTopBorder(label string, width int, frame lipgloss.Style) string {
	labelStyle := lipgloss.NewStyle().Foreground(theme.Subtext)
	labelText := clampVisible(label, width-6)
	labelSeg := frame.Render("─ ") + labelStyle.Render(labelText) + frame.Render(" ")
	topFill := width - 2 - (2 + lipgloss.Width(labelText) + 1)
	return frame.Render("╭") + labelSeg + frame.Render(borderDashes(topFill)) + frame.Render("╮")
}

// boxBottomBorder builds the ╰…(hotkey)…╯ row at exactly `width` cells. Empty
// hotkey → plain dash line. Shared by dropdownBox + panelBox (S2e Change 1,
// review B4). S2e Change 2 (review B2/B3): if the badge plus the minimum
// dashes/corners cannot fit within `width`, fall back to a plain dash line
// instead of letting the row overflow — a dropped badge on an absurdly
// narrow box is acceptable, an overflowing row is not.
func boxBottomBorder(hotkey string, width int, frame lipgloss.Style) string {
	plain := frame.Render("╰") + frame.Render(borderDashes(width-2)) + frame.Render("╯")
	if hotkey == "" {
		return plain
	}
	badge := theme.BindingKey.Render("(" + hotkey + ")")
	badgeSeg := " " + badge + " "
	const minRightDashes = 3
	const minLeftDashes = 1
	// width - 2 corners must fit: leftDashes(>=1) + badgeSeg + rightDashes(3)
	if width-2-lipgloss.Width(badgeSeg)-minRightDashes < minLeftDashes {
		return plain
	}
	right := frame.Render(borderDashes(minRightDashes))
	fill := width - 2 - lipgloss.Width(badgeSeg) - minRightDashes
	return frame.Render("╰") + frame.Render(borderDashes(fill)) + badgeSeg + right + frame.Render("╯")
}

// boxTopBorderHotkey builds the ╭─ label ─…─ (x) ─╮ row at exactly `width`
// cells: boxTopBorder's layout with a hotkey badge parked on the right, the
// same position and (x) shape boxBottomBorder uses.
//
// bean bt-oox1 (#4): the bottom border is the wrong place for a badge on a
// box whose height follows its CONTENT. The Body panel grows with the bean's
// body text, so on a long body its bottom border -- and the (e) badge with
// it -- is scrolled out of the pane, hiding the edit key exactly where it is
// most wanted. A top border is always visible, because a box is scrolled
// into view from the top.
//
// Same defensive fallback as boxBottomBorder: if label plus badge plus the
// minimum dashes cannot fit, the badge is dropped rather than allowed to
// overflow -- a missing badge on an absurdly narrow box is acceptable, a
// broken frame is not.
func boxTopBorderHotkey(label, hotkey string, width int, frame lipgloss.Style) string {
	if hotkey == "" {
		return boxTopBorder(label, width, frame)
	}
	labelStyle := lipgloss.NewStyle().Foreground(theme.Subtext)
	labelText := clampVisible(label, width-6)
	labelSeg := frame.Render("─ ") + labelStyle.Render(labelText) + frame.Render(" ")

	badge := theme.BindingKey.Render("(" + hotkey + ")")
	badgeSeg := " " + badge + " "

	const minRightDashes = 3
	const minLeftDashes = 1
	// width - 2 corners must fit: labelSeg + leftDashes(>=1) + badgeSeg + rightDashes(3)
	used := 2 + lipgloss.Width(labelText) + 1 + lipgloss.Width(badgeSeg) + minRightDashes
	if width-2-used < minLeftDashes {
		return boxTopBorder(label, width, frame)
	}
	fill := width - 2 - used
	return frame.Render("╭") + labelSeg + frame.Render(borderDashes(fill)) +
		badgeSeg + frame.Render(borderDashes(minRightDashes)) + frame.Render("╮")
}

// boxTopBorderBadges is boxTopBorderHotkey with an extra LEFT-anchored badge
// sitting right after the label, with the hotkey parked at the far right:
// ╭─ label ─ <badge> ─…─ (x) ─╮ (bean bt-adkn Rework, US-02, PO 2026-07-21:
// "das (e) immer an der gleichen Stelle belassen und dafuer die Punkte
// verschieben -- das fuehrt zu einer stabilen praesentation"). Anchoring the
// indicator on the LEFT (its start fixed right after the label) and (x) on the
// RIGHT means paging -- which changes the indicator's content/width -- moves
// neither: only the middle dash fill absorbs the width change, so the header
// reads as a stable frame. Empty badge -> exactly boxTopBorderHotkey (byte-
// identical, no golden drift for the fits-case). Same defensive contract as its
// siblings: if label + badge + hotkey + the minimum dashes cannot fit, the
// badge is dropped (falling back to the plain hotkey border) rather than
// overflowing the frame.
func boxTopBorderBadges(label, badge, hotkey string, width int, frame lipgloss.Style) string {
	if badge == "" {
		return boxTopBorderHotkey(label, hotkey, width, frame)
	}
	labelStyle := lipgloss.NewStyle().Foreground(theme.Subtext)
	labelText := clampVisible(label, width-6)
	// labelSeg ends with "─ " so the badge sits after a dash separator, then a
	// trailing space frames it against the middle fill: "─ Body ─ <badge> …".
	labelSeg := frame.Render("─ ") + labelStyle.Render(labelText) + frame.Render(" ─ ")
	badgeSeg := badge + " "

	rightSeg := ""
	if hotkey != "" {
		rightSeg = " " + theme.BindingKey.Render("("+hotkey+")") + " " // " (e) ", flush-right against the dashes
	}

	const minMidDashes = 1
	// minRightDashes MUST match boxTopBorderHotkey's (3), so (e) parks at the
	// exact same column whether the indicator is present (this function) or
	// absent (the badge=="" fallback above, which delegates to boxTopBorderHotkey)
	// -- otherwise (e) jumps by one column as a bean's body switches between
	// fitting and overflowing (bean bt-adkn US-02, 2nd PO-Reject 2026-07-21).
	const minRightDashes = 3
	// interior (width-2) = labelSeg + badgeSeg + midDashes + rightSeg + rightDashes
	used := 3 + lipgloss.Width(labelText) + 2 + lipgloss.Width(badgeSeg) + lipgloss.Width(rightSeg) + minRightDashes
	if width-2-used < minMidDashes {
		return boxTopBorderHotkey(label, hotkey, width, frame)
	}
	mid := width - 2 - used
	return frame.Render("╭") + labelSeg + badgeSeg + frame.Render(borderDashes(mid)) +
		rightSeg + frame.Render(borderDashes(minRightDashes)) + frame.Render("╮")
}

// dropdownBox rendert das 3-Zeilen-Widget in exakt width Zellen Breite.
// focused = Mauve-Rahmen, sonst Overlay. R1 (design-spec.md D08): das Label
// im oberen Rahmen ist NICHT Teil des Rahmens selbst -- es rendert in
// theme.Subtext (gedämpft), während die Rahmenzeichen (╭ ─ ╮ etc.) weiter in
// der Fokus-Farbe (Mauve/Overlay) bleiben. labelStyle trägt nur den
// Label-TEXT, frame weiterhin die Box-Zeichen links/rechts davon.
func dropdownBox(label, value, hotkey string, width int, focused bool) string {
	if width < 8 {
		width = 8
	}
	borderColor := theme.Overlay
	if focused {
		borderColor = theme.Mauve
	}
	frame := lipgloss.NewStyle().Foreground(borderColor)

	top := boxTopBorder(label, width, frame)

	arrow := theme.Chevron.Render("▾")
	inner := width - 6
	val := clampVisible(value, inner)
	pad := inner - lipgloss.Width(val)
	if pad < 0 {
		pad = 0
	}
	mid := frame.Render("│") + " " + val + strings.Repeat(" ", pad) + " " + arrow + " " + frame.Render("│")

	bot := boxBottomBorder(hotkey, width, frame)

	return top + "\n" + mid + "\n" + bot
}
