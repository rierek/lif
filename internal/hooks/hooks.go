// Package hooks runs the tasks that hooks and gates name when an item is
// created or one of its watched fields changes.
package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// Dispatcher runs the after hooks (Fire) and gates (Gate) of one project and
// of the global file ~/.lif/tasks.yml.
//
// The global file and the trust state are read on every call, so a running
// lif serve sees `lif hooks deny` and edits of the global file. The project's
// own .lif.yml is the cfg that the Dispatcher was built with.
type Dispatcher struct {
	cfg       *config.Config
	storeRoot string
	warn      io.Writer

	stateMu sync.Mutex // guards the three fields below and the load in prepare
	runMu   sync.Mutex // hook tasks run one at a time: lif serve handles requests in parallel
	warned  bool       // the not-allowed warning was printed
	key, fp string     // trust key and fingerprint of cfg; set on first use
}

// New returns a Dispatcher for cfg. storeRoot is the directory that
// Item.Path is relative to; New makes it absolute, because a relative
// --lif-path stays relative and LIF_ITEM_PATH must be absolute. Warnings and
// the output of after hooks go to warn.
func New(cfg *config.Config, storeRoot string, warn io.Writer) *Dispatcher {
	if warn == nil {
		warn = io.Discard
	}
	if abs, err := filepath.Abs(storeRoot); err == nil {
		storeRoot = abs
	}
	return &Dispatcher{cfg: cfg, storeRoot: storeRoot, warn: warn}
}

// BlockedError is returned by Gate. It has no Unwrap: exitCode must never see
// the step code of a gate.
type BlockedError struct {
	Hook, ItemID string
	Message      string // the hook's message:, "could not run: <err>", or "hooks could not be loaded: <err> (fix it or set LIF_NO_HOOKS=1)"
	Output       string // the gate task's stdout and stderr, trimmed
}

func (e *BlockedError) Error() string {
	s := fmt.Sprintf("hook %q blocked the change of %s", e.Hook, e.ItemID)
	if e.Message != "" {
		s += ": " + tasks.Printable(e.Message) // the message comes from .lif.yml
	}
	if e.Output != "" {
		s += "\n" + e.Output
	}
	return s
}

// noHooks reports whether LIF_NO_HOOKS asks to skip all hooks and gates: any
// non-empty value does, also "0".
func noHooks() bool { return os.Getenv("LIF_NO_HOOKS") != "" }

// maxHookDepth is the most nested hook levels. The task of a hook gets
// LIF_HOOK_DEPTH one higher than the caller's. A lif call that dropped
// LIF_NO_HOOKS (env -u, an empty value) still carries it, so a hook that calls
// lif again cannot loop forever. A call that drops the whole environment (env -i,
// sudo, ssh) is not covered.
const maxHookDepth = 3

func hookDepth() int {
	n, _ := strconv.Atoi(os.Getenv("LIF_HOOK_DEPTH"))
	return n
}

// skipReason says why this call runs no hooks, or is "".
func skipReason() string {
	if noHooks() {
		return "LIF_NO_HOOKS is set"
	}
	if hookDepth() >= maxHookDepth {
		return fmt.Sprintf("LIF_HOOK_DEPTH=%d reached the limit of %d nested hooks", hookDepth(), maxHookDepth)
	}
	return ""
}

// active is a hook that may run in this call.
type active struct {
	config.Hook
	name    string
	project bool // defined in .lif.yml, not in the global file
}

// Gate runs every active matching gate in name order; the first block wins.
// ran is true when a gate task started.
func (d *Dispatcher) Gate(m itemgraph.Mutation) (ran bool, err error) {
	if d == nil {
		return false, nil
	}
	skip := skipReason()
	d.stateMu.Lock()
	base, gates, err := d.prepare(true)
	d.stateMu.Unlock()
	if err != nil {
		if skip != "" { // the escape hatch must work with a broken file
			return false, nil
		}
		return false, &BlockedError{Hook: "*", ItemID: orEmpty(m.Item).ID,
			Message: "hooks could not be loaded: " + err.Error() + " (fix it or set LIF_NO_HOOKS=1)"}
	}
	gates = slices.DeleteFunc(gates, func(g active) bool { return !match(g.Hook, m) })
	if len(gates) == 0 {
		return false, nil // no lock: a write that no gate watches never waits for a running hook
	}
	if skip != "" {
		for _, g := range gates {
			fmt.Fprintf(d.warn, "warning: gate %q skipped (%s)\n", g.name, skip)
		}
		return false, nil
	}
	d.runMu.Lock()
	defer d.runMu.Unlock()
	for _, g := range gates {
		var out bytes.Buffer
		started, runErr := d.run(base, g, m, &out)
		ran = ran || started
		if runErr == nil {
			continue
		}
		blocked := &BlockedError{Hook: g.name, ItemID: orEmpty(m.Item).ID, Output: strings.TrimSpace(out.String())}
		var step *tasks.StepError
		if errors.As(runErr, &step) && step.Code > 0 {
			blocked.Message = g.Message
		} else {
			blocked.Message = "could not run: " + runErr.Error()
		}
		return ran, blocked
	}
	return ran, nil
}

