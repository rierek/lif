package commands

import (
	"bytes"
	"encoding/json"
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

// beans-1y6w: lif init never replaces a config file that is there. It stops
// with an error that names the file and the flag that forces it, writes
// nothing and exits 1, for every way of calling it and for both file names.
func TestInitRefusesToReplaceAnExistingConfig(t *testing.T) {
	configs := map[string]string{
		config.ConfigFileName:       "lif:\n    prefix: keep-\n",
		config.LegacyConfigFileName: "beans:\n    prefix: keep-\n",
	}
	calls := [][]string{{"init"}, {"init", "--profile", "simple"}, {"init", "--json"}}
	for file, content := range configs {
		for _, args := range calls {
			t.Run(file+" "+strings.Join(args, " "), func(t *testing.T) {
				dir := evalSymlinks(t, t.TempDir())
				writeFixtureFile(t, filepath.Join(dir, file), content)
				before := dirNames(t, dir)

				stdout, stderr, exit := runLif(t, dir, args...)

				if exit != 1 {
					t.Errorf("exit code = %d, want 1\nstdout: %s\nstderr: %s", exit, stdout, stderr)
				}
				msg := stderr
				if args[len(args)-1] == "--json" {
					msg = stdout
				}
				for _, want := range []string{file, "already exists", "--force"} {
					if !strings.Contains(msg, want) {
						t.Errorf("error output lacks %q:\n%s", want, msg)
					}
				}
				if got, _ := os.ReadFile(filepath.Join(dir, file)); string(got) != content {
					t.Errorf("%s changed:\n%s", file, got)
				}
				if got := dirNames(t, dir); strings.Join(got, ",") != strings.Join(before, ",") {
					t.Errorf("directory changed: %v -> %v", before, got)
				}
			})
		}
	}
}

// --force replaces the config with the default one, as init always did, and
// leaves the items alone.
func TestInitForceReplacesTheConfigAndKeepsTheItems(t *testing.T) {
	kept := integrityItem("ex-keep", "Keep", "")
	dir := integrityProject(t, map[string]string{"ex-keep--keep.md": kept})

	if stdout, stderr, exit := runLif(t, dir, "init", "--force", "--profile", "simple"); exit != 0 {
		t.Fatalf("lif init --force: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}

	loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if want := filepath.Base(dir) + "-"; loaded.Items.Prefix != want {
		t.Errorf("prefix = %q, want the default %q: the old config was not replaced", loaded.Items.Prefix, want)
	}
	if !loaded.TypesExclusive {
		t.Error("the profile was not written: types_exclusive is off")
	}
	if got := readStoreFile(t, dir, "ex-keep--keep.md"); got != kept {
		t.Errorf("--force touched an item:\n%s", got)
	}
}

// A legacy project stays a legacy project under --force: same file, same key.
func TestInitForceKeepsTheLegacyNames(t *testing.T) {
	dir := evalSymlinks(t, t.TempDir())
	writeFixtureFile(t, filepath.Join(dir, config.LegacyConfigFileName), "beans:\n    prefix: keep-\n")

	if stdout, stderr, exit := runLif(t, dir, "init", "--force"); exit != 0 {
		t.Fatalf("lif init --force: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}

	if got := strings.Join(dirNames(t, dir), ","); got != config.LegacyDefaultStoreDir+","+config.LegacyConfigFileName {
		t.Errorf("directory = %s, want only the legacy %s and %s", got, config.LegacyDefaultStoreDir, config.LegacyConfigFileName)
	}
	loaded, err := config.Load(filepath.Join(dir, config.LegacyConfigFileName))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SectionKey() != config.LegacySectionKey || loaded.Items.Prefix == "keep-" {
		t.Errorf("config: key %q, prefix %q; want the legacy key with the default prefix", loaded.SectionKey(), loaded.Items.Prefix)
	}
}

// init writes a .gitignore into a new store directory and leaves one that is
// there alone, with or without a config file: it may hold the user's rules.
func TestInitKeepsAnExistingStoreGitignore(t *testing.T) {
	const mine = "# my own rule\n*.tmp\n.conversations/\n"

	dir := evalSymlinks(t, t.TempDir())
	ignore := filepath.Join(dir, config.DefaultStoreDir, ".gitignore")
	writeFixtureFile(t, ignore, mine)
	if stdout, stderr, exit := runLif(t, dir, "init"); exit != 0 {
		t.Fatalf("lif init: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
	if got, _ := os.ReadFile(ignore); string(got) != mine {
		t.Errorf("init rewrote the store .gitignore:\n%s", got)
	}

	scratch := filepath.Join(dir, "scratch")
	writeFixtureFile(t, filepath.Join(scratch, ".gitignore"), mine)
	if stdout, stderr, exit := runLif(t, dir, "init", "--lif-path", scratch); exit != 0 {
		t.Fatalf("lif init --lif-path: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
	if got, _ := os.ReadFile(filepath.Join(scratch, ".gitignore")); string(got) != mine {
		t.Errorf("init --lif-path rewrote the .gitignore:\n%s", got)
	}
}

// handRenamedProject is a project whose item ex-zyo0 was renamed by hand to
// ex-new1, with ex-3ip9 blocked by both ex-zyo0 (the recorded ID, which only
// the renamed file still answers for) and ex-gone (which nothing records).
func handRenamedProject(t *testing.T) string {
	t.Helper()
	dir := integrityProject(t, map[string]string{
		"ex-zyo0--write-docs.md": integrityItem("ex-zyo0", "Write docs", ""),
		"ex-3ip9--ship.md":       integrityItem("ex-3ip9", "Ship", "blocked_by:\n    - ex-zyo0\n    - ex-gone\n"),
	})
	store := filepath.Join(dir, config.DefaultStoreDir)
	if err := os.Rename(filepath.Join(store, "ex-zyo0--write-docs.md"), filepath.Join(store, "ex-new1--write-docs.md")); err != nil {
		t.Fatal(err)
	}
	return dir
}

// beans-533z (review N1): lif check --fix removes the links that point at
// nothing. The links to the ID of a file renamed by hand point at something:
// they work again once the file has its ID back. --fix keeps them and says so,
// and the remedy the check names then restores the item with its links.
func TestCheckFixKeepsTheLinksOfAFileRenamedByHand(t *testing.T) {
	dir := handRenamedProject(t)

	stdout, stderr, exit := runLif(t, dir, "check", "--fix")

	out := stripANSITest(stdout)
	if exit != 1 {
		t.Errorf("lif check --fix: exit code = %d, want 1, the renamed file is still a finding\nstdout: %s\nstderr: %s", exit, out, stderr)
	}
	for _, want := range []string{
		"removed broken link blocked_by:ex-gone",
		"kept broken link blocked_by:ex-zyo0",
		"ex-new1--write-docs.md",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("lif check --fix output lacks %q:\n%s", want, out)
		}
	}
	if ship := readStoreFile(t, dir, "ex-3ip9--ship.md"); !strings.Contains(ship, "- ex-zyo0") || strings.Contains(ship, "ex-gone") {
		t.Errorf("--fix did not keep exactly the link to the recorded ID:\n%s", ship)
	}

	// The remedy: give the item the ID its file records and the links resolve.
	if stdout, stderr, exit := runLif(t, dir, "rename", "ex-new1", "ex-zyo0"); exit != 0 {
		t.Fatalf("lif rename ex-new1 ex-zyo0: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
	if stdout, stderr, exit := runLif(t, dir, "check"); exit != 0 || !strings.Contains(stdout, "All checks passed") {
		t.Errorf("lif check after lif rename: exit code = %d\nstdout: %s\nstderr: %s", exit, stdout, stderr)
	}
}

// The JSON document keeps the link in broken_links, so a script sees that it
// is still broken, and counts only the removed link as fixed.
func TestCheckFixJSONKeepsTheBrokenLinksOfAFileRenamedByHand(t *testing.T) {
	dir := handRenamedProject(t)

	stdout, stderr, exit := runLif(t, dir, "check", "--fix", "--json")

	var result struct {
		Fixed      int `json:"fixed"`
		ItemIssues struct {
			BrokenLinks []struct {
				ItemID   string `json:"item_id"`
				LinkType string `json:"link_type"`
				Target   string `json:"target"`
			} `json:"broken_links"`
		} `json:"item_issues"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("lif check --fix --json printed no JSON: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if exit != 1 || result.Fixed != 1 {
		t.Errorf("exit code = %d, fixed = %d, want 1 and 1", exit, result.Fixed)
	}
	links := result.ItemIssues.BrokenLinks
	if len(links) != 1 || links[0].ItemID != "ex-3ip9" || links[0].LinkType != "blocked_by" || links[0].Target != "ex-zyo0" {
		t.Errorf("broken_links = %+v, want only ex-3ip9 blocked_by ex-zyo0", links)
	}
}
