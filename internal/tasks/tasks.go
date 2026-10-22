// Package tasks is the task engine shared by `lif run` and hooks: it resolves
// task names and dependencies into a plan and runs the plan's shell steps.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/rierek/lif/pkg/config"
)

// Set is the merged view of project and global tasks.
type Set struct {
	Root    string // absolute working directory for steps
	Project map[string]config.Task
	Global  map[string]config.Task
}

// Planned is one task in execution order.
type Planned struct {
	Name, Origin string // Origin is "project" or "global"
	Task         config.Task
}

// Lookup finds name. forceGlobal looks in the global tasks only; otherwise a
// project task replaces a global task of the same name as a whole.
func (s Set) Lookup(name string, forceGlobal bool) (Planned, bool) {
	if !forceGlobal {
		if t, ok := s.Project[name]; ok {
			return Planned{Name: name, Origin: "project", Task: t}, true
		}
	}
	if t, ok := s.Global[name]; ok {
		return Planned{Name: name, Origin: "global", Task: t}, true
	}
	return Planned{}, false
}

// key identifies a task: a global and a project task may share a name.
type key struct{ name, origin string }

// Plan resolves name and its dependencies (post-order DFS, each task once).
// forceGlobal looks in the global tasks only, for the named task and for its
// dependencies: a repository must not run its own task as a dependency of a
// global one that was asked for with -g.
// The last entry is the top task.
func (s Set) Plan(name string, forceGlobal bool) ([]Planned, error) {
	top, ok := s.Lookup(name, forceGlobal)
	if !ok {
		if forceGlobal {
			return nil, fmt.Errorf("no global task %q", name)
		}
		return nil, fmt.Errorf("unknown task %q; run 'lif run' to list tasks", name)
	}
	var (
		plan  []Planned
		done  = map[key]bool{}
		stack []key
	)
	var visit func(p Planned) error
	visit = func(p Planned) error {
		stack = append(stack, key{p.Name, p.Origin})
		defer func() { stack = stack[:len(stack)-1] }()
		for _, d := range p.Task.Deps {
			dp, ok := s.Lookup(d, forceGlobal)
			if !ok {
				return fmt.Errorf("task %q: unknown dependency %q", p.Name, d)
			}
			dk := key{dp.Name, dp.Origin}
			for i, k := range stack {
				if k == dk {
					names := []string{}
					for _, sk := range stack[i:] {
						names = append(names, sk.name)
					}
					return fmt.Errorf("task cycle: %s", strings.Join(append(names, d), " -> "))
				}
			}
			if done[dk] {
				continue
			}
			if len(dp.Task.Args) > 0 {
				return fmt.Errorf("task %q: dependency %q needs arguments (%s); dependencies run without arguments",
					p.Name, d, strings.Join(dp.Task.Args, ", "))
			}
			if err := visit(dp); err != nil {
				return err
			}
		}
		done[key{p.Name, p.Origin}] = true
		plan = append(plan, p)
		return nil
	}
	if err := visit(top); err != nil {
		return nil, err
	}
	return plan, nil
}

// CheckArgs reports missing required arguments of the top task.
func CheckArgs(top Planned, args []string) error {
	if len(args) >= len(top.Task.Args) {
		return nil
	}
	usage := top.Name
	for _, a := range top.Task.Args {
		usage += " <" + a + ">"
	}
	return fmt.Errorf("task %q needs argument(s): %s (usage: lif run %s)",
		top.Name, strings.Join(top.Task.Args, ", "), usage)
}

// IO carries the streams of a run.
type IO struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// LifPath is the lif binary that steps get as $LIF. The main of each binary sets it.
// Empty: Run returns "lif binary not found" before the first step.
var LifPath string

// lifVar matches a use of $LIF or ${LIF}, not of $LIF_ROOT.
var lifVar = regexp.MustCompile(`\$\{?LIF\b`)

// usesLif reports whether a step of plan refers to $LIF. A task that does not
// needs no lif binary.
func usesLif(plan []Planned) bool {
	for _, p := range plan {
		if slices.ContainsFunc(p.Task.Steps, lifVar.MatchString) {
			return true
		}
	}
	return false
}

// StepError reports the failing step. Code > 0 is the step's exit code.
type StepError struct {
	Task              string
	Step, Total, Code int // Step is 1-based
	Err               error
}

