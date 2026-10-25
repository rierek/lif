package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/internal/ui"
)

// evalSymlinks resolves a path so comparisons survive macOS's /var -> /private/var
// symlink: git always reports the resolved path, t.TempDir() does not.
func evalSymlinks(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", p, err)
	}
	return resolved
}

// TestConfiguredThemeReachesTheRenderer verifies the config -> renderer
// chain through the actual entry point: NewRootCmd's PersistentPreRunE, not
// just a manual config.Load()+ui.SetTheme() pairing that would pass whether
// or not root.go itself ever calls ui.SetTheme. It drives the real
// PersistentPreRunE with a config that names the "latte" flavour and checks
// that ui.ActiveTheme() picks it up.
func TestConfiguredThemeReachesTheRenderer(t *testing.T) {
	t.Cleanup(func() { ui.SetTheme("mocha") })

	oldCfg, oldCore, oldConfigPath, oldItemsPath := cfg, core, configPath, storePath
	t.Cleanup(func() { cfg, core, configPath, storePath = oldCfg, oldCore, oldConfigPath, oldItemsPath })

	dir := t.TempDir()
	itemsDir := filepath.Join(dir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test .beans dir: %v", err)
	}
	path := filepath.Join(dir, ".beans.yml")
	body := "beans:\n  path: .beans\n  prefix: t-\ndisplay:\n  theme: latte\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	// NewRootCmd's flag registration (StringVar) resets configPath/itemsPath
	// to their flag defaults as a side effect, so they must be set after
	// building the command, not before.
	root := NewRootCmd()
	configPath = path
	storePath = ""

	if err := root.PersistentPreRunE(root, nil); err != nil {
		t.Fatalf("PersistentPreRunE: %v", err)
	}

	if got := ui.ActiveTheme().Name; got != "latte" {
		t.Errorf(`ActiveTheme() = %q, want "latte"`, got)
	}
}
