// Package theme hält die geteilte Catppuccin-Palette + Status-/Priority-Styles für die
// beans-tui. Port von Eriks devd-TUI (~/dev/DeveloperDashboard/apps/cli-go/
// internal/theme/theme.go) — Farb-Tokens 1:1 übernommen, Enum-Mapping auf beans-Status/
// -Priority angepasst (../docs/topics/v1-port/implementation-plan.md »E1 Task 6«).
// Macchiato-Variante, TrueColor-Hex (Akzent = Mauve).
package theme

import (
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// Catppuccin Macchiato — TrueColor-Hex. 1:1 aus devd internal/theme/theme.go übernommen.
var (
	Rosewater = lipgloss.Color("#f4dbd6")
	Flamingo  = lipgloss.Color("#f0c6c6")
	Pink      = lipgloss.Color("#f5bde6")
	Mauve     = lipgloss.Color("#c6a0f6") // Akzent
	Red       = lipgloss.Color("#ed8796")
	Maroon    = lipgloss.Color("#ee99a0")
	Peach     = lipgloss.Color("#f5a97f")
	Yellow    = lipgloss.Color("#eed49f")
	Green     = lipgloss.Color("#a6da95")
	Teal      = lipgloss.Color("#8bd5ca")
	Sky       = lipgloss.Color("#91d7e3")
	Sapphire  = lipgloss.Color("#7dc4e4")
	Blue      = lipgloss.Color("#8aadf4")
	Lavender  = lipgloss.Color("#b7bdf8")

	Text    = lipgloss.Color("#cad3f5")
	Subtext = lipgloss.Color("#a5adcb")
	Overlay = lipgloss.Color("#8087a2") // Input-/Feld-Border
	Surface = lipgloss.Color("#494d64")
	Base    = lipgloss.Color("#24273a") // App-/Body-Hintergrund
	Mantle  = lipgloss.Color("#1e2030") // Header-/Accordion-Header-BG
	Crust   = lipgloss.Color("#181926") // Form-Rahmen-BG (Modal-Backdrop)

	// Hint = Hinweis/Erklärung (Labels, Sub-Label, Shortcuts, Placeholder, inaktive
	// Tabs). Zwei-Klassen-Text-Regel: muted ggü. echter Info. Bewusst ≠ Overlay
	// (#8087a2 = Feld-Border).
	Hint = lipgloss.Color("#7c7c7c")
	// Select = Interaktions-/Auswahl-Signal (aktiver Auswahl-Button), laut
	// (Latte-Peach). Bewusst ≠ Peach #f5a97f (struktureller Akzent).
	Select = lipgloss.Color("#fe640b")

	Header = lipgloss.NewStyle().Bold(true).Foreground(Mauve)
	// HeaderInactive = B06 EXPERIMENT (design-spec.md §15 PF-16, bean
	// bt-ntoz/bt-czpf, 2026-07-16): CLOSED Accordion-Section-Header title
	// color (accordion.go), Teal instead of Muted/Hint-grey -- the closed-
	// header color was visually indistinguishable from the Meta-Label-
	// Spalte (view_detail_bean.go metaSectionBody, also Muted). PO-Sign-off
	// PENDING (bean bt-czpf Akzeptanz-Checkliste) -- rollback to
	// theme.Muted is a one-line change at accordion.go's single call site
	// if the PO rejects the experiment.
	HeaderInactive = lipgloss.NewStyle().Foreground(Teal)
	Key            = lipgloss.NewStyle().Foreground(Sapphire) // IDs/Keys = Sapphire
	Accent         = lipgloss.NewStyle().Foreground(Mauve)

	// BindingKey/BindingDesc render a keybinding hint's key/description
	// halves (D06, design-spec.md §15 PF-16, bean bt-ntoz/bt-d8kc, Grilling
	// Nachtrag "Optik: Taste in TEAL, Aktions-Wort grau (subtext), KEIN ':'
	// mehr"): renderBindings (view.go) is the ONE shared render path for
	// BOTH Header Zone 1 (globalBindings()) and Footer Zone 3 (every
	// *LocalBindings() set, including footer_context.go's overlay-local
	// sets) -- one token pair, applied uniformly everywhere that function is
	// called ("gilt einheitlich", PO-Wortlaut).
	//
	// Deliberately its OWN pair rather than reusing HeaderInactive (also
	// Teal, B06 EXPERIMENT on the Accordion's closed-section header,
	// bt-czpf): HeaderInactive's PO sign-off is still PENDING and a
	// rejection rolls it back to Muted (this file's own doc-stamp above) --
	// D06 is an ALREADY-decided, unrelated Grilling decision. Sharing one
	// token would silently flip the header/footer keybinding color too the
	// moment B06 is rejected, coupling two independent decisions that only
	// coincidentally agree on the same hex today. BindingKey still reuses
	// the existing Teal color VAR (no new hex literal); BindingDesc reuses
	// Subtext (PO's own word: "subtext").
	BindingKey  = lipgloss.NewStyle().Foreground(Teal)
	BindingDesc = lipgloss.NewStyle().Foreground(Subtext)

	Dim = lipgloss.NewStyle().Foreground(Overlay)
	// Muted = Hinweis/Erklärung: Shortcuts, Sub-Label, Placeholder. Bewusst Hint
	// #7c7c7c, nicht Overlay (= Feld-Border) — Zwei-Klassen-Text-Prinzip.
	Muted = lipgloss.NewStyle().Foreground(Hint)
	// Chevron = struktureller Marker `>`/`o`, Peach.
	Chevron = lipgloss.NewStyle().Foreground(Peach)
)

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

// --- beans-Status: draft/todo/in-progress/completed/scrapped (Port-Adaptation 2) ---

var statusColor = map[string]lipgloss.Color{
	"draft":       Blue,
	"todo":        Green,
	"in-progress": Yellow,
	"completed":   Subtext,
	"scrapped":    Subtext,
}

// StatusColor liefert die Farbe für einen beans-Status (Fallback: Subtext).
// PF-6 (design-spec.md §15, 2026-07-15) hat die Farbtabelle verschoben: todo
// war vormals Text -- der Subtext-Fallback wurde ursprünglich gewählt, um
// nicht mit todo=Text zu kollidieren, das gilt seit PF-6 nicht mehr
// (todo=Green). Der Fallback bleibt trotzdem bewusst Subtext (Plan-Vorgabe
// E7 T2 Step 1: „Unknown-Status→Subtext-Fallback-Assertion bleibt") -- jetzt
// einfach ein stabiler, neutraler Default, keine Kollisions-Sonderrolle mehr
// nötig (completed/scrapped teilen sich ohnehin bereits Subtext). Der
// Type-Fallback (icons.go TypeIcon/TypeStyle) bleibt bewusst bei Text: kein
// realer Typ (Blue/Mauve/Sky/Red) nutzt Text, dort besteht keine Kollision.
func StatusColor(status string) lipgloss.Color {
	if col, ok := configured.status[status]; ok {
		return col
	}
	if col, ok := statusColor[status]; ok {
		return col
	}
	return Subtext
}

// StatusStyle liefert den lipgloss-Style für einen Status (Default: Subtext via StatusColor).
func StatusStyle(status string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(StatusColor(status))
}

// Superseded (PF-6, design-spec.md §15, 2026-07-15): der bisherige
// DD2-176-Grundsatz -- EIN gemeinsamer Glyph ("◉") für ALLE Status,
// Bedeutung trägt ausschließlich die Farbe (StatusColor/StatusStyle) -- wird
// per PO-Direktive EXPLIZIT aufgehoben. Status trägt Bedeutung jetzt
// REDUNDANT über Buchstabe UND Farbe (Barrierefreiheits-Gewinn: Status war
// zuvor nur über Farbe unterscheidbar).
var statusLetter = map[string]string{
	"draft":       "d",
	"todo":        "t",
	"in-progress": "i",
	"completed":   "c",
	"scrapped":    "s",
}

// statusIconGlyph wählt den Status-Buchstaben. Unbekannte Status bekommen den
// generischen Fallback-Glyph (fallbackGlyph) statt eines eigenen Zeichens.
// LIF_TUI_ASCII_ICONS bleibt für Status wirkungslos -- die Buchstaben sind bereits
// ASCII/EAW-neutral, kein Ersatz nötig (PF-6).
func statusIconGlyph(status string) string {
	if letter, ok := statusLetter[status]; ok {
		return letter
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

// --- beans-Priorität: critical/high/normal/low/deferred (Port-Adaptation 4, PF-6) ---

// priorityColor: critical dringend (Rot), high dringend aber weniger (Gelb), normal
// neutral (Text), low/deferred zurückgestellt (Subtext). Unbekannte Werte neutral
// (Text). PF-6 (design-spec.md §15, 2026-07-15) verschob high Rot→Gelb und
// low/deferred Grün/Hint→Subtext gegenüber der vorherigen Tabelle.
func priorityColor(p string) lipgloss.Color {
	if col, ok := configured.priority[p]; ok {
		return col
	}
	switch p {
	case "critical":
		return Red
	case "high":
		return Yellow
	case "low", "deferred":
		return Subtext
	default: // "normal" und unbekannte Werte
		return Text
	}
}

// priorityGlyph/priorityGlyphASCII — PF-6 (design-spec.md §15, 2026-07-15):
// Priorität wird künftig als EIN Glyph statt als ausgeschriebenes Wort
// gerendert. Anders als bei Status/Typ bleibt LIF_TUI_ASCII_ICONS hier weiterhin
// relevant: normal/low/deferred (·/↓/→) sind laut Unicode East-Asian-Width
// als Ambiguous klassifiziert (dieselbe Klasse wie fallbackGlyphChar oben) --
// critical/high (‼/!) sind Neutral/Narrow und daher bereits spaltensicher.
var priorityGlyph = map[string]string{
	"critical": "‼", // U+203C DOUBLE EXCLAMATION MARK (EAW=Neutral)
	"high":     "!", // U+0021 EXCLAMATION MARK (EAW=Narrow)
	"normal":   "·", // U+00B7 MIDDLE DOT (EAW=Ambiguous)
	"low":      "↓", // U+2193 DOWNWARDS ARROW (EAW=Ambiguous)
	"deferred": "→", // U+2192 RIGHTWARDS ARROW (EAW=Ambiguous)
}

var priorityGlyphASCII = map[string]string{
	"critical": "!!",
	"high":     "!",
	"normal":   ".",
	"low":      "v",
	"deferred": ">",
}

// priorityIconGlyph wählt den Priority-Glyph (ASCII-Fallback berücksichtigt).
// Unbekannte Prioritäten bekommen den generischen Fallback-Glyph -- der
// zufällig mit dem "normal"-Glyph (·/.) übereinstimmt, da beide denselben
// neutralen Default-Sinn tragen (analog fallbackGlyph für Status/Typ).
func priorityIconGlyph(p string) string {
	glyph, ok := priorityGlyph[p]
	if !ok {
		return fallbackGlyph()
	}
	if asciiIcons() {
		return priorityGlyphASCII[p]
	}
	return glyph
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
