package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// Contract groups A, G, T and W (hooks, command level): every case runs the
// real root command through runInProject, so PersistentPreRunE builds the
// dispatcher the way a user's call does. Tasks run the lif binary that
// TestMain built, through "$LIF".

// hookEnv isolates one hook test: a private HOME (the developer's global
// hooks and trust file stay out of reach), hooks switched on, and no
// dispatcher left in hookDispatch for the next test. It returns HOME.
func hookEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LIF_NO_HOOKS", "")
	t.Cleanup(func() { hookDispatch = nil })
	return home
}

// hookProject creates a project directory whose .lif.yml has the prefix ex-
// and the given tasks:/hooks: yaml, with an empty store.
func hookProject(t *testing.T, yaml string) string {
	t.Helper()
	dir := evalSymlinks(t, t.TempDir())
	writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n    prefix: ex-\n"+yaml)
	if err := os.MkdirAll(filepath.Join(dir, config.DefaultStoreDir), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func hookItemPath(dir, id string) string {
	return filepath.Join(dir, config.DefaultStoreDir, item.BuildFilename(id, "item"))
}

// hookItem writes an item file; extra is raw front matter after the standard
// fields.
func hookItem(t *testing.T, dir, id, status, typ, extra string) {
	t.Helper()
	text := "---\n# " + id + "\ntitle: " + id + "\nstatus: " + status + "\ntype: " + typ + "\n" +
		"created_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\n" + extra + "---\n\nBody.\n"
	writeFixtureFile(t, hookItemPath(dir, id), text)
}

// todoItems writes one plain todo task per id.
func todoItems(t *testing.T, dir string, ids ...string) {
	t.Helper()
	for _, id := range ids {
		hookItem(t, dir, id, "todo", "task", "")
	}
}

func hookItemBytes(t *testing.T, dir, id string) []byte {
	t.Helper()
	data, err := os.ReadFile(hookItemPath(dir, id))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readHookItem(t *testing.T, dir, id string) *item.Item {
	t.Helper()
	b, err := item.Parse(bytes.NewReader(hookItemBytes(t, dir, id)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// allowHooks runs `lif hooks allow` in dir.
func allowHooks(t *testing.T, dir string) {
	t.Helper()
	if _, _, err := runInProject(t, dir, "hooks", "allow"); err != nil {
		t.Fatalf("lif hooks allow: %v", err)
	}
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// logLines returns the lines of dir/name; a file that does not exist has none.
func logLines(t *testing.T, dir, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// blockingGate blocks every change to completed: exit code 3, the output
// "detail" on stderr and the message "Fix it.".
const blockingGate = `tasks:
  deny:
    steps: ['echo detail >&2; exit 3']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: deny
    message: "Fix it."
`

// blockOnlyB is blockingGate for the item ex-bbbb alone.
const blockOnlyB = `tasks:
  deny-b:
    args: [id]
    steps: ['[ "$id" = ex-bbbb ] || exit 0; echo detail >&2; exit 3']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: deny-b
    args: [id]
    message: "Fix it."
`

// logTask is a task that appends "<hook> <item id>" to the file log.
const logTask = `tasks:
  log:
    steps: ['echo "$LIF_HOOK $LIF_ITEM_ID" >> "$LIF_ROOT/log"']
`

func TestA01_FailingAfterHookWarnsAndTheNextOneRuns(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  t:
    steps: ['exit 4']
  m:
    steps: ['touch "$LIF_ROOT/marker"']
hooks:
  a:
    event: [updated]
    changed: [status]
    run: t
  b:
    event: [updated]
    changed: [status]
    run: m
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	_, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err != nil {
		t.Fatalf("update returned %v, want nil: an after hook cannot veto", err)
	}
	want := `warning: hook "a" (task "t"): task "t": step 1 of 1 failed (exit code 4)` + "\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if !fileExists(dir, "marker") {
		t.Error("hook b did not run after hook a failed")
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", got)
	}
}

func TestA02_HookOutputGoesToStderrNotIntoTheJSON(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  say:
    steps: ['echo HOOKOUT']
hooks:
  h:
    event: [updated]
    changed: [status]
    run: say
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	stdout, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress", "--json")

	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Errorf("stdout is not valid JSON: %q", stdout)
	}
	if strings.Contains(stdout, "HOOKOUT") {
		t.Errorf("stdout carries the hook output: %q", stdout)
	}
	if !strings.Contains(stderr, "HOOKOUT") {
		t.Errorf("stderr = %q, want the hook output", stderr)
	}
}

func TestA03_NestedLifCallRunsWithNoHooksAndFiresNone(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  tag-it:
    args: [id]
    steps:
      - '"$LIF" update "$id" --tag hooked'
      - 'printf %s "$LIF_NO_HOOKS" > "$LIF_ROOT/seen"'
  mark-second:
    steps: ['touch "$LIF_ROOT/second"']
hooks:
  first:
    event: [updated]
    to: [in-progress]
    run: tag-it
    args: [id]
  second:
    event: [updated]
    tags_added: [hooked]
    run: mark-second
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err != nil {
		t.Fatal(err)
	}
	got := readHookItem(t, dir, "ex-aaaa")
	if !slices.Contains(got.Tags, "hooked") {
		t.Errorf("tags on disk = %v, want hooked: the nested lif call did not run", got.Tags)
	}
	if got.Status != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", got.Status)
	}
	if seen := logLines(t, dir, "seen"); !slices.Equal(seen, []string{"1"}) {
		t.Errorf("the task saw LIF_NO_HOOKS = %q, want 1", seen)
	}
	if fileExists(dir, "second") {
		t.Error("hook second fired for the tag that the nested call added")
	}
}

// A relative --lif-path or --config leaves the store root relative; the
// task still gets an absolute LIF_ITEM_PATH that exists, whatever the
// working directory of the task is.
func TestHookItemPathIsAbsoluteWithRelativeStore(t *testing.T) {
	for _, flags := range [][]string{
		{"--lif-path", "../" + config.DefaultStoreDir},
		{"--config", "../" + config.ConfigFileName},
	} {
		t.Run(flags[0], func(t *testing.T) {
			hookEnv(t)
			dir := hookProject(t, `tasks:
  where:
    steps: ['printf %s "$LIF_ITEM_PATH" > "$LIF_ROOT/seen"']
hooks:
  h:
    event: [updated]
    to: [in-progress]
    run: where
`)
			todoItems(t, dir, "ex-aaaa")
			allowHooks(t, dir)
			sub := filepath.Join(dir, "sub")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}

			args := append([]string{"update", "ex-aaaa", "-s", "in-progress"}, flags...)
			if _, _, err := runInProject(t, sub, args...); err != nil {
				t.Fatal(err)
			}

			seen := logLines(t, dir, "seen")
			if len(seen) != 1 || !filepath.IsAbs(seen[0]) {
				t.Fatalf("LIF_ITEM_PATH = %q, want one absolute path", seen)
			}
			if _, err := os.Stat(seen[0]); err != nil {
				t.Errorf("LIF_ITEM_PATH does not exist: %v", err)
			}
		})
	}
}

func TestA09_AHookThatWritesALaterTargetMakesThatTargetConflict(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  touch-b:
    steps: ['"$LIF" update ex-bbbb --tag from-hook']
hooks:
  h:
    event: [updated]
    tags_added: [x]
    run: touch-b
`)
	todoItems(t, dir, "ex-aaaa", "ex-bbbb")
	allowHooks(t, dir)

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "ex-bbbb", "--tag", "x")

	var mismatch *itemcore.ETagMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want an ETagMismatchError", err)
	}
	if got := mutationErrorCode(err); got != "CONFLICT" {
		t.Errorf("error code = %q, want CONFLICT", got)
	}
	if !strings.Contains(err.Error(), "(already written: ex-aaaa)") {
		t.Errorf("error = %q, want it to name ex-aaaa as already written", err)
	}
	if got := readHookItem(t, dir, "ex-aaaa").Tags; !slices.Equal(got, []string{"x"}) {
		t.Errorf("ex-aaaa tags = %v, want [x]", got)
	}
	if got := readHookItem(t, dir, "ex-bbbb").Tags; !slices.Equal(got, []string{"from-hook"}) {
		t.Errorf("ex-bbbb tags = %v, want [from-hook] and no x", got)
	}
}

func TestG01_PassingGateWritesTheChangeAndAfterHooksRun(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  pass:
    steps: ['touch "$LIF_ROOT/gate"']
  after:
    steps: ['touch "$LIF_ROOT/after"']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    run: pass
  a:
    event: [updated]
    changed: [status]
    run: after
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(dir, "gate") {
		t.Error("the gate did not run")
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", got)
	}
	if !fileExists(dir, "after") {
		t.Error("the after hook did not run for the written change")
	}
}

func TestG02_BlockedChangeIsAPolicyErrorWithTheGateText(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, blockingGate)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)
	before := hookItemBytes(t, dir, "ex-aaaa")

	_, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "completed")

	want := "hook \"g\" blocked the change of ex-aaaa: Fix it.\ndetail"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
	// The whole stderr: no "Run 'lif update --help'" line follows.
	if wantStderr := "Error: " + want + "\n"; stderr != wantStderr {
		t.Errorf("stderr = %q, want %q", stderr, wantStderr)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1: the step code 3 must not leak", got)
	}
	if !bytes.Equal(before, hookItemBytes(t, dir, "ex-aaaa")) {
		t.Error("the item file changed")
	}
}

func TestG03_BlockedChangeInJSONCarriesThePolicyCodeAndTheText(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, blockingGate)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	stdout, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "completed", "--json")

	if err == nil {
		t.Fatal("the gate did not block")
	}
	var envelope map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &envelope); jerr != nil {
		t.Fatalf("stdout is not a JSON document: %q", stdout)
	}
	if got := envelope["code"]; got != "POLICY_VIOLATION" {
		t.Errorf("code = %v, want POLICY_VIOLATION", got)
	}
	if want := "hook \"g\" blocked the change of ex-aaaa: Fix it.\ndetail"; envelope["error"] != want {
		t.Errorf("error = %q, want %q", envelope["error"], want)
	}
}

func TestG05_BlockedChangeFiresNoAfterHook(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  deny:
    steps: ['exit 1']
  after:
    steps: ['touch "$LIF_ROOT/after"']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    run: deny
  a:
    event: [updated]
    changed: [status]
    run: after
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err == nil {
		t.Fatal("the gate did not block")
	}
	if fileExists(dir, "after") {
		t.Error("an after hook ran for a change that was never written")
	}
}

func TestG09_GateSeesTheExtraKeyThatCompleteAndUpdateSet(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  see:
    args: [target_entry]
    steps: ['printf %s "$target_entry" > "$LIF_ROOT/seen"']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: see
    args: [target_entry]
`)
	todoItems(t, dir, "ex-aaaa", "ex-bbbb")
	allowHooks(t, dir)

	for _, tc := range []struct {
		id   string
		args []string
	}{
		{"ex-aaaa", []string{"complete", "ex-aaaa", "--set", "target_entry=docs/R.md"}},
		{"ex-bbbb", []string{"update", "ex-bbbb", "-s", "completed", "--set", "target_entry=docs/R.md"}},
	} {
		if err := os.Remove(filepath.Join(dir, "seen")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if _, _, err := runInProject(t, dir, tc.args...); err != nil {
			t.Fatalf("lif %v: %v", tc.args, err)
		}
		if got := logLines(t, dir, "seen"); !slices.Equal(got, []string{"docs/R.md"}) {
			t.Errorf("lif %s: the gate saw target_entry = %q, want docs/R.md", tc.args[0], got)
		}
		got := readHookItem(t, dir, tc.id)
		if got.Status != "completed" || got.Extra["target_entry"] != "docs/R.md" {
			t.Errorf("lif %s: item on disk has status %q and extra %v, want completed and target_entry docs/R.md", tc.args[0], got.Status, got.Extra)
		}
	}
}

func TestG10_BatchGateStopsAtTheBlockedTargetAndEarlierOnesStayWritten(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, blockOnlyB)
	todoItems(t, dir, "ex-aaaa", "ex-bbbb")
	allowHooks(t, dir)
	beforeB := hookItemBytes(t, dir, "ex-bbbb")

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "ex-bbbb", "-s", "completed")

	if err == nil || !strings.Contains(err.Error(), `hook "g" blocked the change of ex-bbbb`) {
		t.Fatalf("err = %v, want a block that names ex-bbbb", err)
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "completed" {
		t.Errorf("ex-aaaa status = %q, want completed: it came before the blocked target", got)
	}
	if !bytes.Equal(beforeB, hookItemBytes(t, dir, "ex-bbbb")) {
		t.Error("ex-bbbb changed")
	}
}

func TestG11_NoHooksBypassesABlockingGate(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, blockingGate)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)
	t.Setenv("LIF_NO_HOOKS", "1")

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "completed")

	if err != nil {
		t.Fatalf("update = %v, want the bypass to let it through", err)
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "completed" {
		t.Errorf("status on disk = %q, want completed", got)
	}
}

func TestG13_RequireTargetEntryExampleBlocksUntilTheEntryExists(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  check-target-entry:
    args: [id, target_entry]
    steps:
      - |
        [ -n "$target_entry" ] || { echo "field target_entry is empty" >&2; exit 1; }
        grep -qF -- "$id" "$target_entry" || { echo "$id not found in $target_entry" >&2; exit 1; }
hooks:
  require-target-entry:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: check-target-entry
    args: [id, target_entry]
    message: "Entry missing in the target file. Add it, then complete the item."
`)
	hookItem(t, dir, "ex-ab12", "todo", "task", "target_entry: docs/RELEASE.md\n")
	writeFixtureFile(t, filepath.Join(dir, "docs", "RELEASE.md"), "# Release\n\n- ex-other\n")
	allowHooks(t, dir)
	before := hookItemBytes(t, dir, "ex-ab12")

	_, stderr, err := runInProject(t, dir, "complete", "ex-ab12")

	if err == nil {
		t.Fatal("complete went through although the target file lacks the ID")
	}
	want := "Error: hook \"require-target-entry\" blocked the change of ex-ab12: Entry missing in the target file. Add it, then complete the item.\n" +
		"ex-ab12 not found in docs/RELEASE.md\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
	if !bytes.Equal(before, hookItemBytes(t, dir, "ex-ab12")) {
		t.Error("the item file changed")
	}

	writeFixtureFile(t, filepath.Join(dir, "docs", "RELEASE.md"), "# Release\n\n- ex-ab12\n")
	if _, _, err := runInProject(t, dir, "complete", "ex-ab12"); err != nil {
		t.Fatalf("complete with the entry in place: %v", err)
	}
	if got := readHookItem(t, dir, "ex-ab12").Status; got != "completed" {
		t.Errorf("status on disk = %q, want completed", got)
	}
}

func TestG14_GateThatWritesTheItemMakesTheOuterWriteConflict(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  tag-x:
    args: [id]
    steps: ['"$LIF" update "$id" --tag x']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: tag-x
    args: [id]
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "completed")

	var mismatch *itemcore.ETagMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want an ETagMismatchError", err)
	}
	if got := mutationErrorCode(err); got != "CONFLICT" {
		t.Errorf("error code = %q, want CONFLICT", got)
	}
	got := readHookItem(t, dir, "ex-aaaa")
	if got.Status != "todo" {
		t.Errorf("status on disk = %q, want todo: the outer change must not be written", got.Status)
	}
	if !slices.Equal(got.Tags, []string{"x"}) {
		t.Errorf("tags on disk = %v, want the gate's tag x to stay", got.Tags)
	}
}

func TestG15_BatchBlockInTextModeNamesTheBlockedAndTheWrittenItem(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, blockOnlyB)
	todoItems(t, dir, "ex-aaaa", "ex-bbbb")
	allowHooks(t, dir)

	_, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "ex-bbbb", "-s", "completed")

	want := "Error: hook \"g\" blocked the change of ex-bbbb: Fix it.\ndetail\n(already written: ex-aaaa)\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	if err == nil {
		t.Fatal("the gate did not block")
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
}

func TestG17_GateKilledBySignalBlocksWithCouldNotRun(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  die:
    steps: ['kill -TERM $$']
hooks:
  g:
    stage: before
    event: [updated]
    changed: [status]
    run: die
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)
	before := hookItemBytes(t, dir, "ex-aaaa")

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	var blocked *hooks.BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("err = %v, want a *hooks.BlockedError", err)
	}
	if !strings.HasPrefix(blocked.Message, "could not run:") {
		t.Errorf("Message = %q, want it to start with %q", blocked.Message, "could not run:")
	}
	if got := exitCode(err); got != 1 {
		t.Errorf("exitCode = %d, want 1", got)
	}
	if !bytes.Equal(before, hookItemBytes(t, dir, "ex-aaaa")) {
		t.Error("the item file changed")
	}
}

func TestG18_BrokenGlobalFileBlocksEvenAProjectWithoutGates(t *testing.T) {
	home := hookEnv(t)
	dir := hookProject(t, "")
	todoItems(t, dir, "ex-aaaa")
	globalPath := filepath.Join(home, ".lif", "tasks.yml")
	writeFixtureFile(t, globalPath, "hooks: [\n")
	before := hookItemBytes(t, dir, "ex-aaaa")

	_, _, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err == nil {
		t.Fatal("update went through with a broken global hooks file")
	}
	prefix := `hook "*" blocked the change of ex-aaaa: hooks could not be loaded: ` + globalPath
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("err = %q, want it to start with %q", err, prefix)
	}
	if suffix := "(fix it or set LIF_NO_HOOKS=1)"; !strings.HasSuffix(err.Error(), suffix) {
		t.Errorf("err = %q, want it to end with %q", err, suffix)
	}
	if !bytes.Equal(before, hookItemBytes(t, dir, "ex-aaaa")) {
		t.Error("the item file changed")
	}
}

func TestG19_NonEmptyNoHooksSkipsTheMatchingGateWithAWarning(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  deny:
    steps: ['exit 1']
hooks:
  g:
    stage: before
    event: [updated]
    to: [completed]
    run: deny
  n:
    stage: before
    event: [updated]
    to: [scrapped]
    run: deny
`)
	todoItems(t, dir, "ex-aaaa")
	allowHooks(t, dir)
	t.Setenv("LIF_NO_HOOKS", "0")

	_, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "completed")

	if err != nil {
		t.Fatalf("update = %v, want the bypass to let it through", err)
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "completed" {
		t.Errorf("status on disk = %q, want completed", got)
	}
	if want := `warning: gate "g" skipped (LIF_NO_HOOKS is set)`; !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want it to contain %q", stderr, want)
	}
	if strings.Contains(stderr, `"n"`) {
		t.Errorf("stderr = %q names the gate n, which does not match the change", stderr)
	}
}

