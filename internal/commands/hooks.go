package commands

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

var hooksSteps bool

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "List hooks and gates, and allow or deny the hooks of this project",
	Long: `Lists every hook and gate: the global ones from ~/.lif/tasks.yml and the ones
of this project's .lif.yml. The first line says whether the project hooks run.

A hook runs a task when an item is created or one of its fields changes. A
gate (stage: before) runs before the write and blocks it when its task fails.
Global hooks always run. Project hooks run only after 'lif hooks allow': a
repository that you cloned must not run commands that you have not read.

Each row shows the name, the origin, the stage, when the hook fires, and the
task with the item fields that it gets as arguments. A hook whose task cannot
be run shows the reason.

--steps also prints the steps of the tasks that the project hooks run, quoted
so that control characters show: read them before 'lif hooks allow'.

LIF_NO_HOOKS, set to any non-empty value, skips all hooks and gates of one call.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadProjectConfig()
		if err != nil {
			return err
		}
		global, err := config.LoadGlobal()
		if err != nil {
			return err
		}
		state, allowed, err := projectHooksState(c)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Project hooks: %s\n", state)
		if err := listHooks(out, c, global, allowed); err != nil {
			return err
		}
		if hooksSteps {
			return printSteps(out, c, global)
		}
		return nil
	},
}

var hooksAllowCmd = &cobra.Command{
	Use:   "allow",
	Short: "Allow the hooks and tasks of this project to run",
	Long: `Records that you have read the hooks and tasks in this project's .lif.yml.
Project hooks and gates run from then on.

The record covers the hooks and tasks as written in .lif.yml, not the scripts
that their steps call. Any change to a hook or a task takes the allow back;
run 'lif hooks allow' again after you have read the change.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("LIF_HOOK") != "" {
			return errors.New("refusing to allow hooks from inside a hook or gate: run 'lif hooks allow' yourself, after you have read .lif.yml")
		}
		c, err := loadProjectConfig()
		if err != nil {
			return err
		}
		if len(c.Hooks) == 0 {
			return errors.New("no project hooks to allow")
		}
		warnReplacedTrustFile(cmd)
		if err := hooks.Allow(hooks.Key(c.ConfigFile()), hooks.Fingerprint(c)); err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Allowed hooks of %s (%d hooks). These steps may now run:\n", c.ConfigFile(), len(c.Hooks))
		global, err := config.LoadGlobal()
		if err != nil {
			return err
		}
		return printSteps(out, c, global)
	},
}

