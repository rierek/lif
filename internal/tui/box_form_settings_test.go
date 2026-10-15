package tui

// box_form_settings_test.go — TDD coverage for the Settings-Form
// (Command-Center "go to settings", E5 Task 5, bean bt-0l8c, epic bt-5h4d):
// prefill from m.settings, submit -> config.SaveUserSettings + LIVE apply
// (configuredEditor + theme.SetAccent, Port devd DD2-221) -- no Confirm-Gate
// (same "editTitle" shape, submitForm's "settings" case, box_confirm_create.go).
// Drives the underlying huh.Form directly via driveForm/advanceFieldsExhausted
// (form_create_bean_test.go) rather than through model.Update -- fast
// (sub-second), same rationale as form_edit_title_test.go: no
// skipSlowHuhDriveInShortMode needed here (that guard is reserved for the
// 7-field Create-Form's model-level round trips, box_confirm_create_test.go's
// own doc comment).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/internal/tui/config"
	"github.com/rierek/lif/internal/tui/theme"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// resetConfiguredEditor guards against configuredEditor (a package-level
// var, editor.go) leaking across tests in this package -- any test that
// drives the Settings-Form to a live submit must restore it, or
// TestEditorBinaryResolvesVisualThenEditorThenVi (or a future test) could
// observe a stale value depending on go test's run order.
func resetConfiguredEditor(t *testing.T) {
	t.Helper()
	orig := configuredEditor
	t.Cleanup(func() { configuredEditor = orig })
}

// resetThemeAccent guards the Golden-Risiko (bean bt-0l8c PFLICHT): any test
// that reaches theme.SetAccent with a REAL accent must restore
// theme.Accent/theme.Header afterwards, or a later golden-snapshot test in
// this SAME package's test binary could render against the wrong accent.
func resetThemeAccent(t *testing.T) {
	t.Helper()
	origAccent, origHeader := theme.Accent, theme.Header
	t.Cleanup(func() {
		theme.Accent, theme.Header = origAccent, origHeader
	})
}

// TestSettingsFormPrefillsCurrentValues guards buildSettingsForm's four
// keyed fields against m.settings' current values, Port devd
// form_edit_settings.go's own buildSettingsForm precedent.
//
// skipSlowHuhDriveInShortMode (box_confirm_create_test.go): advancing
// FOUR keyed fields (repos/editor/accent/tree_width) re-arms huh's own
// self-perpetuating textinput-blink Cmd chain on every field-focus
// transition, same root cause as the 7-field Create-Form's own "Kosten-
// Finding" doc comment -- just proportionally cheaper here (~3-5s per test,
// not ~16-19s) since there are 4 fields, not 7. Measured empirically
// (E5 Task 5, bean bt-0l8c) as still over/near the file's own >5s guard
// threshold, so all three multi-field-driving tests in this file get the
// same guard for a consistent, fast `-short` loop.
func TestSettingsFormPrefillsCurrentValues(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	cur := config.Settings{
		Repos:  []string{"/repo/a", "/repo/b"},
		Editor: "code -w",
		Theme:  config.ThemeSettings{Accent: "#abc123"},
		Layout: config.LayoutSettings{TreeWidth: 42},
	}
	f := buildSettingsForm(cur)
	f, exhausted := advanceFieldsExhausted(f, 4) // repos, editor, accent, tree_width

	requireFieldValue(t, exhausted, "GetString(repos)", f.GetString("repos"), "/repo/a\n/repo/b")
	requireFieldValue(t, exhausted, "GetString(editor)", f.GetString("editor"), "code -w")
	requireFieldValue(t, exhausted, "GetString(accent)", f.GetString("accent"), "#abc123")
	requireFieldValue(t, exhausted, "GetString(tree_width)", f.GetString("tree_width"), "42")
}

