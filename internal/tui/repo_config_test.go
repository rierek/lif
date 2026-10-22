package tui

// repo_config_test.go — beans-179g: menus, create form and colours follow the
// project's .lif.yml instead of fixed built-in lists.

import (
	"github.com/charmbracelet/huh"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
)

// applyDirConfig makes the config found at dir the active one, the way the TUI
// does for an opened store.
func applyDirConfig(t *testing.T, dir string) {
	t.Helper()
	cfg, err := config.LoadFromDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	applyRepoConfig(cfg)
}

func useCustomRepoConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfg := "items:\n  prefix: t-\nstatuses:\n  - name: review\n    color: \"#112233\"\ntypes:\n  - name: story\n    rank: 3\npriorities:\n  - name: urgent\n    color: red\n"
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	applyDirConfig(t, dir)
	t.Cleanup(func() { applyRepoConfig(nil) })
}

func TestStatusMenuOffersConfiguredStatuses(t *testing.T) {
	useCustomRepoConfig(t)
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('s'))
	var got []string
	for _, it := range m.menuItems {
		got = append(got, it.value)
	}
	if !strings.Contains(strings.Join(got, ","), "review") {
		t.Fatalf("status menu = %v, want the configured status review", got)
	}
}

func TestCreateFormOffersConfiguredValues(t *testing.T) {
	useCustomRepoConfig(t)
	for name, tc := range map[string]struct {
		opts []string
		want string
	}{
		"status":   {optionValues(statusSelectOptions()), "review"},
		"type":     {optionValues(typeSelectOptions()), "story"},
		"priority": {optionValues(prioritySelectOptions()), "urgent"},
	} {
		if !strings.Contains(strings.Join(tc.opts, ","), tc.want) {
			t.Errorf("%s options = %v, want %q", name, tc.opts, tc.want)
		}
	}
}

func TestConfiguredColoursOverrideBuiltinPalette(t *testing.T) {
	useCustomRepoConfig(t)
	if got, want := theme.StatusColor("review"), ui.ResolveColor("#112233"); got != want {
		t.Errorf("StatusColor(review) = %v, want configured %v", got, want)
	}
	// An entry without an explicit colour keeps the TUI palette.
	if got := theme.StatusColor("todo"); got != theme.Green {
		t.Errorf("StatusColor(todo) = %v, want built-in %v", got, theme.Green)
	}
	// A repo switch to a repo without config drops the overrides.
	applyDirConfig(t, t.TempDir())
	if got := theme.StatusColor("review"); got != theme.Subtext {
		t.Errorf("StatusColor(review) after switch = %v, want fallback %v", got, theme.Subtext)
	}
	if contains := strings.Contains(strings.Join(data.StatusValues(), ","), "review"); contains {
		t.Error("status list kept the previous repo's configured status")
	}
}

func optionValues(opts []huh.Option[string]) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Value
	}
	return out
}
