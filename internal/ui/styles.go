package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// activeTheme is the palette every colour name resolves against. It is
// process wide because rendering has no other channel to carry it, and it is
// set once at startup from the config (see SetTheme).
var activeTheme = DefaultTheme()

// typeShorts is the process-wide single-character table, set once at startup
// from the config (see SetTypeShorts). Same reasoning as activeTheme:
// rendering has no other channel to carry it, and internal/ui must not
// depend on pkg/config.
//
// It starts out holding the codes the old hardcoded switch carried, so a
// command that renders before startup wiring runs (or a test that never
// calls SetTypeShorts) still gets "M"/"E"/"F"/"B"/"T" instead of "?".
var typeShorts = defaultTypeShorts()

// defaultTypeShorts returns a fresh copy of the built-in type table.
// SetTypeShorts(nil) resets to this, which is also what lets a test restore
// the table instead of leaking an empty one into every test that runs after.
func defaultTypeShorts() map[string]string {
	return map[string]string{
		"milestone": "M",
		"epic":      "E",
		"feature":   "F",
		"bug":       "B",
		"task":      "T",
	}
}

// SetTypeShorts replaces the single-character table. Passing nil resets it
// to the built-in defaults.
func SetTypeShorts(shorts map[string]string) {
	if shorts == nil {
		typeShorts = defaultTypeShorts()
		return
	}
	typeShorts = make(map[string]string, len(shorts))
	for name, short := range shorts {
		typeShorts[name] = short
	}
}

// statusShorts is the process-wide single-character table for statuses, set
// once at startup from the config (see SetStatusShorts). Same reasoning as
// typeShorts: rendering has no other channel to carry it, and internal/ui
// must not depend on pkg/config.
var statusShorts = defaultStatusShorts()

// defaultStatusShorts returns a fresh copy of the built-in status table.
// SetStatusShorts(nil) resets to this, which is also what lets a test
// restore the table instead of leaking an empty one into every test that
// runs after.
func defaultStatusShorts() map[string]string {
	return map[string]string{
		"draft":       "D",
		"todo":        "T",
		"in-progress": "I",
		"completed":   "C",
		"scrapped":    "S",
	}
}

// SetStatusShorts replaces the single-character table. Passing nil resets it
// to the built-in defaults.
func SetStatusShorts(shorts map[string]string) {
	if shorts == nil {
		statusShorts = defaultStatusShorts()
		return
	}
	statusShorts = make(map[string]string, len(shorts))
	for name, short := range shorts {
		statusShorts[name] = short
	}
}

// prioritySymbols is the process-wide symbol table for priorities, set once
// at startup from the config (see SetPrioritySymbols). Same reasoning as
// typeShorts.
var prioritySymbols = defaultPrioritySymbols()

// defaultPrioritySymbols returns a fresh copy of the built-in priority
// symbol table. SetPrioritySymbols(nil) resets to this. "normal" is
// deliberately absent: it has no symbol and needs no explanation, the same
// as an unconfigured priority.
func defaultPrioritySymbols() map[string]string {
	return map[string]string{
		"critical": "‼",
		"high":     "!",
		"low":      "↓",
		"deferred": "→",
	}
}

// SetPrioritySymbols replaces the priority symbol table. Passing nil resets
// it to the built-in defaults.
func SetPrioritySymbols(symbols map[string]string) {
	if symbols == nil {
		prioritySymbols = defaultPrioritySymbols()
		return
	}
	prioritySymbols = make(map[string]string, len(symbols))
	for name, symbol := range symbols {
		prioritySymbols[name] = symbol
	}
}

// fullTypeColumnWidth is the width the long-form type column claims. It
// follows the longest configured type name instead of a literal.
var fullTypeColumnWidth = 10

// SetTypeColumnWidths sets the short-form and long-form type column widths.
func SetTypeColumnWidths(short, full int) {
	ColWidthType = short
	fullTypeColumnWidth = full
}

// FullTypeColumnWidth returns the long-form type column width.
func FullTypeColumnWidth() int { return fullTypeColumnWidth }

// legacyColorAliases maps colour names that were valid .beans.yml values
// before the Catppuccin switch to the tone that now carries their intent.
// green/yellow/red/blue kept working by coincidence - they already are
// Catppuccin tone names - but gray/grey/purple/cyan are not, so without this
// table a config nobody touched would silently stop resolving those colours,
// and IsValidColor would flip from true to false under it.
var legacyColorAliases = map[string]string{
	"gray":   "overlay1",
	"grey":   "overlay1",
	"purple": "mauve",
	"cyan":   "teal",
}

