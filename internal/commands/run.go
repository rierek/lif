package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
a global task of the same name as a whole; -g runs the global one instead,
and resolves its dependencies in the global file only.

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
  LIF_PATH         absolute store path, when --lif-path or LIF_PATH is set

A failing step stops the run. lif then exits with the exit code of that step.
Other failures (signal, unknown task, bad config) exit 1. A multi-line step
runs without 'set -e': start it with 'set -e' to stop at the first failure.`,
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
		top := plan[len(plan)-1]
		if _, shadows := set.Global[top.Name]; shadows && top.Origin == "project" {
			// The project file may come from a clone: say whose task runs.
			fmt.Fprintf(cmd.ErrOrStderr(), "running project task %q (overrides global; use -g for the global one)\n", top.Name)
		}
		if err := tasks.CheckArgs(top, args[1:]); err != nil {
			return err
		}
		return tasks.Run(set.Root, plan, args[1:], tasks.IO{Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}, storeEnv())
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
			out[i] = n + "\t" + tasks.Printable(p.Task.Desc)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	},
}

// loadProjectConfig reads the project config for a command that skips the
// store bootstrap: the file of --config, else the one found from the working
// directory.
func loadProjectConfig() (*config.Config, error) {
	if configPath != "" {
		if _, err := os.Stat(configPath); err != nil {
			return nil, fmt.Errorf("config file not found: %s", configPath)
		}
		return config.Load(configPath)
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return config.LoadFromDirectory(wd)
}

// loadTaskSet loads project and global tasks. run skips the store bootstrap,
// so it reads the config itself.
func loadTaskSet() (tasks.Set, error) {
	c, err := loadProjectConfig()
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
	global, err := config.LoadGlobal()
	if err != nil {
		return tasks.Set{}, err
	}
	return tasks.Set{Root: root, Project: c.Tasks, Global: global.Tasks}, nil
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
	var buf strings.Builder
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
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
		// Always three cells, desc on one line, so the columns stay aligned.
		fmt.Fprintf(tw, "%s\t[%s]\t%s\n", usage, origin, tasks.Printable(strings.Join(strings.Fields(p.Task.Desc), " ")))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// An empty desc leaves the padding of the origin column at line end.
	for _, line := range strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		fmt.Fprintln(cmd.OutOrStdout(), strings.TrimRight(line, " "))
	}
	return nil
}

// storeEnv hands the store to the steps as an absolute LIF_PATH when --lif-path
// or a relative LIF_PATH names it: steps run in the project root, not in the
// directory the user typed the path in.
func storeEnv() []string {
	p := storePath
	if p == "" {
		p = os.Getenv(config.StoreEnvVar)
	}
	if p == "" {
		return nil
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return []string{config.StoreEnvVar + "=" + p}
}

// RegisterRunCmd adds the run command to root.
func RegisterRunCmd(root *cobra.Command) {
	runCmd.Flags().BoolVarP(&runGlobal, "global", "g", false, "Use the global task even if the project defines one with the same name")
	// Everything after the task name goes to the task, --help and -- included.
	runCmd.Flags().SetInterspersed(false)
	root.AddCommand(runCmd)
}
