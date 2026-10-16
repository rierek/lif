package hooks

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tasks"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

func TestMain(m *testing.M) {
	// No test here starts lif. A step that did would find nothing at this
	// path, instead of re-running this test binary through os.Executable.
	tasks.LifPath = "/nonexistent/lif"
	// A hook-spawned shell carries LIF_NO_HOOKS. Tests that want hooks clear
	// it (isolate).
	os.Setenv("LIF_NO_HOOKS", "1")
	os.Exit(m.Run())
}

// isolate points HOME at a fresh directory, so the test sees no real global
// hooks or trust file, and clears LIF_NO_HOOKS, so hooks are active.
func isolate(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LIF_NO_HOOKS", "")
	return home
}

// task renders one entry of a tasks: section. extra lines (args:, deps:) sit
// beside steps.
func task(name, script string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s:\n", name)
	for _, l := range extra {
		fmt.Fprintf(&b, "    %s\n", l)
	}
	b.WriteString("    steps:\n      - |\n")
	for _, l := range strings.Split(strings.TrimSpace(script), "\n") {
		fmt.Fprintf(&b, "        %s\n", l)
	}
	return b.String()
}

// hook renders one entry of a hooks: section from its mapping lines.
func hook(name string, lines ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  %s:\n", name)
	for _, l := range lines {
		fmt.Fprintf(&b, "    %s\n", l)
	}
	return b.String()
}

func tasksOf(entries ...string) string { return "tasks:\n" + strings.Join(entries, "") }
func hooksOf(entries ...string) string { return "hooks:\n" + strings.Join(entries, "") }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func loadFile(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// allow stores the allow of cfg, as `lif hooks allow` does.
func allow(t *testing.T, cfg *config.Config) {
	t.Helper()
	if err := Allow(Key(cfg.ConfigFile()), Fingerprint(cfg)); err != nil {
		t.Fatal(err)
	}
}

// proj is a project directory with its .lif.yml, a Dispatcher over it, and
// the buffer that receives the Dispatcher's warnings and hook output.
type proj struct {
	t     *testing.T
	home  string // HOME: ~/.lif lives here
	dir   string // holds .lif.yml; task steps run here
	store string // the Dispatcher's storeRoot
	cfg   *config.Config
	warn  *bytes.Buffer
	d     *Dispatcher
}

// newProj isolates the test and builds a project from yaml.
func newProj(t *testing.T, yaml string) *proj {
	t.Helper()
	home := isolate(t)
	dir := t.TempDir()
	store := filepath.Join(dir, ".beans")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, config.ConfigFileName)
	writeFile(t, path, yaml)
	cfg := loadFile(t, path)
	warn := &bytes.Buffer{}
	return &proj{t: t, home: home, dir: dir, store: store, cfg: cfg, warn: warn, d: New(cfg, store, warn)}
}

func (p *proj) allow() { allow(p.t, p.cfg) }

// global writes ~/.lif/tasks.yml and returns its path.
func (p *proj) global(yaml string) string {
	p.t.Helper()
	path := filepath.Join(p.home, config.HomeDir, config.GlobalTasksFileName)
	writeFile(p.t, path, yaml)
	return path
}

func (p *proj) exists(name string) bool {
	_, err := os.Stat(filepath.Join(p.dir, name))
	return err == nil
}

func (p *proj) read(name string) string {
	p.t.Helper()
	b, err := os.ReadFile(filepath.Join(p.dir, name))
	if err != nil {
		p.t.Fatal(err)
	}
	return string(b)
}

// it is a task item with the given status and tags.
func it(status string, tags ...string) *item.Item {
	return &item.Item{ID: "beans-x1", Status: status, Type: "task", Tags: tags}
}

// upd is the update of prev to next that changed the given fields.
func upd(prev, next *item.Item, changed ...string) itemgraph.Mutation {
	return itemgraph.Mutation{Event: "updated", Item: next, Prev: prev, Changed: changed}
}

// toProgress is the update todo to in-progress.
func toProgress() itemgraph.Mutation { return upd(it("todo"), it("in-progress"), "status") }

// git runs git in dir with a fixed identity.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// gitInit makes dir a repository whose first commit holds .lif.yml.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	git(t, dir, "init", "-b", "main")
	git(t, dir, "add", config.ConfigFileName)
	git(t, dir, "commit", "-m", "init")
}

func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
