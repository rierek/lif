package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemgraph"
)

func TestNilDispatcherDoesNothing(t *testing.T) {
	var d *Dispatcher
	if ran, err := d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v; want false, nil", ran, err)
	}
	if d.Fire(toProgress()) {
		t.Error("Fire = true, want false")
	}
	// The method values of a nil Dispatcher fit the resolver seam.
	_ = &itemgraph.CoreResolver{BeforeMutation: d.Gate, AfterMutation: d.Fire}
}

// TestNoHooksIgnoresABrokenGlobalFile: the escape hatch must work with a
// broken file. Fire does not even load it; Gate skips without a block.
func TestNoHooksIgnoresABrokenGlobalFile(t *testing.T) {
	p := newProj(t, tasksOf(task("t", "exit 1"))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	p.global("tasks: [unclosed\n")
	t.Setenv("LIF_NO_HOOKS", "1")

	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v; want false, nil", ran, err)
	}
	if p.warn.Len() != 0 {
		t.Errorf("warn = %q, want nothing", p.warn.String())
	}
}

// TestNoHooksSkipsGatesAndNamesTheMatchingOnes is the Dispatcher side of G11
// and G19: any non-empty value, also 0, bypasses the gates; only a gate that
// matches the change is named.
func TestNoHooksSkipsGatesAndNamesTheMatchingOnes(t *testing.T) {
	for _, value := range []string{"1", "0"} {
		t.Run("LIF_NO_HOOKS="+value, func(t *testing.T) {
			p := newProj(t, tasksOf(task("block", `touch "$LIF_ROOT/marker"`+"\nexit 1"))+
				hooksOf(
					hook("g", "stage: before", "event: [updated]", "run: block"),
					hook("n", "stage: before", "event: [updated]", "to: [completed]", "run: block")))
			p.allow()
			t.Setenv("LIF_NO_HOOKS", value)

			ran, err := p.d.Gate(toProgress())
			if ran || err != nil {
				t.Errorf("Gate = %v, %v; want false, nil", ran, err)
			}
			if p.exists("marker") {
				t.Error("a gate ran although LIF_NO_HOOKS is set")
			}
			if got, want := p.warn.String(), `warning: gate "g" skipped (LIF_NO_HOOKS is set)`+"\n"; got != want {
				t.Errorf("warn = %q, want %q", got, want)
			}
		})
	}
}

// TestFireLoadErrorWarns is the Fire side of G08: a broken global file warns
// and runs nothing.
func TestFireLoadErrorWarns(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()
	globalPath := p.global("tasks: [unclosed\n")

	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if p.exists("ran") {
		t.Error("an after hook ran although the global file is broken")
	}
	if prefix := "warning: hooks could not be loaded: " + globalPath; !strings.HasPrefix(p.warn.String(), prefix) {
		t.Errorf("warn = %q, want the prefix %q", p.warn.String(), prefix)
	}
}

// TestBrokenTrustFileBlocksEvenAProjectWithoutHooks: a trust file that cannot
// be read is a load error like a broken global file, in every project.
func TestBrokenTrustFileBlocksEvenAProjectWithoutHooks(t *testing.T) {
	p := newProj(t, tasksOf(task("noop", "true")))
	trustPath := filepath.Join(p.home, config.HomeDir, "allowed-hooks.yml")
	writeFile(t, trustPath, "[unclosed\n")

	ran, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if blocked.Hook != "*" || !strings.HasPrefix(blocked.Message, "hooks could not be loaded: "+trustPath) {
		t.Errorf("blocked by %q: %q, want * and a load error of %s", blocked.Hook, blocked.Message, trustPath)
	}
	if ran {
		t.Error("ran = true, want false")
	}
	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if prefix := "warning: hooks could not be loaded: " + trustPath; !strings.HasPrefix(p.warn.String(), prefix) {
		t.Errorf("warn = %q, want the prefix %q", p.warn.String(), prefix)
	}
}

