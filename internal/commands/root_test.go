package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
)

func TestResolveItemsPath(t *testing.T) {
	// Create a valid items directory for tests that need one
	tmpDir := t.TempDir()
	validItemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(validItemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	altItemsDir := filepath.Join(tmpDir, "alt-beans")
	if err := os.MkdirAll(altItemsDir, 0755); err != nil {
		t.Fatalf("failed to create alt beans dir: %v", err)
	}

	// Config that points to the valid items dir
	cfg := config.Default()
	cfg.SetConfigDir(tmpDir)

	t.Run("flag takes highest precedence", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, altItemsDir)

		got, _, err := resolveStorePath(validItemsDir, cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != validItemsDir {
			t.Errorf("expected flag path %q, got %q", validItemsDir, got)
		}
	})

	t.Run("flag overrides env var", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/nonexistent/should/not/be/used")

		got, _, err := resolveStorePath(validItemsDir, cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != validItemsDir {
			t.Errorf("expected flag path %q, got %q", validItemsDir, got)
		}
	})

	t.Run("env var used when flag is empty", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, altItemsDir)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != altItemsDir {
			t.Errorf("expected env var path %q, got %q", altItemsDir, got)
		}
	})

	t.Run("config used when flag and env var are empty", func(t *testing.T) {
		neutralizeStoreEnv(t)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		expected, err := cfg.ResolveItemsPath()
		if err != nil {
			t.Fatalf("ResolveItemsPath() error = %v", err)
		}
		if got != expected {
			t.Errorf("expected config path %q, got %q", expected, got)
		}
	})

	t.Run("invalid flag path returns error", func(t *testing.T) {
		_, _, err := resolveStorePath("/nonexistent/path", cfg)
		if err == nil {
			t.Fatal("expected error for invalid flag path, got nil")
		}
		if !strings.Contains(err.Error(), "does not exist or is not a directory") {
			t.Errorf("expected 'does not exist' error, got %q", err.Error())
		}
	})

	t.Run("invalid env var path returns error", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/nonexistent/env/path")

		_, _, err := resolveStorePath("", cfg)
		if err == nil {
			t.Fatal("expected error for invalid env var path, got nil")
		}
		if !strings.Contains(err.Error(), "does not exist or is not a directory") {
			t.Errorf("expected 'does not exist' error, got %q", err.Error())
		}
	})

	t.Run("invalid config path returns init suggestion", func(t *testing.T) {
		neutralizeStoreEnv(t)

		// Config pointing to a nonexistent directory
		badCfg := config.Default()
		badCfg.SetConfigDir("/nonexistent/config/dir")

		_, _, err := resolveStorePath("", badCfg)
		if err == nil {
			t.Fatal("expected error for invalid config path, got nil")
		}
		if !strings.Contains(err.Error(), "lif init") {
			t.Errorf("expected error to suggest 'lif init', got %q", err.Error())
		}
	})

	t.Run("file path rejected as not a directory", func(t *testing.T) {
		// Create a regular file (not a directory)
		filePath := filepath.Join(tmpDir, "not-a-dir")
		if err := os.WriteFile(filePath, []byte("hello"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, _, err := resolveStorePath(filePath, cfg)
		if err == nil {
			t.Fatal("expected error for file path (not directory), got nil")
		}
		if !strings.Contains(err.Error(), "does not exist or is not a directory") {
			t.Errorf("expected 'does not exist' error, got %q", err.Error())
		}
	})

	// beans-6ap8: a live .beans dir absent because a stageAndSwap crashed
	// mid-swap -- and a non-empty .beans.bak-* sibling holding the real
	// data -- must not be reported as "no store here." Failing this
	// precheck before Core.Load ever runs is exactly what made
	// detectOrphanBackup unreachable from the actual CLI entry point.
	t.Run("missing dir with a recoverable backup sibling is not an error", func(t *testing.T) {
		repoDir := t.TempDir()
		liveItems := filepath.Join(repoDir, ".beans")
		backup := filepath.Join(repoDir, ".beans.bak-123")
		if err := os.MkdirAll(backup, 0755); err != nil {
			t.Fatalf("failed to create backup dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(backup, "beans-abcd--x.md"), []byte("---\ntitle: x\nstatus: todo\n---\n"), 0644); err != nil {
			t.Fatalf("failed to seed backup content: %v", err)
		}

		got, _, err := resolveStorePath(liveItems, cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v, want nil (a recoverable backup should defer to Core.Load, not fail here)", err)
		}
		if got != liveItems {
			t.Errorf("resolveStorePath() = %q, want %q", got, liveItems)
		}
	})
}

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

// initWorktreePair creates a git repo with a secondary worktree, gives both a
// .beans directory, and returns the two resolved roots.
func initWorktreePair(t *testing.T) (mainRoot, worktreeRoot string) {
	t.Helper()

	dir := t.TempDir()
	setup := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}
	for _, args := range setup {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s: %v", args, out, err)
		}
	}

	wt := filepath.Join(t.TempDir(), "secondary")
	cmd := exec.Command("git", "worktree", "add", wt, "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %s: %v", out, err)
	}

	for _, d := range []string{dir, wt} {
		if err := os.MkdirAll(filepath.Join(d, ".beans"), 0755); err != nil {
			t.Fatalf("failed to create .beans in %q: %v", d, err)
		}
	}

	return evalSymlinks(t, dir), evalSymlinks(t, wt)
}

