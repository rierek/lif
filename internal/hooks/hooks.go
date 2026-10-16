// Package hooks runs the tasks that hooks and gates name when an item is
// created or one of its watched fields changes.
package hooks

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

	mu      sync.Mutex // hook tasks run one at a time: lif serve handles requests in parallel
	warned  bool       // the not-allowed warning was printed
	key, fp string     // trust key and fingerprint of cfg; set on first use
}

// New returns a Dispatcher for cfg. storeRoot is the directory that
// Item.Path is relative to. Warnings and the output of after hooks go to
// warn.
func New(cfg *config.Config, storeRoot string, warn io.Writer) *Dispatcher {
	if warn == nil {
		warn = io.Discard
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
		s += ": " + e.Message
	}
	if e.Output != "" {
		s += "\n" + e.Output
	}
	return s
}

// noHooks reports whether LIF_NO_HOOKS asks to skip all hooks and gates: any
// non-empty value does, also "0".
func noHooks() bool { return os.Getenv("LIF_NO_HOOKS") != "" }

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
	skip := noHooks()
	d.mu.Lock()
	defer d.mu.Unlock()

	base, gates, err := d.prepare(true)
	if err != nil {
		if skip { // the escape hatch must work with a broken file
			return false, nil
		}
		return false, &BlockedError{Hook: "*", ItemID: orEmpty(m.Item).ID,
			Message: "hooks could not be loaded: " + err.Error() + " (fix it or set LIF_NO_HOOKS=1)"}
	}
	for _, g := range gates {
		if !match(g.Hook, m) {
			continue
		}
		if skip {
			fmt.Fprintf(d.warn, "warning: gate %q skipped (LIF_NO_HOOKS is set)\n", g.name)
			continue
		}
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
// failing hook prints a warning and the next one runs. ran is true when a
// hook task started.
func (d *Dispatcher) Fire(m itemgraph.Mutation) (ran bool) {
	if d == nil || noHooks() {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	base, hooks, err := d.prepare(false)
	if err != nil {
		fmt.Fprintf(d.warn, "warning: hooks could not be loaded: %v\n", err)
		return false
	}
	for _, h := range hooks {
		if !match(h.Hook, m) {
			continue
		}
		started, err := d.run(base, h, m, d.warn)
		ran = ran || started
		if err != nil {
			fmt.Fprintf(d.warn, "warning: hook %q (task %q): %v\n", h.name, h.Run, err)
		}
	}
	return ran
}

// prepare loads the state of one call and returns the active hooks of a
// stage, in name order, with the task set that holds the global tasks and
// the project directory. The caller holds d.mu.
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
		allowed = isAllowed(trusted, key, fp)
		if !allowed {
			d.warnNotAllowed()
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
	streams := tasks.IO{Stdin: strings.NewReader(""), Stdout: out, Stderr: out} // hooks are not interactive
	return true, tasks.Run(base.Root, plan, args, streams, d.env(h.name, stage, m))
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
