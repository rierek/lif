package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func TestD01_EnvAndArgsCarryBeforeAndAfterState(t *testing.T) {
	p := newProj(t, tasksOf(task("t",
		`printf '%s %s>%s %s>%s [%s] %s' "$LIF_ITEM_ID" "$LIF_PREV_STATUS" "$LIF_STATUS" "$LIF_PREV_PARENT" "$LIF_PARENT" "$LIF_CHANGED" "$1"`))+
		hooksOf(hook("h", "event: [updated]", "run: t", "args: [prev_type]")))
	p.allow()
	prev := &item.Item{ID: "beans-x1", Status: "todo", Type: "task", Parent: "m1"}
	next := &item.Item{ID: "beans-x1", Status: "in-progress", Type: "feature", Parent: "m2"}

	if !p.d.Fire(upd(prev, next, "status", "type", "parent")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "beans-x1 todo>in-progress m1>m2 [status type parent] task"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

func TestD02_ExtraKeyReachesArgsAndEnvBeforeAndAfter(t *testing.T) {
	p := newProj(t, tasksOf(task("t",
		`printf '%s|%s|%s|%s' "$1" "$2" "$LIF_FIELD_TARGET_ENTRY" "$LIF_PREV_FIELD_TARGET_ENTRY"`))+
		hooksOf(hook("h", "event: [updated]", "run: t", "args: [target_entry, prev_target_entry]")))
	p.allow()
	prev, next := it("todo"), it("todo")
	prev.Extra = map[string]any{"target_entry": ""}
	next.Extra = map[string]any{"target_entry": "docs/R.md"}

	if !p.d.Fire(upd(prev, next, "target_entry")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "docs/R.md||docs/R.md|"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

func TestD03_ExtraValuesAreFormattedByKind(t *testing.T) {
	p := newProj(t, tasksOf(task("t",
		`printf '%s|%s|%s|%s|%s' "$1" "$2" "$3" "$4" "$LIF_FIELD_MY_KEY"`))+
		hooksOf(hook("h", "event: [updated]", "run: t", "args: [n, ok, list, m]")))
	p.allow()
	next := it("todo")
	next.Extra = map[string]any{
		"n":      3,
		"ok":     true,
		"list":   []any{"a", "b"},
		"m":      map[string]any{"k": "v"},
		"my-key": "z",
	}

	if !p.d.Fire(upd(it("todo"), next, "list", "m", "my-key", "n", "ok")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), `3|true|a b|{"k":"v"}|z`; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

func TestD04_EnvNamesHookStageAndEvent(t *testing.T) {
	const script = `printf '%s %s %s %s' "$LIF_HOOK" "$LIF_STAGE" "$LIF_EVENT" "$LIF_TAGS"`
	prev, next := it("todo"), it("todo", "a", "b")
	m := upd(prev, next, "tags")

	t.Run("after hook output goes to warn", func(t *testing.T) {
		p := newProj(t, tasksOf(task("t", script))+hooksOf(hook("h", "event: [updated]", "run: t")))
		p.allow()
		if !p.d.Fire(m) {
			t.Fatal("Fire reported no run")
		}
		if got, want := p.warn.String(), "h after updated a b"; got != want {
			t.Errorf("warn = %q, want %q", got, want)
		}
	})

	t.Run("gate output is the block output", func(t *testing.T) {
		p := newProj(t, tasksOf(task("t", script+"; exit 1"))+hooksOf(hook("h", "stage: before", "event: [updated]", "run: t")))
		p.allow()
		_, err := p.d.Gate(m)
		var blocked *BlockedError
		if !errors.As(err, &blocked) {
			t.Fatalf("Gate error = %v, want *BlockedError", err)
		}
		if want := "h before updated a b"; blocked.Output != want {
			t.Errorf("Output = %q, want %q", blocked.Output, want)
		}
	})
}

func TestD05_ArgsAreNotInterpretedByTheShell(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `printf '%s' "$1"`))+
		hooksOf(hook("h", "event: [updated]", "run: t", "args: [title]")))
	p.allow()
	next := it("todo")
	next.Title = "$(echo pwned)"

	if !p.d.Fire(upd(it("todo"), next, "tags")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "$(echo pwned)"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

func TestD06_GateSeesTheOldFileAndTheNewStatus(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "grep status: \"$LIF_ITEM_PATH\"\necho \"$LIF_STATUS\"\nexit 1"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
	p.allow()
	writeFile(t, filepath.Join(p.store, "sub", "beans-x1--t.md"), "---\ntitle: T\nstatus: todo\n---\n")
	prev, next := it("todo"), it("completed")
	prev.Path, next.Path = "sub/beans-x1--t.md", "sub/beans-x1--t.md"

	_, err := p.d.Gate(upd(prev, next, "status"))
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("Gate error = %v, want *BlockedError", err)
	}
	for _, want := range []string{"todo", "completed"} {
		if !strings.Contains(blocked.Output, want) {
			t.Errorf("Output = %q, want it to contain %q", blocked.Output, want)
		}
	}
	if strings.Contains(blocked.Output, "status: completed") {
		t.Errorf("Output = %q: the file already holds the new status", blocked.Output)
	}
}

func TestD07_StdinIsEmpty(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "if read l; then echo line; else echo EOF; fi"))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	// Input waits on the process's own stdin; the task must not see it.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("typed ahead\n"); err != nil {
		t.Fatal(err)
	}
	w.Close()
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin; r.Close() })

	if !p.d.Fire(toProgress()) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "EOF\n"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

