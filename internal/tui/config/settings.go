// Package config persists lif tui's TUI-wide configuration
// (~/.config/lif/tui/config.yaml) and lightweight runtime state
// (~/.config/lif/tui/state.json) -- ONE config directory for BOTH files
// (E5 Task 5, bean bt-0l8c, design decision c), a deliberate deviation from
// devd's split (~/.config/devd-cli/config.yaml vs. ~/.config/dd/state.json,
// Port devd internal/config/{settings,state}.go).
package config

// settings.go — Port devd internal/config/settings.go, reduced to the
// design-spec's own field set (§7/§9: "Settings (~/.config/lif/tui/
// config.yaml: repos, editor, Akzentfarbe, Baumbreite)") -- MINUS devd's
// ModalWidth/StartProject/Keybindings (not part of beans-tui's design-spec,
// YAGNI here per the bean's own acceptance checklist), PLUS Repos []string
// (design-spec §6 US-14: "Liste konfigurierter Repos", T6's Lobby/Repo-Picker
// consumes it). EIN Pfad-Layer (~/.config/lif/tui/config.yaml) -- unlike
// devd's second CWD-override layer (its own settingsPaths()), which is
// deliberately NOT ported (YAGNI, design decision c).
//
// DEVIATION vs. devd's DefaultSettings: Editor defaults to "" here, NOT
// devd's "nvim" -- design decision c is explicit that Settings.Editor's
// default must leave editorBinary()'s pre-existing $VISUAL/$EDITOR/vi
// cascade (editor.go, bean bt-sl45) byte-for-byte unchanged out of the box.
// A "nvim" default would silently override that cascade for every PO who
// has never touched config.yaml.
//
// Example ~/.config/lif/tui/config.yaml:
//
//	repos:
//	  - /Users/po/work/repo-a
//	  - /Users/po/work/repo-b
//	repo_beans_paths:               # optional: repo dir -> explicit BEANS_PATH
//	  /Users/po/work/repo-a: /Users/po/work/shared-store/.beans
//	editor: "code -w"          # empty (default) = $VISUAL/$EDITOR/vi cascade
//	theme:
//	  accent: "#f5a97f"        # empty (default) = built-in Mauve
//	layout:
//	  tree_width: 40           # tree column width floor, 24-60

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Settings is lif tui's TUI configuration (YAML). Zero values mean "not
// set" and fall back to DefaultSettings()/the previous layer on merge.
type Settings struct {
	Repos  []string       `yaml:"repos,omitempty"`
	Editor string         `yaml:"editor,omitempty"`
	Theme  ThemeSettings  `yaml:"theme"`
	Layout LayoutSettings `yaml:"layout"`

	// RepoStorePaths maps a Repos entry (repo dir, exact string match) to
	// an explicit store path the Lobby passes as --lif-path for THAT repo
	// (data.Store.StorePath). Exists for repos whose config `path` cannot
	// resolve the store on its own. Absent entries let lif resolve it.
	RepoStorePaths map[string]string `yaml:"repo_store_paths,omitempty"`

	// LegacyRepoBeansPaths reads the former key `repo_beans_paths`. It is
	// folded into RepoStorePaths on load and never written back.
	LegacyRepoBeansPaths map[string]string `yaml:"repo_beans_paths,omitempty"`
}

// ThemeSettings carries the Accent override (design-spec §9 "Akzentfarbe").
type ThemeSettings struct {
	Accent string `yaml:"accent"` // Hex (#rrggbb) or "" = built-in Mauve
}

// LayoutSettings carries the tree-column width floor (design-spec §9
// "Baumbreite"). Wired into the live render since T6b (bean bt-pd22,
// T5-Review I01): clickPaneGeometry (mouse.go) resolves the model's
// m.settings.Layout.TreeWidth into masterDetailWidths' treeWidthFloor
// parameter (via mouse.go's own treeWidthFloor helper -- 0/unset falls back
// to 24, the value every View function hardcoded before T6b). E5 Task 5
// (bean bt-0l8c) originally only persisted/validated this field; the render
// wiring was its own deliberately-scoped-out follow-up, closed here.
type LayoutSettings struct {
	TreeWidth int `yaml:"tree_width"`
}

// Defaults -- Single Source of the built-in values (= current behavior).
const (
	defTreeWidth = 36
	minTreeWidth = 24
	maxTreeWidth = 60
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// DefaultSettings returns the built-in defaults (apply without a config
// file). Editor/Theme.Accent/Repos default to zero values -- see the
// DEVIATION doc-stamp above for why Editor is "" here, not devd's "nvim".
func DefaultSettings() Settings {
	return Settings{Layout: LayoutSettings{TreeWidth: defTreeWidth}}
}

// configDir resolves ~/.config/lif/tui -- the ONE directory config.yaml
// AND state.json (state.go) both live in (design decision c). Tests inject
// a fake HOME via t.Setenv("HOME", t.TempDir()) -- os.UserHomeDir() reads
// $HOME directly on the platforms bt targets (darwin/linux), so this needs
// no separate package-level override var (unlike configuredEditor, which
// has no environment-variable equivalent to piggyback on).
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lif", "tui"), nil
}