// Fire runs every active matching after hook. It never returns an error: a
// failing hook prints a warning and the next one runs; an interrupted hook
// stops the rest. ran is true when a hook task started.
func (d *Dispatcher) Fire(m itemgraph.Mutation) (ran bool) {
	if d == nil {
		return false
	}
	if skip := skipReason(); skip != "" {
		if noHooks() {
			return false
		}
		fmt.Fprintf(d.warn, "warning: hooks skipped (%s)\n", skip)
		return false
	}
	d.stateMu.Lock()
	base, hooks, err := d.prepare(false)
	d.stateMu.Unlock()
	if err != nil {
		fmt.Fprintf(d.warn, "warning: hooks could not be loaded: %v\n", err)
		return false
	}
	hooks = slices.DeleteFunc(hooks, func(h active) bool { return !match(h.Hook, m) })
	if len(hooks) == 0 {
		return false
	}
	d.runMu.Lock()
	defer d.runMu.Unlock()
	for _, h := range hooks {
		started, err := d.run(base, h, m, d.warn)
		ran = ran || started
		if err != nil {
			fmt.Fprintf(d.warn, "warning: hook %q (task %q): %v\n", h.name, h.Run, err)
			if errors.Is(err, tasks.ErrInterrupted) {
				break
			}
		}
	}
	return ran
}

// prepare loads the state of one call and returns the active hooks of a
// stage, in name order, with the task set that holds the global tasks and
// the project directory. The caller holds d.stateMu.
func (d *Dispatcher) prepare(gates bool) (base tasks.Set, hooks []active, err error) {
	root, err := filepath.Abs(d.cfg.ConfigDir()) // an empty directory is the working directory
	if err != nil {
		return tasks.Set{}, nil, err
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return tasks.Set{}, nil, err
	}
	trusted, err := readTrust()
	if err != nil {
		return tasks.Set{}, nil, err
	}

	allowed := false
	if len(d.cfg.Hooks) > 0 {
		key, fp := d.trustID()
		switch {
		case d.changedOnDisk(fp):
			d.warnChanged()
		case !isAllowed(trusted, key, fp):
			d.warnNotAllowed()
		default:
			allowed = true
		}
	}

	// A project hook replaces a global hook of the same name, but only when
	// project hooks are allowed.
	merged := make(map[string]active, len(global.Hooks)+len(d.cfg.Hooks))
	for name, h := range global.Hooks {
		merged[name] = active{Hook: h, name: name}
	}
	if allowed {
		for name, h := range d.cfg.Hooks {
			merged[name] = active{Hook: h, name: name, project: true}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(merged)) {
		if h := merged[name]; h.IsGate() == gates {
			hooks = append(hooks, h)
		}
	}
	return tasks.Set{Root: root, Global: global.Tasks}, hooks, nil
}

// configPath is the path of the project config, for the trust key and the
// warning.
func (d *Dispatcher) configPath() string {
	if path := d.cfg.ConfigFile(); path != "" {
		return path
	}
	return filepath.Join(d.cfg.ConfigDir(), config.ConfigFileName)
}

// trustID returns the trust key and the fingerprint of the project config.
// Both depend only on paths and on the config read at start, so they are
// computed once, on first use: Key runs git, and a project without hooks
// never needs them.
func (d *Dispatcher) trustID() (key, fp string) {
	if d.key == "" {
		d.key, d.fp = Key(d.configPath()), Fingerprint(d.cfg)
	}
	return d.key, d.fp
}

// changedOnDisk reports whether the project config on disk no longer has the
// fingerprint fp of the config that d was built with. A step that runs
// `$LIF run <task>` reads the file again, so the allow of the old content
// says nothing about what such a step runs. A file that is gone or no longer
// loads counts as changed.
func (d *Dispatcher) changedOnDisk(fp string) bool {
	if d.cfg.ConfigFile() == "" {
		return false // the defaults: no file to read again
	}
	disk, err := config.Load(d.configPath())
	return err != nil || Fingerprint(disk) != fp
}

// warnChanged prints, once per Dispatcher, that the project config changed
// after this process read it.
func (d *Dispatcher) warnChanged() {
	if d.warned {
		return
	}
	d.warned = true
	fmt.Fprintf(d.warn, "warning: %s changed since this process started; its hooks are not allowed until you restart and run 'lif hooks allow'\n", d.configPath())
}

// warnNotAllowed prints, once per Dispatcher, that the project hooks do not
// run. It counts the gates, because a gate that is off is a policy that is off.
func (d *Dispatcher) warnNotAllowed() {
	if d.warned {
		return
	}
	d.warned = true
	gates := 0
	for _, h := range d.cfg.Hooks {
		if h.IsGate() {
			gates++
		}
	}
	note := ""
	if gates > 0 {
		note = fmt.Sprintf(" (%d gates not enforced)", gates)
	}
	fmt.Fprintf(d.warn, "warning: hooks in %s are not allowed%s; run 'lif hooks allow' to enable them\n", d.configPath(), note)
}

// run plans and runs the task of h for m; stdout and stderr go to out. A
// project hook sees project and global tasks, a global hook only the global
// ones, so that an allowed global hook never reaches unreviewed project code.
// started is true once tasks.Run began: from then on the task may have
// written items.
func (d *Dispatcher) run(base tasks.Set, h active, m itemgraph.Mutation, out io.Writer) (started bool, err error) {
	set := base
	if h.project {
		set.Project = d.cfg.Tasks
	}
	plan, err := set.Plan(h.Run, false)
	if err != nil {
		return false, err
	}
	fields := itemFields(m)
	args := make([]string, len(h.Args))
	for i, name := range h.Args {
		args[i] = fields[name]
	}
	if err := tasks.CheckArgs(plan[len(plan)-1], args); err != nil {
		return false, err
	}
	stage := "after"
	if h.IsGate() {
		stage = "before"
	}
	ctx := context.Background()
	if limit := h.TimeoutDuration(); limit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeoutCause(ctx, limit, fmt.Errorf("timed out after %s", limit))
		defer cancel()
	}
	capped := &cappedWriter{w: out, left: maxOutput}
	streams := tasks.IO{Stdin: strings.NewReader(""), Stdout: capped, Stderr: capped} // hooks are not interactive
	err = tasks.RunContext(ctx, base.Root, plan, args, streams, d.env(h.name, stage, m))
	if capped.cut {
		fmt.Fprintf(out, "\n(output cut after %d KiB)\n", maxOutput>>10)
	}
	return true, err
}

