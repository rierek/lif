package tui

// box_form_settings.go — the Settings-Form (Command-Center "go to
// settings", E5 Task 5, bean bt-0l8c, epic bt-5h4d): edits beans-tui's user
// config (~/.config/beans-tui/config.yaml) directly from the TUI. Port devd
// form_edit_settings.go's field-set/validator shape, reduced to
// design-spec §7/§9's own four fields (repos/editor/accent/tree_width) --
// no ModalWidth/StartProject/Keybindings (not part of beans-tui's
// design-spec, YAGNI here). NO dedicated keybinding (design-spec §7 knows
// none) -- reachable ONLY via overlay_palette.go's "go to settings"
// action (openSettingsForm below). Submit fires LIVE (Port devd DD2-221):
// configuredEditor + theme.SetAccent update immediately, no restart needed
// -- submitForm's "settings" case (box_confirm_create.go) reuses the SAME
// no-Confirm-Gate shape "editTitle" already established. forms_shared.go's
// shared huh-Form-Hosting infra (styleForm/formChrome) is reused unchanged.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rierek/lif/internal/tui/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

var settingsHexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateAccent: empty (= built-in accent) OR a valid #rrggbb hex --
// mirrors config.validateSettings' own regex (theme.go's accentHexRe is the
// THIRD independent copy, by design: the form rejects bad input up front,
// config.validateSettings clamps it a second time on load as a defense-in-
// depth backstop, theme.SetAccent guards a third time against any direct
// caller -- three independent layers, no shared import chain between
// tui/config/theme to keep that boundary).
func validateAccent(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !settingsHexRe.MatchString(s) {
		return fmt.Errorf("expected hex #rrggbb (or empty for default)")
	}
	return nil
}

// validateTreeWidth: an integer in [24,60] -- mirrors config.validateSettings'
// own clamp range (minTreeWidth/maxTreeWidth, settings.go). The form REJECTS
// out-of-range input rather than silently clamping it, so the PO sees the
// constraint instead of a surprising post-save value.
func validateTreeWidth(s string) error {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return fmt.Errorf("expected a number")
	}
	if n < 24 || n > 60 {
		return fmt.Errorf("expected 24-60")
	}
	return nil
}

// reposToLines/linesToRepos convert Settings.Repos <-> the repos Textarea's
// newline-delimited value (one path per line) -- mirrors buildCreateBeanForm's
// own Body field being a plain huh.Text, not a structured list widget.
func reposToLines(repos []string) string {
	return strings.Join(repos, "\n")
}

func linesToRepos(s string) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// buildSettingsForm constructs the Settings-Form, pre-filled from cur (a
// snapshot of m.settings at open time, openSettingsForm below).
func buildSettingsForm(cur config.Settings) *huh.Form {
	repos := reposToLines(cur.Repos)
	editor := cur.Editor
	accent := cur.Theme.Accent
	treeWidth := strconv.Itoa(cur.Layout.TreeWidth)
	return huh.NewForm(huh.NewGroup(
		huh.NewText().Key("repos").Title("repos").
			Description("one repo path per line").Value(&repos).ExternalEditor(false),
		huh.NewInput().Key("editor").Title("editor").
			Description("empty = $VISUAL/$EDITOR/vi").Value(&editor),
		huh.NewInput().Key("accent").Title("accent").
			Description("hex #rrggbb — empty = built-in mauve").Value(&accent).Validate(validateAccent),
		huh.NewInput().Key("tree_width").Title("tree_width").
			Description("tree column width floor (24-60)").Value(&treeWidth).Validate(validateTreeWidth),
	))
}

// openSettingsForm opens the Settings-Form (Command-Center "settings:
// öffnen", overlay_palette.go dispatchPalette). No mutTarget -- settings are
// repo-independent, not a per-bean node action (unlike editTitle/reject).
func (m model) openSettingsForm() (tea.Model, tea.Cmd) {
	m.formKind = "settings"
	m.form = m.styleForm(buildSettingsForm(m.settings))
	return m, m.form.Init()
}