func (e *StepError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("task %q: step %d of %d failed (exit code %d)", e.Task, e.Step, e.Total, e.Code)
	}
	return fmt.Sprintf("task %q: step %d of %d: %v", e.Task, e.Step, e.Total, e.Err)
}

func (e *StepError) Unwrap() error { return e.Err }

// ErrInterrupted is the Err of a StepError after lif got SIGINT, SIGTERM or
// SIGHUP while a step ran. The run stops after that step.
var ErrInterrupted = errors.New("interrupted")

// shell is sh from PATH, else /bin/sh: cron and launchd start with a bare PATH.
func shell() string {
	if p, err := exec.LookPath("sh"); err == nil {
		return p
	}
	return "/bin/sh"
}

// Run executes plan in order. args go to the last entry (the top task) only.
func Run(root string, plan []Planned, args []string, streams IO, extraEnv []string) error {
	return RunContext(context.Background(), root, plan, args, streams, extraEnv)
}

// RunContext is Run with a context: when ctx ends, the running step is killed
// and the StepError carries context.Cause(ctx).
//
// Signals: while a step runs, SIGINT, SIGTERM and SIGHUP go to the step instead
// of ending lif, and lif waits for it, so a trap in the step can clean up and
// its exit code comes back. Unless stdin is a terminal, the step runs in its own
// process group and gets the signal as a group, so no child is orphaned. With a
// terminal on stdin the step stays in the foreground group, which the terminal
// signals on Ctrl-C itself; only SIGTERM and SIGHUP are forwarded, to the step.
func RunContext(ctx context.Context, root string, plan []Planned, args []string, streams IO, extraEnv []string) error {
	if LifPath == "" && usesLif(plan) {
		return errors.New("lif binary not found")
	}
	for i, p := range plan {
		var taskArgs []string
		if i == len(plan)-1 {
			taskArgs = args
		}
		env := append(os.Environ(),
			"LIF="+LifPath,
			"LIF_ROOT="+root,
			"LIF_TASK="+p.Name,
			"LIF_TASK_ORIGIN="+p.Origin,
		)
		env = append(env, extraEnv...)
		for j, name := range p.Task.Args {
			if j < len(taskArgs) {
				env = append(env, name+"="+taskArgs[j])
			}
		}
		for j, step := range p.Task.Steps {
			cmd := exec.CommandContext(ctx, shell(), append([]string{"-ec", step, p.Name}, taskArgs...)...)
			cmd.Dir = root
			cmd.Env = env
			cmd.Stdin, cmd.Stdout, cmd.Stderr = streams.Stdin, streams.Stdout, streams.Stderr
			// WaitDelay only cuts a background child that holds a pipe lift
			// opened for the step (a non-file writer, as hooks use). A step
			// whose stdout is an *os.File gets that fd directly, so `lif run`
			// into a pipe waits for such a child.
			cmd.WaitDelay = 2 * time.Second
			interrupted, err := runStep(cmd, streams.Stdin)
			if err == nil && !interrupted || errors.Is(err, exec.ErrWaitDelay) && !interrupted {
				continue
			}
			se := &StepError{Task: p.Name, Step: j + 1, Total: len(p.Task.Steps), Err: err}
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() > 0 {
				se.Code = ee.ExitCode()
			}
			if ctx.Err() != nil {
				se.Err = context.Cause(ctx)
			} else if interrupted {
				se.Err = ErrInterrupted
			}
			return se
		}
	}
	return nil
}

// runStep runs cmd and forwards signals to it; see RunContext.
func runStep(cmd *exec.Cmd, stdin io.Reader) (interrupted bool, err error) {
	f, isFile := stdin.(*os.File)
	group := !(isFile && term.IsTerminal(int(f.Fd())))
	if group {
		ownGroup(cmd)
	}
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)
	if err := cmd.Start(); err != nil {
		return false, err
	}
	done := make(chan struct{})
	got := make(chan bool, 1)
	go func() {
		seen := false
		for {
			select {
			case s := <-sigs:
				seen = true
				if group || s != os.Interrupt { // the terminal already signaled a foreground step
					signalStep(cmd, s, group)
				}
			case <-done:
				got <- seen
				return
			}
		}
	}()
	err = cmd.Wait()
	close(done)
	return <-got, err
}
