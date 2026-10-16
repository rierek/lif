package commands

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/rierek/lif/internal/hooks"
	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

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
		state, err := projectHooksState(c)
		if err != nil {
			return err
		}
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Project hooks: %s\n", state)
		return listHooks(out, c, global)
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
		c, err := loadProjectConfig()
		if err != nil {
			return err
		}
		if len(c.Hooks) == 0 {
			return errors.New("no project hooks to allow")
		}
		if err := hooks.Allow(hooks.Key(c.ConfigFile()), hooks.Fingerprint(c)); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Allowed hooks of %s (%d hooks).\n", c.ConfigFile(), len(c.Hooks))
		return nil
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
		if err := hooks.Deny(hooks.Key(path)); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Hooks of %s are no longer allowed.\n", path)
		return nil
	},
}

// projectHooksState says whether the hooks of the project config run.
func projectHooksState(c *config.Config) (string, error) {
	if len(c.Hooks) == 0 {
		return "none", nil
	}
	ok, err := hooks.Allowed(hooks.Key(c.ConfigFile()), hooks.Fingerprint(c))
	if err != nil {
		return "", err
	}
	if !ok {
		return "not allowed (run 'lif hooks allow')", nil
	}
	return "allowed", nil
}

// listedHook is one row of the listing.
type listedHook struct {
	hook    config.Hook
	origin  string
	project bool
}

// listHooks prints one row per hook, sorted by name. A project hook replaces
// a global hook of the same name.
func listHooks(out io.Writer, c *config.Config, global config.Global) error {
	all := make(map[string]listedHook, len(global.Hooks)+len(c.Hooks))
	for name, h := range global.Hooks {
		all[name] = listedHook{hook: h, origin: "global"}
	}
	for name, h := range c.Hooks {
		origin := "project"
		if _, ok := all[name]; ok {
			origin = "project, overrides global"
		}
		all[name] = listedHook{hook: h, origin: origin, project: true}
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	for _, name := range slices.Sorted(maps.Keys(all)) {
		l := all[name]
		stage := "after"
		if l.hook.IsGate() {
			stage = "before"
		}
		// The task set that the dispatcher gives the hook: a global hook never sees project tasks.
		set := tasks.Set{Global: global.Tasks}
		if l.project {
			set.Project = c.Tasks
		}
		fmt.Fprintf(tw, "%s\t[%s]\t%s\t%s\t%s\n", name, l.origin, stage, hookWhen(l.hook), hookRun(l.hook, set))
	}
	return tw.Flush()
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
	hooksCmd.AddCommand(hooksAllowCmd, hooksDenyCmd)
	root.AddCommand(hooksCmd)
}
