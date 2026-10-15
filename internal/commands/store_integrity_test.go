package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// Store integrity (epic beans-snbj): hand-edited, renamed and concurrently
// written item files, run against the real binary the way a user runs it.

// integrityProject builds a project directory with a minimal config (prefix
// "ex-") and the given item files, named as given, in its store.
func integrityProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := evalSymlinks(t, t.TempDir())
	writeFixtureFile(t, filepath.Join(dir, config.ConfigFileName), "lif:\n    prefix: ex-\n")
	for name, content := range files {
		writeFixtureFile(t, filepath.Join(dir, config.DefaultStoreDir, name), content)
	}
	return dir
}

// integrityItem is the text of an item file whose recorded ID is id. extra is
// raw front matter appended after the standard fields.
func integrityItem(id, title, extra string) string {
	return "---\n# " + id + "\ntitle: " + title + "\nstatus: todo\ntype: task\n" +
		"created_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\n" + extra +
		"---\n\nBody of " + title + ".\n"
}

// runLif runs the built lif binary in dir with a private HOME and no store
// variables, and returns its output and exit code.
func runLif(t *testing.T, dir string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(completionTestBinary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), config.StoreEnvVar+"=", config.LegacyStoreEnvVar+"=")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("lif %v: %v", args, err)
		}
		exit = ee.ExitCode()
	}
	return out.String(), errOut.String(), exit
}

func readStoreFile(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, config.DefaultStoreDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// beans-1wnd: a .nan in the front matter ends a command with an error that
// names the file and the key. It used to end `lif graphql` with a panic trace
// and exit code 2, and `lif list --json` with a bare "unsupported value".
func TestNumberJSONCannotCarryFailsWithFileAndKey(t *testing.T) {
	good := integrityItem("ex-good", "Good", "")
	bad := integrityItem("ex-bad1", "Bad", "ratio: .nan\n")
	dir := integrityProject(t, map[string]string{
		"ex-good--good.md": good,
		"ex-bad1--bad.md":  bad,
	})

	for _, args := range [][]string{
		{"list", "--json"},
		{"show", "ex-good"},
		{"graphql", "--json", `{ item(id: "ex-bad1") { id extra } }`},
	} {
		stdout, stderr, exit := runLif(t, dir, args...)
		if exit != 1 {
			t.Errorf("lif %v: exit code = %d, want 1\nstderr: %s", args, exit, stderr)
		}
		if strings.Contains(stderr, "panic") || strings.Contains(stderr, "goroutine") {
			t.Errorf("lif %v: stderr holds a panic trace:\n%s", args, stderr)
		}
		for _, want := range []string{"ex-bad1--bad.md", `key "ratio"`} {
			if !strings.Contains(stderr, want) {
				t.Errorf("lif %v: stderr lacks %q:\n%s", args, want, stderr)
			}
		}
		if stdout != "" {
			t.Errorf("lif %v: stdout = %q, want nothing", args, stdout)
		}
	}

	// The refusal reads only: neither file changed.
	if got := readStoreFile(t, dir, "ex-good--good.md"); got != good {
		t.Errorf("the good item changed:\n%s", got)
	}
	if got := readStoreFile(t, dir, "ex-bad1--bad.md"); got != bad {
		t.Errorf("the offending item changed:\n%s", got)
	}

	// Quoting the value is the fix the message names.
	writeFixtureFile(t, filepath.Join(dir, config.DefaultStoreDir, "ex-bad1--bad.md"),
		integrityItem("ex-bad1", "Bad", "ratio: \".nan\"\n"))
	if _, stderr, exit := runLif(t, dir, "list", "--json"); exit != 0 {
		t.Errorf("lif list --json after quoting the value: exit code = %d\nstderr: %s", exit, stderr)
	}
}

// beans-ih2r: an item file saved with a UTF-8 BOM keeps exactly one front
// matter block through `lif update`.
func TestUpdateOfAFileWithBOMKeepsOneFrontMatterBlockEndToEnd(t *testing.T) {
	dir := integrityProject(t, map[string]string{
		"ex-bom1--notepad.md": "\xEF\xBB\xBF" + integrityItem("ex-bom1", "Notepad", ""),
	})

	if stdout, stderr, exit := runLif(t, dir, "show", "ex-bom1", "--json"); exit != 0 || !strings.Contains(stdout, `"title": "Notepad"`) {
		t.Fatalf("lif show --json: exit code = %d, want the title behind the BOM\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
	if _, stderr, exit := runLif(t, dir, "update", "ex-bom1", "--status", "in-progress"); exit != 0 {
		t.Fatalf("lif update: exit code = %d\nstderr: %s", exit, stderr)
	}

	got := readStoreFile(t, dir, "ex-bom1--notepad.md")
	if strings.Contains(got, "\xEF\xBB\xBF") {
		t.Errorf("updated file still carries a BOM:\n%q", got)
	}
	if n := strings.Count(got, "\n---\n"); n != 1 || !strings.HasPrefix(got, "---\n") {
		t.Errorf("updated file holds %d closing delimiters after an opening one, want exactly one front matter block:\n%s", n, got)
	}
	if !strings.Contains(got, "status: in-progress") || !strings.Contains(got, "title: Notepad") {
		t.Errorf("updated file lost a field:\n%s", got)
	}
}