func TestT10_UnsetHomeOnlyWarnsAndTheWriteSucceeds(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  t:
    steps: ['touch "$LIF_ROOT/ran"']
hooks:
  h:
    event: [updated]
    changed: [status]
    run: t
`)
	todoItems(t, dir, "ex-aaaa")
	t.Setenv("HOME", "") // registers the restore; the next line is the real unset
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runInProject(t, dir, "update", "ex-aaaa", "-s", "in-progress")

	if err != nil {
		t.Fatalf("update = %v, want no error without HOME", err)
	}
	want := "warning: hooks in " + filepath.Join(dir, ".lif.yml") + " are not allowed; run 'lif hooks allow' to enable them\n"
	if stderr != want {
		t.Errorf("stderr = %q, want only %q", stderr, want)
	}
	if got := readHookItem(t, dir, "ex-aaaa").Status; got != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", got)
	}
	if fileExists(dir, "ran") {
		t.Error("a hook ran without an allow")
	}
}

func TestW01_CreateFiresTheCreatedHookOnce(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, logTask+`hooks:
  h:
    event: [created]
    to: [in-progress]
    run: log
`)
	allowHooks(t, dir)

	stdout, _, err := runInProject(t, dir, "create", "Thing", "-s", "in-progress", "--json")

	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if jerr := json.Unmarshal([]byte(stdout), &created); jerr != nil || created.ID == "" {
		t.Fatalf("create printed %q, want the item as JSON", stdout)
	}
	if got, want := logLines(t, dir, "log"), []string{"h " + created.ID}; !slices.Equal(got, want) {
		t.Errorf("hook log = %q, want %q", got, want)
	}
}

func TestW02_EveryMutatingVerbFiresTheHookOncePerItem(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, logTask+`hooks:
  h:
    event: [updated]
    changed: [status, tags]
    run: log
`)
	todoItems(t, dir, "ex-aaaa", "ex-bbbb", "ex-cccc", "ex-dddd", "ex-eeee")
	allowHooks(t, dir)

	var want []string
	for _, args := range [][]string{
		{"start", "ex-aaaa"},
		{"complete", "ex-bbbb"},
		{"scrap", "ex-cccc", "--reason", "not needed"},
		{"tag", "ex-dddd", "--tag", "t"},
		{"update", "ex-eeee", "-s", "in-progress"},
	} {
		if _, _, err := runInProject(t, dir, args...); err != nil {
			t.Fatalf("lif %v: %v", args, err)
		}
		want = append(want, "h "+args[1])
		if got := logLines(t, dir, "log"); !slices.Equal(got, want) {
			t.Fatalf("after lif %s the hook log is %q, want %q", args[0], got, want)
		}
	}
}

// listFields splits a listing row at its column gaps (two or more spaces).
func listFields(row string) []string {
	return regexp.MustCompile(` {2,}`).Split(row, -1)
}

func TestW03_ListShowsTheProjectStateOriginAndStage(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `tasks:
  g-task:
    steps: ['true']
hooks:
  g-gate:
    stage: before
    event: [updated]
    changed: [status]
    to: [completed]
    run: g-task
    message: "No."
`)
	gRow := []string{"g-gate", "[global]", "before", "updated changed status to completed", "g-task"}
	pRow := []string{"p-hook", "[project]", "after", "created,updated changed status to in-progress type feature,task", "p-task id"}

	for _, tc := range []struct {
		name    string
		project string
		allow   bool
		first   string
		rows    [][]string
	}{
		{"none", "", false, "Project hooks: none", [][]string{gRow}},
		{"not allowed", projectHookYAML, false, "Project hooks: not allowed (run 'lif hooks allow')", [][]string{gRow, pRow}},
		{"allowed", projectHookYAML, true, "Project hooks: allowed", [][]string{gRow, pRow}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := hookProject(t, tc.project)
			if tc.allow {
				allowHooks(t, dir)
			}

			stdout, _, err := runInProject(t, dir, "hooks")

			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if lines[0] != tc.first {
				t.Errorf("first line = %q, want %q", lines[0], tc.first)
			}
			if len(lines)-1 != len(tc.rows) {
				t.Fatalf("listing has %d rows, want %d:\n%s", len(lines)-1, len(tc.rows), stdout)
			}
			for i, want := range tc.rows {
				if got := listFields(lines[i+1]); !slices.Equal(got, want) {
					t.Errorf("row %d = %q, want %q", i+1, got, want)
				}
			}
		})
	}
}

const projectHookYAML = `tasks:
  p-task:
    args: [id]
    steps: ['true']
hooks:
  p-hook:
    event: [created, updated]
    changed: [status]
    to: [in-progress]
    type: [feature, task]
    run: p-task
    args: [id]
`

func TestW04_AllowWithoutProjectHooksIsAnError(t *testing.T) {
	home := hookEnv(t)
	dir := hookProject(t, "tasks:\n  t:\n    steps: ['true']\n")

	_, _, err := runInProject(t, dir, "hooks", "allow")

	if err == nil || err.Error() != "no project hooks to allow" {
		t.Fatalf("err = %v, want %q", err, "no project hooks to allow")
	}
	if _, serr := os.Stat(filepath.Join(home, ".lif", "allowed-hooks.yml")); serr == nil {
		t.Error("the trust file was written")
	}
}

func TestW06_PromoteFiresTheCreatedHookOncePerItem(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, logTask+`hooks:
  h:
    event: [created]
    run: log
`)
	allowHooks(t, dir)
	artifact := writeArtifact(t, `{"findings":[`+validBugRecord+`,`+validImprovementRecord+`]}`)

	stdout, _, err := runInProject(t, dir, "promote", artifact, "B01", "I02", "--json")

	if err != nil {
		t.Fatal(err)
	}
	var made []struct {
		ID string `json:"id"`
	}
	if jerr := json.Unmarshal([]byte(stdout), &made); jerr != nil || len(made) != 2 {
		t.Fatalf("promote printed %q, want a JSON array of two items", stdout)
	}
	want := []string{"h " + made[0].ID, "h " + made[1].ID}
	if got := logLines(t, dir, "log"); !slices.Equal(got, want) {
		t.Errorf("hook log = %q, want %q", got, want)
	}
}

func TestW07_GraphQLMutationsFireTheirHooksInProcess(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, logTask+`hooks:
  h-create:
    event: [created]
    run: log
  h-update:
    event: [updated]
    to: [in-progress]
    run: log
  h-parent:
    event: [updated]
    changed: [parent]
    run: log
`)
	todoItems(t, dir, "ex-aaaa")
	hookItem(t, dir, "ex-pppp", "todo", "epic", "")
	todoItems(t, dir, "ex-cccc")
	allowHooks(t, dir)

	created, _, err := runInProject(t, dir, "graphql", "--json",
		`mutation { createItem(input: {title: "G"}) { id } }`)
	if err != nil {
		t.Fatalf("createItem: %v", err)
	}
	var data struct {
		CreateItem struct {
			ID string `json:"id"`
		} `json:"createItem"`
	}
	if jerr := json.Unmarshal([]byte(created), &data); jerr != nil || data.CreateItem.ID == "" {
		t.Fatalf("createItem printed %q", created)
	}
	if _, _, err := runInProject(t, dir, "graphql", "--json",
		`mutation { updateItem(id: "ex-aaaa", input: {status: "in-progress"}) { id } }`); err != nil {
		t.Fatalf("updateItem: %v", err)
	}
	if _, _, err := runInProject(t, dir, "graphql", "--json",
		`mutation { setParent(id: "ex-cccc", parentId: "ex-pppp") { id } }`); err != nil {
		t.Fatalf("setParent: %v", err)
	}

	want := []string{"h-create " + data.CreateItem.ID, "h-update ex-aaaa", "h-parent ex-cccc"}
	if got := logLines(t, dir, "log"); !slices.Equal(got, want) {
		t.Errorf("hook log = %q, want %q", got, want)
	}
}

func TestW08_DeletingAParentFiresNoHookForItsChild(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, logTask+`hooks:
  h:
    event: [updated]
    changed: [parent]
    run: log
`)
	hookItem(t, dir, "ex-pppp", "todo", "epic", "")
	hookItem(t, dir, "ex-cccc", "todo", "task", "parent: ex-pppp\n")
	allowHooks(t, dir)

	if _, _, err := runInProject(t, dir, "delete", "ex-pppp", "-f"); err != nil {
		t.Fatal(err)
	}

	if got := readHookItem(t, dir, "ex-cccc").Parent; got != "" {
		t.Fatalf("child parent = %q, want it cleared by the delete: the case would prove nothing", got)
	}
	if got := logLines(t, dir, "log"); got != nil {
		t.Errorf("hook log = %q, want the hook not to fire for the cleared parent", got)
	}
}

func TestW09_CreatedHookThatWritesItsItemDoesNotBreakCreateUnset(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  tag-t:
    args: [id]
    steps: ['"$LIF" update "$id" --tag t']
hooks:
  h:
    event: [created]
    run: tag-t
    args: [id]
`)
	allowHooks(t, dir)

	stdout, _, err := runInProject(t, dir, "create", "Thing", "--set", "k=v", "--unset", "k", "--json")

	if err != nil {
		t.Fatalf("create = %v, want it to succeed: --unset belongs to the create write", err)
	}
	var created struct {
		Path string `json:"path"`
	}
	if jerr := json.Unmarshal([]byte(stdout), &created); jerr != nil || created.Path == "" {
		t.Fatalf("create printed %q, want the item as JSON", stdout)
	}
	data, rerr := os.ReadFile(filepath.Join(dir, config.DefaultStoreDir, created.Path))
	if rerr != nil {
		t.Fatal(rerr)
	}
	got, perr := item.Parse(bytes.NewReader(data))
	if perr != nil {
		t.Fatal(perr)
	}
	if _, ok := got.Extra["k"]; ok {
		t.Errorf("extra = %v, want no key k", got.Extra)
	}
	if !slices.Equal(got.Tags, []string{"t"}) {
		t.Errorf("tags = %v, want [t]: the hook's write did not land", got.Tags)
	}
}

func TestW10_ListShowsAGlobalHookThatNamesAProjectOnlyTaskAsAnError(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `hooks:
  gh:
    event: [updated]
    run: x
`)
	dir := hookProject(t, "tasks:\n  x:\n    steps: ['true']\n")

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "gh") {
		t.Fatalf("listing = %q, want the state line and the row of gh", stdout)
	}
	want := `(error: unknown task "x"; run 'lif run' to list tasks)`
	if !strings.HasSuffix(lines[1], want) {
		t.Errorf("row = %q, want it to end with %q", lines[1], want)
	}
}