// TestResolveItemsPathAnchor covers the repo-root anchor: the store follows the
// repository the config belongs to, not the working tree the caller happens to
// stand in. This is the capability that BEANS_PATH used to stand in for.
func TestResolveItemsPathAnchor(t *testing.T) {
	mainRoot, worktreeRoot := initWorktreePair(t)

	t.Run("repo-root anchor resolves to the main worktree store", func(t *testing.T) {
		neutralizeStoreEnv(t)

		cfg := config.Default()
		cfg.Items.Anchor = config.AnchorRepoRoot
		cfg.SetConfigDir(worktreeRoot)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(mainRoot, ".beans")
		if got != want {
			t.Errorf("expected main worktree store %q, got %q", want, got)
		}
	})

	t.Run("no anchor keeps the worktree-local store", func(t *testing.T) {
		neutralizeStoreEnv(t)

		cfg := config.Default()
		cfg.SetConfigDir(worktreeRoot)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(worktreeRoot, ".beans")
		if got != want {
			t.Errorf("expected worktree-local store %q, got %q", want, got)
		}
	})

	t.Run("repo-root anchor is a no-op in the main worktree", func(t *testing.T) {
		neutralizeStoreEnv(t)

		cfg := config.Default()
		cfg.Items.Anchor = config.AnchorRepoRoot
		cfg.SetConfigDir(mainRoot)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(mainRoot, ".beans")
		if got != want {
			t.Errorf("expected main store %q, got %q", want, got)
		}
	})

	t.Run("repo-root anchor outside a git repo falls back to the config dir", func(t *testing.T) {
		neutralizeStoreEnv(t)

		plain := t.TempDir()
		if err := os.MkdirAll(filepath.Join(plain, ".beans"), 0755); err != nil {
			t.Fatalf("failed to create .beans: %v", err)
		}

		cfg := config.Default()
		cfg.Items.Anchor = config.AnchorRepoRoot
		cfg.SetConfigDir(plain)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(plain, ".beans")
		if got != want {
			t.Errorf("expected config-dir store %q, got %q", want, got)
		}
	})

	t.Run("env var wins when no config file was found, even with an anchor set", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, filepath.Join(worktreeRoot, ".beans"))

		cfg := config.Default()
		cfg.Items.Anchor = config.AnchorRepoRoot
		cfg.SetConfigDir(worktreeRoot)

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(worktreeRoot, ".beans")
		if got != want {
			t.Errorf("expected env var store %q, got %q", want, got)
		}
	})

	t.Run("anchor from a found config file outranks the env var", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, t.TempDir())

		if err := os.WriteFile(filepath.Join(worktreeRoot, config.ConfigFileName), []byte("beans:\n  path: .beans\n  anchor: repo-root\n"), 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}
		cfg, err := config.LoadFromDirectory(worktreeRoot)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}

		got, _, err := resolveStorePath("", cfg)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		want := filepath.Join(mainRoot, ".beans")
		if got != want {
			t.Errorf("expected anchored store %q, got %q", want, got)
		}
	})

	t.Run("unknown anchor value is rejected", func(t *testing.T) {
		neutralizeStoreEnv(t)

		cfg := config.Default()
		cfg.Items.Anchor = "repo-rooot"
		cfg.SetConfigDir(worktreeRoot)

		_, _, err := resolveStorePath("", cfg)
		if err == nil {
			t.Fatal("expected error for unknown anchor value, got nil")
		}
		if !strings.Contains(err.Error(), "anchor") {
			t.Errorf("expected error to name the anchor setting, got %q", err.Error())
		}
	})
}

