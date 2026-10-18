package hooks

import (
	"errors"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tasks"
)

// blockedBy asserts that err is a *BlockedError and returns it.
func blockedBy(t *testing.T, err error) *BlockedError {
	t.Helper()
	var blocked *BlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("Gate error = %v, want *BlockedError", err)
	}
	return blocked
}

func TestG04_GateWithoutMessageShowsOnlyTheTaskOutput(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "echo detail\nexit 1"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
	p.allow()

	_, err := p.d.Gate(toProgress())
	if err == nil {
		t.Fatal("Gate error = nil, want a block")
	}
	if got, want := err.Error(), "hook \"g\" blocked the change of beans-x1\ndetail"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestG06_FirstBlockingGateWinsAndLaterGatesDoNotRun(t *testing.T) {
	p := newProj(t, tasksOf(
		task("fail", "exit 1"),
		task("mark", `touch "$LIF_ROOT/marker"`))+
		hooksOf(
			hook("b", "stage: before", "event: [updated]", "run: mark"),
			hook("a", "stage: before", "event: [updated]", "run: fail")))
	p.allow()

	ran, err := p.d.Gate(toProgress())
	if blocked := blockedBy(t, err); blocked.Hook != "a" {
		t.Errorf("blocked by %q, want a", blocked.Hook)
	}
	if !ran {
		t.Error("ran = false, want true: gate a started")
	}
	if p.exists("marker") {
		t.Error("gate b ran after gate a blocked")
	}
}

// TestPassingGateLetsTheNextGateRun: exit 0 is no block, the next gate follows.
func TestPassingGateLetsTheNextGateRun(t *testing.T) {
	p := newProj(t, tasksOf(
		task("pass", `touch "$LIF_ROOT/marker-a"`),
		task("fail", `touch "$LIF_ROOT/marker-b"`+"\nexit 1"))+
		hooksOf(
			hook("a", "stage: before", "event: [updated]", "run: pass"),
			hook("b", "stage: before", "event: [updated]", "run: fail")))
	p.allow()

	_, err := p.d.Gate(toProgress())
	if blocked := blockedBy(t, err); blocked.Hook != "b" {
		t.Errorf("blocked by %q, want b", blocked.Hook)
	}
	if !p.exists("marker-a") || !p.exists("marker-b") {
		t.Errorf("gate a ran = %v, gate b ran = %v; want both", p.exists("marker-a"), p.exists("marker-b"))
	}
}

func TestG07_GateNamingAnUnknownTaskBlocks(t *testing.T) {
	p := newProj(t, tasksOf(task("other", "true"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: nosuch")))
	p.allow()

	ran, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if !strings.HasPrefix(blocked.Message, "could not run:") || !strings.Contains(blocked.Message, "unknown task") {
		t.Errorf("Message = %q, want it to start with %q and contain %q", blocked.Message, "could not run:", "unknown task")
	}
	if ran {
		t.Error("ran = true, want false: no task started")
	}
}

func TestG08_BrokenGlobalFileBlocksEveryGateCheck(t *testing.T) {
	p := newProj(t, tasksOf(task("mark", `touch "$LIF_ROOT/marker"`))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: mark")))
	p.allow()
	globalPath := p.global("tasks: [unclosed\n")

	ran, err := p.d.Gate(toProgress())
	if err == nil {
		t.Fatal("Gate error = nil, want a block")
	}
	blockedBy(t, err)
	prefix := `hook "*" blocked the change of beans-x1: hooks could not be loaded: ` + globalPath
	if !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("Error() = %q, want the prefix %q", err.Error(), prefix)
	}
	if suffix := " (fix it or set LIF_NO_HOOKS=1)"; !strings.HasSuffix(err.Error(), suffix) {
		t.Errorf("Error() = %q, want the suffix %q", err.Error(), suffix)
	}
	if ran || p.exists("marker") {
		t.Errorf("ran = %v, marker = %v; no gate may run on a load error", ran, p.exists("marker"))
	}
}

func TestG12_GateDoesNotRunForANonMatchingChange(t *testing.T) {
	p := newProj(t, tasksOf(task("mark", `touch "$LIF_ROOT/marker"`))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "to: [completed]", "run: mark")))
	p.allow()

	ran, err := p.d.Gate(toProgress())
	if ran || err != nil {
		t.Errorf("Gate = %v, %v; want false, nil", ran, err)
	}
	if p.exists("marker") {
		t.Error("the gate ran for a change to in-progress")
	}

	// Control: the same gate does run for the change it names.
	ran, err = p.d.Gate(upd(it("todo"), it("completed"), "status"))
	if !ran || err != nil || !p.exists("marker") {
		t.Errorf("control: Gate = %v, %v, marker = %v; want true, nil, true", ran, err, p.exists("marker"))
	}
}

func TestG16_GateWithAMissingTaskArgumentBlocks(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "true", "args: [a, b]"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t", "args: [id]")))
	p.allow()

	_, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if !strings.HasPrefix(blocked.Message, "could not run:") || !strings.Contains(blocked.Message, "needs argument(s)") {
		t.Errorf("Message = %q, want it to start with %q and contain %q", blocked.Message, "could not run:", "needs argument(s)")
	}
}

// TestGateBlockTextAndStepCode is the Dispatcher side of G02: the text of a
// block, and no way to reach the step code of the gate through the error.
func TestGateBlockTextAndStepCode(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "echo detail >&2\nexit 3"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t", `message: "Fix it."`)))
	p.allow()

	_, err := p.d.Gate(toProgress())
	if err == nil {
		t.Fatal("Gate error = nil, want a block")
	}
	if got, want := err.Error(), "hook \"g\" blocked the change of beans-x1: Fix it.\ndetail"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	var step *tasks.StepError
	if errors.As(err, &step) || errors.Unwrap(err) != nil {
		t.Errorf("the block unwraps to %v: exitCode would see the step code of the gate", errors.Unwrap(err))
	}
}

// TestGateMergesStdoutAndStderr: both streams of a gate land in one buffer, in order.
func TestGateMergesStdoutAndStderr(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "echo out\necho err >&2\nexit 1"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
	p.allow()

	_, err := p.d.Gate(toProgress())
	if got, want := blockedBy(t, err).Output, "out\nerr"; got != want {
		t.Errorf("Output = %q, want %q", got, want)
	}
}

// TestGateThatCannotRunBlocks is the Dispatcher side of G17: a gate killed by
// a signal blocks, and so does a missing lif binary.
func TestGateThatCannotRunBlocks(t *testing.T) {
	t.Run("signal", func(t *testing.T) {
		p := newProj(t, tasksOf(task("t", "kill -TERM $$"))+
			hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
		p.allow()

		_, err := p.d.Gate(toProgress())
		if msg := blockedBy(t, err).Message; !strings.HasPrefix(msg, "could not run:") {
			t.Errorf("Message = %q, want it to start with %q", msg, "could not run:")
		}
	})

	t.Run("no lif binary", func(t *testing.T) {
		old := tasks.LifPath
		tasks.LifPath = ""
		t.Cleanup(func() { tasks.LifPath = old })
		p := newProj(t, tasksOf(task("t", "true"))+
			hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
		p.allow()

		_, err := p.d.Gate(toProgress())
		msg := blockedBy(t, err).Message
		if !strings.HasPrefix(msg, "could not run:") || !strings.Contains(msg, "lif binary not found") {
			t.Errorf("Message = %q, want %q and %q", msg, "could not run:", "lif binary not found")
		}
	})
}

func TestBlockedErrorText(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  BlockedError
		want string
	}{
		{"message and output", BlockedError{Hook: "g", ItemID: "beans-x1", Message: "Fix it.", Output: "detail"},
			"hook \"g\" blocked the change of beans-x1: Fix it.\ndetail"},
		{"message only", BlockedError{Hook: "g", ItemID: "beans-x1", Message: "Fix it."},
			`hook "g" blocked the change of beans-x1: Fix it.`},
		{"output only", BlockedError{Hook: "g", ItemID: "beans-x1", Output: "detail"},
			"hook \"g\" blocked the change of beans-x1\ndetail"},
		{"neither", BlockedError{Hook: "g", ItemID: "beans-x1"},
			`hook "g" blocked the change of beans-x1`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TRUST-11: a gate message from .lif.yml must not carry escape sequences.
func TestBlockedErrorDropsControlCharactersOfTheMessage(t *testing.T) {
	e := &BlockedError{Hook: "h", ItemID: "x", Message: "no\x1b]0;PWNED\a\x1b[2J"}
	if got, want := e.Error(), `hook "h" blocked the change of x: no]0;PWNED[2J`; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// GATES-12: a block scalar message ends with a newline, and the task output may carry escapes.
func TestBlockedErrorTrimsTheMessageAndCleansTheOutput(t *testing.T) {
	e := &BlockedError{Hook: "g", ItemID: "x", Message: "Fix it.\n", Output: "a\x1b[31mb"}
	if got, want := e.Error(), "hook \"g\" blocked the change of x: Fix it.\na[31mb"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
