package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tasks"
	"github.com/spf13/cobra"
)

// runEnv builds a project dir (with .lif.yml when project != "") and a HOME
// (with .lif/tasks.yml when global != ""), moves into the project dir and
// returns it. No store dir is created.
func runEnv(t *testing.T, project, global string) string {
	t.Helper()
	projectDir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	homeDir := t.TempDir()
	if project != "" {
		writeFixtureFile(t, filepath.Join(projectDir, ".lif.yml"), project)
	}
	if global != "" {
		writeFixtureFile(t, filepath.Join(homeDir, ".lif", "tasks.yml"), global)
	}
	t.Chdir(projectDir)
	t.Setenv("HOME", homeDir)
	neutralizeStoreEnv(t)
	return projectDir
}

// runTR runs the shared root from the current directory through runInProject,
// which takes back commands that other tests re-parented, and returns stdout,
// stderr (including what reportExecutionError printed) and the error.
func runTR(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return runInProject(t, wd, args...)
}

func assertNoStoreDir(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{".beans", ".lif"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s exists in %s", name, dir)
		}
	}
}

func TestTR01_ArgsPwdOriginReachStep(t *testing.T) {
	dir := runEnv(t, `tasks:
  show:
    args: [id]
    steps: ['printf "%s|%s|%s" "$id" "$PWD" "$LIF_TASK_ORIGIN"']
`, "")
	out, _, err := runTR(t, "run", "show", "beans-x1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "beans-x1|" + dir + "|project"; out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
	assertNoStoreDir(t, dir)
}

func TestTR02_PwdIsProjectDirFromSubdir(t *testing.T) {
	dir := runEnv(t, `tasks:
  show:
    args: [id]
    steps: ['printf "%s|%s|%s" "$id" "$PWD" "$LIF_TASK_ORIGIN"']
`, "")
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	out, _, err := runTR(t, "run", "show", "beans-x1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "beans-x1|" + dir + "|project"; out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTR03_ProjectWinsGlobalFlagPicksGlobal(t *testing.T) {
	runEnv(t, "tasks:\n  who:\n    steps: ['echo project']\n", "tasks:\n  who:\n    steps: ['echo global']\n")
	out, _, err := runTR(t, "run", "who")
	if err != nil || out != "project\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	out, _, err = runTR(t, "run", "-g", "who")
	if err != nil || out != "global\n" {
		t.Fatalf("-g: out = %q, err = %v", out, err)
	}
}

func TestTR04_EverythingAfterNameGoesToTask(t *testing.T) {
	runEnv(t, "tasks:\n  echo-args:\n    steps: ['printf \"%s \" \"$@\"']\n", "")
	out, _, err := runTR(t, "run", "echo-args", "--help", "--", "x")
	if err != nil {
		t.Fatal(err)
	}
	if want := "--help -- x "; out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

const trListProject = `tasks:
  hello:
    desc: greet
    args: [name]
    steps: ['echo hi']
  who:
    steps: ['echo project']
`

const trListGlobal = `tasks:
  who:
    steps: ['echo global']
  ver:
    steps: ['echo ver']
`

func TestTR05_ListSortedWithOrigin(t *testing.T) {
	runEnv(t, trListProject, trListGlobal)
	out, _, err := runTR(t, "run")
	if err != nil {
		t.Fatal(err)
	}
	want := "hello <name>  [project]                    greet\n" +
		"ver           [global]\n" +
		"who           [project, overrides global]\n"
	if out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTR06_ListGlobalOnly(t *testing.T) {
	runEnv(t, trListProject, trListGlobal)
	out, _, err := runTR(t, "run", "-g")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ver  [global]\nwho  [global]\n"; out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTR07_EmptySetPrintsHint(t *testing.T) {
	runEnv(t, "", "")
	out, _, err := runTR(t, "run")
	if err != nil {
		t.Fatal(err)
	}
	want := "No tasks. Define them under tasks: in .lif.yml or in ~/.lif/tasks.yml.\n"
	if out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTR08_GlobalTaskRunsWithoutProject(t *testing.T) {
	runEnv(t, "", "tasks:\n  g:\n    steps: ['printf ok']\n")
	out, _, err := runTR(t, "run", "g")
	if err != nil || out != "ok" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestTR09_ConfigFlagSetsRoot(t *testing.T) {
	runEnv(t, "", "")
	other, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(other, ".lif.yml"), "tasks:\n  where:\n    steps: ['pwd']\n")
	out, _, err := runTR(t, "--config", filepath.Join(other, ".lif.yml"), "run", "where")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(out), other; got != want {
		t.Fatalf("out = %q; want %q", got, want)
	}
}

func TestTR10_StepExitCodePassesThrough(t *testing.T) {
	runEnv(t, "tasks:\n  fail:\n    steps: ['exit 3', 'echo never']\n", "")
	_, stderr, err := runTR(t, "run", "fail")
	if got := exitCode(err); got != 3 {
		t.Fatalf("exitCode = %d; want 3 (err %v)", got, err)
	}
	if want := "Error: task \"fail\": step 1 of 2 failed (exit code 3)\n"; stderr != want {
		t.Fatalf("stderr = %q; want %q", stderr, want)
	}
}

func TestTR11_OtherErrorsExitOne(t *testing.T) {
	runEnv(t, "", "")
	_, _, err := runTR(t, "run", "nope")
	if err == nil {
		t.Fatal("want error for unknown task")
	}
	if got := exitCode(err); got != 1 {
		t.Fatalf("exitCode = %d; want 1", got)
	}
	// Only *tasks.StepError carries a code, even a wrapped ExitError does not.
	wrapped := fmt.Errorf("x: %w", &exec.ExitError{})
	if got := exitCode(wrapped); got != 1 {
		t.Fatalf("exitCode(wrapped ExitError) = %d; want 1", got)
	}
	if got := exitCode(&tasks.StepError{Task: "t", Step: 1, Total: 1, Err: errors.New("sig")}); got != 1 {
		t.Fatalf("exitCode(StepError without code) = %d; want 1", got)
	}
}

func TestTR12_BadTaskNameFailsRunAndList(t *testing.T) {
	dir := runEnv(t, "tasks:\n  Bad Name:\n    steps: ['echo x']\n", "")
	for _, args := range [][]string{{"run"}, {"list"}} {
		_, _, err := runTR(t, args...)
		if err == nil {
			t.Fatalf("%v: want config error", args)
		}
		if !strings.Contains(err.Error(), filepath.Join(dir, ".lif.yml")) {
			t.Errorf("%v: error %q does not name .lif.yml path", args, err)
		}
	}
}

func TestTR13_CompletionNamesAndDirective(t *testing.T) {
	runEnv(t, trListProject, trListGlobal)
	runCmd := sharedTestRoot(t)
	var cmd *cobra.Command
	for _, c := range runCmd.Commands() {
		if c.Name() == "run" {
			cmd = c
		}
	}
	if cmd == nil || cmd.ValidArgsFunction == nil {
		t.Fatal("run has no ValidArgsFunction")
	}
	got, dir := cmd.ValidArgsFunction(cmd, nil, "")
	want := []string{"hello\tgreet", "ver\t", "who\t"}
	if strings.Join(got, "|") != strings.Join(want, "|") || dir != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("got %q dir %v", got, dir)
	}
	got, dir = cmd.ValidArgsFunction(cmd, []string{"hello"}, "")
	if len(got) != 0 || dir != cobra.ShellCompDirectiveDefault {
		t.Fatalf("after name: got %q dir %v", got, dir)
	}
}

// TR14: the pinned set itself is TestBootstrapSkipMarksExactlyStorelessVerbs;
// this case selects the run command's marker under the TR prefix.
func TestTR14_RunIsBootstrapSkip(t *testing.T) {
	cmd, _, err := sharedTestRoot(t).Find([]string{"run"})
	if err != nil || cmd.Name() != "run" {
		t.Fatalf("run not found: %v", err)
	}
	if !IsBootstrapSkip(cmd) {
		t.Error("run is not bootstrap-skip")
	}
	if !IsUserFacing(cmd) {
		t.Error("run must stay user-facing")
	}
}

// TRUST-09: a project task that hides a global one says so on stderr.
func TestTR_ProjectTaskShadowingAGlobalOneIsAnnounced(t *testing.T) {
	runEnv(t, "tasks:\n  who:\n    steps: ['echo project']\n", "tasks:\n  who:\n    steps: ['echo global']\n")
	out, errOut, err := runTR(t, "run", "who")
	if err != nil || out != "project\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
	if want := `running project task "who" (overrides global`; !strings.Contains(errOut, want) {
		t.Errorf("stderr = %q, want it to contain %q", errOut, want)
	}
	_, errOut, err = runTR(t, "run", "-g", "who")
	if err != nil || strings.Contains(errOut, "overrides") {
		t.Errorf("-g: stderr = %q, err = %v; want no notice", errOut, err)
	}
}

// TRUST-11: no escape sequence of a task desc reaches the listing.
func TestTR_ListDropsControlCharactersOfDesc(t *testing.T) {
	runEnv(t, "tasks:\n  t:\n    desc: \"\\e[31mRED\\e]0;PWNED\\a\\e[2J\"\n    steps: ['true']\n", "")
	out, _, err := runTR(t, "run")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out, "\x1b\a") || !strings.Contains(out, "RED") {
		t.Errorf("listing = %q, want the desc without control characters", out)
	}
}

func TestTR15_MissingConfigFileIsError(t *testing.T) {
	runEnv(t, trListProject, "")
	_, _, err := runTR(t, "--config", "/nonexistent/.lif.yml", "run", "hello")
	if err == nil || !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("err = %v; want config file not found", err)
	}
}

func TestTR16_StorePathReachesStepsAbsolute(t *testing.T) {
	dir := runEnv(t, "tasks:\n    p:\n        steps: ['echo \"$LIF_PATH\"']\n", "")
	out, _, err := runTR(t, "--lif-path", "rel/store", "run", "p")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "rel/store") + "\n"; out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTR16b_RelativeEnvStorePathBecomesAbsolute(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	storePath = ""
	t.Setenv("LIF_PATH", "relenv")
	got := storeEnv()
	want, _ := filepath.Abs("relenv")
	if len(got) != 1 || got[0] != "LIF_PATH="+want {
		t.Fatalf("storeEnv() = %v; want LIF_PATH=%s", got, want)
	}
}

func TestTR17_ListingCellsStayAligned(t *testing.T) {
	runEnv(t, "tasks:\n    a:\n        desc: \"l1\\nl2\\ttab\"\n        steps: ['true']\n    b:\n        steps: ['true']\n    c:\n        desc: d\n        steps: ['true']\n", "")
	out, _, err := runTR(t, "run")
	if err != nil {
		t.Fatal(err)
	}
	want := "a  [project]  l1 l2 tab\nb  [project]\nc  [project]  d\n"
	if out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}
