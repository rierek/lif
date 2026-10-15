package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func noop(*cobra.Command, []string) {}

// fixture is a small lif-like tree: two plain commands, a hidden and a deprecated one, `completion` with two
// shells, and Cobra's built-in help.
func fixture() *cobra.Command {
	root := &cobra.Command{Use: "lif", Short: "root short"}
	root.AddCommand(
		&cobra.Command{Use: "list", Short: "List items", Run: noop},
		&cobra.Command{Use: "show", Short: "Show an item", Run: noop},
		&cobra.Command{Use: "secret", Short: "hidden one", Hidden: true, Run: noop},
		&cobra.Command{Use: "old", Short: "deprecated one", Deprecated: "use list", Run: noop},
	)
	completion := &cobra.Command{Use: "completion", Short: "Generate completion"}
	completion.AddCommand(
		&cobra.Command{Use: "bash", Short: "bash script", Run: noop},
		&cobra.Command{Use: "zsh", Short: "zsh script", Run: noop},
	)
	root.AddCommand(completion)
	root.InitDefaultHelpCmd()
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestCompletionShellsShareOnePage(t *testing.T) {
	out := t.TempDir()
	if err := generate(fixture(), out); err != nil {
		t.Fatal(err)
	}
	page := read(t, filepath.Join(out, "lif-completion.md"))
	for _, want := range []string{"## lif completion\n", "## lif completion bash\n", "## lif completion zsh\n", "(#lif-completion-bash)"} {
		if !strings.Contains(page, want) {
			t.Errorf("lif-completion.md lacks %q", want)
		}
	}
	for _, shell := range []string{"lif-completion-bash.md", "lif-completion-zsh.md"} {
		if exists(filepath.Join(out, shell)) {
			t.Errorf("%s exists, the shells must share lif-completion.md", shell)
		}
	}
}

func TestStalePagesAreDeletedAndHandWrittenPagesStay(t *testing.T) {
	out := t.TempDir()
	for name, body := range map[string]string{
		"lif-help.md": "old help page", // D57: dropped
		"lif-gone.md": "command that no longer exists",
		"lif-list.md": "outdated",
		"index.md":    "hand-written overview",
	} {
		if err := os.WriteFile(filepath.Join(out, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := generate(fixture(), out); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"lif-help.md", "lif-gone.md"} {
		if exists(filepath.Join(out, gone)) {
			t.Errorf("%s survived the run", gone)
		}
	}
	if got := read(t, filepath.Join(out, "lif-list.md")); !strings.Contains(got, "List items") {
		t.Errorf("lif-list.md was not regenerated: %q", got)
	}
	if got := read(t, filepath.Join(out, "index.md")); got != "hand-written overview" {
		t.Errorf("index.md was touched: %q", got)
	}
}

func TestOnlyUsableCommandsGetPages(t *testing.T) {
	out := t.TempDir()
	if err := generate(fixture(), out); err != nil {
		t.Fatal(err)
	}
	var got []string
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		got = append(got, e.Name())
	}
	want := "_command-tree.md lif-completion.md lif-list.md lif-show.md lif.md"
	if strings.Join(got, " ") != want { // no help, hidden or deprecated command
		t.Errorf("generated files:\n got %v\nwant %s", got, want)
	}
}

func TestCommandTreeShowsEachCommandUnderItsParent(t *testing.T) {
	out := t.TempDir()
	if err := generate(fixture(), out); err != nil {
		t.Fatal(err)
	}
	want := "- [`lif`](lif/): root short\n" +
		"  - [`lif completion`](lif-completion/): Generate completion\n" +
		"    - [`lif completion bash`](lif-completion/#lif-completion-bash): bash script\n" +
		"    - [`lif completion zsh`](lif-completion/#lif-completion-zsh): zsh script\n" +
		"  - [`lif list`](lif-list/): List items\n" +
		"  - [`lif show`](lif-show/): Show an item\n"
	if got := read(t, filepath.Join(out, treeFile)); got != want {
		t.Errorf("command tree:\n got %q\nwant %q", got, want)
	}
}

// The real lif tree: D57 drops the help page, D25 keeps the shared completion page.
func TestRealTree(t *testing.T) {
	out := t.TempDir()
	if err := generate(newRoot(), out); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(out, "lif-help.md")) {
		t.Error("lif-help.md was generated")
	}
	for _, name := range []string{"lif.md", "lif-list.md", "lif-completion.md", treeFile} {
		if !exists(filepath.Join(out, name)) {
			t.Errorf("%s missing", name)
		}
	}
	tree := read(t, filepath.Join(out, treeFile))
	for _, want := range []string{"- [`lif`](lif/): ", "  - [`lif list`](lif-list/): ", "    - [`lif completion bash`](lif-completion/#lif-completion-bash): "} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree lacks %q", want)
		}
	}
}

func TestIndentedExampleCommentsAreFencedNotHeadings(t *testing.T) {
	in := "Examples:\n  # List all\n  lif list\n\n  # Get one\n  lif show x\n\n```\nlif y\n```\n"
	got := fenceIndentedComments(in)
	want := "Examples:\n```\n# List all\nlif list\n\n# Get one\nlif show x\n```\n\n```\nlif y\n```\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSupplementIsInsertedBeforeSeeAlso(t *testing.T) {
	out := t.TempDir()
	dir := filepath.Join(out, supplementDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_lif_list.md"), []byte("### Extra\n\nhand-written\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := generate(fixture(), out); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(out, "lif-list.md"))
	extra, see := strings.Index(got, "### Extra"), strings.Index(got, "### SEE ALSO")
	if extra < 0 || see < 0 || extra > see {
		t.Errorf("supplement not before SEE ALSO:\n%s", got)
	}
	if strings.Contains(read(t, filepath.Join(out, "lif-show.md")), "hand-written") {
		t.Error("supplement leaked into another page")
	}
}