// TestSettingsFormValidatesAccentAndTreeWidth guards the two Validate()
// gates (box_form_settings.go): an out-of-range tree_width or a malformed
// accent hex must block huh.StateCompleted, mirroring config.validateSettings'
// own range/regex -- the form REJECTS bad input up front instead of
// silently clamping it post-save.
func TestSettingsFormValidatesAccentAndTreeWidth(t *testing.T) {
	skipSlowHuhDriveInShortMode(t) // see TestSettingsFormPrefillsCurrentValues' doc comment
	badAccent := buildSettingsForm(config.Settings{Theme: config.ThemeSettings{Accent: "nope"}})
	badAccent, _ = advanceFieldsExhausted(badAccent, 4)
	if badAccent.State == huh.StateCompleted {
		t.Error("an invalid accent hex reached StateCompleted, want the accent validator to block it")
	}

	badWidth := buildSettingsForm(config.Settings{Layout: config.LayoutSettings{TreeWidth: 5}})
	badWidth, _ = advanceFieldsExhausted(badWidth, 4)
	if badWidth.State == huh.StateCompleted {
		t.Error("an out-of-range tree_width (5) reached StateCompleted, want the tree_width validator to block it")
	}
}

// TestSettingsFormSubmitSavesAndAppliesLive guards submitForm's "settings"
// case end to end (design decision c's DD2-221 Port): SaveUserSettings
// persists to disk, m.settings/configuredEditor/theme.Accent all update
// LIVE (no restart), and NO Confirm-Gate opens (mirrors "editTitle"'s
// direct-fire shape, box_confirm_create.go).
func TestSettingsFormSubmitSavesAndAppliesLive(t *testing.T) {
	skipSlowHuhDriveInShortMode(t) // see TestSettingsFormPrefillsCurrentValues' doc comment
	resetConfiguredEditor(t)
	resetThemeAccent(t)
	t.Setenv("HOME", t.TempDir())

	cur := config.Settings{
		Repos:  []string{"/repo/a"},
		Editor: "code -w",
		Theme:  config.ThemeSettings{Accent: "#f5a97f"},
		Layout: config.LayoutSettings{TreeWidth: 50},
	}
	f := buildSettingsForm(cur)
	f = advanceFields(f, 4) // settle every field unedited -- submitForm reads GetString verbatim
	if f.State != huh.StateCompleted {
		t.Fatalf("setup: form.State = %v, want StateCompleted after the last field's enter", f.State)
	}

	m := fixtureModel(t, fixtureBeans())
	m.form = f
	m.formKind = "settings"

	tm, cmd := m.submitForm()
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("submitForm() did not return a model, got %T", tm)
	}
	if nm.form != nil {
		t.Fatal("form still open after submitForm, want nil")
	}
	if nm.formKind != "" {
		t.Fatalf("formKind = %q, want empty", nm.formKind)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (settings: no Confirm-Gate, mirrors editTitle)", nm.overlay)
	}
	if cmd != nil {
		t.Fatal("submitForm(settings) returned a non-nil Cmd, want nil (pure local FS write + package-var mutation, no async mutation)")
	}

	// LIVE apply, in-memory:
	if nm.settings.Editor != "code -w" {
		t.Errorf("m.settings.Editor = %q, want %q", nm.settings.Editor, "code -w")
	}
	if nm.settings.Theme.Accent != "#f5a97f" {
		t.Errorf("m.settings.Theme.Accent = %q, want %q", nm.settings.Theme.Accent, "#f5a97f")
	}
	if nm.settings.Layout.TreeWidth != 50 {
		t.Errorf("m.settings.Layout.TreeWidth = %d, want 50", nm.settings.Layout.TreeWidth)
	}
	if len(nm.settings.Repos) != 1 || nm.settings.Repos[0] != "/repo/a" {
		t.Errorf("m.settings.Repos = %v, want [/repo/a]", nm.settings.Repos)
	}
	if configuredEditor != "code -w" {
		t.Errorf("configuredEditor = %q, want %q (DD2-221 live apply)", configuredEditor, "code -w")
	}
	if got := theme.Accent.GetForeground(); got != lipgloss.Color("#f5a97f") {
		t.Errorf("theme.Accent.GetForeground() = %v, want #f5a97f (SetAccent live apply)", got)
	}

	// Persisted to disk, independent of the in-memory model:
	persisted, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("config.LoadSettings() after submit: %v", err)
	}
	if persisted.Editor != "code -w" || persisted.Theme.Accent != "#f5a97f" || persisted.Layout.TreeWidth != 50 {
		t.Errorf("config.LoadSettings() after submit = %+v, want editor=code -w accent=#f5a97f treeWidth=50", persisted)
	}
}

