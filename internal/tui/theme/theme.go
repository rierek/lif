// Package theme hält die geteilte Catppuccin-Palette + Status-/Priority-Styles für die
// beans-tui. Port von Eriks devd-TUI (~/dev/DeveloperDashboard/apps/cli-go/
// internal/theme/theme.go) — Enum-Mapping auf beans-Status/-Priority angepasst.
// Die Palette folgt `display.theme` (Apply); Farben und Glyphen der Werte kommen
// aus der Config (SetConfiguredColors, ui-Tabellen).
package theme

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rierek/lif/internal/ui"
)

// asciiIcons — Opt-in ASCII-Ersatz-Glyphen für Terminals/Fonts, die die (EAW-neutrale,
// aber font-seitig schwach abgedeckte) Unicode-Geometrie nicht darstellen. ASCII ist
// garantiert verfügbar UND EAW=Neutral (kein Spalten-Drift). Aktiv via
// LIF_TUI_ASCII_ICONS=1|true|yes|on (Port-Adaptation 1: devd DEVD_ASCII_ICONS → LIF_TUI_ASCII_ICONS).
// Pro Aufruf gelesen (Render ist nicht perf-kritisch) → auch in Tests via t.Setenv
// setz-/rücksetzbar, kein Package-State-Caching.
func asciiIcons() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LIF_TUI_ASCII_ICONS"))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// Palette colours. Apply fills them from the active ui.Theme; init starts with
// the default theme so a process that never calls Apply still has colours.
var (
	Rosewater lipgloss.Color
	Flamingo  lipgloss.Color
	Pink      lipgloss.Color
	Mauve     lipgloss.Color // Akzent
	Red       lipgloss.Color
	Maroon    lipgloss.Color
	Peach     lipgloss.Color
	Yellow    lipgloss.Color
	Green     lipgloss.Color
	Teal      lipgloss.Color
	Sky       lipgloss.Color
	Sapphire  lipgloss.Color
	Blue      lipgloss.Color
	Lavender  lipgloss.Color

	Text    lipgloss.Color
	Subtext lipgloss.Color
	Overlay lipgloss.Color // Input-/Feld-Border
	Surface lipgloss.Color
	Base    lipgloss.Color // App-/Body-Hintergrund
)

// Hint = Hinweis/Erklärung (Labels, Sub-Label, Shortcuts, Placeholder, inaktive
// Tabs). Zwei-Klassen-Text-Regel: muted ggü. echter Info. Bewusst ≠ Overlay
// (Feld-Border). Fest, folgt dem Theme nicht.
// Select = Interaktions-/Auswahl-Signal (aktiver Auswahl-Button), Latte-Peach.
// Bewusst ≠ Peach (struktureller Akzent). Fest, folgt dem Theme nicht.
const (
	Hint   = lipgloss.Color("#7c7c7c")
	Select = lipgloss.Color("#fe640b")
)

// Styles built from the palette above; rebuildStyles refreshes them.
var (
	Header = lipgloss.NewStyle()
	// HeaderInactive = closed section header title (Teal).
	HeaderInactive = lipgloss.NewStyle()
	Key            = lipgloss.NewStyle() // IDs/Keys = Sapphire
	Accent         = lipgloss.NewStyle()

	// BindingKey/BindingDesc render a keybinding hint's key/description
	// halves (renderBindings in view.go): key Teal, description Subtext.
	// Deliberately their own pair, not HeaderInactive (separate decision).
	BindingKey  = lipgloss.NewStyle()
	BindingDesc = lipgloss.NewStyle()

	Dim = lipgloss.NewStyle()
	// Muted = Hinweis/Erklärung: Shortcuts, Sub-Label, Placeholder. Bewusst Hint,
	// nicht Overlay (= Feld-Border) — Zwei-Klassen-Text-Prinzip.
	Muted = lipgloss.NewStyle()
	// Chevron = struktureller Marker `>`/`o`, Peach.
	Chevron = lipgloss.NewStyle()
)

func init() { Apply(ui.DefaultTheme()) }

// Apply puts the palette of t in force and rebuilds every style derived from
// it. A user accent (SetAccent) does not survive: call SetAccent afterwards.
func Apply(t ui.Theme) {
	for c, tone := range map[*lipgloss.Color]string{
		&Rosewater: "rosewater", &Flamingo: "flamingo", &Pink: "pink", &Mauve: "mauve",
		&Red: "red", &Maroon: "maroon", &Peach: "peach", &Yellow: "yellow",
		&Green: "green", &Teal: "teal", &Sky: "sky", &Sapphire: "sapphire",
		&Blue: "blue", &Lavender: "lavender", &Text: "text", &Subtext: "subtext0",
		&Overlay: "overlay1", &Surface: "surface1", &Base: "base",
	} {
		*c = lipgloss.Color(t.Hex(tone))
	}
	rebuildStyles()
}

func rebuildStyles() {
	Header = lipgloss.NewStyle().Bold(true).Foreground(Mauve)
	HeaderInactive = lipgloss.NewStyle().Foreground(Teal)
	Key = lipgloss.NewStyle().Foreground(Sapphire)
	Accent = lipgloss.NewStyle().Foreground(Mauve)
	BindingKey = lipgloss.NewStyle().Foreground(Teal)
	BindingDesc = lipgloss.NewStyle().Foreground(Subtext)
	Dim = lipgloss.NewStyle().Foreground(Overlay)
	Muted = lipgloss.NewStyle().Foreground(Hint)
	Chevron = lipgloss.NewStyle().Foreground(Peach)
}

