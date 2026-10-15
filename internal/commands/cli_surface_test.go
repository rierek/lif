package commands

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// runRootIn runs the shared root command with args in dir, with no
// --lif-path, and returns stdout, stderr and the error.
func runRootIn(t *testing.T, dir string, args ...string) (string, string, error) {
	t.Helper()
	t.Chdir(dir)
	neutralizeStoreEnv(t)
	root := sharedTestRoot(t)
	resetFlags(root)
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	_, err := root.ExecuteC()
	return outBuf.String(), errBuf.String(), err
}

// beans-3slb: --version prints what the version command prints.
func TestVersionFlagMatchesVersionCommand(t *testing.T) {
	dir := t.TempDir()
	want, _, err := runRootIn(t, dir, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	got, _, err := runRootIn(t, dir, "--version")
	if err != nil {
		t.Fatalf("--version: %v", err)
	}
	if got != want || got == "" {
		t.Errorf("--version printed %q, version printed %q", got, want)
	}
}

// beans-mgx8: help, completion and __complete need no store.
func TestHelpAndCompletionWorkOutsideAProject(t *testing.T) {
	for _, args := range [][]string{
		{"completion", "bash"},
		{"completion", "zsh"},
		{"help", "create"},
		{"create", "--help"},
	} {
		stdout, stderr, err := runRootIn(t, t.TempDir(), args...)
		if err != nil {
			t.Errorf("%v: error %v (stderr %q)", args, err, stderr)
		}
		// The completion scripts go straight to os.Stdout, not the command's writer.
		if stdout == "" && args[0] != "completion" {
			t.Errorf("%v: empty stdout", args)
		}
	}
}

// beans-mgx8: __complete needs no store. It runs in the built test binary,
// because the in-process root would keep cobra's hidden __complete command
// registered for every later test.
func TestCompleteWorksOutsideAProject(t *testing.T) {
	for _, args := range [][]string{
		{"__complete", "show", ""},
		{"__complete", "create", "--type", ""},
	} {
		out, err := runItemsCompletion(t, t.TempDir(), nil, args)
		if err != nil {
			t.Errorf("%v: %v\nstdout: %s", args, err, out)
		}
		if args[1] == "create" && !strings.Contains(out, "milestone") {
			t.Errorf("%v: want built-in type names, got %q", args, out)
		}
	}
}

// beans-mgx8: create and update help list the configured type names.
func TestCreateUpdateHelpListConfiguredTypes(t *testing.T) {
	project := t.TempDir()
	writeFixtureFile(t, filepath.Join(project, config.ConfigFileName), `types:
  - name: zork-type
    color: pink
`)
	outside := t.TempDir()

	for _, verb := range []string{"create", "update"} {
		for _, args := range [][]string{{"help", verb}, {verb, "--help"}} {
			in, _, err := runRootIn(t, project, args...)
			if err != nil || !strings.Contains(in, "zork-type") {
				t.Errorf("%v in project: err %v, want help naming zork-type:\n%s", args, err, in)
			}
			out, _, err := runRootIn(t, outside, args...)
			if err != nil || strings.Contains(out, "zork-type") || !strings.Contains(out, "milestone, epic, feature, bug, task") {
				t.Errorf("%v outside project: err %v, want built-in names only:\n%s", args, err, out)
			}
		}
	}
}