// TestSettingsFormSubmitSaveErrorShowsToast guards submitForm's "settings"
// SaveUserSettings failure branch (box_confirm_create.go:91, bt-81f0's
// Inventar) -- forced deterministically by pre-occupying the config
// DIRECTORY's own path (~/.config/beans-tui) with a plain FILE, so
// config.SaveUserSettings' os.MkdirAll(filepath.Dir(path)) fails (ENOTDIR)
// rather than depending on any real filesystem permission quirk. m.err
// alone used to be the ONLY feedback; post-bt-81f0 it renders nowhere, so a
// Toast must carry it (mirrors every other overlay's vanished-target-guard
// toast assertion, box_confirm_delete_test.go et al.).
func TestSettingsFormSubmitSaveErrorShowsToast(t *testing.T) {
	skipSlowHuhDriveInShortMode(t)
	resetConfiguredEditor(t)
	resetThemeAccent(t)
	home := t.TempDir()
	configParent := filepath.Join(home, ".config")
	if err := os.MkdirAll(configParent, 0o755); err != nil {
		t.Fatalf("setup MkdirAll(%s): %v", configParent, err)
	}
	// A regular FILE at the exact path SaveUserSettings needs as a
	// DIRECTORY (~/.config/beans-tui) -- os.MkdirAll must fail on it.
	if err := os.WriteFile(filepath.Join(configParent, "lif"), []byte("blocker"), 0o644); err != nil {
		t.Fatalf("setup WriteFile blocker: %v", err)
	}
	t.Setenv("HOME", home)

	f := buildSettingsForm(config.Settings{Layout: config.LayoutSettings{TreeWidth: 40}})
	f = advanceFields(f, 4) // settle every field unedited
	if f.State != huh.StateCompleted {
		t.Fatalf("setup: form.State = %v, want StateCompleted", f.State)
	}

	m := fixtureModel(t, fixtureBeans())
	m.form = f
	m.formKind = "settings"

	tm, cmd := m.submitForm()
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("submitForm() did not return a model, got %T", tm)
	}
	// bt-81f0: cmd is no longer nil on the save-error path -- it is now the
	// Toast's own auto-dismiss tick (non-sticky). Not invoked here --
	// toastError's own 8s duration would block the test.
	if cmd == nil {
		t.Fatal("submitForm(settings) save-error must still fire a Cmd (the Toast's own auto-dismiss tick, bt-81f0)")
	}
	if nm.err == "" {
		t.Fatal("submitForm(settings) save-error must set a status-line note (m.err)")
	}
	if nm.toast == nil {
		t.Fatal("submitForm(settings) save-error must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if nm.toast.kind != toastError {
		t.Errorf("toast.kind = %v, want toastError (bt-81f0 bindender Rahmen)", nm.toast.kind)
	}
}

// TestDispatchPaletteSettingsOpensForm guards the Command-Center entry point
// (design-spec §7: NO dedicated keybinding, ONLY reachable via
// overlay_palette.go's "go to settings" action).
func TestDispatchPaletteSettingsOpensForm(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "settings", label: "go to settings"})
	mm, ok := nm.(model)
	if !ok {
		t.Fatalf("dispatchPalette did not return a model, got %T", nm)
	}
	if mm.form == nil || mm.formKind != "settings" {
		t.Fatalf("dispatchPalette(settings) did not open the settings form (form=%v formKind=%q)", mm.form, mm.formKind)
	}
}