// legacyConfigDir is the former bt config directory. It is read when a file
// is missing in configDir; it is never written or deleted.
func legacyConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "beans-tui"), nil
}

// readUserFile reads name from configDir, falling back to legacyConfigDir
// only when the file does not exist there.
func readUserFile(name string) ([]byte, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if !errors.Is(err, fs.ErrNotExist) {
		return data, err
	}
	legacy, lerr := legacyConfigDir()
	if lerr != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(legacy, name))
}

// normalizeStorePaths folds the legacy repo_beans_paths key into
// RepoStorePaths (the new key wins when set) and clears the legacy field.
func normalizeStorePaths(s *Settings) {
	if len(s.RepoStorePaths) == 0 {
		s.RepoStorePaths = s.LegacyRepoBeansPaths
	}
	s.LegacyRepoBeansPaths = nil
}

// settingsPath returns ~/.config/lif/tui/config.yaml.
func settingsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadSettings loads Defaults -> user config (merged) and validates/clamps
// the result. A missing or unreadable/malformed config.yaml is NOT an error
// (Port devd's own "fehlende Dateien sind kein Fehler" contract) -- Defaults
// apply instead, so a fresh ~/.config/lif/tui (or no $HOME at all) never
// blocks TUI startup. err is only ever non-nil for a genuinely unexpected
// os.UserHomeDir failure.
func LoadSettings() (Settings, error) {
	s := DefaultSettings()
	data, err := readUserFile("config.yaml")
	if err != nil {
		return validateSettings(s), nil // missing -> defaults
	}
	over, err := parseSettings(data)
	if err != nil {
		return validateSettings(s), nil // corrupt YAML -> defaults, never crash
	}
	return validateSettings(mergeSettings(s, over)), nil
}

// SaveUserSettings writes repos/editor/theme.accent/layout.tree_width into
// the user config (design decision c: ONE path, no CWD-override layer).
// Read-modify-write (Port devd SaveUserSettings's own rationale): an
// existing file is read first and unmarshalled into the SAME Settings
// struct this function writes back out -- this preserves a FUTURE Settings
// struct field this task's form does not cover yet (schema growth this repo
// has not shipped), NOT arbitrary foreign config.yaml keys (T6b, bean
// bt-pd22, T5-Review I02 -- an unknown YAML key does not survive this
// typed Unmarshal/Marshal roundtrip; empirically verified against both this
// port and the devd original it mirrors). A yaml.Node-based roundtrip that
// WOULD preserve arbitrary foreign keys is deliberately out of scope (YAGNI
// -- no known caller writes config.yaml by hand today). Directory created
// if missing.
func SaveUserSettings(repos []string, editor, accent string, treeWidth int) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	var s Settings
	if data, rerr := readUserFile("config.yaml"); rerr == nil {
		_ = yaml.Unmarshal(data, &s) // corrupt YAML -> start from zero rather than fail the save
	}
	normalizeStorePaths(&s)
	s.Repos = reposTrimmed(repos)
	s.Editor = editor
	s.Theme.Accent = accent
	s.Layout.TreeWidth = treeWidth
	out, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// parseSettings decodes YAML into Settings (pure, FS-free -- testable).
func parseSettings(data []byte) (Settings, error) {
	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Settings{}, err
	}
	normalizeStorePaths(&s)
	return s, nil
}

// mergeSettings layers over onto base: only SET (non-zero) fields win. An
// empty Repos slice counts as "not set" (zero value), same as an empty
// Editor/Accent string, a zero TreeWidth, or a nil/empty RepoStorePaths map.
func mergeSettings(base, over Settings) Settings {
	if len(over.Repos) > 0 {
		base.Repos = over.Repos
	}
	if over.Editor != "" {
		base.Editor = over.Editor
	}
	if over.Theme.Accent != "" {
		base.Theme.Accent = over.Theme.Accent
	}
	if over.Layout.TreeWidth != 0 {
		base.Layout.TreeWidth = over.Layout.TreeWidth
	}
	if len(over.RepoStorePaths) > 0 {
		base.RepoStorePaths = over.RepoStorePaths
	}
	return base
}

// validateSettings clamps TreeWidth into [minTreeWidth,maxTreeWidth] and
// discards an invalid Accent (not #rrggbb) -> built-in Mauve stays.
func validateSettings(s Settings) Settings {
	s.Layout.TreeWidth = clampInt(s.Layout.TreeWidth, minTreeWidth, maxTreeWidth, defTreeWidth)
	if s.Theme.Accent != "" && !hexColor.MatchString(s.Theme.Accent) {
		s.Theme.Accent = ""
	}
	return s
}

// clampInt keeps v in [lo,hi]; 0/negative (= "not set") -> def.
func clampInt(v, lo, hi, def int) int {
	if v <= 0 {
		return def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// reposTrimmed strips surrounding whitespace from every repo path and drops
// empty entries -- shared by SaveUserSettings' caller (box_form_settings.go's
// linesToRepos) and available here for any future direct config.Settings{}
// construction that wants the same normalization.
func reposTrimmed(repos []string) []string {
	out := make([]string, 0, len(repos))
	for _, r := range repos {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}
