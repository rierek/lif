package commands

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/storeroot"
	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

// Legacy-name resolution at command level (R-05..R-07, R-09, R-12). The names
// come from the production constants; every assertion goes through the
// production resolvers or the real root command.

// neutralizeStoreEnv clears both store-path variables for the test. A test
// that touches the store env var must neutralize both, or a developer's
// exported LIF_PATH / BEANS_PATH leaks into the result.
func neutralizeStoreEnv(t *testing.T) {
	t.Helper()
	t.Setenv(config.StoreEnvVar, "")
	t.Setenv(config.LegacyStoreEnvVar, "")
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

const legacyItemFile = `---
title: First
status: todo
type: task
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
---

Body of the first item.
`

// legacyProject builds what baseline `beans init` made: a .beans.yml with key
// beans: and path: .beans, and a .beans store holding two items.
func legacyProject(t *testing.T) string {
	t.Helper()
	dir := evalSymlinks(t, t.TempDir())
	writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName),
		"beans:\n    path: "+config.LegacyDefaultStoreDir+"\n    prefix: leg-\n    id_length: 4\n    default_status: todo\n    default_type: task\n")
	writeFixtureFile(t, filepath.Join(dir, config.LegacyDefaultStoreDir, "leg-aaa1--first.md"), legacyItemFile)
	writeFixtureFile(t, filepath.Join(dir, config.LegacyDefaultStoreDir, "leg-bbb2--second.md"),
		strings.Replace(legacyItemFile, "First", "Second", 1))
	return dir
}

// runInProject runs the shared root command with dir as working directory and
// no store flag, the way a user in a project directory does.
func runInProject(t *testing.T, dir string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	neutralizeStoreEnv(t)
	t.Chdir(dir)

	root := sharedTestRoot(t)
	// Other tests register commands (list, milestones, roadmap, order) on
	// throwaway roots, which re-parents the package-level command objects:
	// from then on they run under that root's hooks, not the shared root's.
	// Take them back, so PersistentPreRunE really runs for every command here.
	cmds := append([]*cobra.Command(nil), root.Commands()...)
	root.RemoveCommand(cmds...)
	root.AddCommand(cmds...)
	resetFlags(root)

	// The shared command objects keep whatever out stream an earlier test set
	// on them; commands that print through cmd.OutOrStdout would otherwise
	// land there. Route every command to this run's buffers and give the
	// streams back afterwards, so no other test finds them redirected.
	var outBuf, errBuf bytes.Buffer
	var route func(c *cobra.Command, out, errW io.Writer)
	route = func(c *cobra.Command, out, errW io.Writer) {
		c.SetOut(out)
		c.SetErr(errW)
		for _, sub := range c.Commands() {
			route(sub, out, errW)
		}
	}
	route(root, &outBuf, &errBuf)
	t.Cleanup(func() { route(root, nil, nil); root.SetArgs(nil) })
	root.SetArgs(args)

	out := captureTagStdout(t, func() {
		var cmd *cobra.Command
		cmd, err = root.ExecuteC()
		if err != nil {
			reportExecutionError(cmd, err)
		}
	})
	return outBuf.String() + string(out), errBuf.String(), err
}

// itemFileHashes maps the name of every item file in storeDir to the hash of
// its bytes.
func itemFileHashes(t *testing.T, storeDir string) map[string]string {
	t.Helper()
	hashes := map[string]string{}
	entries, err := os.ReadDir(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(storeDir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hashes[e.Name()] = hex.EncodeToString(sum[:])
	}
	return hashes
}

func sortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func requireAbsent(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s exists in %s (err = %v), want it absent", n, dir, err)
		}
	}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
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