// A .beans.yml found by upward search from the cwd is the repository's own
// declaration of where its store lives. An inherited BEANS_PATH -- direnv
// exports it in most of this fleet's workspaces -- must not redirect calls
// made inside such a repository to a foreign store.
func TestResolveItemsPathConfigFileOutranksEnv(t *testing.T) {
	foreignRoot := t.TempDir()
	foreignItems := filepath.Join(foreignRoot, ".beans")
	if err := os.MkdirAll(foreignItems, 0755); err != nil {
		t.Fatalf("failed to create foreign store: %v", err)
	}

	localRoot := t.TempDir()
	localItems := filepath.Join(localRoot, ".beans")
	if err := os.MkdirAll(localItems, 0755); err != nil {
		t.Fatalf("failed to create local store: %v", err)
	}
	configFile := filepath.Join(localRoot, config.ConfigFileName)
	if err := os.WriteFile(configFile, []byte("beans:\n  path: .beans\n  prefix: loc\n"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	t.Run("config file found upward outranks env var", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, foreignItems)

		c, err := config.LoadFromDirectory(localRoot)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}

		got, _, err := resolveStorePath("", c)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != localItems {
			t.Errorf("expected local store %q, got %q", localItems, got)
		}
	})

	t.Run("explicit --config outranks env var", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, foreignItems)

		c, err := config.Load(configFile)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		got, _, err := resolveStorePath("", c)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != localItems {
			t.Errorf("expected local store %q, got %q", localItems, got)
		}
	})

	t.Run("flag still outranks the config file", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, foreignItems)

		c, err := config.LoadFromDirectory(localRoot)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}

		got, _, err := resolveStorePath(foreignItems, c)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != foreignItems {
			t.Errorf("expected flag path %q, got %q", foreignItems, got)
		}
	})

	// Without a config file there is nothing repo-local to honour, so the
	// env var remains the next-best answer.
	t.Run("env var still wins when no config file exists", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, foreignItems)

		bare := t.TempDir()
		if err := os.MkdirAll(filepath.Join(bare, ".beans"), 0755); err != nil {
			t.Fatalf("failed to create bare store: %v", err)
		}

		c, err := config.LoadFromDirectory(bare)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}

		got, _, err := resolveStorePath("", c)
		if err != nil {
			t.Fatalf("resolveStorePath() error = %v", err)
		}
		if got != foreignItems {
			t.Errorf("expected env var store %q, got %q", foreignItems, got)
		}
	})

	// A config file that points at a store which is not there must say so
	// instead of silently falling back to the inherited env var.
	t.Run("config file pointing at a missing store errors, no env fallback", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, foreignItems)

		emptyRoot := t.TempDir()
		if err := os.WriteFile(filepath.Join(emptyRoot, config.ConfigFileName), []byte("beans:\n  path: .beans\n"), 0644); err != nil {
			t.Fatalf("failed to write config file: %v", err)
		}

		c, err := config.LoadFromDirectory(emptyRoot)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}

		_, _, err = resolveStorePath("", c)
		if err == nil {
			t.Fatal("expected an error for a config file pointing at a missing store, got nil")
		}
		if !strings.Contains(err.Error(), "lif init") {
			t.Errorf("expected error to suggest 'lif init', got %q", err.Error())
		}
	})
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

// TestFeedTypeTablesSetsFullWidthFromLongestTypeName drives the actual
// "follows the longest name" computation in feedTypeTables (longest+1),
// which internal/ui's own setter/getter test cannot reach: internal/ui must
// not import pkg/config, so the config-derived fixture has to live here.
// "extraordinarily-long-type-name" is 30 characters, one longer than any
// DefaultTypes entry, so it must drive the merged type list's longest name.
func TestFeedTypeTablesSetsFullWidthFromLongestTypeName(t *testing.T) {
	t.Cleanup(func() {
		ui.SetTypeColumnWidths(3, 10)
		// feedTypeTables also mutates internal/ui's process-wide type-shorts
		// table (ui.SetTypeShorts in root.go); SetTypeShorts(nil) resets it
		// to the built-in baseline, the same restore the styles_test.go
		// tests in internal/ui use for the same table.
		ui.SetTypeShorts(nil)
	})

	const longName = "extraordinarily-long-type-name"
	c := &config.Config{Types: []config.TypeOverride{{Name: longName}}}

	feedTypeTables(c)

	want := len(longName) + 1
	if got := ui.FullTypeColumnWidth(); got != want {
		t.Errorf("FullTypeColumnWidth() = %d, want %d (len(%q)+1)", got, want, longName)
	}
}

// TestFeedStatusPriorityTablesReflectsConfiguredOverrides pins the
// beans-krej fix end to end: a status or priority added or overridden
// through .beans.yml must actually reach internal/ui's process-wide
// tables, not just the merge helpers in pkg/config. Without
// feedStatusPriorityTables wiring this into root.go's PersistentPreRunE,
// ShortStatus/GetPrioritySymbol would keep rendering "?" / nothing for
// anything not in the hardcoded built-in tables forever.
func TestFeedStatusPriorityTablesReflectsConfiguredOverrides(t *testing.T) {
	t.Cleanup(func() {
		ui.SetStatusShorts(nil)
		ui.SetPrioritySymbols(nil)
	})

	c := &config.Config{
		Statuses:   []config.StatusOverride{{Name: "blocked", Short: "X"}},
		Priorities: []config.PriorityOverride{{Name: "urgent", Symbol: "@"}},
	}

	feedStatusPriorityTables(c)

	if got := ui.ShortStatus("blocked"); got != "X" {
		t.Errorf(`ShortStatus("blocked") = %q, want "X"`, got)
	}
	// A built-in status untouched by the override must still resolve from
	// the merged defaults, not disappear because Statuses was non-empty.
	if got := ui.ShortStatus("todo"); got != "T" {
		t.Errorf(`ShortStatus("todo") = %q, want "T"`, got)
	}
	if got := ui.GetPrioritySymbol("urgent"); got != "@" {
		t.Errorf(`GetPrioritySymbol("urgent") = %q, want "@"`, got)
	}
}