// maxOutput is the most output of one hook or gate task that is kept. A task
// that writes more is not stopped; the rest is dropped.
const maxOutput = 64 << 10

// cappedWriter passes the first left bytes to w and drops the rest. It reports
// every write as complete, so the task never sees a broken pipe.
type cappedWriter struct {
	w    io.Writer
	left int
	cut  bool
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	n := len(p)
	if n > c.left {
		p, c.cut = p[:c.left], true
	}
	c.left -= len(p)
	if len(p) > 0 {
		if _, err := c.w.Write(p); err != nil {
			return 0, err
		}
	}
	return n, nil
}

// match reports whether h fires for m: every condition that h sets must
// hold. The before state is m.Prev, or an empty item for a created item.
func match(h config.Hook, m itemgraph.Mutation) bool {
	cur, prev := orEmpty(m.Item), orEmpty(m.Prev)
	is := func(want string) func(string) bool { return func(s string) bool { return s == want } }
	oneOf := func(list []string) func(string) bool { return func(s string) bool { return slices.Contains(list, s) } }
	return holds(h.Event, is(m.Event)) &&
		holds(h.Changed, oneOf(m.Changed)) &&
		holds(h.From, is(prev.Status)) &&
		holds(h.To, is(cur.Status)) &&
		holds(h.Type, is(cur.Type)) &&
		holds(h.Tags, oneOf(cur.Tags)) &&
		holds(h.TagsAdded, func(tag string) bool { return slices.Contains(cur.Tags, tag) && !slices.Contains(prev.Tags, tag) }) &&
		holds(h.TagsRemoved, func(tag string) bool { return slices.Contains(prev.Tags, tag) && !slices.Contains(cur.Tags, tag) })
}

// holds reports whether a condition is unset or one of its values satisfies ok.
func holds(cond []string, ok func(string) bool) bool {
	return len(cond) == 0 || slices.ContainsFunc(cond, ok)
}

// orEmpty returns it, or an empty item for nil.
func orEmpty(it *item.Item) *item.Item {
	if it == nil {
		return &item.Item{}
	}
	return it
}
