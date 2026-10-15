package theme

import "github.com/charmbracelet/lipgloss"

// --- beans-Type Buchstaben-Icons (PF-6, design-spec.md §15, 2026-07-15) ---
//
// Superseded: vormals geometrische Unicode-Icons (⬢/✦/⯅/⯁, Port-Adaptation 3
// aus devds typeIcon/typeColor), Bedeutung nur über Farbe getrennt -- epic und
// feature teilten sich sogar denselben Glyphen (✦). PO-Direktive: EIN
// kapitalisierter Typ-Buchstabe je Typ + Farbe, redundante Kodierung wie bei
// Status (theme.go statusLetter). Alle Buchstaben sind ASCII/EAW=Narrow --
// LIF_TUI_ASCII_ICONS bleibt für Typ wirkungslos, kein typeIconASCII mehr nötig.
var typeIcon = map[string]string{
	"milestone": "M",
	"epic":      "E",
	"feature":   "F",
	"task":      "T",
	"bug":       "B",
}

var typeColor = map[string]lipgloss.Color{
	"milestone": Blue,
	"epic":      Mauve,
	"feature":   Mauve,
	"task":      Sky,
	"bug":       Red,
}

// typeGlyph liefert den Type-Buchstaben. Unbekannte Typen bekommen den
// generischen Fallback-Glyph (fallbackGlyph, siehe theme.go).
func typeGlyph(t string) string {
	ic, ok := typeIcon[t]
	if !ok {
		return fallbackGlyph()
	}
	return ic
}

// TypeIcon liefert das gefärbte Text-Icon eines beans-Typs (Fallback: Text-Farbe +
// fallbackGlyph für unbekannte Typen).
func TypeIcon(t string) string {
	return lipgloss.NewStyle().Foreground(typeColorOf(t)).Render(typeGlyph(t))
}

// TypeStyle färbt einen Typ-Text (Fallback: Text-Farbe für unbekannte Typen).
func TypeStyle(t string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(typeColorOf(t))
}

// typeColorOf: configured colour, else the built-in one, else Text.
func typeColorOf(t string) lipgloss.Color {
	if col, ok := configured.typ[t]; ok {
		return col
	}
	if col, ok := typeColor[t]; ok {
		return col
	}
	return Text
}
