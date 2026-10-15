package commands

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rierek/lif/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// testRoot is built once: the command objects and their flag variables are
// package-level singletons, so calling RegisterCoreCommands a second time
// makes pflag panic on the duplicate registration.
var (
	testRootOnce sync.Once
	testRootCmd  *cobra.Command
)

func sharedTestRoot(t *testing.T) *cobra.Command {
	t.Helper()
	testRootOnce.Do(func() {
		testRootCmd = NewRootCmd()
		RegisterCoreCommands(testRootCmd)
	})
	return testRootCmd
}

// resetFlags puts every flag of the tree back to its declared default. The
// flag variables outlive a single run, so without this a --json from one test
// would still be set in the next.
func resetFlags(cmd *cobra.Command) {
	reset := func(f *pflag.Flag) {
		// A slice flag's DefValue is the rendered form "[]", and Set appends
		// its argument — passing DefValue back in would add "[]" as a literal
		// element. Replace is the only way back to empty.
		if sv, ok := f.Value.(pflag.SliceValue); ok && f.DefValue == "[]" {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	cmd.Flags().VisitAll(reset)
	cmd.PersistentFlags().VisitAll(reset)
	for _, sub := range cmd.Commands() {
		resetFlags(sub)
	}
}

// runRootWithArgs runs the shared root command over a throwaway store with
// args, and returns whatever it wrote to stdout and stderr. It exercises the
// real ExecuteC path plus reportExecutionError, which is the pair that decides
// the shape of a failure — Execute itself only adds os.Exit on top.
func runRootWithArgs(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test .beans dir: %v", err)
	}

	root := sharedTestRoot(t)
	resetFlags(root)

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--lif-path", itemsDir}, args...))

	cmd, err := root.ExecuteC()
	if err != nil {
		// The reporting path writes through the command's own streams, and
		// a subcommand inherits the root's, so both land in errBuf.
		reportExecutionError(cmd, err)
	}
	return outBuf.String(), errBuf.String(), err
}

// TestRuntimeErrorPrintsTwoLines is the defect beans-ra75 was filed for: a
// one-line runtime failure used to ship with the command's whole 33-line flag
// documentation behind it.
func TestRuntimeErrorPrintsTwoLines(t *testing.T) {
	_, stderr, err := runRootWithArgs(t, "update", "beans-nope", "-s", "completed")
	if err == nil {
		t.Fatal("expected an error for an unknown bean")
	}

	if strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr still carries the usage block:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Run 'lif update --help' for usage.") {
		t.Errorf("stderr does not point at the manual:\n%s", stderr)
	}
	if n := len(strings.Split(strings.TrimSpace(stderr), "\n")); n != 2 {
		t.Errorf("stderr has %d lines, want 2:\n%s", n, stderr)
	}
}

// TestInvocationErrorUsesTheSameShape pins D07 of the output contract, which
// supersedes ra75's original AC-2: an unparsable invocation gets the same two
// lines, not the block. The mechanism ra75 proposed for keeping the block on
// this path cannot work — SilenceUsage is a conjunction over command and root.
func TestInvocationErrorUsesTheSameShape(t *testing.T) {
	_, stderr, err := runRootWithArgs(t, "update", "--nonexistent-flag")
	if err == nil {
		t.Fatal("expected an error for an unknown flag")
	}

	if strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr still carries the usage block:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Run 'lif update --help' for usage.") {
		t.Errorf("stderr does not point at the manual:\n%s", stderr)
	}
}

// TestUnknownCommandIsNotReportedTwice guards the seam: cobra reports an
// unknown command on its own error path, in exactly this shape. With its
// printing silenced and ours taking over, the risk is two copies.
func TestUnknownCommandIsNotReportedTwice(t *testing.T) {
	_, stderr, err := runRootWithArgs(t, "nonexistent-command")
	if err == nil {
		t.Fatal("expected an error for an unknown command")
	}

	if got := strings.Count(stderr, "Run '"); got != 1 {
		t.Errorf("the pointer line appears %d times, want 1:\n%s", got, stderr)
	}
	if strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr carries the usage block:\n%s", stderr)
	}
}

// TestJSONErrorLeavesStderrEmpty pins D06: with --json the machine-readable
// document on stdout is the only artifact. Before this, the same failure was
// reported twice — a correct document plus 33 lines of manual on stderr.
func TestJSONErrorLeavesStderrEmpty(t *testing.T) {
	_, stderr, err := runRootWithArgs(t, "update", "--json", "beans-nope", "-s", "completed")
	if err == nil {
		t.Fatal("expected an error for an unknown bean")
	}
	if strings.TrimSpace(stderr) != "" {
		t.Errorf("stderr is not empty under --json:\n%s", stderr)
	}
}

// TestErrorWithoutADocumentStillReaches TheUser guards the other half of that
// rule: suppressing stderr is keyed on a document having been emitted, not on
// --json being present. An error raised before the output layer is reached
// must not vanish.
func TestErrorWithoutADocumentStillReachesTheUser(t *testing.T) {
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "lif"}
	cmd.SetErr(&buf)

	reportExecutionError(cmd, errors.New("loading config: broken"))

	if !strings.Contains(buf.String(), "loading config: broken") {
		t.Errorf("a plain error was swallowed:\n%s", buf.String())
	}
	if output.Emitted(errors.New("plain")) {
		t.Error("a plain error must not count as already emitted")
	}
}
