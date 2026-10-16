package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

var runGlobal bool

var runCmd = &cobra.Command{
	Use:   "run [task] [args...]",
	Short: "Run a task defined in .lif.yml or ~/.lif/tasks.yml",
	Long: `Run a named task, or list the tasks when no name is given.

Tasks live under "tasks:" in the project .lif.yml and in the global file
~/.lif/tasks.yml (a standard shared across projects). A project task replaces
a global task of the same name as a whole; -g runs the global one instead.

  tasks:
    po-accept:
      desc: Accept an item         # shown by 'lif run'
      args: [id]                   # required positional args, exported as $id
      deps: [test]                 # tasks that run first, without args
      steps:                       # each step is one 'sh -c' script
        - lif update "$id" -s completed

Steps run in the project root (the directory of .lif.yml; the current
directory when there is none). Everything after the task name goes to the
task unparsed: the declared args become shell variables, and all args are
also $1, $2, ... of every step of the task.

Environment of every step:
  LIF              path of the lif binary
  LIF_ROOT         the directory the steps run in
  LIF_TASK         name of the running task (a dependency has its own name)
  LIF_TASK_ORIGIN  "project" or "global"

A failing step stops the run. lif then exits with the exit code of that step.`,
	Example: `  lif run
  lif run test
  lif run po-accept lif-ab12
  lif run -g wt start lif-ab12 milestone/0.16`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		set, err := loadTaskSet()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			return listTasks(cmd, set)
		}
		plan, err := set.Plan(args[0], runGlobal)
		if err != nil {
			return err
		}
		if err := tasks.CheckArgs(plan[len(plan)-1], args[1:]); err != nil {
			return err
		}
		return tasks.Run(set.Root, plan, args[1:], tasks.IO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}, nil)
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveDefault
		}
		set, err := loadTaskSet()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		names := taskNames(set)
		out := make([]string, len(names))
		for i, n := range names {
			p, _ := set.Lookup(n, runGlobal)
			out[i] = n + "\t" + p.Task.Desc
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	},
}

// loadTaskSet loads project and global tasks. run skips the store bootstrap,
// so it reads the config itself.
func loadTaskSet() (tasks.Set, error) {
	var c *config.Config
	var err error
	if configPath != "" {
		c, err = config.Load(configPath)
	} else {
		var wd string
		if wd, err = os.Getwd(); err == nil {
			c, err = config.LoadFromDirectory(wd)
		}
	}
	if err != nil {
		return tasks.Set{}, err
	}
	root := c.ConfigDir()
	if root == "" {
		if root, err = os.Getwd(); err != nil {
			return tasks.Set{}, err
		}
	}
	if root, err = filepath.Abs(root); err != nil {
		return tasks.Set{}, err
	}
	global, err := config.LoadGlobalTasks()
	if err != nil {
		return tasks.Set{}, err
	}
	return tasks.Set{Root: root, Project: c.Tasks, Global: global}, nil
}

// taskNames returns the sorted task names that a listing shows: project and
// global without -g, global only with -g.
func taskNames(set tasks.Set) []string {
	seen := map[string]bool{}
	var names []string
	add := func(m map[string]config.Task) {
		for n := range m {
			if !seen[n] {
				seen[n] = true
				names = append(names, n)
			}
		}
	}
	if !runGlobal {
		add(set.Project)
	}
	add(set.Global)
	sort.Strings(names)
	return names
}

func listTasks(cmd *cobra.Command, set tasks.Set) error {
	names := taskNames(set)
	if len(names) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks. Define them under tasks: in .lif.yml or in ~/.lif/tasks.yml.")
		return nil
	}
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, n := range names {
		p, _ := set.Lookup(n, runGlobal)
		usage := n
		for _, a := range p.Task.Args {
			usage += " <" + a + ">"
		}
		origin := p.Origin
		if _, ok := set.Global[n]; ok && p.Origin == "project" {
			origin = "project, overrides global"
		}
		row := usage + "\t[" + origin + "]"
		if p.Task.Desc != "" {
			row += "\t" + p.Task.Desc
		}
		fmt.Fprintln(tw, row)
	}
	return tw.Flush()
}

// RegisterRunCmd adds the run command to root.
func RegisterRunCmd(root *cobra.Command) {
	runCmd.Flags().BoolVarP(&runGlobal, "global", "g", false, "Use the global task even if the project defines one with the same name")
	// Everything after the task name goes to the task, --help and -- included.
	runCmd.Flags().SetInterspersed(false)
	root.AddCommand(runCmd)
}
