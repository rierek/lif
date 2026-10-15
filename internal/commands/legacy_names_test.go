package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// isolateHome points HOME at a temp directory, so a search index or a worktree
// root never touches the developer's real ~/.lif or ~/.beans.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := evalSymlinks(t, t.TempDir())
	t.Setenv("HOME", home)
	return home
}

// newProject builds what `lif init` makes in an empty directory.
func newProject(t *testing.T) string {
	t.Helper()
	dir := evalSymlinks(t, t.TempDir())
	if _, stderr, err := runInProject(t, dir, "init"); err != nil {
		t.Fatalf("init error = %v\nstderr: %s", err, stderr)
	}
	return dir
}

func bangLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "!") {
			lines = append(lines, l)
		}
	}
	return lines
}

type checkJSONResult struct {
	Success     bool     `json:"success"`
	LegacyNames []string `json:"legacy_names"`
}

func runCheckJSON(t *testing.T, dir string, args ...string) checkJSONResult {
	t.Helper()
	stdout, stderr, err := runInProject(t, dir, append([]string{"check", "--json"}, args...)...)
	if err != nil {
		t.Fatalf("check --json %v error = %v\nstderr: %s", args, err, stderr)
	}
	var res checkJSONResult
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("check --json output is not JSON: %v\n%s", err, stdout)
	}
	return res
}

// SC-01: one warning per legacy name in use, each with its step; exit 0 with
// and without --strict.
func TestCheck_LegacyProject_WarnsPerLegacyNameWithStep(t *testing.T) {
	isolateHome(t)
	dir := legacyProject(t)
	for _, args := range [][]string{{"check"}, {"check", "--strict"}} {
		stdout, stderr, err := runInProject(t, dir, args...)
		if err != nil {
			t.Fatalf("lif %v error = %v\nstderr: %s", args, err, stderr)
		}
		lines := bangLines(stdout)
		if len(lines) != 3 {
			t.Fatalf("lif %v printed %d warning lines, want 3 (config file, section key, store directory):\n%s", args, len(lines), stdout)
		}
		all := strings.Join(lines, "\n")
		for _, want := range []string{
			"git mv .beans.yml .lif.yml",
			"section key beans:",
			"rename the key to lif:",
			"git mv .beans .lif",
		} {
			if !strings.Contains(all, want) {
				t.Errorf("lif %v warnings lack %q:\n%s", args, want, all)
			}
		}
	}
}

// AC-02: where a path: names the legacy store directory, that warning names
// the directory move, the path: edit and the config-file move.
func TestCheck_ExplicitPathWarningNamesAllThreeSteps(t *testing.T) {
	isolateHome(t)
	dir := legacyProject(t)
	res := runCheckJSON(t, dir)
	var storeWarning string
	for _, w := range res.LegacyNames {
		if strings.Contains(w, "git mv .beans .lif") {
			storeWarning = w
		}
	}
	if storeWarning == "" {
		t.Fatalf("no store-directory warning in %v", res.LegacyNames)
	}
	for _, want := range []string{"git mv .beans .lif", "path: .lif", "git mv .beans.yml .lif.yml"} {
		if !strings.Contains(storeWarning, want) {
			t.Errorf("store-directory warning %q lacks %q", storeWarning, want)
		}
	}
}

func TestCheck_LegacyProject_JSONKeyAndExitStatus(t *testing.T) {
	isolateHome(t)
	dir := legacyProject(t)
	for _, extra := range [][]string{nil, {"--strict"}} {
		res := runCheckJSON(t, dir, extra...)
		if !res.Success {
			t.Errorf("check --json %v success = false: legacy names must not count as issues", extra)
		}
		if len(res.LegacyNames) != 3 {
			t.Errorf("check --json %v legacy_names = %v, want 3 entries", extra, res.LegacyNames)
		}
	}
}

func TestCheck_NewProject_ReportsNoLegacyName(t *testing.T) {
	isolateHome(t)
	dir := newProject(t)
	stdout, _, err := runInProject(t, dir, "check")
	if err != nil {
		t.Fatalf("check error = %v", err)
	}
	if lines := bangLines(stdout); len(lines) != 0 {
		t.Errorf("check on a new project printed warnings: %v", lines)
	}
	raw, _, _ := runInProject(t, dir, "check", "--json")
	if strings.Contains(raw, "legacy_names") {
		t.Errorf("check --json on a new project carries legacy_names:\n%s", raw)
	}
}