// Chrome colours: the roles the UI itself needs, as opposed to the semantic
// colours an item carries. Declared without initializers - and kept as vars,
// not consts - so the TUI's existing call sites (internal/tui) keep
// compiling while following the theme through SetTheme. Their values come
// from chromeColorTones via applyChromeColors, the single place each
// colour's tone is named; init and SetTheme both call it rather than each
// repeating the ten assignments.
var (
	ColorPrimary   lipgloss.Color
	ColorSecondary lipgloss.Color
	ColorSuccess   lipgloss.Color
	ColorWarning   lipgloss.Color
	ColorDanger    lipgloss.Color
	ColorMuted     lipgloss.Color
	ColorSubtle    lipgloss.Color
	ColorBlue      lipgloss.Color
	ColorCyan      lipgloss.Color
	ColorID        lipgloss.Color
)

// chromeColorTones is the single declaration of which tone backs each
// chrome colour above; applyChromeColors iterates it.
var chromeColorTones = []struct {
	target *lipgloss.Color
	tone   string
}{
	{&ColorPrimary, "mauve"},
	{&ColorSecondary, "overlay1"},
	{&ColorSuccess, "green"},
	{&ColorWarning, "peach"},
	{&ColorDanger, "red"},
	{&ColorMuted, "overlay1"},
	{&ColorSubtle, "surface2"},
	{&ColorBlue, "blue"},
	{&ColorCyan, "teal"},
	{&ColorID, "subtext0"},
}

// applyChromeColors resolves every chrome colour against the active theme.
func applyChromeColors() {
	for _, c := range chromeColorTones {
		*c.target = ResolveColor(c.tone)
	}
}

// SetTheme switches the active palette and refreshes everything derived from
// it. An unknown name leaves the current theme in place, so a typo in
// .beans.yml degrades to the default instead of stripping all colour.
func SetTheme(name string) {
	t, ok := ThemeByName(name)
	if !ok {
		return
	}
	activeTheme = t
	applyChromeColors()
	rebuildStyles()
}

// ActiveTheme returns the palette currently in force.
func ActiveTheme() Theme { return activeTheme }

// ResolveColor converts a tone name or hex code to a lipgloss.Color. Hex
// passes through untouched, which is what lets .beans.yml override a single
// colour without replacing the whole theme. A legacy colour name (see
// legacyColorAliases) resolves through the tone it aliases.
func ResolveColor(color string) lipgloss.Color {
	if strings.HasPrefix(color, "#") {
		return lipgloss.Color(color)
	}
	name := strings.ToLower(color)
	if tone, ok := legacyColorAliases[name]; ok {
		name = tone
	}
	if hex := activeTheme.Hex(name); hex != "" {
		return lipgloss.Color(hex)
	}
	return lipgloss.Color(activeTheme.Hex("subtext0"))
}

