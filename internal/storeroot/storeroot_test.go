package storeroot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

func neutralizeStoreEnv(t *testing.T) {
	t.Helper()
	t.Setenv(config.StoreEnvVar, "")
	t.Setenv(config.LegacyStoreEnvVar, "")
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func requireErrNames(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one naming %q", wants)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q does not name %q", err.Error(), w)
		}
	}
}

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

		got, _, err := Resolve(validItemsDir, cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got != validItemsDir {
			t.Errorf("expected flag path %q, got %q", validItemsDir, got)
		}
	})

	t.Run("flag overrides env var", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/nonexistent/should/not/be/used")

		got, _, err := Resolve(validItemsDir, cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got != validItemsDir {
			t.Errorf("expected flag path %q, got %q", validItemsDir, got)
		}
	})

	t.Run("env var used when flag is empty", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, altItemsDir)

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got != altItemsDir {
			t.Errorf("expected env var path %q, got %q", altItemsDir, got)
		}
	})

	t.Run("config used when flag and env var are empty", func(t *testing.T) {
		neutralizeStoreEnv(t)

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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
		_, _, err := Resolve("/nonexistent/path", cfg)
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

		_, _, err := Resolve("", cfg)
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

		_, _, err := Resolve("", badCfg)
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

		_, _, err := Resolve(filePath, cfg)
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

		got, _, err := Resolve(liveItems, cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v, want nil (a recoverable backup should defer to Core.Load, not fail here)", err)
		}
		if got != liveItems {
			t.Errorf("Resolve() = %q, want %q", got, liveItems)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", cfg)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		_, _, err := Resolve("", cfg)
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

		got, _, err := Resolve("", c)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", c)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve(foreignItems, c)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		got, _, err := Resolve("", c)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
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

		_, _, err = Resolve("", c)
		if err == nil {
			t.Fatal("expected an error for a config file pointing at a missing store, got nil")
		}
		if !strings.Contains(err.Error(), "lif init") {
			t.Errorf("expected error to suggest 'lif init', got %q", err.Error())
		}
	})
}