// AC-04 / D06: no other command prints a legacy-name notice.
func TestOtherCommands_PrintNoLegacyNotice(t *testing.T) {
	isolateHome(t)
	dir := legacyProject(t)
	for _, args := range [][]string{{"list"}, {"path"}, {"show", "leg-aaa1"}, {"init", "--force"}} {
		stdout, stderr, err := runInProject(t, dir, args...)
		if err != nil {
			t.Fatalf("lif %v error = %v", args, err)
		}
		if strings.Contains(stdout+stderr, "git mv") || strings.Contains(stdout+stderr, "is a legacy name") {
			t.Errorf("lif %v printed a legacy notice:\nstdout: %s\nstderr: %s", args, stdout, stderr)
		}
	}
}

// SC-02: following the printed steps ends every legacy warning and leaves the
// items as they were.
func TestCheck_PrintedStepsEndTheWarnings(t *testing.T) {
	isolateHome(t)
	dir := legacyProject(t)
	before := itemFileHashes(t, filepath.Join(dir, config.LegacyDefaultStoreDir))

	// The three steps, as the warnings print them.
	cfgPath := filepath.Join(dir, config.LegacyConfigFileName)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "beans:", "lif:", 1))
	data = []byte(strings.Replace(string(data), "path: .beans", "path: .lif", 1))
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(cfgPath, filepath.Join(dir, config.ConfigFileName)); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(dir, config.LegacyDefaultStoreDir), filepath.Join(dir, config.DefaultStoreDir)); err != nil {
		t.Fatal(err)
	}

	if res := runCheckJSON(t, dir); len(res.LegacyNames) != 0 || !res.Success {
		t.Errorf("after the steps: success %v, legacy_names %v; want none", res.Success, res.LegacyNames)
	}
	after := itemFileHashes(t, filepath.Join(dir, config.DefaultStoreDir))
	if strings.Join(sortedNames(after), ",") != strings.Join(sortedNames(before), ",") {
		t.Errorf("item files after the steps = %v, want %v", sortedNames(after), sortedNames(before))
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Errorf("%s changed by the steps", name)
		}
	}
}