// IsValidColor reports whether a colour is a hex code, a tone of the active
// theme, or a legacy colour name aliased to one (see legacyColorAliases).
func IsValidColor(color string) bool {
	if strings.HasPrefix(color, "#") {
		// Valid hex: #RGB or #RRGGBB. The digits are checked, not just the
		// length -- "#zzz" is the right length and no colour at all, and
		// lipgloss renders it as nothing rather than rejecting it.
		if len(color) != 4 && len(color) != 7 {
			return false
		}
		for _, r := range color[1:] {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
		return true
	}
	name := strings.ToLower(color)
	if tone, ok := legacyColorAliases[name]; ok {
		name = tone
	}
	return activeTheme.Hex(name) != ""
}

// Status badge styles (for inline use, like in show command)
var (
	StatusOpen           lipgloss.Style
	StatusDone           lipgloss.Style
	StatusInProgress     lipgloss.Style
	StatusOpenText       lipgloss.Style
	StatusDoneText       lipgloss.Style
	StatusInProgressText lipgloss.Style
	TagBadge             lipgloss.Style
	Bold                 lipgloss.Style
	Muted                lipgloss.Style
	Primary              lipgloss.Style
	Success              lipgloss.Style
	Warning              lipgloss.Style
	Danger               lipgloss.Style
	Secondary            lipgloss.Style
	ID                   lipgloss.Style
	TreeLine             lipgloss.Style
	Title                lipgloss.Style
	Path                 lipgloss.Style
	Header               lipgloss.Style
)

func init() {
	applyChromeColors()
	rebuildStyles()
}

// rebuildStyles re-derives every style below from the colours currently in
// force. Called once at init and again by SetTheme, since these used to be
// package-level var initializers computed once from a hardcoded palette and
// would otherwise not follow a theme switch.
func rebuildStyles() {
	white := lipgloss.Color("#fff")

	StatusOpen = lipgloss.NewStyle().Foreground(white).Background(ColorSuccess).Padding(0, 1).Bold(true)
	StatusDone = lipgloss.NewStyle().Foreground(white).Background(ColorSecondary).Padding(0, 1)
	StatusInProgress = lipgloss.NewStyle().Foreground(white).Background(ColorWarning).Padding(0, 1).Bold(true)

	StatusOpenText = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	StatusDoneText = lipgloss.NewStyle().Foreground(ColorSecondary)
	StatusInProgressText = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)

	TagBadge = lipgloss.NewStyle().Foreground(lipgloss.Color("#000")).Background(ColorMuted).Padding(0, 1)

	Bold = lipgloss.NewStyle().Bold(true)
	Muted = lipgloss.NewStyle().Foreground(ColorMuted)
	Primary = lipgloss.NewStyle().Foreground(ColorPrimary)
	Success = lipgloss.NewStyle().Foreground(ColorSuccess)
	Warning = lipgloss.NewStyle().Foreground(ColorWarning)
	Danger = lipgloss.NewStyle().Foreground(ColorDanger)
	Secondary = lipgloss.NewStyle().Foreground(ColorSecondary)

	ID = lipgloss.NewStyle().Foreground(ColorID).Bold(true)
	TreeLine = lipgloss.NewStyle().Foreground(ColorSubtle)
	Title = lipgloss.NewStyle().Bold(true)
	Path = lipgloss.NewStyle().Foreground(ColorMuted)
	Header = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).MarginBottom(1)
}

// RenderTag renders a single tag as a badge
func RenderTag(tag string) string {
	return TagBadge.Render(tag)
}

// RenderTags renders multiple tags as badges separated by spaces
func RenderTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	rendered := make([]string, len(tags))
	for i, tag := range tags {
		rendered[i] = RenderTag(tag)
	}
	return strings.Join(rendered, " ")
}

// RenderTagsCompact renders tags for list views with a max count.
// Shows up to maxTags badges, with "+N" indicator if there are more.
// Tags longer than 12 chars are truncated.
func RenderTagsCompact(tags []string, maxTags int) string {
	if len(tags) == 0 {
		return ""
	}
	if maxTags <= 0 {
		maxTags = 1
	}

	showTags := tags
	var extra int
	if len(tags) > maxTags {
		showTags = tags[:maxTags]
		extra = len(tags) - maxTags
	}

	rendered := make([]string, len(showTags))
	for i, tag := range showTags {
		// Truncate long tags
		displayTag := tag
		if len(displayTag) > 12 {
			displayTag = displayTag[:10] + ".."
		}
		rendered[i] = RenderTag(displayTag)
	}

	result := strings.Join(rendered, " ")
	if extra > 0 {
		result += Muted.Render(fmt.Sprintf(" +%d", extra))
	}
	return result
}

// RenderStatus returns a styled status badge based on the status string (legacy, uses hardcoded colors)
func RenderStatus(status string) string {
	switch status {
	case "todo", "draft":
		return StatusOpen.Render(status)
	case "completed", "scrapped":
		return StatusDone.Render(status)
	case "in-progress", "in_progress":
		return StatusInProgress.Render(status)
	default:
		return Muted.Render(status)
	}
}

// RenderStatusText returns styled status text (for tables, no background) (legacy, uses hardcoded colors)
func RenderStatusText(status string) string {
	switch status {
	case "todo", "draft":
		return StatusOpenText.Render(status)
	case "completed", "scrapped":
		return StatusDoneText.Render(status)
	case "in-progress", "in_progress":
		return StatusInProgressText.Render("in-progress")
	default:
		return Muted.Render(status)
	}
}