var hooksDenyCmd = &cobra.Command{
	Use:   "deny",
	Short: "Take back the allow: the hooks of this project stop running",
	Long: `Removes the allow of this project. Its hooks and gates stop running. A running
'lif serve' honours it from its next change.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := loadProjectConfig()
		if err != nil {
			return err
		}
		path := c.ConfigFile()
		if path == "" {
			return fmt.Errorf("no %s found", config.ConfigFileName)
		}
		warnReplacedTrustFile(cmd)
		if err := hooks.Deny(hooks.Key(path)); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Hooks of %s are no longer allowed.\n", path)
		return nil
	},
}

// warnReplacedTrustFile tells the user that allow or deny is about to replace
// a trust file that does not parse.
func warnReplacedTrustFile(cmd *cobra.Command) {
	if err := hooks.TrustFileProblem(); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: replacing the unreadable trust file, all allows are dropped: %v\n", err)
	}
}

// projectHooksState says whether the hooks of the project config run;
// allowed is true only when they do.
func projectHooksState(c *config.Config) (state string, allowed bool, err error) {
	if len(c.Hooks) == 0 {
		return "none", false, nil
	}
	ok, err := hooks.Allowed(hooks.Key(c.ConfigFile()), hooks.Fingerprint(c))
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "not allowed (run 'lif hooks allow')", false, nil
	}
	return "allowed", true, nil
}

// listedHook is one row of the listing.
type listedHook struct {
	name    string
	hook    config.Hook
	origin  string
	project bool
}

// listHooks prints one row per hook, sorted by name. An allowed project hook
// replaces a global hook of the same name. While the project is not allowed
// its hooks do not run and replace nothing: both rows show, the project row
// marked "not allowed" (the header line says that for the others).
func listHooks(out io.Writer, c *config.Config, global config.Global, allowed bool) error {
	var rows []listedHook
	for name, h := range global.Hooks {
		if _, replaced := c.Hooks[name]; replaced && allowed {
			continue
		}
		rows = append(rows, listedHook{name: name, hook: h, origin: "global"})
	}
	for name, h := range c.Hooks {
		origin := "project"
		_, overrides := global.Hooks[name]
		switch {
		case overrides && !allowed:
			origin = "project, not allowed"
		case overrides:
			origin = "project, overrides global"
		}
		rows = append(rows, listedHook{name: name, hook: h, origin: origin, project: true})
	}
	slices.SortFunc(rows, func(a, b listedHook) int {
		if c := strings.Compare(a.name, b.name); c != 0 {
			return c
		}
		return strings.Compare(a.origin, b.origin)
	})
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, l := range rows {
		stage := "after"
		if l.hook.IsGate() {
			stage = "before"
		}
		// The task set that the dispatcher gives the hook: a global hook never sees project tasks.
		set := tasks.Set{Global: global.Tasks}
		if l.project {
			set.Project = c.Tasks
		}
		fmt.Fprintf(tw, "%s\t[%s]\t%s\t%s\t%s%s\n", l.name, l.origin, stage, hookWhen(l.hook), hookRun(l.hook, set), unknownStatuses(l, c))
	}
	return tw.Flush()
}

// unknownStatuses warns about a status in a global hook that this project
// does not know: validation skips the global file, so a typo there is a gate
// that never fires.
func unknownStatuses(l listedHook, c *config.Config) string {
	if l.project {
		return ""
	}
	var warn string
	for _, status := range append(slices.Clone(l.hook.From), l.hook.To...) {
		if !c.IsValidStatus(status) {
			warn += fmt.Sprintf("  (warning: status %q unknown in this project)", status)
		}
	}
	return warn
}

// printSteps prints what the project hooks can run: for each hook the tasks
// that it reaches (dependencies first) and their steps, quoted so that
// control and look-alike characters show. This is what an allow covers.
func printSteps(out io.Writer, c *config.Config, global config.Global) error {
	set := tasks.Set{Project: c.Tasks, Global: global.Tasks}
	for _, name := range slices.Sorted(maps.Keys(c.Hooks)) {
		plan, err := set.Plan(c.Hooks[name].Run, false)
		if err != nil {
			fmt.Fprintf(out, "\nhook %s: (error: %v)\n", name, err)
			continue
		}
		for _, p := range plan {
			fmt.Fprintf(out, "\nhook %s runs task %s [%s]:\n", name, p.Name, p.Origin)
			for i, step := range p.Task.Steps {
				fmt.Fprintf(out, "  step %d: %q\n", i+1, step)
			}
		}
	}
	return nil
}

// hookWhen renders the conditions that h sets: the events, then each set
// condition as its key and values, for example
// "created,updated changed status to in-progress type feature,task".
func hookWhen(h config.Hook) string {
	parts := []string{strings.Join(h.Event, ",")}
	for _, cond := range []struct {
		key    string
		values []string
	}{
		{"changed", h.Changed}, {"from", h.From}, {"to", h.To}, {"type", h.Type},
		{"tags", h.Tags}, {"tags_added", h.TagsAdded}, {"tags_removed", h.TagsRemoved},
	} {
		if len(cond.values) > 0 {
			parts = append(parts, cond.key+" "+strings.Join(cond.values, ","))
		}
	}
	return strings.Join(parts, " ")
}

// hookRun renders the task of h and its arguments. A task that cannot be
// planned, or that h gives too few arguments, ends the cell with the reason:
// the hook would fail the same way when it fires.
func hookRun(h config.Hook, set tasks.Set) string {
	cell := strings.Join(append([]string{h.Run}, h.Args...), " ")
	plan, err := set.Plan(h.Run, false)
	if err == nil {
		err = tasks.CheckArgs(plan[len(plan)-1], h.Args)
	}
	if err != nil {
		cell += "  (error: " + err.Error() + ")"
	}
	return cell
}

// RegisterHooksCmd adds the hooks command and its sub-commands to root.
func RegisterHooksCmd(root *cobra.Command) {
	hooksCmd.Flags().BoolVar(&hooksSteps, "steps", false, "Also print the steps of the tasks that the project hooks run")
	hooksCmd.AddCommand(hooksAllowCmd, hooksDenyCmd)
	root.AddCommand(hooksCmd)
}
