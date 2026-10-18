package commands

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A resolver that does not come from newResolver carries no dispatcher, so
// the writes made through it fire no hook and run no gate. The pin lists
// every place of the package that builds one: only newResolver (root.go)
// may, so a missed or a new call site fails here.
func TestEveryResolverComesFromNewResolver(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var sites []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.CompositeLit); ok {
				if sel, ok := lit.Type.(*ast.SelectorExpr); ok && sel.Sel.Name == "CoreResolver" {
					sites = append(sites, name)
				}
			}
			return true
		})
	}
	if want := []string{"root.go"}; !slices.Equal(sites, want) {
		t.Errorf("itemgraph.CoreResolver is built in %v, want only %v (newResolver): use newResolver()", sites, want)
	}
}

// W11 proves that exemptWriteTimeout lifts the deadline. This pin proves that
// lif serve uses it for the endpoint that carries every mutation of the web
// UI: a hook that runs longer than the write timeout must not cost the
// response of a mutation that is already written.
func TestServeMountsGraphQLBehindTheWriteTimeoutExemption(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "serve.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Any" {
			return true
		}
		if lit, ok := call.Args[0].(*ast.BasicLit); !ok || lit.Value != `"/api/graphql"` {
			return true
		}
		found = true
		mw, ok := call.Args[1].(*ast.CallExpr)
		if !ok {
			t.Error(`/api/graphql is mounted without exemptWriteTimeout() as its first handler`)
			return true
		}
		if id, ok := mw.Fun.(*ast.Ident); !ok || id.Name != "exemptWriteTimeout" {
			t.Error(`/api/graphql is mounted without exemptWriteTimeout() as its first handler`)
		}
		return true
	})
	if !found {
		t.Error(`serve.go does not mount "/api/graphql" with router.Any any more`)
	}
}

// listState returns the first line of `lif hooks`.
func listState(t *testing.T, dir string) string {
	t.Helper()
	stdout, _, err := runInProject(t, dir, "hooks")
	if err != nil {
		t.Fatalf("lif hooks: %v", err)
	}
	first, _, _ := strings.Cut(stdout, "\n")
	return first
}

func TestHooksAllowAndDenyChangeTheListedState(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, projectHookYAML)
	path := filepath.Join(dir, ".lif.yml")
	const notAllowed = "Project hooks: not allowed (run 'lif hooks allow')"

	if got := listState(t, dir); got != notAllowed {
		t.Errorf("before allow: %q, want %q", got, notAllowed)
	}
	out, _, err := runInProject(t, dir, "hooks", "allow")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Allowed hooks of " + path + " (1 hooks). These steps may now run:\n"; !strings.HasPrefix(out, want) {
		t.Errorf("allow printed %q, want the prefix %q", out, want)
	}
	if !strings.Contains(out, "step 1: \"true\"") {
		t.Errorf("allow printed %q, want the steps that it allows", out)
	}
	if got := listState(t, dir); got != "Project hooks: allowed" {
		t.Errorf("after allow: %q, want allowed", got)
	}

	// The allow covers the hooks and tasks as reviewed: a changed step revokes it.
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, path, strings.Replace(string(text), "steps: ['true']", "steps: ['false']", 1))
	if got := listState(t, dir); got != notAllowed {
		t.Errorf("after a task step changed: %q, want %q", got, notAllowed)
	}

	if _, _, err := runInProject(t, dir, "hooks", "allow"); err != nil {
		t.Fatal(err)
	}
	out, _, err = runInProject(t, dir, "hooks", "deny")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Hooks of " + path + " are no longer allowed.\n"; out != want {
		t.Errorf("deny printed %q, want %q", out, want)
	}
	if got := listState(t, dir); got != notAllowed {
		t.Errorf("after deny: %q, want %q", got, notAllowed)
	}
}

func TestHooksListNamesAProjectHookThatOverridesAGlobalOne(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `tasks:
  t:
    steps: ['true']
hooks:
  x:
    event: [updated]
    run: t
`)
	dir := hookProject(t, `tasks:
  t:
    steps: ['true']
hooks:
  x:
    event: [updated]
    changed: [status]
    run: t
`)
	allowHooks(t, dir)

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("listing = %q, want the state line and one row", stdout)
	}
	want := []string{"x", "[project, overrides global]", "after", "updated changed status", "t"}
	if got := listFields(lines[1]); !slices.Equal(got, want) {
		t.Errorf("row = %q, want %q", got, want)
	}
}

