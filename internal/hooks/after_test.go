package hooks

import (
	"strings"
	"sync"
	"testing"
)

func TestA04_NoHooksSkipsAfterHooks(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	t.Setenv("LIF_NO_HOOKS", "1")

	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if p.exists("ran") {
		t.Error("the after hook ran although LIF_NO_HOOKS is set")
	}
	if p.warn.Len() != 0 {
		t.Errorf("warn = %q, want nothing", p.warn.String())
	}
}

func TestA05_HooksRunInNameOrder(t *testing.T) {
	p := newProj(t, tasksOf(
		task("ta", `echo a >> "$LIF_ROOT/order"`),
		task("tb", `echo b >> "$LIF_ROOT/order"`))+
		hooksOf(
			hook("b", "event: [updated]", "run: tb"),
			hook("a", "event: [updated]", "run: ta")))
	p.allow()

	if !p.d.Fire(toProgress()) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.read("order"), "a\nb\n"; got != want {
		t.Errorf("run order = %q, want %q", got, want)
	}
}

func TestA06_GlobalHookCannotReachAProjectOnlyTask(t *testing.T) {
	p := newProj(t, tasksOf(task("only-project", `touch "$LIF_ROOT/ran"`)))
	p.global(hooksOf(hook("g", "event: [updated]", "run: only-project")))

	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if p.exists("ran") {
		t.Error("the global hook reached the project task")
	}
	out := p.warn.String()
	if !strings.HasPrefix(out, `warning: hook "g" (task "only-project"): `) || !strings.Contains(out, "unknown task") {
		t.Errorf("warn = %q, want a warning for hook g that contains \"unknown task\"", out)
	}
}

func TestA07_ProjectHookReplacesAGlobalHookOfTheSameName(t *testing.T) {
	p := newProj(t, tasksOf(task("tp", `touch "$LIF_ROOT/project"`))+
		hooksOf(hook("x", "event: [updated]", "run: tp")))
	p.global(tasksOf(task("tg", `touch "$LIF_ROOT/global"`)) +
		hooksOf(hook("x", "event: [updated]", "run: tg")))
	p.allow()

	if !p.d.Fire(toProgress()) {
		t.Fatal("Fire reported no run")
	}
	if !p.exists("project") {
		t.Error("the project hook did not run")
	}
	if p.exists("global") {
		t.Error("the global hook of the same name ran too")
	}
}

func TestA08_GlobalTaskDependenciesResolveInTheGlobalSetOnly(t *testing.T) {
	globalTasks := tasksOf(
		task("d", `touch "$LIF_ROOT/global-d"`),
		task("t", `touch "$LIF_ROOT/t"`, "deps: [d]"))
	hookT := hooksOf(hook("h", "event: [updated]", "run: t"))

	t.Run("d in the project and in the global file", func(t *testing.T) {
		p := newProj(t, tasksOf(task("d", `touch "$LIF_ROOT/project-d"`)))
		p.global(globalTasks + hookT)

		if !p.d.Fire(toProgress()) {
			t.Fatal("Fire reported no run")
		}
		if !p.exists("global-d") || !p.exists("t") {
			t.Errorf("global-d ran = %v, t ran = %v; want both", p.exists("global-d"), p.exists("t"))
		}
		if p.exists("project-d") {
			t.Error("the project task d ran for a global hook")
		}
	})

	t.Run("d only in the project", func(t *testing.T) {
		p := newProj(t, tasksOf(task("d", `touch "$LIF_ROOT/project-d"`)))
		p.global(tasksOf(task("t", `touch "$LIF_ROOT/t"`, "deps: [d]")) + hookT)

		if p.d.Fire(toProgress()) {
			t.Error("Fire reported a run")
		}
		if p.exists("project-d") || p.exists("t") {
			t.Error("a task ran although its dependency is missing")
		}
		if want := `task "t": unknown dependency "d"`; !strings.Contains(p.warn.String(), want) {
			t.Errorf("warn = %q, want it to contain %q", p.warn.String(), want)
		}
	})
}

func TestA10_TaskRunsDoNotOverlap(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "echo start >> \"$LIF_ROOT/log\"\nsleep 0.2\necho end >> \"$LIF_ROOT/log\""))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()

	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() { p.d.Fire(toProgress()) })
	}
	wg.Wait()

	if got, want := p.read("log"), "start\nend\nstart\nend\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}

// TestFireFailureWarnsAndContinues is the Dispatcher side of A01.
func TestFireFailureWarnsAndContinues(t *testing.T) {
	p := newProj(t, tasksOf(
		task("t", "exit 4"),
		task("m", `touch "$LIF_ROOT/marker"`))+
		hooksOf(
			hook("a", "event: [updated]", "run: t"),
			hook("b", "event: [updated]", "run: m")))
	p.allow()

	if !p.d.Fire(toProgress()) {
		t.Error("Fire reported no run")
	}
	if got, want := p.warn.String(), `warning: hook "a" (task "t"): task "t": step 1 of 1 failed (exit code 4)`+"\n"; got != want {
		t.Errorf("warn = %q, want %q", got, want)
	}
	if !p.exists("marker") {
		t.Error("the second hook did not run after the first one failed")
	}
}

// TestFireOutputGoesToWarn: stdout and stderr of an after hook both reach the
// warn writer, so a --json stdout stays valid.
func TestFireOutputGoesToWarn(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "echo out\necho err >&2"))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()

	p.d.Fire(toProgress())
	out := p.warn.String()
	if !strings.Contains(out, "out\n") || !strings.Contains(out, "err\n") {
		t.Errorf("warn = %q, want stdout and stderr of the hook", out)
	}
}

// TRUST-05: an allowed project hook must not switch off a global gate of the same name.
func TestProjectHookDoesNotReplaceAGlobalGateOfTheSameName(t *testing.T) {
	p := newProj(t, tasksOf(task("ok", "true"))+
		hooksOf(hook("x", "event: [updated]", "run: ok")))
	p.global(tasksOf(task("deny", "exit 1")) +
		hooksOf(hook("x", "stage: before", "event: [updated]", "run: deny", "message: policy")))
	p.allow()

	if _, err := p.d.Gate(toProgress()); err == nil {
		t.Fatal("Gate error = nil, want the global gate to block")
	}
	if !strings.Contains(p.warn.String(), `hook "x"`) {
		t.Errorf("warning = %q, want it to name the ignored hook", p.warn)
	}
}
