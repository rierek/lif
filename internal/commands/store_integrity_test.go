package commands

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
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

// beans-533z: a file renamed by hand answers to the ID in its new name. check
// names both IDs and the file, and the links that now dangle.
func TestItemIDIssuesNamesBothIDsTheFileAndDanglingLinks(t *testing.T) {
	items := []*item.Item{
		{ID: "ex-new1", Path: "ex-new1--docs.md", RecordedID: "ex-old1"},
		{ID: "ex-ok01", Path: "ex-ok01--fine.md", RecordedID: "ex-ok01"},
		{ID: "ex-nore", Path: "ex-nore--none.md"},
		{ID: "ex-ref1", Path: "ex-ref1--ship.md", RecordedID: "ex-ref1", Parent: "ex-old1", BlockedBy: []string{"ex-old1"}},
		{ID: "ex-ref2", Path: "ex-ref2--hold.md", RecordedID: "ex-ref2", Blocking: []string{"ex-old1"}},
	}

	got := itemIDIssues(items)

	if len(got) != 1 {
		t.Fatalf("itemIDIssues() = %q, want exactly one issue, for the renamed file", got)
	}
	for _, want := range []string{"ex-new1", "ex-old1", "ex-new1--docs.md", "ex-ref1 blocked_by", "ex-ref1 parent", "ex-ref2 blocking"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("issue lacks %q:\n%s", want, got[0])
		}
	}
}

// A file that records the ID of another loaded item (a copy made by hand)
// is reported too, but no link dangles because of it.
func TestItemIDIssuesReportsACopyWithoutDanglingLinks(t *testing.T) {
	items := []*item.Item{
		{ID: "ex-ok01", Path: "ex-ok01--fine.md", RecordedID: "ex-ok01"},
		{ID: "ex-copy", Path: "ex-copy--fine.md", RecordedID: "ex-ok01"},
		{ID: "ex-ref1", Path: "ex-ref1--ship.md", RecordedID: "ex-ref1", BlockedBy: []string{"ex-ok01"}},
	}

	got := itemIDIssues(items)

	if len(got) != 1 || !strings.Contains(got[0], "ex-copy") || !strings.Contains(got[0], "ex-ok01") {
		t.Fatalf("itemIDIssues() = %q, want one issue naming ex-copy and ex-ok01", got)
	}
	if strings.Contains(got[0], "dangle") {
		t.Errorf("issue claims dangling links although ex-ok01 exists:\n%s", got[0])
	}
}

// beans-533z, end to end: after a hand rename, lif check fails and says why.
func TestCheckReportsAFileRenamedByHand(t *testing.T) {
	dir := integrityProject(t, map[string]string{
		"ex-zyo0--write-docs.md": integrityItem("ex-zyo0", "Write docs", ""),
		"ex-3ip9--ship.md":       integrityItem("ex-3ip9", "Ship", "blocked_by:\n    - ex-zyo0\n"),
	})

	if stdout, stderr, exit := runLif(t, dir, "check"); exit != 0 || !strings.Contains(stdout, "All checks passed") {
		t.Fatalf("lif check on the untouched store: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}

	store := filepath.Join(dir, config.DefaultStoreDir)
	if err := os.Rename(filepath.Join(store, "ex-zyo0--write-docs.md"), filepath.Join(store, "ex-new1--write-docs.md")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, exit := runLif(t, dir, "check")
	if exit != 1 {
		t.Errorf("lif check after the rename: exit code = %d, want 1\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
	out := stripANSITest(stdout)
	for _, want := range []string{"ex-new1--write-docs.md", "ex-new1", "ex-zyo0", "ex-3ip9 blocked_by"} {
		if !strings.Contains(out, want) {
			t.Errorf("lif check output lacks %q:\n%s", want, out)
		}
	}

	jsonOut, _, exit := runLif(t, dir, "check", "--json")
	if exit != 1 || !strings.Contains(jsonOut, `"config_errors"`) || !strings.Contains(jsonOut, "ex-new1--write-docs.md") {
		t.Errorf("lif check --json: exit code = %d, want 1 with the file in config_errors\n%s", exit, jsonOut)
	}
}