// The listing resolves a hook's task the way the dispatcher does: a project
// hook sees project and global tasks, a global hook only global tasks. A
// hook that gives its task too few arguments shows the reason too.
func TestHooksListResolvesTasksLikeTheDispatcher(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `tasks:
  gt:
    steps: ['true']
hooks:
  g-global-task:
    event: [updated]
    run: gt
  g-project-task:
    event: [updated]
    run: pt
`)
	dir := hookProject(t, `tasks:
  pt:
    steps: ['true']
  two:
    args: [id, x]
    steps: ['true']
hooks:
  p-global-task:
    event: [updated]
    run: gt
  p-project-task:
    event: [updated]
    run: pt
  p-short-args:
    event: [updated]
    run: two
    args: [id]
`)

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")[1:] {
		rows[listFields(line)[0]] = line
	}
	for name, wantError := range map[string]string{
		"g-global-task":  "",
		"p-global-task":  "",
		"p-project-task": "",
		"g-project-task": `(error: unknown task "pt"; run 'lif run' to list tasks)`,
		"p-short-args":   `(error: task "two" needs argument(s): id, x (usage: lif run two <id> <x>))`,
	} {
		row, ok := rows[name]
		switch {
		case !ok:
			t.Errorf("no row for %s in:\n%s", name, stdout)
		case wantError == "" && strings.Contains(row, "(error:"):
			t.Errorf("row %q shows an error, want none", row)
		case wantError != "" && !strings.HasSuffix(row, wantError):
			t.Errorf("row %q, want it to end with %q", row, wantError)
		}
	}
}

func TestHooksDenyNeedsAProjectConfig(t *testing.T) {
	hookEnv(t)
	dir := evalSymlinks(t, t.TempDir())

	_, _, err := runInProject(t, dir, "hooks", "deny")

	if err == nil || err.Error() != "no .lif.yml found" {
		t.Fatalf("err = %v, want %q", err, "no .lif.yml found")
	}
}

// The when column names every condition that the hook sets, in the order of
// the schema, and none that it leaves unset.
func TestHooksListShowsEverySetCondition(t *testing.T) {
	hookEnv(t)
	dir := hookProject(t, `tasks:
  t:
    steps: ['true']
hooks:
  all:
    event: [updated]
    changed: [status, tags]
    from: [todo]
    to: [in-progress]
    type: [task, bug]
    tags: [a]
    tags_added: [b]
    tags_removed: [c]
    run: t
  minimal:
    event: [created]
    run: t
`)

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("listing = %q, want the state line and two rows", stdout)
	}
	want := map[string]string{
		"all":     "updated changed status,tags from todo to in-progress type task,bug tags a tags_added b tags_removed c",
		"minimal": "created",
	}
	for _, line := range lines[1:] {
		fields := listFields(line)
		if got := fields[3]; got != want[fields[0]] {
			t.Errorf("when of %s = %q, want %q", fields[0], got, want[fields[0]])
		}
	}
}

// TRUST-07: a project that is not allowed hides no global hook in the listing.
func TestHooksListShowsTheGlobalHookWhileTheProjectIsNotAllowed(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `tasks:
  t:
    steps: ['true']
hooks:
  x:
    stage: before
    event: [updated]
    run: t
`)
	dir := hookProject(t, `tasks:
  t:
    steps: ['true']
hooks:
  x:
    event: [updated]
    run: t
`)

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("listing = %q, want the state line and two rows", stdout)
	}
	if got, want := listFields(lines[1]), []string{"x", "[global]", "before", "updated", "t"}; !slices.Equal(got, want) {
		t.Errorf("row 1 = %q, want %q", got, want)
	}
	if got, want := listFields(lines[2]), []string{"x", "[project, not allowed]", "after", "updated", "t"}; !slices.Equal(got, want) {
		t.Errorf("row 2 = %q, want %q", got, want)
	}
}

// TRUST-17: a hook or gate must not allow hooks.
func TestHooksAllowRefusesInsideAHook(t *testing.T) {
	hookEnv(t)
	t.Setenv("LIF_HOOK", "h")
	dir := hookProject(t, projectHookYAML)

	_, _, err := runInProject(t, dir, "hooks", "allow")

	if err == nil || !strings.Contains(err.Error(), "refusing to allow") {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

// TRUST-14: deny and allow repair a corrupt trust file.
func TestHooksDenyReplacesACorruptTrustFile(t *testing.T) {
	home := hookEnv(t)
	dir := hookProject(t, projectHookYAML)
	writeFixtureFile(t, filepath.Join(home, ".lif", "allowed-hooks.yml"), "a: [b\n")

	_, stderr, err := runInProject(t, dir, "hooks", "deny")

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "replacing the unreadable trust file") {
		t.Errorf("stderr = %q, want a notice", stderr)
	}
	allowHooks(t, dir)
	if got := listState(t, dir); got != "Project hooks: allowed" {
		t.Errorf("state = %q, want allowed", got)
	}
}

// TRUST-18: a global hook with a status that the project does not know is flagged.
func TestHooksListFlagsAnUnknownStatusOfAGlobalHook(t *testing.T) {
	home := hookEnv(t)
	writeFixtureFile(t, filepath.Join(home, ".lif", "tasks.yml"), `tasks:
  t:
    steps: ['true']
hooks:
  g:
    event: [updated]
    to: [complted]
    run: t
`)
	dir := hookProject(t, "")

	stdout, _, err := runInProject(t, dir, "hooks")

	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `(warning: status "complted" unknown in this project)`) {
		t.Errorf("listing = %q, want the warning", stdout)
	}
}