// R-12 / SC-01 / SC-02: on a project made by baseline `beans init`, the read
// commands, `check` and `init` succeed without a manual step and leave every
// item file (name and bytes) as it was; no lif name appears.
func TestLegacyProject_ReadCommandsAndInitLeaveItemFilesAlone(t *testing.T) {
	cases := [][]string{
		{"list"},
		{"show", "leg-aaa1"},
		{"path"},
		{"check"},
		{"check", "--json"},
		{"check", "--strict"},
		{"list", "-S", "First"},
		{"prime"},
		{"init", "--force"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			home := isolateHome(t)
			// A fresh copy per command, so one command's effect on an item
			// cannot hide behind or leak into the next.
			dir := legacyProject(t)
			store := filepath.Join(dir, config.LegacyDefaultStoreDir)
			before := itemFileHashes(t, store)
			if len(before) != 2 {
				t.Fatalf("fixture holds %d items, want 2", len(before))
			}

			stdout, stderr, err := runInProject(t, dir, args...)
			if err != nil {
				t.Fatalf("lif %v error = %v\nstderr: %s", args, err, stderr)
			}
			switch args[0] {
			case "list":
				if len(args) > 1 {
					// the index is in memory: a search writes nothing under HOME and nothing next to the items
					if !strings.Contains(stdout, "leg-aaa1") {
						t.Errorf("list %v does not find the first item: %q", args[1:], stdout)
					}
					if _, err := os.Stat(filepath.Join(home, ".lif", "index")); !os.IsNotExist(err) {
						t.Errorf("search left an index under ~/.lif/index (stat err = %v)", err)
					}
					if _, err := os.Stat(filepath.Join(home, ".beans")); !os.IsNotExist(err) {
						t.Errorf("search touched ~/.beans (stat err = %v)", err)
					}
					break
				}
				if !strings.Contains(stdout, "leg-aaa1") || !strings.Contains(stdout, "leg-bbb2") {
					t.Errorf("list does not show the legacy items: %q", stdout)
				}
			case "path":
				if got := strings.TrimSpace(stdout); got != store {
					t.Errorf("path = %q, want %q", got, store)
				}
			case "prime":
				if strings.TrimSpace(stdout) == "" {
					t.Error("prime printed nothing: the legacy config file was not found")
				}
			case "init":
				loaded, err := config.Load(filepath.Join(dir, config.LegacyConfigFileName))
				if err != nil {
					t.Fatalf("Load() after init error = %v", err)
				}
				if loaded.SectionKey() != config.LegacySectionKey || loaded.Items.Path != config.LegacyDefaultStoreDir {
					t.Errorf("config after init: key %q, path %q; want %q and %q",
						loaded.SectionKey(), loaded.Items.Path, config.LegacySectionKey, config.LegacyDefaultStoreDir)
				}
			}

			after := itemFileHashes(t, store)
			if strings.Join(sortedNames(after), ",") != strings.Join(sortedNames(before), ",") {
				t.Errorf("item file names changed: %v -> %v", sortedNames(before), sortedNames(after))
			}
			for name, sum := range before {
				if after[name] != sum {
					t.Errorf("%s changed after lif %v", name, args)
				}
			}
			requireAbsent(t, dir, config.DefaultStoreDir, config.ConfigFileName)
		})
	}
}

func TestLegacyProject_CreateAndUpdateWorkInPlace(t *testing.T) {
	dir := legacyProject(t)
	store := filepath.Join(dir, config.LegacyDefaultStoreDir)

	if _, stderr, err := runInProject(t, dir, "create", "Third item"); err != nil {
		t.Fatalf("create error = %v\nstderr: %s", err, stderr)
	}
	if got := len(itemFileHashes(t, store)); got != 3 {
		t.Errorf("store holds %d items after create, want 3", got)
	}
	if _, stderr, err := runInProject(t, dir, "update", "leg-aaa1", "-s", "in-progress"); err != nil {
		t.Fatalf("update error = %v\nstderr: %s", err, stderr)
	}
	data, err := os.ReadFile(filepath.Join(store, "leg-aaa1--first.md"))
	if err != nil || !strings.Contains(string(data), "status: in-progress") {
		t.Errorf("update did not write the legacy item in place: %v\n%s", err, data)
	}
	requireAbsent(t, dir, config.DefaultStoreDir, config.ConfigFileName)
}

