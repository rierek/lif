package tasks

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
)

const testLifPath = "/opt/lif-test/lif"

func TestMain(m *testing.M) {
	LifPath = testLifPath
	os.Exit(m.Run())
}

func steps(s ...string) config.Task { return config.Task{Steps: s} }

// runTask plans and runs name; it returns stdout, stderr and the run error.
func runTask(t *testing.T, set Set, name string, forceGlobal bool, args []string, stdin string, extraEnv []string) (string, string, error) {
	t.Helper()
	plan, err := set.Plan(name, forceGlobal)
	if err != nil {
		t.Fatalf("Plan(%q): %v", name, err)
	}
	var out, errb bytes.Buffer
	io := IO{Stdout: &out, Stderr: &errb}
	if stdin != "" {
		io.Stdin = strings.NewReader(stdin)
	}
	err = Run(set.Root, plan, args, io, extraEnv)
	return out.String(), errb.String(), err
}

func TestTP01_LookupProjectFirst(t *testing.T) {
	set := Set{
		Project: map[string]config.Task{"who": steps("p")},
		Global:  map[string]config.Task{"who": steps("g")},
	}
	p, ok := set.Lookup("who", false)
	if !ok || p.Origin != "project" || p.Name != "who" {
		t.Fatalf("Lookup = %+v, %v; want origin project", p, ok)
	}
}

func TestTP02_ForceGlobal(t *testing.T) {
	set := Set{
		Project: map[string]config.Task{"who": steps("p"), "x": steps("p")},
		Global:  map[string]config.Task{"who": steps("g")},
	}
	p, ok := set.Lookup("who", true)
	if !ok || p.Origin != "global" {
		t.Fatalf("Lookup = %+v, %v; want origin global", p, ok)
	}
	_, err := set.Plan("x", true)
	if err == nil || err.Error() != `no global task "x"` {
		t.Fatalf("Plan err = %v", err)
	}
}

func TestTP03_UnknownTask(t *testing.T) {
	_, err := Set{}.Plan("nope", false)
	want := `unknown task "nope"; run 'lif run' to list tasks`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v; want %s", err, want)
	}
}

func TestTP04_DepsOrderEachOnce(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"a": {Deps: []string{"b", "c"}, Steps: []string{"echo a"}},
		"b": {Deps: []string{"c"}, Steps: []string{"echo b"}},
		"c": steps("echo c"),
	}}
	out, _, err := runTask(t, set, "a", false, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "c\nb\na\n" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP05_DepsIgnoreForceGlobal(t *testing.T) {
	set := Set{
		Project: map[string]config.Task{"b": steps("echo project-b")},
		Global: map[string]config.Task{
			"a": {Deps: []string{"b"}, Steps: []string{"echo a"}},
			"b": steps("echo global-b"),
		},
	}
	plan, err := set.Plan("a", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || plan[0].Name != "b" || plan[0].Origin != "project" || plan[1].Name != "a" || plan[1].Origin != "global" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestTP06_UnknownDependency(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	set := Set{Project: map[string]config.Task{
		"a": {Deps: []string{"zz"}, Steps: []string{"touch " + marker}},
	}}
	_, err := set.Plan("a", false)
	want := `task "a": unknown dependency "zz"`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v; want %s", err, want)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("step ran")
	}
}

func TestTP07_Cycle(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	set := Set{Project: map[string]config.Task{
		"a": {Deps: []string{"b"}, Steps: []string{"touch " + marker}},
		"b": {Deps: []string{"a"}, Steps: []string{"touch " + marker}},
	}}
	_, err := set.Plan("a", false)
	want := "task cycle: a -> b -> a"
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v; want %s", err, want)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("step ran")
	}
}

func TestTP08_DependencyWithArgs(t *testing.T) {
	set := Set{Project: map[string]config.Task{
		"a": {Deps: []string{"b"}, Steps: []string{"echo a"}},
		"b": {Args: []string{"id"}, Steps: []string{"echo b"}},
	}}
	_, err := set.Plan("a", false)
	want := `task "a": dependency "b" needs arguments (id); dependencies run without arguments`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v; want %s", err, want)
	}
}

func TestTP09_CheckArgsMissing(t *testing.T) {
	set := Set{Project: map[string]config.Task{"show": {Args: []string{"id"}, Steps: []string{"echo"}}}}
	plan, err := set.Plan("show", false)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckArgs(plan[len(plan)-1], nil)
	want := `task "show" needs argument(s): id (usage: lif run show <id>)`
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v; want %s", err, want)
	}
}

