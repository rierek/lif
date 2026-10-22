package ui

import (
	"strings"
	"testing"
)

func TestRenderItemRow_NarrowWidth(t *testing.T) {
	// Test that RenderItemRow doesn't panic with very small MaxTitleWidth values
	// This was a bug where MaxTitleWidth < 4 caused a slice bounds panic

	tests := []struct {
		name          string
		maxTitleWidth int
		title         string
	}{
		{"zero width", 0, "Test Title"},
		{"width 1", 1, "Test Title"},
		{"width 2", 2, "Test Title"},
		{"width 3", 3, "Test Title"},
		{"width 4", 4, "Test Title"},
		{"width 5", 5, "Test Title"},
		{"short title fits", 10, "Hi"},
		{"exact fit", 10, "0123456789"},
		{"needs truncation", 10, "This is a longer title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderBeanRow panicked with MaxTitleWidth=%d: %v", tt.maxTitleWidth, r)
				}
			}()

			cfg := ItemRowConfig{
				MaxTitleWidth: tt.maxTitleWidth,
				StatusColor:   "green",
				TypeColor:     "blue",
			}

			result := RenderItemRow("abc123", "todo", "task", tt.title, cfg)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestRenderItemRow_NarrowWidthWithPriority(t *testing.T) {
	// Priority symbol takes 2 extra chars, which reduces available title width
	// This tests that the adjustment doesn't cause negative slice bounds

	tests := []struct {
		name          string
		maxTitleWidth int
		priority      string
	}{
		{"width 1 with priority", 1, "high"},
		{"width 2 with priority", 2, "high"},
		{"width 3 with priority", 3, "critical"},
		{"width 4 with priority", 4, "high"},
		{"width 5 with priority", 5, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderBeanRow panicked with MaxTitleWidth=%d and priority=%s: %v",
						tt.maxTitleWidth, tt.priority, r)
				}
			}()

			cfg := ItemRowConfig{
				MaxTitleWidth: tt.maxTitleWidth,
				Priority:      tt.priority,
				PriorityColor: "red",
				StatusColor:   "green",
				TypeColor:     "blue",
			}

			result := RenderItemRow("abc123", "todo", "task", "Long title that needs truncation", cfg)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestShortType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"milestone", "M"},
		{"epic", "E"},
		{"bug", "B"},
		{"feature", "F"},
		{"task", "T"},
		{"unknown", "?"},
		{"", "?"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ShortType(tt.input)
			if result != tt.expected {
				t.Errorf("ShortType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Task 7 of docs/topics/beans-type-profiles: the type table moves off a
// hardcoded switch onto a process-wide table fed from config at startup,
// the same pattern SetTheme/activeTheme already use.

func TestShortTypeReadsTheConfiguredTable(t *testing.T) {
	SetTypeShorts(map[string]string{"milestone": "M", "chore": "C"})
	defer SetTypeShorts(nil)

	if got := ShortType("chore"); got != "C" {
		t.Errorf("ShortType(\"chore\") = %q, want \"C\"", got)
	}
	if got := ShortType("milestone"); got != "M" {
		t.Errorf("ShortType(\"milestone\") = %q, want \"M\"", got)
	}
}

func TestShortTypeFallsBackToAQuestionMark(t *testing.T) {
	SetTypeShorts(map[string]string{"task": "T"})
	defer SetTypeShorts(nil)

	if got := ShortType("unheard-of"); got != "?" {
		t.Errorf("ShortType(\"unheard-of\") = %q, want \"?\"", got)
	}
}

// TestSetTypeColumnWidthsStoresBothValues exercises the setter/getter pair
// on their own -- it does not, and cannot, prove that any caller derives 12
// from a longest type name of 11 characters. That derivation lives in
// internal/uiconfig.Apply (longest+1) and is covered separately by
// that package's tests, since internal/ui must not import pkg/config to
// build the config-derived fixture itself.
func TestSetTypeColumnWidthsStoresBothValues(t *testing.T) {
	SetTypeColumnWidths(3, 12)
	defer SetTypeColumnWidths(3, 10)

	if ColWidthType != 3 {
		t.Errorf("ColWidthType = %d, want 3", ColWidthType)
	}
	if FullTypeColumnWidth() != 12 {
		t.Errorf("FullTypeColumnWidth() = %d, want 12", FullTypeColumnWidth())
	}
}

// Task 3: colour names now resolve against the active Catppuccin theme
// instead of a hand-rolled hex palette.

func TestResolveColorUsesTheActiveTheme(t *testing.T) {
	t.Cleanup(func() { SetTheme("mocha") })

	SetTheme("mocha")
	if got := string(ResolveColor("mauve")); got != "#cba6f7" {
		t.Errorf(`mocha mauve = %q, want "#cba6f7"`, got)
	}

	SetTheme("latte")
	if got := string(ResolveColor("mauve")); got != "#8839ef" {
		t.Errorf(`latte mauve = %q, want "#8839ef"`, got)
	}
}

func TestResolveColorPassesHexThrough(t *testing.T) {
	if got := string(ResolveColor("#ff0000")); got != "#ff0000" {
		t.Errorf(`ResolveColor("#ff0000") = %q, want passthrough`, got)
	}
}

func TestSetThemeIgnoresUnknownNames(t *testing.T) {
	t.Cleanup(func() { SetTheme("mocha") })
	SetTheme("mocha")
	SetTheme("nonesuch")
	if got := ActiveTheme().Name; got != "mocha" {
		t.Errorf(`ActiveTheme() = %q after an unknown name, want "mocha"`, got)
	}
}

func TestUnknownToneFallsBackToSubtext0(t *testing.T) {
	t.Cleanup(func() { SetTheme("mocha") })
	SetTheme("mocha")
	// An unknown tone must stay legible rather than vanish into the background.
	if got := string(ResolveColor("chartreuse")); got != "#a6adc8" {
		t.Errorf(`unknown tone = %q, want mocha subtext0 "#a6adc8"`, got)
	}
}

func TestSetThemeRefreshesDerivedStyles(t *testing.T) {
	t.Cleanup(func() { SetTheme("mocha") })
	SetTheme("mocha")
	before := ColorPrimary
	SetTheme("latte")
	if ColorPrimary == before {
		t.Error("ColorPrimary did not follow the theme switch")
	}
}

// Fix round 1: NamedColors used to be the set of colour names a user was
// allowed to write in .beans.yml. green/yellow/red/blue kept working after
// the Catppuccin switch because they happen to already be tone names; these
// four did not, and needed an explicit alias so an existing "color: purple"
// (or the literal "gray" several commands still pass as a sentinel) keeps
// resolving instead of silently degrading.

func TestLegacyColorAliasesResolveToTheirCatppuccinTone(t *testing.T) {
	t.Cleanup(func() { SetTheme("mocha") })
	SetTheme("mocha")

	aliases := map[string]string{
		"gray":   "overlay1",
		"grey":   "overlay1",
		"purple": "mauve",
		"cyan":   "teal",
	}
	for alias, tone := range aliases {
		want := ResolveColor(tone)
		if got := ResolveColor(alias); got != want {
			t.Errorf("ResolveColor(%q) = %q, want %q (alias for tone %q)", alias, got, want, tone)
		}
		upper := strings.ToUpper(alias)
		if got := ResolveColor(upper); got != want {
			t.Errorf("ResolveColor(%q) = %q, want %q (alias for tone %q)", upper, got, want, tone)
		}
	}
}

func TestLegacyColorAliasesAreValid(t *testing.T) {
	for _, alias := range []string{"gray", "grey", "purple", "cyan", "GRAY", "Purple", "CYAN"} {
		if !IsValidColor(alias) {
			t.Errorf("IsValidColor(%q) = false, want true: it is a legacy alias", alias)
		}
	}
}

func TestUnknownColorNameIsNeitherAliasNorValid(t *testing.T) {
	// The alias table must not become a catch-all: a genuine typo still
	// falls through to the unknown-tone fallback and reports invalid.
	if IsValidColor("chartreuse") {
		t.Error(`IsValidColor("chartreuse") = true, want false`)
	}
}

func TestShortStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"draft", "D"},
		{"todo", "T"},
		{"in-progress", "I"},
		{"completed", "C"},
		{"scrapped", "S"},
		{"unknown", "?"},
		{"", "?"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ShortStatus(tt.input)
			if result != tt.expected {
				t.Errorf("ShortStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestShortStatusReadsTheConfiguredTable mirrors
// TestShortTypeReadsTheConfiguredTable: a status added through config must
// render its configured code, not fall back to "?" forever (beans-krej).
func TestShortStatusReadsTheConfiguredTable(t *testing.T) {
	SetStatusShorts(map[string]string{"draft": "D", "blocked": "B"})
	defer SetStatusShorts(nil)

	if got := ShortStatus("blocked"); got != "B" {
		t.Errorf("ShortStatus(\"blocked\") = %q, want \"B\"", got)
	}
	if got := ShortStatus("draft"); got != "D" {
		t.Errorf("ShortStatus(\"draft\") = %q, want \"D\"", got)
	}
}

func TestShortStatusFallsBackToAQuestionMark(t *testing.T) {
	SetStatusShorts(map[string]string{"todo": "T"})
	defer SetStatusShorts(nil)

	if got := ShortStatus("unheard-of"); got != "?" {
		t.Errorf("ShortStatus(\"unheard-of\") = %q, want \"?\"", got)
	}
}

// TestGetPrioritySymbolReadsTheConfiguredTable: a priority added through
// config must render its configured symbol instead of being dropped from
// the legend (beans-krej).
func TestGetPrioritySymbolReadsTheConfiguredTable(t *testing.T) {
	SetPrioritySymbols(map[string]string{"critical": "‼", "urgent": "★"})
	defer SetPrioritySymbols(nil)

	if got := GetPrioritySymbol("urgent"); got != "★" {
		t.Errorf("GetPrioritySymbol(\"urgent\") = %q, want \"★\"", got)
	}
	if got := GetPrioritySymbol("critical"); got != "‼" {
		t.Errorf("GetPrioritySymbol(\"critical\") = %q, want \"‼\"", got)
	}
}

func TestGetPrioritySymbolFallsBackToEmpty(t *testing.T) {
	SetPrioritySymbols(map[string]string{"critical": "‼"})
	defer SetPrioritySymbols(nil)

	if got := GetPrioritySymbol("normal"); got != "" {
		t.Errorf("GetPrioritySymbol(\"normal\") = %q, want empty", got)
	}
	if got := GetPrioritySymbol("unheard-of"); got != "" {
		t.Errorf("GetPrioritySymbol(\"unheard-of\") = %q, want empty", got)
	}
}

func TestRenderItemRowSurvivesForTheTUI(t *testing.T) {
	// The TUI renders through this. It must keep working, and it must follow
	// the theme without being rewritten.
	row := RenderItemRow("beans-abcd", "todo", "task", "A title", ItemRowConfig{
		StatusColor: "green", TypeColor: "blue", MaxTitleWidth: 40,
	})
	if row == "" {
		t.Fatal("RenderBeanRow returned nothing")
	}
}

func TestRenderItemRowFollowsTheTheme(t *testing.T) {
	// Force a real colour profile: under `go test` there is no tty, so
	// lipgloss strips all colour and mocha/latte would render identically
	// regardless of whether the code follows the theme at all (see
	// withTrueColor in markdown_test.go).
	withTrueColor(t)
	prev := ActiveTheme().Name
	t.Cleanup(func() { SetTheme(prev) })

	SetTheme("mocha")
	mocha := RenderItemRow("beans-abcd", "todo", "task", "A title", ItemRowConfig{
		StatusColor: "green", TypeColor: "blue", MaxTitleWidth: 40,
	})

	SetTheme("latte")
	latte := RenderItemRow("beans-abcd", "todo", "task", "A title", ItemRowConfig{
		StatusColor: "green", TypeColor: "blue", MaxTitleWidth: 40,
	})

	if mocha == latte {
		t.Error("RenderBeanRow did not follow the theme switch")
	}
}

func TestCalculateResponsiveColumnsStillExists(t *testing.T) {
	if got := CalculateResponsiveColumns(140, true); got.ID == 0 {
		t.Error("CalculateResponsiveColumns must stay for the TUI")
	}
}

func TestCalculateResponsiveColumnsIgnoresTheme(t *testing.T) {
	// Widths are not a colour concern: a theme switch must not change
	// which columns get shown or how wide they are.
	prev := ActiveTheme().Name
	t.Cleanup(func() { SetTheme(prev) })

	SetTheme("mocha")
	mocha := CalculateResponsiveColumns(140, true)

	SetTheme("latte")
	latte := CalculateResponsiveColumns(140, true)

	if mocha != latte {
		t.Errorf("CalculateResponsiveColumns followed the theme: mocha=%+v latte=%+v", mocha, latte)
	}
}

// TestIsValidColorRejectsNonHexDigits pins a gap the final review's check
// finding exposed: the hex branch only measured LENGTH, so "#zzz" and
// "#gggggg" were accepted as valid colours. They reach lipgloss as a colour
// it cannot parse, and `lif check` -- whose whole job is to catch a broken
// .beans.yml before it confuses someone -- reported them as fine.
//
// Mutation: restore `return len(color) == 4 || len(color) == 7` and the
// invalid cases below go green.
func TestIsValidColorRejectsNonHexDigits(t *testing.T) {
	valid := []string{"#abc", "#ABC", "#a1b2c3", "#FFFFFF", "#000"}
	invalid := []string{"#zzz", "#gggggg", "#12g", "#", "#ab", "#abcd", "#12345", "#1234567"}

	for _, c := range valid {
		if !IsValidColor(c) {
			t.Errorf("IsValidColor(%q) = false, want true", c)
		}
	}
	for _, c := range invalid {
		if IsValidColor(c) {
			t.Errorf("IsValidColor(%q) = true, want false", c)
		}
	}
}

func TestNormalizeColor(t *testing.T) {
	tests := map[string]string{
		"green":     "green",
		"Green":     "green",
		" sky ":     "sky",
		"purple":    "mauve",
		"GRAY":      "overlay1",
		"#AABBCC":   "#aabbcc",
		"#abc":      "#abc",
		" #FF0000 ": "#ff0000",
		"#zzz":      "",
		"#abcd":     "",
		"nope":      "",
		"":          "",
	}
	for in, want := range tests {
		if got := NormalizeColor(in); got != want {
			t.Errorf("NormalizeColor(%q) = %q, want %q", in, got, want)
		}
	}
}