// RenderStatusWithColor returns a styled status badge using the specified color.
func RenderStatusWithColor(status, color string, isArchiveStatus bool) string {
	c := ResolveColor(color)
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fff")).
		Background(c).
		Padding(0, 1)

	if !isArchiveStatus {
		style = style.Bold(true)
	}

	return style.Render(status)
}

// RenderStatusTextWithColor returns styled status text (for tables) using the specified color.
func RenderStatusTextWithColor(status, color string, isArchiveStatus bool) string {
	c := ResolveColor(color)
	style := lipgloss.NewStyle().Foreground(c)

	if !isArchiveStatus {
		style = style.Bold(true)
	}

	return style.Render(status)
}

// RenderTypeText returns styled type text using the specified color.
// If color is empty, uses muted styling.
func RenderTypeText(typeName, color string) string {
	if typeName == "" {
		return ""
	}
	if color == "" {
		return Muted.Render(typeName)
	}
	c := ResolveColor(color)
	return lipgloss.NewStyle().Foreground(c).Render(typeName)
}

// RenderTypeWithColor returns a styled type badge with colored background.
func RenderTypeWithColor(typeName, color string) string {
	if typeName == "" {
		return ""
	}
	c := ResolveColor(color)
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#fff")).
		Background(c).
		Bold(true).
		Padding(0, 1)
	return style.Render(typeName)
}

// RenderPriorityWithColor returns a styled priority badge using the specified color.
func RenderPriorityWithColor(priority, color string) string {
	if priority == "" {
		return ""
	}
	c := ResolveColor(color)
	style := lipgloss.NewStyle().
		Foreground(c).
		Bold(priority == "critical" || priority == "high")
	return style.Render("[" + priority + "]")
}

// RenderPriorityText returns styled priority text for tables.
func RenderPriorityText(priority, color string) string {
	if priority == "" {
		return ""
	}
	c := ResolveColor(color)
	style := lipgloss.NewStyle().Foreground(c)
	if priority == "critical" || priority == "high" {
		style = style.Bold(true)
	}
	return style.Render(priority)
}

// ShortType returns the single-character code for an item type, from the
// process-wide table (see SetTypeShorts). Falls back to "?" for a type the
// table doesn't carry.
func ShortType(t string) string {
	if s, ok := typeShorts[t]; ok {
		return s
	}
	return "?"
}

// ShortStatus returns a single-character code for the item status, from the
// process-wide table (see SetStatusShorts). Falls back to "?" for a status
// the table doesn't carry — same convention as ShortType.
func ShortStatus(s string) string {
	if v, ok := statusShorts[s]; ok {
		return v
	}
	return "?"
}

// GetPrioritySymbol returns the raw symbol for a priority without styling,
// from the process-wide table (see SetPrioritySymbols). Returns empty
// string for normal/empty priority or a priority the table doesn't carry.
func GetPrioritySymbol(priority string) string {
	return prioritySymbols[priority]
}

// RenderPrioritySymbol returns a compact symbol for priority (used in TUI).
// Returns empty string for normal/empty priority.
func RenderPrioritySymbol(priority, color string) string {
	symbol := GetPrioritySymbol(priority)
	if symbol == "" {
		return ""
	}

	c := ResolveColor(color)
	style := lipgloss.NewStyle().Foreground(c)
	if priority == "critical" || priority == "high" {
		style = style.Bold(true)
	}
	return style.Render(symbol)
}

// ItemRowConfig holds configuration for rendering an item row
type ItemRowConfig struct {
	StatusColor    string
	TypeColor      string
	PriorityColor  string
	Priority       string // Priority value (critical, high, normal, low, deferred)
	IsArchive      bool
	MaxTitleWidth  int  // 0 means no truncation
	ShowCursor     bool // Show selection cursor
	IsSelected     bool
	IsMarked       bool     // Marked for multi-select batch operations
	Tags           []string // Tags to display (optional)
	ShowTags       bool     // Whether to show tags column
	TagsColWidth   int      // Width of tags column (0 = default)
	MaxTags        int      // Max tags to show (0 = default of 1)
	TreePrefix     string   // Tree prefix (e.g., "├─" or "  └─") to prepend to ID
	Dimmed         bool     // Render row dimmed (for unmatched ancestor items in tree)
	IDColWidth     int      // Width of ID column (0 = default of ColWidthID)
	UseFullNames   bool     // Use full type/status names instead of single-char abbreviations
	ImplicitStatus string   // Implicit terminal status from an ancestor (e.g., "scrapped")
}