func loadConfigFile(t *testing.T, file, body string) *config.Config {
	t.Helper()
	path := filepath.Join(evalSymlinks(t, t.TempDir()), file)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The step depends on where the store directory name comes from.
func TestLegacyNameWarnings_StepFollowsTheNameSource(t *testing.T) {
	cfgWithPath := loadConfigFile(t, config.LegacyConfigFileName, "beans:\n    path: ../.beans\n")
	cfgNew := loadConfigFile(t, config.ConfigFileName, "lif:\n    path: ../.beans\n")
	bare := config.Default()

	t.Run("env var", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{LegacyEnv: true}, bare, ""), "\n")
		if !strings.Contains(got, "BEANS_PATH") || !strings.Contains(got, "LIF_PATH") {
			t.Errorf("warnings = %q, want BEANS_PATH replaced by LIF_PATH", got)
		}
	})
	t.Run("flag", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{LegacyStoreDir: true, StoreDirSource: storeSourceFlag}, bare, "/p/.beans"), "\n")
		if !strings.Contains(got, "git mv .beans .lif") || !strings.Contains(got, "--lif-path") {
			t.Errorf("warnings = %q, want the move and the --lif-path value", got)
		}
	})
	t.Run("env source", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{LegacyStoreDir: true, StoreDirSource: storeSourceEnv}, bare, "/p/.beans"), "\n")
		if !strings.Contains(got, "git mv .beans .lif") || !strings.Contains(got, "LIF_PATH") {
			t.Errorf("warnings = %q, want the move and the LIF_PATH value", got)
		}
	})
	t.Run("default", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{LegacyStoreDir: true, StoreDirSource: storeSourceDefault}, bare, "/p/.beans"), "\n")
		if !strings.Contains(got, "git mv .beans .lif") || strings.Contains(got, "path:") {
			t.Errorf("warnings = %q, want only the move", got)
		}
	})
	t.Run("config path keeps its parent", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{
			LegacyConfigFile: true, LegacyStoreDir: true, StoreDirSource: storeSourceConfig,
		}, cfgWithPath, "/p/.beans"), "\n")
		for _, want := range []string{"git mv .beans .lif", "path: ../.lif", "git mv .beans.yml .lif.yml"} {
			if !strings.Contains(got, want) {
				t.Errorf("warnings = %q, lack %q", got, want)
			}
		}
	})
	t.Run("config path in a new config file", func(t *testing.T) {
		got := strings.Join(legacyNameWarnings(storeResolution{
			LegacyStoreDir: true, StoreDirSource: storeSourceConfig,
		}, cfgNew, "/p/.beans"), "\n")
		if strings.Count(got, ".beans.yml") != 0 {
			t.Errorf("warnings = %q name the config-file move although the file is new", got)
		}
	})
	t.Run("nothing legacy", func(t *testing.T) {
		if got := legacyNameWarnings(storeResolution{StoreDirSource: storeSourceDefault}, bare, "/p/.lif"); len(got) != 0 {
			t.Errorf("warnings = %v, want none", got)
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestLegacyWorktreeWarnings_RootAndBranches(t *testing.T) {
	home := isolateHome(t)
	repo := evalSymlinks(t, t.TempDir())
	runGit(t, repo, "init", "-q", "-b", "main")
	runGit(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	wt := filepath.Join(evalSymlinks(t, t.TempDir()), "wt")
	runGit(t, repo, "worktree", "add", "-q", "-b", "beans/x1", wt)
	runGit(t, repo, "worktree", "add", "-q", "-b", "lif/y2", filepath.Join(filepath.Dir(wt), "wt2"))

	c := config.Default()
	c.SetConfigDir(repo)

	t.Run("no legacy root", func(t *testing.T) {
		got := strings.Join(legacyWorktreeWarnings(c, repo), "\n")
		if strings.Contains(got, "worktree root") {
			t.Errorf("warnings = %q name a worktree root that is not there", got)
		}
		if !strings.Contains(got, "beans/x1") || !strings.Contains(got, "lif/x1") || !strings.Contains(got, wt) {
			t.Errorf("warnings = %q, want the beans/x1 worktree with its rename to lif/x1", got)
		}
		if strings.Contains(got, "lif/y2") {
			t.Errorf("warnings = %q name a worktree on a lif/ branch", got)
		}
	})

	t.Run("legacy root", func(t *testing.T) {
		root := filepath.Join(home, ".beans", "worktrees", filepath.Base(repo))
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		got := strings.Join(legacyWorktreeWarnings(c, repo), "\n")
		for _, want := range []string{root, "remove the remaining worktrees", "~/.lif/worktrees/" + filepath.Base(repo)} {
			if !strings.Contains(got, want) {
				t.Errorf("warnings = %q, lack %q", got, want)
			}
		}
	})

	t.Run("not a git repository", func(t *testing.T) {
		plain := evalSymlinks(t, t.TempDir())
		for _, w := range legacyWorktreeWarnings(c, plain) {
			if strings.Contains(w, "legacy branch") {
				t.Errorf("branch warning outside a git repository: %q", w)
			}
		}
	})
}

// D18: runtime messages print the name the project uses.
func TestStoreNameMessages_FollowTheLayout(t *testing.T) {
	isolateHome(t)

	t.Run("init --json", func(t *testing.T) {
		legacy := legacyProject(t)
		out, _, err := runInProject(t, legacy, "init", "--force", "--json")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Initialized .beans directory") {
			t.Errorf("init --json in a legacy project = %s, want .beans named", out)
		}
		fresh := evalSymlinks(t, t.TempDir())
		out, _, err = runInProject(t, fresh, "init", "--json")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "Initialized .lif directory") {
			t.Errorf("init --json in a new project = %s, want .lif named", out)
		}
	})

	t.Run("archive", func(t *testing.T) {
		done := strings.Replace(legacyItemFile, "status: todo", "status: completed", 1)
		legacy := legacyProject(t)
		writeFixtureFile(t, filepath.Join(legacy, config.LegacyDefaultStoreDir, "leg-ccc3--done.md"), done)
		out, _, err := runInProject(t, legacy, "archive")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "to .beans/archive/") {
			t.Errorf("archive in a legacy project = %q, want .beans/archive/", out)
		}
		fresh := newProject(t)
		writeFixtureFile(t, filepath.Join(fresh, config.DefaultStoreDir, "nw-ccc3--done.md"), done)
		out, _, err = runInProject(t, fresh, "archive")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "to .lif/archive/") {
			t.Errorf("archive in a new project = %q, want .lif/archive/", out)
		}
	})

	t.Run("rename dry run", func(t *testing.T) {
		legacy := legacyProject(t)
		out, _, err := runInProject(t, legacy, "rename", "--prefix", "neu-", "--dry-run")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "config: .beans.yml prefix") {
			t.Errorf("rename dry run in a legacy project = %q, want .beans.yml named", out)
		}
		fresh := newProject(t)
		writeFixtureFile(t, filepath.Join(fresh, config.DefaultStoreDir, filepath.Base(fresh)+"-aaa1--first.md"), legacyItemFile)
		out, _, err = runInProject(t, fresh, "rename", "--prefix", "neu-", "--dry-run")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out, "config: .lif.yml prefix") {
			t.Errorf("rename dry run in a new project = %q, want .lif.yml named", out)
		}
	})

	t.Run("anchor error", func(t *testing.T) {
		for _, tc := range []struct{ file, key string }{
			{config.ConfigFileName, config.SectionKey},
			{config.LegacyConfigFileName, config.LegacySectionKey},
		} {
			dir := evalSymlinks(t, t.TempDir())
			writeFixtureFile(t, filepath.Join(dir, tc.file), tc.key+":\n    anchor: sideways\n")
			_, _, err := runInProject(t, dir, "list")
			if err == nil || !strings.Contains(err.Error(), "unknown "+tc.key+".anchor") {
				t.Errorf("list with a bad anchor under %s: error = %v, want %s.anchor named", tc.key, err, tc.key)
			}
		}
	})

	t.Run("missing store", func(t *testing.T) {
		legacy := evalSymlinks(t, t.TempDir())
		writeFixtureFile(t, filepath.Join(legacy, config.LegacyConfigFileName), "beans:\n    path: .beans\n")
		_, _, err := runInProject(t, legacy, "list")
		if err == nil || !strings.Contains(err.Error(), "no .beans directory found") {
			t.Errorf("legacy project without a store: error = %v, want .beans named", err)
		}
		fresh := evalSymlinks(t, t.TempDir())
		_, _, err = runInProject(t, fresh, "list")
		if err == nil || !strings.Contains(err.Error(), "no .lif directory found") {
			t.Errorf("new project without a store: error = %v, want .lif named", err)
		}
	})
}

