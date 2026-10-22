package data

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
)

// TaskSet is the merged project and global task set, the same one `lif run`
// builds: steps run in the directory of .lif.yml, else in RepoDir.
func (s *Store) TaskSet() (tasks.Set, error) {
	root := s.cfg.ConfigDir()
	if root == "" {
		root = s.RepoDir
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return tasks.Set{}, err
	}
	global, err := config.LoadGlobal()
	if err != nil {
		return tasks.Set{}, err
	}
	return tasks.Set{Root: root, Project: s.cfg.Tasks, Global: global.Tasks}, nil
}

// RunTask runs the named task and returns its combined stdout and stderr. A
// task with one argument gets itemID; a task with more is refused, since the
// TUI knows no other value. Like `lif run` there is no trust check. Stdin is
// empty, so a task that reads input fails.
func (s *Store) RunTask(ctx context.Context, name, itemID string) (string, error) {
	set, err := s.TaskSet()
	if err != nil {
		return "", err
	}
	plan, err := set.Plan(name, false)
	if err != nil {
		return "", err
	}
	top := plan[len(plan)-1]
	var args []string
	switch n := len(top.Task.Args); {
	case n == 1:
		if itemID == "" {
			return "", fmt.Errorf("task %q needs an item", name)
		}
		args = []string{itemID}
	case n > 1:
		return "", fmt.Errorf("task %q needs %d args; TUI actions pass only the item ID", name, n)
	}
	if err := tasks.CheckArgs(top, args); err != nil {
		return "", err
	}
	var buf bytes.Buffer
	err = tasks.RunContext(ctx, set.Root, plan, args,
		tasks.IO{Stdin: strings.NewReader(""), Stdout: &buf, Stderr: &buf},
		[]string{"LIF_PATH=" + s.Core.Root(), "LIF_ITEM_ID=" + itemID})
	return buf.String(), err
}