func TestD08_TwoKeysOfOneEnvNameFirstInSortedOrderWins(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `printf '[%s]' "$LIF_FIELD_MY_KEY"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	next := it("todo")
	next.Extra = map[string]any{"my-key": "a", "my_key": "b"}

	if !p.d.Fire(upd(it("todo"), next, "my-key", "my_key")) {
		t.Fatal("Fire reported no run")
	}
	out := p.warn.String()
	if !strings.Contains(out, "[a]") {
		t.Errorf("warn = %q, want the task to print [a]", out)
	}
	if want := `warning: hook "h": extra key "my_key" maps to LIF_FIELD_MY_KEY, taken by "my-key"; ignored` + "\n"; !strings.Contains(out, want) {
		t.Errorf("warn = %q, want it to contain %q", out, want)
	}
}

func TestD09_BuiltInNameWinsOverAnExtraKey(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `printf '%s' "$1"`))+
		hooksOf(hook("h", "event: [updated]", "run: t", "args: [id]")))
	p.allow()
	next := it("todo")
	next.Extra = map[string]any{"id": "x"}

	if !p.d.Fire(upd(it("todo"), next, "id")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "beans-x1"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}

// TestEnvListsEveryVariable pins the whole environment contract of a hook
// task: D01-D09 each look at some of these variables.
func TestEnvListsEveryVariable(t *testing.T) {
	p := newProj(t, tasksOf(task("t",
		`printf '%s\n' "NO_HOOKS=$LIF_NO_HOOKS" "HOOK=$LIF_HOOK" "STAGE=$LIF_STAGE" "EVENT=$LIF_EVENT" "CHANGED=$LIF_CHANGED" `+
			`"ID=$LIF_ITEM_ID" "PATH=$LIF_ITEM_PATH" "STATUS=$LIF_STATUS" "TYPE=$LIF_TYPE" "PARENT=$LIF_PARENT" "TAGS=$LIF_TAGS" `+
			`"PREV_STATUS=$LIF_PREV_STATUS" "PREV_TYPE=$LIF_PREV_TYPE" "PREV_PARENT=$LIF_PREV_PARENT" "PREV_TAGS=$LIF_PREV_TAGS" `+
			`"FIELD_K=$LIF_FIELD_K" "PREV_FIELD_K=$LIF_PREV_FIELD_K"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	prev := &item.Item{ID: "beans-x1", Path: "p/x.md", Status: "todo", Type: "task", Parent: "m1", Tags: []string{"a"},
		Extra: map[string]any{"k": "w"}}
	next := &item.Item{ID: "beans-x1", Path: "p/x.md", Status: "in-progress", Type: "feature", Parent: "m2", Tags: []string{"a", "b"},
		Extra: map[string]any{"k": "v"}}

	if !p.d.Fire(upd(prev, next, "status", "type", "parent", "tags", "k")) {
		t.Fatal("Fire reported no run")
	}
	want := strings.Join([]string{
		"NO_HOOKS=1", "HOOK=h", "STAGE=after", "EVENT=updated", "CHANGED=status type parent tags k",
		"ID=beans-x1", "PATH=" + filepath.Join(p.store, "p", "x.md"), "STATUS=in-progress", "TYPE=feature", "PARENT=m2", "TAGS=a b",
		"PREV_STATUS=todo", "PREV_TYPE=task", "PREV_PARENT=m1", "PREV_TAGS=a",
		"FIELD_K=v", "PREV_FIELD_K=w",
	}, "\n") + "\n"
	if got := p.warn.String(); got != want {
		t.Errorf("task environment:\n%s\nwant:\n%s", got, want)
	}
}