// TestUnsetHomeIsNoError is the Dispatcher side of T10: nothing is allowed,
// nothing fails.
func TestUnsetHomeIsNoError(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+
		hooksOf(
			hook("g", "stage: before", "event: [updated]", "run: t"),
			hook("a", "event: [updated]", "run: t")))
	os.Unsetenv("HOME") // t.Setenv in newProj restores it

	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v; want false, nil", ran, err)
	}
	if p.d.Fire(toProgress()) {
		t.Error("Fire reported a run")
	}
	if got, want := p.warn.String(), notAllowed(p.cfg, " (1 gates not enforced)"); got != want {
		t.Errorf("warn = %q, want only %q", got, want)
	}
	if p.exists("ran") {
		t.Error("a project hook ran without an allow")
	}

	if ok, err := Allowed("k", "f"); ok || err != nil {
		t.Errorf("Allowed = %v, %v; want false, nil", ok, err)
	}
	if err := Deny("k"); err != nil {
		t.Errorf("Deny = %v, want nil", err)
	}
	if err := Allow("k", "f"); err == nil {
		t.Error("Allow = nil, want an error: there is no place to store it")
	}
}

// TestGateLoadErrorOfAProjectWithoutGates is the Dispatcher side of G18.
func TestGateLoadErrorOfAProjectWithoutGates(t *testing.T) {
	p := newProj(t, tasksOf(task("noop", "true")))
	globalPath := p.global("tasks: [unclosed\n")

	_, err := p.d.Gate(upd(it("todo"), it("in-progress"), "status"))
	prefix := `hook "*" blocked the change of beans-x1: hooks could not be loaded: ` + globalPath
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Errorf("Gate error = %v, want the prefix %q", err, prefix)
	}
}

// TestGateRunsOnlyGatesAndFireOnlyAfterHooks: the two stages never run each
// other's hooks.
func TestGateRunsOnlyGatesAndFireOnlyAfterHooks(t *testing.T) {
	p := newProj(t, tasksOf(
		task("g", `touch "$LIF_ROOT/gate"`),
		task("a", `touch "$LIF_ROOT/after"`))+
		hooksOf(
			hook("gate", "stage: before", "event: [updated]", "run: g"),
			hook("after", "event: [updated]", "run: a")))
	p.allow()

	if ran, err := p.d.Gate(toProgress()); !ran || err != nil {
		t.Fatalf("Gate = %v, %v; want true, nil", ran, err)
	}
	if !p.exists("gate") || p.exists("after") {
		t.Errorf("after Gate: gate ran = %v, after hook ran = %v; want true, false", p.exists("gate"), p.exists("after"))
	}

	if !p.d.Fire(toProgress()) {
		t.Fatal("Fire reported no run")
	}
	if !p.exists("after") {
		t.Error("Fire did not run the after hook")
	}
}

// TestFireSkipsAChangeThatDoesNotMatch: Fire runs an after hook only for a
// change that matches it.
func TestFireSkipsAChangeThatDoesNotMatch(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+
		hooksOf(hook("h", "event: [updated]", "to: [completed]", "run: t")))
	p.allow()

	if p.d.Fire(toProgress()) || p.exists("ran") {
		t.Errorf("a hook for completed ran for a change to in-progress (marker %v)", p.exists("ran"))
	}
	if !p.d.Fire(upd(it("todo"), it("completed"), "status")) || !p.exists("ran") {
		t.Error("control: the hook did not run for a change to completed")
	}
}

// TestFireCreatedHasAnEmptyBeforeState: a created item has no Prev; the task
// sees empty before values.
func TestFireCreatedHasAnEmptyBeforeState(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `printf '%s|%s|%s|%s' "$LIF_EVENT" "$LIF_PREV_STATUS" "$LIF_STATUS" "$1"`))+
		hooksOf(hook("h", "event: [created]", "run: t", "args: [prev_tags]")))
	p.allow()

	if !p.d.Fire(created(it("todo", "a"), "status", "type", "tags")) {
		t.Fatal("Fire reported no run")
	}
	if got, want := p.warn.String(), "created||todo|"; got != want {
		t.Errorf("task output = %q, want %q", got, want)
	}
}
