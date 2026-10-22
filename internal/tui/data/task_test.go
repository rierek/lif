package data

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tasks"
)

// taskRepo is a repo whose config carries the given YAML after the base
// config. HOME is isolated so no global tasks leak in.
func taskRepo(t *testing.T, extra string) *Store {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	repo := newTestRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".lif.yml"), []byte(testLifYML+extra), 0o644); err != nil {
		t.Fatal(err)
	}
	return openStore(t, repo)
}

func TestRunTaskPassesItemIDAndCapturesOutput(t *testing.T) {
	s := taskRepo(t, `
tasks:
  hello:
    args: [id]
    steps:
      - 'echo "$id $LIF_ITEM_ID"; echo "err" >&2'
`)
	out, err := s.RunTask(context.Background(), "hello", "tt-task")
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if want := "tt-task tt-task\nerr\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestRunTaskExportsStorePath(t *testing.T) {
	s := taskRepo(t, `
tasks:
  where:
    steps:
      - 'echo "$LIF_PATH"'
`)
	out, err := s.RunTask(context.Background(), "where", "")
	if err != nil {
		t.Fatalf("RunTask: %v", err)
	}
	if got, want := strings.TrimSpace(out), s.Core.Root(); got != want {
		t.Errorf("LIF_PATH = %q, want %q", got, want)
	}
}

func TestRunTaskRejectsTaskWithTwoArgs(t *testing.T) {
	s := taskRepo(t, `
tasks:
  two:
    args: [a, b]
    steps:
      - 'true'
`)
	_, err := s.RunTask(context.Background(), "two", "tt-task")
	want := `task "two" needs 2 args; TUI actions pass only the item ID`
	if err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestRunTaskNeedsItemForOneArg(t *testing.T) {
	s := taskRepo(t, `
tasks:
  one:
    args: [id]
    steps:
      - 'true'
`)
	_, err := s.RunTask(context.Background(), "one", "")
	if want := `task "one" needs an item`; err == nil || err.Error() != want {
		t.Errorf("err = %v, want %q", err, want)
	}
}

func TestRunTaskFailureKeepsOutputAndStepError(t *testing.T) {
	s := taskRepo(t, `
tasks:
  fail:
    steps:
      - 'echo boom; exit 3'
`)
	out, err := s.RunTask(context.Background(), "fail", "")
	var se *tasks.StepError
	if !errors.As(err, &se) || se.Code != 3 {
		t.Fatalf("err = %v, want StepError code 3", err)
	}
	if out != "boom\n" {
		t.Errorf("output = %q, want %q", out, "boom\n")
	}
}
