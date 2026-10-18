package hooks

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGateTimeoutBlocksAndKillsTheTask(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "sleep 30"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t", "timeout: 1")))
	p.allow()

	start := time.Now()
	_, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if !strings.Contains(blocked.Message, "could not run: ") || !strings.Contains(blocked.Message, "timed out after 1s") {
		t.Errorf("message = %q", blocked.Message)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Gate took %v", d)
	}
}

func TestAfterHookTimeoutWarnsAndNextHookRuns(t *testing.T) {
	p := newProj(t, tasksOf(task("slow", "sleep 30"), task("mark", `touch "$LIF_ROOT/ran"`))+
		hooksOf(
			hook("a", "event: [updated]", "run: slow", "timeout: 1"),
			hook("b", "event: [updated]", "run: mark")))
	p.allow()

	p.d.Fire(toProgress())
	if !strings.Contains(p.warn.String(), `hook "a" (task "slow")`) || !strings.Contains(p.warn.String(), "timed out after 1s") {
		t.Errorf("warn = %q", p.warn.String())
	}
	if !p.exists("ran") {
		t.Error("hook b did not run")
	}
}

func TestGateOutputIsCapped(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "head -c 3000000 /dev/zero | tr '\\0' x\nexit 1"))+
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: t")))
	p.allow()

	_, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if len(blocked.Output) > maxBlockedOutput+100 || !strings.Contains(blocked.Output, "(output cut after 8 KiB)") {
		t.Errorf("output: %d bytes, tail %q", len(blocked.Output), blocked.Output[max(0, len(blocked.Output)-60):])
	}
}

func TestUnwatchedWriteDoesNotWaitForARunningHook(t *testing.T) {
	p := newProj(t, tasksOf(task("slow", "sleep 3"))+
		hooksOf(hook("h", "event: [updated]", "to: [draft]", "run: slow"),
			hook("g", "stage: before", "event: [updated]", "to: [draft]", "run: slow")))
	p.allow()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.d.Fire(upd(it("todo"), it("draft"), "status"))
	}()
	time.Sleep(300 * time.Millisecond)
	start := time.Now()
	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v", ran, err)
	}
	if p.d.Fire(toProgress()) {
		t.Error("Fire ran a hook")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("non-matching calls waited %v", d)
	}
	wg.Wait()
}

func TestHookDepthLimitStopsRecursion(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+
		hooksOf(hook("h", "event: [updated]", "run: t"),
			hook("g", "stage: before", "event: [updated]", "run: t")))
	p.allow()
	t.Setenv("LIF_HOOK_DEPTH", "3")

	if p.d.Fire(toProgress()) {
		t.Error("Fire ran at the depth limit")
	}
	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v", ran, err)
	}
	if p.exists("ran") || !strings.Contains(p.warn.String(), "LIF_HOOK_DEPTH=3") {
		t.Errorf("ran = %v, warn = %q", p.exists("ran"), p.warn.String())
	}
}

func TestHookTaskGetsDepthPlusOne(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `echo "$LIF_HOOK_DEPTH" > "$LIF_ROOT/depth"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	t.Setenv("LIF_HOOK_DEPTH", "1")

	p.d.Fire(toProgress())
	if got := strings.TrimSpace(p.read("depth")); got != "2" {
		t.Errorf("depth = %q, want 2", got)
	}
}