// Risk "store split through Save": a prefix rebrand on a legacy project saves
// the config. It must stay in .beans.yml under its key, never appear as
// .lif.yml next to it.
func TestLegacyProject_RenamePrefixSavesInPlace(t *testing.T) {
	dir := legacyProject(t)
	if _, stderr, err := runInProject(t, dir, "rename", "--prefix", "neu-", "--yes"); err != nil {
		t.Fatalf("rename --prefix error = %v\nstderr: %s", err, stderr)
	}
	requireAbsent(t, dir, config.ConfigFileName, config.DefaultStoreDir)
	loaded, err := config.Load(filepath.Join(dir, config.LegacyConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Items.Prefix != "neu-" || loaded.SectionKey() != config.LegacySectionKey {
		t.Errorf("config = prefix %q, key %q; want neu- under %q", loaded.Items.Prefix, loaded.SectionKey(), config.LegacySectionKey)
	}
	if _, _, err := runInProject(t, dir, "list"); err != nil {
		t.Errorf("list after rename error = %v, want the project to keep resolving", err)
	}
}

// D22: prime stays silent and exits 0 whatever is wrong with the config.
func TestPrime_StaysSilentOnAmbiguousNames(t *testing.T) {
	cases := map[string]func(t *testing.T, dir string){
		"both config files": func(t *testing.T, dir string) {
			writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n    prefix: a-\n")
			writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName), "beans:\n    prefix: b-\n")
		},
		"both section keys": func(t *testing.T, dir string) {
			writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n    prefix: a-\nbeans:\n    prefix: b-\n")
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := evalSymlinks(t, t.TempDir())
			setup(t, dir)
			stdout, _, err := runInProject(t, dir, "prime")
			if err != nil {
				t.Errorf("prime error = %v, want exit 0", err)
			}
			if stdout != "" {
				t.Errorf("prime printed %q, want nothing", stdout)
			}
		})
	}
}

// R-13 flag part: --lif-path is the store flag; --beans-path is gone without
// an alias (SC-04).
func TestRootFlag_LifPathReplacesBeansPath(t *testing.T) {
	dir := legacyProject(t)
	store := filepath.Join(dir, config.LegacyDefaultStoreDir)
	elsewhere := evalSymlinks(t, t.TempDir())

	t.Run("--lif-path lists the store at the given directory", func(t *testing.T) {
		stdout, stderr, err := runInProject(t, elsewhere, "--lif-path", store, "list")
		if err != nil {
			t.Fatalf("list --lif-path error = %v\nstderr: %s", err, stderr)
		}
		if !strings.Contains(stdout, "leg-aaa1") {
			t.Errorf("list does not show the store at --lif-path:\n%s", stdout)
		}
	})

	t.Run("--beans-path is rejected as an unknown flag", func(t *testing.T) {
		_, _, err := runInProject(t, elsewhere, "--beans-path", store, "list")
		requireErrNames(t, err, "unknown flag", "--beans-path")
	})

	t.Run("completion requests read --lif-path", func(t *testing.T) {
		flagVal, _ := completionAwareRootFlags(&cobra.Command{Use: cobra.ShellCompRequestCmd}, []string{"--lif-path", store, "show", ""})
		if flagVal != store {
			t.Errorf("completionAwareRootFlags() store = %q, want %q", flagVal, store)
		}
		oldVal, _ := completionAwareRootFlags(&cobra.Command{Use: cobra.ShellCompRequestCmd}, []string{"--beans-path", store, "show", ""})
		if oldVal != "" {
			t.Errorf("completionAwareRootFlags() read the removed --beans-path: %q", oldVal)
		}
	})
}

