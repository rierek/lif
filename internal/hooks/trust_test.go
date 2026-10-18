package hooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// notAllowed is the not-allowed warning for cfg; gates is its parenthesis
// (" (1 gates not enforced)") or empty.
func notAllowed(cfg *config.Config, gates string) string {
	return "warning: hooks in " + cfg.ConfigFile() + " are not allowed" + gates + "; run 'lif hooks allow' to enable them\n"
}

func TestT01_NotAllowedProjectHooksDoNotRunAndWarnOnce(t *testing.T) {
	p := newProj(t, tasksOf(
		task("pa", `touch "$LIF_ROOT/project-after"`),
		task("pg", `touch "$LIF_ROOT/project-gate"`))+
		hooksOf(
			hook("p-after", "event: [updated]", "run: pa"),
			hook("p-gate", "stage: before", "event: [updated]", "run: pg")))
	p.global(tasksOf(task("ga", `touch "$LIF_ROOT/global-after"`)) +
		hooksOf(hook("g-after", "event: [updated]", "run: ga")))

	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate = %v, %v; want false, nil: the project gate is not allowed", ran, err)
	}
	if !p.d.Fire(toProgress()) {
		t.Error("Fire reported no run: the global hook must run")
	}

	if p.exists("project-after") || p.exists("project-gate") {
		t.Errorf("project hooks ran: after = %v, gate = %v", p.exists("project-after"), p.exists("project-gate"))
	}
	if !p.exists("global-after") {
		t.Error("the global hook did not run")
	}
	if got, want := p.warn.String(), notAllowed(p.cfg, " (1 gates not enforced)"); got != want {
		t.Errorf("warn = %q, want exactly %q", got, want)
	}
}