// D18 / SC-03: the help texts name the lif data names; a legacy name appears
// only on a line that describes the fallback.
func TestHelpTexts_NameLifDataNames(t *testing.T) {
	root := sharedTestRoot(t)
	text := map[string]string{}
	for _, name := range []string{"init", "path", "list", "archive"} {
		sub, _, err := root.Find([]string{name})
		if err != nil || sub == root {
			t.Fatalf("command %q not found: %v", name, err)
		}
		text[name] = sub.Short + "\n" + sub.Long
	}
	text["--lif-path"] = root.PersistentFlags().Lookup("lif-path").Usage
	text["--config"] = root.PersistentFlags().Lookup("config").Usage

	for name, want := range map[string][]string{
		"init":       {".lif directory", ".lif.yml config file"},
		"path":       {"--lif-path, then the .lif.yml", "then LIF_PATH"},
		"list":       {"in the .lif directory"},
		"archive":    {"directory (.lif/archive/)", "main .lif directory"},
		"--lif-path": {"LIF_PATH env var"},
		"--config":   {"searches upward for .lif.yml"},
	} {
		for _, w := range want {
			if !strings.Contains(text[name], w) {
				t.Errorf("help of %s lacks %q:\n%s", name, w, text[name])
			}
		}
	}
	for name, body := range text {
		for _, line := range strings.Split(body, "\n") {
			legacy := strings.Contains(line, ".beans") || strings.Contains(line, "BEANS_PATH") || strings.Contains(line, "--beans-path")
			if legacy && !strings.Contains(strings.ToLower(line), "legacy") {
				t.Errorf("help of %s names a legacy name outside a fallback description: %q", name, line)
			}
		}
	}
}

func TestInitProfileGuard_NamesTheLifFile(t *testing.T) {
	isolateHome(t)
	dir := evalSymlinks(t, t.TempDir())
	_, _, err := runInProject(t, dir, "init", "--lif-path", filepath.Join(dir, "x"), "--profile", "simple")
	if err == nil {
		t.Fatal("init --lif-path --profile error = nil, want the guard error")
	}
	if !strings.Contains(err.Error(), ".lif.yml") || strings.Contains(err.Error(), ".beans") {
		t.Errorf("guard error = %q, want .lif.yml and no .beans", err)
	}
}

// D20: the repository tracks .lif.yml under key lif: and no .beans.yml.
func TestRepositoryConfig_IsTheLifFile(t *testing.T) {
	root := filepath.Join("..", "..")
	if _, err := os.Stat(filepath.Join(root, config.LegacyConfigFileName)); !os.IsNotExist(err) {
		t.Errorf("repository still tracks %s (stat err = %v)", config.LegacyConfigFileName, err)
	}
	path := filepath.Join(root, config.ConfigFileName)
	c, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(%s) error = %v", path, err)
	}
	if c.SectionKey() != config.SectionKey || c.Items.Path != "../.beans" || c.Items.Anchor != config.AnchorRepoRoot || c.GetProjectName() != "beans" {
		t.Errorf("config = key %q, path %q, anchor %q, project %q; want lif, ../.beans, repo-root, beans",
			c.SectionKey(), c.Items.Path, c.Items.Anchor, c.GetProjectName())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"https://github.com/rierek/lif", "lif-serve"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("%s lacks %q", config.ConfigFileName, want)
		}
	}
	if strings.Contains(string(raw), "hmans") || strings.Contains(string(raw), "beans-serve") {
		t.Errorf("%s still carries former-tool comment lines", config.ConfigFileName)
	}
	ignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(ignore), "\n")
	for _, want := range []string{"/.lif/", "/.beans/"} {
		found := false
		for _, l := range lines {
			if strings.TrimSpace(l) == want {
				found = true
			}
		}
		if !found {
			t.Errorf(".gitignore lacks the line %q", want)
		}
	}
}