// Resolve: the env var, default store directory and outcome levels.
func TestResolve_Levels(t *testing.T) {
	loadedCfg := func(t *testing.T, dir, name, content string) *config.Config {
		t.Helper()
		path := filepath.Join(dir, name)
		writeFixtureFile(t, path, content)
		c, err := config.Load(path)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		return c
	}

	t.Run("env var: legacy only is used and recorded", func(t *testing.T) {
		neutralizeStoreEnv(t)
		store := evalSymlinks(t, t.TempDir())
		t.Setenv(config.LegacyStoreEnvVar, store)
		c := config.Default()
		c.SetConfigDir(t.TempDir())

		got, res, err := Resolve("", c)
		if err != nil || got != store {
			t.Fatalf("Resolve() = %q, %v; want %q", got, err, store)
		}
		if !res.LegacyEnv || res.StoreDirSource != "env" {
			t.Errorf("resolution = %+v, want LegacyEnv and source env", res)
		}
	})

	t.Run("env var: new only is used and not legacy", func(t *testing.T) {
		neutralizeStoreEnv(t)
		store := evalSymlinks(t, t.TempDir())
		t.Setenv(config.StoreEnvVar, store)
		c := config.Default()
		c.SetConfigDir(t.TempDir())

		got, res, err := Resolve("", c)
		if err != nil || got != store {
			t.Fatalf("Resolve() = %q, %v; want %q", got, err, store)
		}
		if res.LegacyEnv || res.StoreDirSource != "env" {
			t.Errorf("resolution = %+v, want source env, not legacy", res)
		}
	})

	t.Run("env var: both without flag or config fail naming both", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/a")
		t.Setenv(config.LegacyStoreEnvVar, "/b")
		c := config.Default()
		c.SetConfigDir(t.TempDir())

		_, _, err := Resolve("", c)
		requireErrNames(t, err, config.StoreEnvVar, config.LegacyStoreEnvVar)
	})

	t.Run("env var: a found config outranks the legacy variable", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		t.Setenv(config.LegacyStoreEnvVar, "/must/not/be/used")
		c := loadedCfg(t, dir, config.ConfigFileName, "lif:\n    prefix: a-\n")

		got, res, err := Resolve("", c)
		if err != nil || got != filepath.Join(dir, config.LegacyDefaultStoreDir) {
			t.Fatalf("Resolve() = %q, %v; want the config's default store", got, err)
		}
		if !res.LegacyEnv || res.StoreDirSource != "default" {
			t.Errorf("resolution = %+v, want LegacyEnv recorded and source default", res)
		}
	})

	t.Run("env var: both are not resolved when the flag decides", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/a")
		t.Setenv(config.LegacyStoreEnvVar, "/b")
		store := evalSymlinks(t, t.TempDir())

		got, res, err := Resolve(store, config.Default())
		if err != nil || got != store || res.StoreDirSource != "flag" {
			t.Errorf("Resolve() = %q, %+v, %v; want the flag store, source flag", got, res, err)
		}
	})

	t.Run("env var: both are not resolved when a config file is found", func(t *testing.T) {
		neutralizeStoreEnv(t)
		t.Setenv(config.StoreEnvVar, "/a")
		t.Setenv(config.LegacyStoreEnvVar, "/b")
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
		c := loadedCfg(t, dir, config.ConfigFileName, "lif:\n    prefix: a-\n")

		got, _, err := Resolve("", c)
		if err != nil || got != filepath.Join(dir, config.DefaultStoreDir) {
			t.Errorf("Resolve() = %q, %v; want the config's store", got, err)
		}
	})

	t.Run("default store directory: legacy only", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		c := config.Default()
		c.SetConfigDir(dir)

		got, res, err := Resolve("", c)
		if err != nil || got != filepath.Join(dir, config.LegacyDefaultStoreDir) {
			t.Fatalf("Resolve() = %q, %v", got, err)
		}
		if !res.LegacyStoreDir || res.StoreDirSource != "default" {
			t.Errorf("resolution = %+v, want LegacyStoreDir, source default", res)
		}
	})

	t.Run("default store directory: new only", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
		c := config.Default()
		c.SetConfigDir(dir)

		got, res, err := Resolve("", c)
		if err != nil || got != filepath.Join(dir, config.DefaultStoreDir) {
			t.Fatalf("Resolve() = %q, %v", got, err)
		}
		if res.LegacyStoreDir {
			t.Errorf("resolution = %+v, want LegacyStoreDir false", res)
		}
	})

	t.Run("default store directory: both fail naming both", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
		mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		c := config.Default()
		c.SetConfigDir(dir)

		_, _, err := Resolve("", c)
		requireErrNames(t, err, config.DefaultStoreDir, config.LegacyDefaultStoreDir, dir)
	})

	t.Run("default store directory: neither names the new directory", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		c := config.Default()
		c.SetConfigDir(dir)

		_, _, err := Resolve("", c)
		requireErrNames(t, err, filepath.Join(dir, config.DefaultStoreDir))
	})

	t.Run("an explicit path is honored verbatim next to both default names", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
		mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		c := loadedCfg(t, dir, config.LegacyConfigFileName, "beans:\n    path: "+config.LegacyDefaultStoreDir+"\n")

		got, res, err := Resolve("", c)
		if err != nil || got != filepath.Join(dir, config.LegacyDefaultStoreDir) {
			t.Fatalf("Resolve() = %q, %v", got, err)
		}
		if res.StoreDirSource != "config" || !res.LegacyConfigFile || !res.LegacySectionKey || !res.LegacyStoreDir {
			t.Errorf("resolution = %+v, want source config and all three legacy names", res)
		}
	})

	t.Run("new names record no legacy outcome", func(t *testing.T) {
		neutralizeStoreEnv(t)
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
		c := loadedCfg(t, dir, config.ConfigFileName, "lif:\n    path: "+config.DefaultStoreDir+"\n")

		_, res, err := Resolve("", c)
		if err != nil {
			t.Fatal(err)
		}
		if res.LegacyConfigFile || res.LegacySectionKey || res.LegacyStoreDir || res.LegacyEnv {
			t.Errorf("resolution = %+v, want no legacy flag", res)
		}
	})
}

// The anchored default resolves in the main worktree root, whatever the
// worktree-local directories are called.
func TestResolve_AnchoredDefault(t *testing.T) {
	neutralizeStoreEnv(t)
	mainRoot, worktreeRoot := initWorktreePair(t)
	// initWorktreePair gives both a legacy store; give the main root the new
	// name instead so the anchor's choice is visible.
	if err := os.Remove(filepath.Join(mainRoot, config.LegacyDefaultStoreDir)); err != nil {
		t.Fatal(err)
	}
	mustMkdirAll(t, filepath.Join(mainRoot, config.DefaultStoreDir))

	c := config.Default()
	c.Items.Anchor = config.AnchorRepoRoot
	c.SetConfigDir(worktreeRoot)

	got, _, err := Resolve("", c)
	if err != nil || got != filepath.Join(mainRoot, config.DefaultStoreDir) {
		t.Errorf("Resolve() = %q, %v; want the main root's %s", got, err, config.DefaultStoreDir)
	}

	t.Run("both default names in the main root fail naming both", func(t *testing.T) {
		mustMkdirAll(t, filepath.Join(mainRoot, config.LegacyDefaultStoreDir))
		_, _, err := Resolve("", c)
		requireErrNames(t, err, config.DefaultStoreDir, config.LegacyDefaultStoreDir, mainRoot)
	})
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