// Base column widths for item lists (minimum sizes)
const (
	ColWidthID     = 12
	ColWidthStatus = 3
	ColWidthTags   = 24
)

// ColWidthType is the short-form type column width. It is a var, not a
// const, because SetTypeColumnWidths sets it at startup from the merged
// type list (see SetTypeColumnWidths).
var ColWidthType = 3

// ResponsiveColumns holds calculated column widths based on available space
type ResponsiveColumns struct {
	ID                int
	Status            int
	Type              int
	Tags              int
	MaxTags           int // How many tags to show
	ShowTags          bool
	UseFullTypeStatus bool // Use full names instead of single-char abbreviations
}

// CalculateResponsiveColumns determines column widths based on available width.
// Prioritizes title width - tags are only shown when there's plenty of room.
func CalculateResponsiveColumns(totalWidth int, hasTags bool) ResponsiveColumns {
	cols := ResponsiveColumns{
		ID:       ColWidthID,
		Status:   ColWidthStatus,
		Type:     ColWidthType,
		Tags:     0,
		MaxTags:  0,
		ShowTags: false,
	}

	// Use full type/status names when terminal is wide enough
	const minWidthForFullNames = 120
	if totalWidth >= minWidthForFullNames {
		cols.UseFullTypeStatus = true
		cols.Status = 12 // "in-progress" needs 11 chars
		cols.Type = FullTypeColumnWidth()
	}

	// Don't show tags in narrow viewports - prioritize title space
	// Only consider showing tags if terminal is wide enough (140+ columns)
	const minWidthForTags = 140

	if !hasTags || totalWidth < minWidthForTags {
		return cols
	}

	// At this point we have at least 140 columns
	// Base usage: cursor (2) + ID + status + type (use responsive widths)
	cursorWidth := 2
	baseWidth := cursorWidth + cols.ID + cols.Status + cols.Type
	available := totalWidth - baseWidth

	// Reserve generous space for title, then allocate remaining to tags
	tuiMinTitleWidth := 50
	spaceForTags := available - tuiMinTitleWidth

	if spaceForTags >= ColWidthTags {
		cols.ShowTags = true

		if spaceForTags >= 80 {
			// Lots of space: show all tags (up to 5)
			cols.Tags = 70
			cols.MaxTags = 5
		} else if spaceForTags >= 60 {
			// Good space: show 4 tags
			cols.Tags = 55
			cols.MaxTags = 4
		} else if spaceForTags >= 45 {
			// Moderate space: show 3 tags
			cols.Tags = 42
			cols.MaxTags = 3
		} else if spaceForTags >= 35 {
			// Limited space: show 2 tags
			cols.Tags = 32
			cols.MaxTags = 2
		} else {
			// Minimal: show 1 tag
			cols.Tags = ColWidthTags
			cols.MaxTags = 1
		}
	}

	return cols
}