func TestT02_ChangedTaskStepRevokesTheAllow(t *testing.T) {
	const when = "event: [updated]"
	v1 := tasksOf(task("t", `echo v1 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", when, "run: t"))
	v2 := tasksOf(task("t", `echo v2 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", when, "run: t"))
	p := newProj(t, v1)
	p.allow()
	if !p.d.Fire(toProgress()) {
		t.Fatal("the allowed hook did not run")
	}

	path := p.cfg.ConfigFile()
	writeFile(t, path, v2)
	cfg2 := loadFile(t, path)
	if Fingerprint(cfg2) == Fingerprint(p.cfg) {
		t.Fatal("the fingerprint did not change with the step")
	}
	warn := &bytes.Buffer{}
	d2 := New(cfg2, p.store, warn)

	if d2.Fire(toProgress()) {
		t.Error("the hook ran with a changed step and no new allow")
	}
	if got, want := warn.String(), notAllowed(cfg2, ""); got != want {
		t.Errorf("warn = %q, want %q", got, want)
	}

	allow(t, cfg2)
	if !d2.Fire(toProgress()) {
		t.Error("the hook did not run after the next allow")
	}
	if got, want := p.read("log"), "v1\nv2\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestT03_DenyTakesTheAllowBack(t *testing.T) {
	isolate(t)
	const key, fp = "/repo/.lif.yml", "fp1"

	if ok, err := Allowed(key, fp); ok || err != nil {
		t.Fatalf("before Allow: Allowed = %v, %v; want false, nil", ok, err)
	}
	if err := Allow(key, fp); err != nil {
		t.Fatal(err)
	}
	if ok, err := Allowed(key, fp); !ok || err != nil {
		t.Fatalf("after Allow: Allowed = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := Allowed(key, "other"); ok {
		t.Error("Allowed(key, other fingerprint) = true, want false")
	}
	if err := Deny(key); err != nil {
		t.Fatal(err)
	}
	if ok, err := Allowed(key, fp); ok || err != nil {
		t.Errorf("after Deny: Allowed = %v, %v; want false, nil", ok, err)
	}
}

func TestT04_TrustFileIsPrivate(t *testing.T) {
	home := isolate(t)
	if err := Allow("/repo/.lif.yml", "fp1"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, config.HomeDir)
	file := filepath.Join(dir, "allowed-hooks.yml")

	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("trust file: %v, %v; want mode 0600", info, err)
	}
	if info, err := os.Stat(dir); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("trust directory: %v, %v; want mode 0700", info, err)
	}

	// A file that exists with a looser mode comes out private after the next write.
	if err := os.Chmod(file, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Allow("/other/.lif.yml", "fp2"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(file); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("rewritten trust file: %v, %v; want mode 0600", info, err)
	}
}

func TestT05_GlobalGateNeedsNoAllow(t *testing.T) {
	p := newProj(t, tasksOf(task("noop", "true")))
	p.global(tasksOf(task("deny", "echo nope\nexit 1")) +
		hooksOf(hook("g", "stage: before", "event: [updated]", "run: deny")))

	ran, err := p.d.Gate(toProgress())
	blocked := blockedBy(t, err)
	if blocked.Hook != "g" || blocked.Output != "nope" {
		t.Errorf("blocked by %q with output %q, want g with nope", blocked.Hook, blocked.Output)
	}
	if !ran {
		t.Error("ran = false, want true")
	}
}

func TestT06_OneDispatcherFollowsTrustAndGlobalFileChanges(t *testing.T) {
	p := newProj(t, tasksOf(task("t", `echo ran >> "$LIF_ROOT/log"`))+
		hooksOf(hook("h", "event: [updated]", "run: t")))
	p.allow()

	if !p.d.Fire(toProgress()) {
		t.Fatal("allowed project hook did not run")
	}
	if got, want := p.read("log"), "ran\n"; got != want {
		t.Fatalf("log = %q, want %q", got, want)
	}

	if err := Deny(Key(p.cfg.ConfigFile())); err != nil {
		t.Fatal(err)
	}
	if p.d.Fire(toProgress()) {
		t.Error("the hook ran after Deny")
	}
	if got, want := p.read("log"), "ran\n"; got != want {
		t.Errorf("log = %q after Deny, want it unchanged", got)
	}

	globalPath := p.global("hooks: [unclosed\n")
	_, err := p.d.Gate(toProgress())
	prefix := `hook "*" blocked the change of beans-x1: hooks could not be loaded: ` + globalPath
	if err == nil || !strings.HasPrefix(err.Error(), prefix) || !strings.HasSuffix(err.Error(), " (fix it or set LIF_NO_HOOKS=1)") {
		t.Errorf("Gate error = %v, want the block of a broken global file", err)
	}

	p.global("tasks: {}\n")
	p.allow()
	if ran, err := p.d.Gate(toProgress()); ran || err != nil {
		t.Errorf("Gate after the fix = %v, %v; want false, nil", ran, err)
	}
	if !p.d.Fire(toProgress()) {
		t.Error("the hook did not run after the fix and a new allow")
	}
	if got, want := p.read("log"), "ran\nran\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}

func TestT07_OneAllowCoversEveryWorktreeOfTheRepository(t *testing.T) {
	isolate(t)
	v1 := tasksOf(task("t", `echo v1 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", "event: [updated]", "run: t"))
	v2 := tasksOf(task("t", `echo v2 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", "event: [updated]", "run: t"))
	mainDir := t.TempDir()
	writeFile(t, filepath.Join(mainDir, config.ConfigFileName), v1)
	gitInit(t, mainDir)
	wtDir := filepath.Join(t.TempDir(), "wt")
	git(t, mainDir, "worktree", "add", wtDir, "-b", "side")
	allow(t, loadFile(t, filepath.Join(mainDir, config.ConfigFileName)))

	// The worktree has the identical .lif.yml: its hooks run.
	wtConfig := filepath.Join(wtDir, config.ConfigFileName)
	warn := &bytes.Buffer{}
	if !New(loadFile(t, wtConfig), "", warn).Fire(toProgress()) {
		t.Fatalf("the hook did not run in the worktree; warn = %q", warn.String())
	}
	if got, want := readFile(t, filepath.Join(wtDir, "log")), "v1\n"; got != want {
		t.Fatalf("worktree log = %q, want %q", got, want)
	}

	// With a changed task step the worktree is not allowed.
	writeFile(t, wtConfig, v2)
	cfg := loadFile(t, wtConfig)
	warn.Reset()
	if New(cfg, "", warn).Fire(toProgress()) {
		t.Error("the hook ran in the worktree with a changed step")
	}
	if got, want := warn.String(), notAllowed(cfg, ""); got != want {
		t.Errorf("warn = %q, want %q", got, want)
	}
	if got, want := readFile(t, filepath.Join(wtDir, "log")), "v1\n"; got != want {
		t.Errorf("worktree log = %q, want it unchanged", got)
	}
}

func TestT08_SymlinkedAndPhysicalPathShareTheAllow(t *testing.T) {
	for _, tc := range []struct {
		name  string
		inGit bool
	}{{"outside git", false}, {"in a git checkout", true}} {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			phys := t.TempDir()
			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink(phys, link); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(phys, config.ConfigFileName),
				tasksOf(task("t", `touch "$LIF_ROOT/ran"`))+hooksOf(hook("h", "event: [updated]", "run: t")))
			if tc.inGit {
				gitInit(t, phys)
			}
			allow(t, loadFile(t, filepath.Join(link, config.ConfigFileName)))

			cfg := loadFile(t, filepath.Join(phys, config.ConfigFileName))
			warn := &bytes.Buffer{}
			if !New(cfg, "", warn).Fire(toProgress()) {
				t.Errorf("the hook did not run under the physical path; warn = %q", warn.String())
			}
		})
	}
}

func TestT09_NotAllowedProjectDoesNotHideAGlobalGate(t *testing.T) {
	p := newProj(t, tasksOf(task("never", `touch "$LIF_ROOT/project-ran"`))+
		hooksOf(hook("x", "stage: before", "event: [updated]", "tags: [never]", "run: never")))
	p.global(tasksOf(task("block", "exit 1")) +
		hooksOf(hook("x", "stage: before", "event: [updated]", "run: block")))

	_, err := p.d.Gate(toProgress())
	if blocked := blockedBy(t, err); blocked.Hook != "x" {
		t.Errorf("blocked by %q, want the global gate x", blocked.Hook)
	}
	if p.exists("project-ran") {
		t.Error("the not-allowed project hook ran")
	}
}

func TestT11_CommentsAndKeyOrderKeepTheAllow(t *testing.T) {
	const v1 = `tasks:
  t:
    steps:
      - |
        echo ran >> "$LIF_ROOT/log"
hooks:
  h:
    event: [updated]
    run: t
`
	const v2 = `# project config, edited
hooks:
  # runs the task below
  h:
    run: t # which task
    event: [updated]
tasks:
  t:
    steps:
      - |
        echo ran >> "$LIF_ROOT/log"
`
	p := newProj(t, v1)
	p.allow()

	path := p.cfg.ConfigFile()
	writeFile(t, path, v2)
	cfg := loadFile(t, path)
	if ok, err := Allowed(Key(path), Fingerprint(cfg)); !ok || err != nil {
		t.Errorf("Allowed after comments and reordering = %v, %v; want true, nil", ok, err)
	}
	if !New(cfg, p.store, &bytes.Buffer{}).Fire(toProgress()) {
		t.Error("the hook did not run after comments and reordering")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestKeyIsThePhysicalConfigPathOfTheMainCheckout pins the key itself: T07 and
// T08 only show that two paths share one key.
func TestKeyIsThePhysicalConfigPathOfTheMainCheckout(t *testing.T) {
	isolate(t)

	t.Run("outside git", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, config.ConfigFileName), "")
		want := filepath.Join(evalSymlinks(t, dir), config.ConfigFileName)
		if got := Key(filepath.Join(dir, config.ConfigFileName)); got != want {
			t.Errorf("Key = %q, want %q", got, want)
		}
	})

	t.Run("main checkout, subdirectory and worktree", func(t *testing.T) {
		mainDir := t.TempDir()
		writeFile(t, filepath.Join(mainDir, config.ConfigFileName), "")
		writeFile(t, filepath.Join(mainDir, "sub", config.ConfigFileName), "")
		gitInit(t, mainDir)
		physMain := evalSymlinks(t, mainDir)
		wtDir := filepath.Join(t.TempDir(), "wt")
		git(t, mainDir, "worktree", "add", wtDir, "-b", "side")

		for name, c := range map[string]struct{ path, want string }{
			"main":         {filepath.Join(mainDir, config.ConfigFileName), filepath.Join(physMain, config.ConfigFileName)},
			"subdirectory": {filepath.Join(mainDir, "sub", config.ConfigFileName), filepath.Join(physMain, "sub", config.ConfigFileName)},
			"worktree":     {filepath.Join(wtDir, config.ConfigFileName), filepath.Join(physMain, config.ConfigFileName)},
		} {
			if got := Key(c.path); got != c.want {
				t.Errorf("Key(%s) = %q, want %q", name, got, c.want)
			}
		}
	})
}

func TestFingerprintCoversHooksAndTasks(t *testing.T) {
	isolate(t)
	base := tasksOf(task("t", "echo a")) + hooksOf(hook("h", "event: [updated]", "run: t"))
	fp := func(yaml string) string {
		dir := t.TempDir()
		path := filepath.Join(dir, config.ConfigFileName)
		writeFile(t, path, yaml)
		return Fingerprint(loadFile(t, path))
	}

	if fp(base) != fp(base) {
		t.Error("the same config gives two fingerprints")
	}
	for name, changed := range map[string]string{
		"task step":  tasksOf(task("t", "echo b")) + hooksOf(hook("h", "event: [updated]", "run: t")),
		"hook":       tasksOf(task("t", "echo a")) + hooksOf(hook("h", "event: [updated]", "to: [completed]", "run: t")),
		"added task": tasksOf(task("t", "echo a"), task("u", "echo a")) + hooksOf(hook("h", "event: [updated]", "run: t")),
	} {
		if fp(changed) == fp(base) {
			t.Errorf("a changed %s leaves the fingerprint as it was", name)
		}
	}
}

// A running dispatcher must not trust the .lif.yml that it read at start when
// the file on disk has changed: a step that runs `$LIF run <task>` reads the
// new file.
func TestT20_ConfigChangedOnDiskAfterStartIsNotAllowed(t *testing.T) {
	const when = "event: [updated]"
	v1 := tasksOf(task("t", `echo v1 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", when, "run: t"))
	v2 := tasksOf(task("t", `echo v2 >> "$LIF_ROOT/log"`)) + hooksOf(hook("h", when, "run: t"))
	p := newProj(t, v1)
	p.allow()
	if !p.d.Fire(toProgress()) {
		t.Fatal("the allowed hook did not run")
	}

	writeFile(t, p.cfg.ConfigFile(), v2) // a git pull while the dispatcher lives
	if p.d.Fire(toProgress()) {
		t.Error("the hook ran although .lif.yml changed on disk since start")
	}
	if got, want := p.read("log"), "v1\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
	if !strings.Contains(p.warn.String(), "changed since") {
		t.Errorf("warn = %q, want a note that the file changed", p.warn.String())
	}
}