// accentHexRe validates SetAccent's own input independently of any caller
// (E5 Task 5, bean bt-0l8c) -- the theme package does not trust
// config.validateSettings to have run first.
var accentHexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// SetAccent überschreibt den Akzentstil (Cursor/Header) mit einer User-Farbe
// (Settings.Theme.Accent, config.yaml, E5 Task 5). Ein leerer ODER
// ungültiger Hex (nicht #rrggbb) ist ein No-Op — der eingebaute Mauve-Akzent
// bleibt (Golden-Risiko: die 7 Golden-Snapshots rendern gegen den Default und
// müssen byte-identisch bleiben, wenn config.yaml nie angefasst wurde bzw.
// Settings.Theme.Accent auf seinem Zero-Value "" steht). Globale
// Theme-Mutation über package-level vars — Accent/Header werden von JEDEM
// Render-Aufruf live gelesen (kein Caching), ein SetAccent-Aufruf wirkt
// daher sofort auf jede noch folgende Render-Ausgabe (DD2-221-Live-Apply-
// Prinzip), ohne Neustart.
func SetAccent(hex string) {
	if !accentHexRe.MatchString(hex) {
		return
	}
	c := lipgloss.Color(hex)
	Accent = lipgloss.NewStyle().Foreground(c)
	Header = lipgloss.NewStyle().Bold(true).Foreground(c)
}

// configured holds the colours the active project's config sets explicitly
// (status/type/priority `color:`), keyed by value name. They win over the
// built-in palette below; SetConfiguredColors replaces them wholesale, so a
// repo switch never leaks the previous repo's colours.
var configured struct{ status, typ, priority map[string]lipgloss.Color }

// SetConfiguredColors installs the active project's explicit colours (nil maps
// clear them).
func SetConfiguredColors(status, typ, priority map[string]lipgloss.Color) {
	configured.status, configured.typ, configured.priority = status, typ, priority
}

// --- Status ---

// StatusColor liefert die konfigurierte Farbe eines Status, sonst Text.
func StatusColor(status string) lipgloss.Color {
	if col, ok := configured.status[status]; ok {
		return col
	}
	return Text
}

// StatusStyle liefert den lipgloss-Style für einen Status (Default: Text via StatusColor).
func StatusStyle(status string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(StatusColor(status))
}

// statusIconGlyph liefert das Kürzel des Status aus der Config (ui-Tabelle).
// Unbekannte Status bekommen den generischen Fallback-Glyph (fallbackGlyph).
func statusIconGlyph(status string) string {
	if g := ui.ShortStatus(status); g != "?" {
		return g
	}
	return fallbackGlyph()
}

// StatusIcon liefert den einheitlichen, statusgefärbten Status-Buchstaben.
func StatusIcon(status string) string {
	return StatusStyle(status).Render(statusIconGlyph(status))
}

// --- Fallback für unbekannte Enum-Werte (Status/Type) ---
//
// Dokumentierte Design-Entscheidung (Task-6-Vorgabe): unbekannte beans-Enum-Werte
// (z.B. ein künftig hinzugefügter Status/Type) rendern neutral statt zu verschwinden
// oder falsch zu signalisieren — Farbe Text, Glyph „·" (U+00B7 MIDDLE DOT). Hinweis:
// U+00B7 ist laut Unicode East-Asian-Width-Tabelle als Ambiguous klassifiziert (anders
// als die übrigen hier verwendeten Glyphen, die bewusst EAW=Neutral/Narrow sind, um
// Spalten-Drift in Ambiguous=Wide-Terminals zu vermeiden). Für den seltenen
// Fallback-Pfad (unbekannter Enum-Wert) wird das in Kauf genommen — exakt der in der
// Aufgabenstellung vorgegebene Glyph.
const fallbackGlyphChar = "·"      // U+00B7 MIDDLE DOT
const fallbackGlyphCharASCII = "." // ASCII-Ersatz

func fallbackGlyph() string {
	if asciiIcons() {
		return fallbackGlyphCharASCII
	}
	return fallbackGlyphChar
}

// --- Priorität ---

// priorityColor: configured colour, else Text.
func priorityColor(p string) lipgloss.Color {
	if col, ok := configured.priority[p]; ok {
		return col
	}
	return Text
}

// priorityGlyphASCII: ASCII-Ersatz (LIF_TUI_ASCII_ICONS), weil die Unicode-
// Symbole normal/low/deferred laut East-Asian-Width Ambiguous sind.
var priorityGlyphASCII = map[string]string{
	"critical": "!!",
	"high":     "!",
	"normal":   ".",
	"low":      "v",
	"deferred": ">",
}

// priorityIconGlyph wählt den Priority-Glyph: ASCII-Tabelle im ASCII-Modus,
// sonst das Symbol der Config. Leer oder unbekannt → fallbackGlyph.
func priorityIconGlyph(p string) string {
	g := ui.GetPrioritySymbol(p)
	if asciiIcons() {
		g = priorityGlyphASCII[p]
	}
	if g == "" {
		return fallbackGlyph()
	}
	return g
}

// Priority rendert eine beans-Priorität als gefärbten Glyph (PF-6; critical/high
// bleiben fett zur Hervorhebung, wie schon in der Wort-Variante).
func Priority(p string) string {
	st := lipgloss.NewStyle().Foreground(priorityColor(p))
	if p == "critical" || p == "high" {
		st = st.Bold(true)
	}
	return st.Render(priorityIconGlyph(p))
}