// RenderItemRow renders an item as a single row with ID, Type, Status, Tags (optional), Title
func RenderItemRow(id, status, typeName, title string, cfg ItemRowConfig) string {
	// Column styles - use responsive widths if provided
	idColWidth := ColWidthID
	if cfg.IDColWidth > 0 {
		idColWidth = cfg.IDColWidth
	}
	typeStyle := lipgloss.NewStyle().Width(ColWidthType)
	statusStyle := lipgloss.NewStyle().Width(ColWidthStatus)

	tagsColWidth := ColWidthTags
	if cfg.TagsColWidth > 0 {
		tagsColWidth = cfg.TagsColWidth
	}
	tagsStyle := lipgloss.NewStyle().Width(tagsColWidth)

	maxTags := 1
	if cfg.MaxTags > 0 {
		maxTags = cfg.MaxTags
	}

	// Highlight style for marked rows
	highlightStyle := lipgloss.NewStyle().Foreground(ColorWarning)

	// Build ID column with manual padding
	// (lipgloss Width() doesn't correctly handle Unicode box-drawing characters)
	var idCol string
	// Calculate visual width: tree prefix (in runes) + ID length
	visualWidth := len([]rune(cfg.TreePrefix)) + len(id)
	padding := ""
	if idColWidth > visualWidth {
		padding = strings.Repeat(" ", idColWidth-visualWidth)
	}
	if cfg.Dimmed {
		idCol = Muted.Render(cfg.TreePrefix) + Muted.Render(id) + padding
	} else if cfg.IsMarked {
		// Only highlight the ID when marked
		idCol = highlightStyle.Render(cfg.TreePrefix) + highlightStyle.Render(id) + padding
	} else {
		idCol = TreeLine.Render(cfg.TreePrefix) + ID.Render(id) + padding
	}

	// Type column - single character or full name
	var typeStr string
	if cfg.UseFullNames {
		typeStr = typeName
		typeStyle = typeStyle.Width(FullTypeColumnWidth()) // wider for full names
	} else {
		typeStr = ShortType(typeName)
	}
	var typeCol string
	if cfg.Dimmed {
		typeCol = typeStyle.Render(Muted.Render(typeStr))
	} else {
		typeCol = typeStyle.Render(RenderTypeText(typeStr, cfg.TypeColor))
	}

	// Status column - single character or full name
	var statusStr string
	if cfg.UseFullNames {
		statusStr = status
		statusStyle = statusStyle.Width(12) // wider for full names
	} else {
		statusStr = ShortStatus(status)
	}
	var statusCol string
	if cfg.Dimmed {
		statusCol = statusStyle.Render(Muted.Render(statusStr))
	} else {
		statusCol = statusStyle.Render(RenderStatusTextWithColor(statusStr, cfg.StatusColor, cfg.IsArchive))
	}

	// Tags column (optional)
	var tagsCol string
	if cfg.ShowTags {
		if cfg.Dimmed {
			if len(cfg.Tags) > 0 {
				tagsCol = tagsStyle.Render(Muted.Render(cfg.Tags[0]))
			} else {
				tagsCol = tagsStyle.Render("")
			}
		} else {
			tagsCol = tagsStyle.Render(RenderTagsCompact(cfg.Tags, maxTags))
		}
	}

	// Priority symbol (prepended to title)
	var prioritySymbol string
	if !cfg.Dimmed {
		prioritySymbol = RenderPrioritySymbol(cfg.Priority, cfg.PriorityColor)
		if prioritySymbol != "" {
			prioritySymbol += " "
		}
	}

	// Title (truncate if needed, accounting for priority symbol width)
	displayTitle := title
	titleColWidth := cfg.MaxTitleWidth // Save original for padding
	maxWidth := cfg.MaxTitleWidth
	if maxWidth > 0 && prioritySymbol != "" {
		maxWidth -= 2 // Account for symbol + space
	}
	if maxWidth > 3 && len(title) > maxWidth {
		displayTitle = title[:maxWidth-3] + "..."
	} else if maxWidth > 0 && maxWidth <= 3 && len(title) > maxWidth {
		displayTitle = title[:maxWidth]
	}

	// Cursor and title styling
	var cursor string
	var titleStyled string
	if cfg.ShowCursor {
		if cfg.IsSelected {
			cursor = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("▌")
			titleStyled = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(displayTitle)
		} else {
			cursor = " "
			if cfg.Dimmed {
				titleStyled = Muted.Render(displayTitle)
			} else {
				titleStyled = displayTitle
			}
		}
	} else {
		cursor = ""
		if cfg.Dimmed {
			titleStyled = Muted.Render(displayTitle)
		} else {
			titleStyled = displayTitle
		}
	}

	// Implicit status annotation (muted suffix, only when not dimmed)
	var implicitAnnotation string
	if cfg.ImplicitStatus != "" && !cfg.Dimmed {
		implicitAnnotation = Muted.Render(" ↑" + cfg.ImplicitStatus)
	}

	if cfg.ShowTags {
		// Pad title column to fixed width so tags align in a column
		// Calculate padding needed: titleColWidth - (priority symbol width + title length)
		titleLen := len(displayTitle)
		if prioritySymbol != "" {
			titleLen += 2 // symbol + space
		}
		padding := ""
		if titleColWidth > titleLen {
			padding = strings.Repeat(" ", titleColWidth-titleLen)
		}
		return cursor + idCol + " " + typeCol + " " + statusCol + " " + prioritySymbol + titleStyled + padding + " " + tagsCol + implicitAnnotation
	}
	return cursor + idCol + " " + typeCol + " " + statusCol + " " + prioritySymbol + titleStyled + implicitAnnotation
}