// AC-6/7/8: init in an empty, a legacy and an ambiguous directory.
func TestInit_NamesByDirectoryState(t *testing.T) {
	t.Run("empty directory creates only lif names", func(t *testing.T) {
		dir := evalSymlinks(t, t.TempDir())
		if _, stderr, err := runInProject(t, dir, "init"); err != nil {
			t.Fatalf("init error = %v\nstderr: %s", err, stderr)
		}
		if got := strings.Join(dirNames(t, dir), ","); got != config.DefaultStoreDir+","+config.ConfigFileName {
			t.Errorf("directory = %s, want only %s and %s", got, config.DefaultStoreDir, config.ConfigFileName)
		}
		loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.SectionKey() != config.SectionKey || loaded.Items.Path != config.DefaultStoreDir {
			t.Errorf("config: key %q, path %q; want %q and %q", loaded.SectionKey(), loaded.Items.Path, config.SectionKey, config.DefaultStoreDir)
		}
		raw, _ := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
		if strings.Contains(string(raw), config.LegacySectionKey+":") {
			t.Errorf("new config carries the legacy key:\n%s", raw)
		}
	})

	t.Run("legacy project is re-initialized in place", func(t *testing.T) {
		dir := legacyProject(t)
		before := dirNames(t, dir)
		if _, stderr, err := runInProject(t, dir, "init", "--force"); err != nil {
			t.Fatalf("init error = %v\nstderr: %s", err, stderr)
		}
		if got := dirNames(t, dir); strings.Join(got, ",") != strings.Join(before, ",") {
			t.Errorf("directory = %v, want the legacy names only: %v", got, before)
		}
	})

	t.Run("a legacy store without config gets a new config that points at it", func(t *testing.T) {
		dir := evalSymlinks(t, t.TempDir())
		mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		if _, stderr, err := runInProject(t, dir, "init"); err != nil {
			t.Fatalf("init error = %v\nstderr: %s", err, stderr)
		}
		requireAbsent(t, dir, config.DefaultStoreDir)
		loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if loaded.Items.Path != config.LegacyDefaultStoreDir {
			t.Errorf("path = %q, want %q (the store that is there)", loaded.Items.Path, config.LegacyDefaultStoreDir)
		}
	})

	// A directory with only a legacy config file is a project with only
	// legacy names: init creates the legacy store beside it, as baseline
	// `beans init` did, and no lif name appears.
	for name, content := range map[string]string{
		"default path":  "beans:\n    prefix: old-\n",
		"explicit path": "beans:\n    path: ../store\n    prefix: old-\n",
	} {
		t.Run("only a legacy config file ("+name+") gets the legacy store", func(t *testing.T) {
			dir := evalSymlinks(t, t.TempDir())
			writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName), content)
			if _, stderr, err := runInProject(t, dir, "init", "--force"); err != nil {
				t.Fatalf("init error = %v\nstderr: %s", err, stderr)
			}
			if got := strings.Join(dirNames(t, dir), ","); got != config.LegacyDefaultStoreDir+","+config.LegacyConfigFileName {
				t.Errorf("directory = %s, want only %s and %s", got, config.LegacyDefaultStoreDir, config.LegacyConfigFileName)
			}
			loaded, err := config.Load(filepath.Join(dir, config.LegacyConfigFileName))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if loaded.SectionKey() != config.LegacySectionKey || loaded.Items.Path != config.LegacyDefaultStoreDir {
				t.Errorf("config: key %q, path %q; want %q and %q", loaded.SectionKey(), loaded.Items.Path, config.LegacySectionKey, config.LegacyDefaultStoreDir)
			}
		})
	}

	t.Run("only a new config file gets the new store", func(t *testing.T) {
		dir := evalSymlinks(t, t.TempDir())
		writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n    prefix: new-\n")
		if _, stderr, err := runInProject(t, dir, "init", "--force"); err != nil {
			t.Fatalf("init error = %v\nstderr: %s", err, stderr)
		}
		if got := strings.Join(dirNames(t, dir), ","); got != config.DefaultStoreDir+","+config.ConfigFileName {
			t.Errorf("directory = %s, want only %s and %s", got, config.DefaultStoreDir, config.ConfigFileName)
		}
	})

	ambiguous := map[string]struct {
		setup func(t *testing.T, dir string)
		names []string
	}{
		"both config files": {func(t *testing.T, dir string) {
			writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n")
			writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName), "beans:\n")
		}, []string{config.ConfigFileName, config.LegacyConfigFileName}},
		"both store directories": {func(t *testing.T, dir string) {
			mustMkdirAll(t, filepath.Join(dir, config.DefaultStoreDir))
			mustMkdirAll(t, filepath.Join(dir, config.LegacyDefaultStoreDir))
		}, []string{config.DefaultStoreDir, config.LegacyDefaultStoreDir}},
		"both section keys": {func(t *testing.T, dir string) {
			writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName), "lif:\n    prefix: a-\nbeans:\n    prefix: b-\n")
		}, []string{config.SectionKey + ":", config.LegacySectionKey + ":"}},
	}
	for name, tc := range ambiguous {
		t.Run("ambiguous: "+name+" fails before anything is written", func(t *testing.T) {
			dir := evalSymlinks(t, t.TempDir())
			tc.setup(t, dir)
			before := dirNames(t, dir)
			_, _, err := runInProject(t, dir, "init")
			requireErrNames(t, err, tc.names...)
			if got := dirNames(t, dir); strings.Join(got, ",") != strings.Join(before, ",") {
				t.Errorf("directory changed: %v -> %v", before, got)
			}
			for _, n := range before {
				if info, _ := os.Stat(filepath.Join(dir, n)); info != nil && info.IsDir() {
					if len(dirNames(t, filepath.Join(dir, n))) != 0 {
						t.Errorf("%s gained content", n)
					}
				}
			}
		})
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// AC-10: the root command records the outcome of the four levels in the
// package value `resolution`, which `lif check` reads.
func TestRoot_RecordsResolutionOutcome(t *testing.T) {
	stale := storeroot.Resolution{StoreDirSource: "stale", LegacyEnv: true}

	t.Run("legacy project: all three names legacy, path from the config", func(t *testing.T) {
		dir := legacyProject(t)
		resolution = stale
		if _, stderr, err := runInProject(t, dir, "list"); err != nil {
			t.Fatalf("list error = %v\nstderr: %s", err, stderr)
		}
		want := storeroot.Resolution{LegacyConfigFile: true, LegacySectionKey: true, LegacyStoreDir: true, StoreDirSource: "config"}
		if resolution != want {
			t.Errorf("resolution = %+v, want %+v", resolution, want)
		}
	})

	t.Run("new project: no legacy name", func(t *testing.T) {
		dir := evalSymlinks(t, t.TempDir())
		if _, stderr, err := runInProject(t, dir, "init"); err != nil {
			t.Fatalf("init error = %v\nstderr: %s", err, stderr)
		}
		resolution = stale
		if _, stderr, err := runInProject(t, dir, "list"); err != nil {
			t.Fatalf("list error = %v\nstderr: %s", err, stderr)
		}
		want := storeroot.Resolution{StoreDirSource: "config"}
		if resolution != want {
			t.Errorf("resolution = %+v, want %+v", resolution, want)
		}
	})

	t.Run("flag: the flag decides, the store name is judged on its own", func(t *testing.T) {
		dir := legacyProject(t)
		resolution = stale
		if _, stderr, err := runInProject(t, evalSymlinks(t, t.TempDir()), "--lif-path", filepath.Join(dir, config.LegacyDefaultStoreDir), "list"); err != nil {
			t.Fatalf("list error = %v\nstderr: %s", err, stderr)
		}
		want := storeroot.Resolution{LegacyStoreDir: true, StoreDirSource: "flag"}
		if resolution != want {
			t.Errorf("resolution = %+v, want %+v", resolution, want)
		}
	})
}

// lif-serve builds its worktree manager from the resolved store: item changes
// inside a worktree are detected under the directory that carries the main
// store's name (AC-9). Nothing else wires that name into DetectItemIDs.
func TestNewWorktreeManager_UsesTheMainStoreDirName(t *testing.T) {
	for _, name := range []string{config.LegacyDefaultStoreDir, config.DefaultStoreDir} {
		t.Run(name, func(t *testing.T) {
			storeRoot := filepath.Join(evalSymlinks(t, t.TempDir()), name)
			mgr := newWorktreeManager(config.Default(), storeRoot, t.TempDir())
			if got := mgr.StoreDir(); got != name {
				t.Errorf("StoreDir() = %q, want %q", got, name)
			}
		})
	}
}

// A --json invocation without a store reports NO_LIF_DIR as the JSON error
// object; without --json the plain text error stays.
func TestMissingStoreJSONErrorCode(t *testing.T) {
	dir := evalSymlinks(t, t.TempDir())

	stdout, _, err := runInProject(t, dir, "list", "--json")
	if err == nil {
		t.Fatal("list --json without a store: want an error")
	}
	var resp struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
	}
	if jerr := json.Unmarshal([]byte(stdout), &resp); jerr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jerr, stdout)
	}
	if resp.Success || resp.Code != output.ErrNoLifDir {
		t.Errorf("response = %+v, want success=false code=%s", resp, output.ErrNoLifDir)
	}

	stdout, _, err = runInProject(t, dir, "list")
	if err == nil || !strings.Contains(err.Error(), "no .lif directory found") || stdout != "" {
		t.Errorf("plain list: err = %v, stdout = %q, want plain error and no stdout", err, stdout)
	}
}
