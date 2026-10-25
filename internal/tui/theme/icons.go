package theme

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/rierek/lif/internal/ui"
)

// typeGlyph liefert das Typ-Kürzel aus der Config (ui-Tabelle). Unbekannte
// Typen bekommen den generischen Fallback-Glyph (fallbackGlyph, siehe theme.go).
func typeGlyph(t string) string {
	if g := ui.ShortType(t); g != "?" {
		return g
	}
	return fallbackGlyph()
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

// typeColorOf: configured colour, else Text.
func typeColorOf(t string) lipgloss.Color {
	if col, ok := configured.typ[t]; ok {
		return col
	}
	return Text
}