func TestTP10_ArgsAndPositional(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"show": {Args: []string{"a"}, Steps: []string{`printf '%s|%s|%s' "$0" "$a" "$*"`}},
	}}
	out, _, err := runTask(t, set, "show", false, []string{"x", "y", "z"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "show|x|x y z" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP11_DepGetsNoArgs(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"top": {Args: []string{"id"}, Deps: []string{"dep"}, Steps: []string{`printf 'top%s' "$#"`}},
		"dep": steps(`printf '%s ' "$#"`),
	}}
	out, _, err := runTask(t, set, "top", false, []string{"7"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "0 top1" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP12_StepEnvironment(t *testing.T) {
	old := LifPath
	LifPath = "/literal/path/to/lif"
	t.Cleanup(func() { LifPath = old })
	root := t.TempDir()
	step := `echo "$LIF_TASK $LIF_TASK_ORIGIN $LIF_ROOT $LIF"`
	set := Set{
		Root:    root,
		Project: map[string]config.Task{"b": steps(step)},
		Global:  map[string]config.Task{"a": {Deps: []string{"b"}, Steps: []string{step}}},
	}
	out, _, err := runTask(t, set, "a", true, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "b project " + root + " /literal/path/to/lif\n" +
		"a global " + root + " /literal/path/to/lif\n"
	if out != want {
		t.Fatalf("out = %q; want %q", out, want)
	}
}

func TestTP13_ExtraEnvAndArgPrecedence(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"t": {Args: []string{"a"}, Steps: []string{`printf '%s|%s' "$E" "$a"`}},
	}}
	out, _, err := runTask(t, set, "t", false, []string{"arg"}, "", []string{"E=1", "a=env"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "1|arg" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP14_ArgsStayInert(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"t": {Args: []string{"a"}, Steps: []string{`printf '%s' "$a"`}},
	}}
	out, _, err := runTask(t, set, "t", false, []string{"$(echo pwned)"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "$(echo pwned)" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP15_MultiLineStepIsOneScript(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"t": steps("v=1\necho \"$v\""),
	}}
	out, _, err := runTask(t, set, "t", false, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "1\n" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP16_FailingStepStopsRun(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"fail":  steps("exit 3", "echo never"),
		"later": {Deps: []string{"fail"}, Steps: []string{"echo later"}},
	}}
	out, _, err := runTask(t, set, "later", false, nil, "", nil)
	var se *StepError
	if !errors.As(err, &se) || se.Code != 3 || se.Task != "fail" || se.Step != 1 || se.Total != 2 {
		t.Fatalf("err = %#v", err)
	}
	want := `task "fail": step 1 of 2 failed (exit code 3)`
	if err.Error() != want {
		t.Fatalf("msg = %q; want %q", err.Error(), want)
	}
	if strings.Contains(out, "never") || strings.Contains(out, "later") {
		t.Fatalf("out = %q", out)
	}
}

func TestTP17_SignalIsStepErrorWithoutCode(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"x": steps("kill -TERM $$"),
	}}
	_, _, err := runTask(t, set, "x", false, nil, "", nil)
	var se *StepError
	if !errors.As(err, &se) || se.Code != 0 || se.Err == nil {
		t.Fatalf("err = %#v", err)
	}
	if !strings.HasPrefix(err.Error(), `task "x": step 1 of 1: `) {
		t.Fatalf("msg = %q", err.Error())
	}
}

func TestTP18_StepsRunInRoot(t *testing.T) {
	root := t.TempDir()
	set := Set{Root: root, Project: map[string]config.Task{"x": steps("pwd")}}
	out, _, err := runTask(t, set, "x", false, nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err1 := filepath.EvalSymlinks(strings.TrimSpace(out))
	want, err2 := filepath.EvalSymlinks(root)
	if err1 != nil || err2 != nil || got != want {
		t.Fatalf("pwd = %q (%v); want %q (%v)", got, err1, want, err2)
	}
}

func TestTP19_StdinReachesStep(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"x": steps(`read l; echo "$l"`),
	}}
	out, _, err := runTask(t, set, "x", false, nil, "hi\n", nil)
	if err != nil {
		t.Fatal(err)
	}
	if out != "hi\n" {
		t.Fatalf("out = %q", out)
	}
}

func TestTP20_EmptyLifPath(t *testing.T) {
	old := LifPath
	LifPath = ""
	t.Cleanup(func() { LifPath = old })
	root := t.TempDir()
	marker := filepath.Join(root, "marker")
	set := Set{Root: root, Project: map[string]config.Task{"x": steps("touch " + marker)}}
	plan, err := set.Plan("x", false)
	if err != nil {
		t.Fatal(err)
	}
	err = Run(root, plan, nil, IO{Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}, nil)
	if err == nil || err.Error() != "lif binary not found" {
		t.Fatalf("err = %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("step ran")
	}
}

func TestTP21_BackgroundChildDoesNotHang(t *testing.T) {
	set := Set{Root: t.TempDir(), Project: map[string]config.Task{
		"x": steps("sleep 5 & exit 0"),
	}}
	start := time.Now()
	_, _, err := runTask(t, set, "x", false, nil, "", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d >= 4*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestTP22_CycleKeyedByOrigin(t *testing.T) {
	set := Set{
		Project: map[string]config.Task{"shadow": steps("echo project")},
		Global:  map[string]config.Task{"shadow": {Deps: []string{"shadow"}, Steps: []string{"echo global"}}},
	}
	plan, err := set.Plan("shadow", true)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan) != 2 || plan[0].Origin != "project" || plan[1].Origin != "global" {
		t.Fatalf("plan = %+v; want project then global", plan)
	}
	// A real cycle through a same-named pair still fails.
	set.Project["shadow"] = config.Task{Deps: []string{"shadow"}, Steps: []string{"true"}}
	if _, err := set.Plan("shadow", true); err == nil || !strings.Contains(err.Error(), "task cycle") {
		t.Fatalf("err = %v; want task cycle", err)
	}
}

func TestTP23_ShellFallbackWithoutPath(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	root := t.TempDir()
	set := Set{Root: root, Project: map[string]config.Task{"x": steps("echo hi")}}
	out, _, err := runTask(t, set, "x", false, nil, "", nil)
	if err != nil || out != "hi\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}
