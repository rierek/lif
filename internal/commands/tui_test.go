package commands

// tui_test.go — E5 Task 6 (bean bt-zhwl, design decision d, "Startup-
// Trigger"): decideStartup is the WHOLE decision, factored out pure
// specifically so it's testable without ever invoking tui.Run (an
// interactive AltScreen tea.Program that a `command go test` process cannot
// safely drive -- no TTY).

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

func TestDecideStartupPrioritiesInOrder(t *testing.T) {
	cwdOK := error(nil)
	cwdFail := errors.New("not a lif repo")

	cases := []struct {
		name    string
		argPath string
		cwdErr  error
		repos   int
		want    startupDecision
	}{
		{"explicit path always wins, even with cwd ok and >=2 repos", "/some/repo", cwdOK, 5, startupUseArgRepo},
		{"explicit path wins even with cwd failing and 0 repos", "/some/repo", cwdFail, 0, startupUseArgRepo},
		{"no arg, cwd resolves -- US-01 unchanged, ignores repo count", "", cwdOK, 5, startupUseCwdRepo},
		{"no arg, cwd resolves, 0 repos -- still cwd, no regression", "", cwdOK, 0, startupUseCwdRepo},
		{"no arg, cwd fails, >=2 repos -- Lobby", "", cwdFail, 2, startupLobby},
		{"no arg, cwd fails, many repos -- Lobby", "", cwdFail, 5, startupLobby},
		{"no arg, cwd fails, exactly 1 repo -- pre-Lobby error, unchanged (US-01 core case)", "", cwdFail, 1, startupError},
		{"no arg, cwd fails, 0 repos -- pre-Lobby error, unchanged", "", cwdFail, 0, startupError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decideStartup(tc.argPath, tc.cwdErr, tc.repos)
			if got != tc.want {
				t.Fatalf("decideStartup(%q, err=%v, repos=%d) = %v, want %v", tc.argPath, tc.cwdErr, tc.repos, got, tc.want)
			}
		})
	}
}

// startupRepo writes a repository whose config prefix is "demo-" and whose
// store holds one task per id (ids are written as given, prefixed or not).
// An id in completed makes that task completed.
func startupRepo(t *testing.T, configName string, prefix string, ids ...string) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, configName)
	yml := "project:\n  name: startup-test\nlif:\n  path: .lif\n  prefix: \"" + prefix + "\"\n"
	if err := os.WriteFile(cfgPath, []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	storeDir := filepath.Join(dir, ".lif")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	core := itemcore.New(storeDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		b := &item.Item{ID: id, Slug: "t", Title: "Title " + id, Status: "todo", Type: "task"}
		if id == "demo-done" {
			b.Status = "completed"
		}
		if id == "demo-child" {
			b.Parent = "demo-parent"
		}
		if id == "demo-grand" {
			b.Parent = "demo-child"
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	return dir
}

func TestResolveStartupItem(t *testing.T) {
	dir := startupRepo(t, ".lif.yml", "demo-", "demo-parent", "demo-child", "demo-done", "demo-grand", "dup", "demo-dup")
	store, err := data.Open(dir, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	for in, want := range map[string]string{
		"demo-child":  "demo-child", // full ID
		"child":       "demo-child", // complete short ID
		"grand":       "demo-grand", // nested item
		"demo-grand":  "demo-grand",
		"done":        "demo-done", // completed item
		"dup":         "dup",       // exact ID wins over prefix-added
		"demo-dup":    "demo-dup",
		"demo-parent": "demo-parent",
	} {
		got, err := resolveStartupItem(store, in)
		if err != nil || got != want {
			t.Errorf("resolveStartupItem(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"par", "demo-chil", "hild", "missing", "demo-missing"} {
		if got, err := resolveStartupItem(store, in); err == nil || got != "" || !strings.Contains(err.Error(), "item not found: "+in) {
			t.Errorf("resolveStartupItem(%q) = %q, %v; want item-not-found error", in, got, err)
		}
	}
}

// runTUIBinary runs the compiled lif with an isolated HOME and without
// LIF_PATH/BEANS_PATH. Only runs that fail before the terminal starts belong
// here: a valid start needs a TTY.
func runTUIBinary(t *testing.T, cwd, home string, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	cmd := exec.Command(completionTestBinary, args...)
	cmd.Dir = cwd
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, config.StoreEnvVar+"=") || strings.HasPrefix(kv, config.LegacyStoreEnvVar+"=") ||
			strings.HasPrefix(kv, "HOME=") || strings.HasPrefix(kv, "XDG_CONFIG_HOME=") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	cmd.Env = append(cmd.Env, "HOME="+home)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
		t.Fatalf("lif %v exited 0, want failure\nstdout: %s\nstderr: %s", args, out.String(), errb.String())
	case errors.As(err, &ee):
		exit = ee.ExitCode()
	default:
		t.Fatal(err)
	}
	return out.String(), errb.String(), exit
}

func TestTUIItemFlagFailsBeforeTerminal(t *testing.T) {
	const altScreen = "\x1b[?1049h"
	repo := startupRepo(t, ".lif.yml", "demo-", "demo-parent", "demo-child")
	other := startupRepo(t, ".lif.yml", "demo-", "demo-only-other")
	noPrefixCfg := filepath.Join(repo, "noprefix.yml")
	if err := os.WriteFile(noPrefixCfg, []byte("project:\n  name: x\nlif:\n  path: .lif\n  prefix: \"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyStore := t.TempDir()

	// Two configured repositories: a missing repo must not fall back to the Lobby.
	home := t.TempDir()
	tuiDir := filepath.Join(home, ".config", "lif", "tui")
	if err := os.MkdirAll(tuiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reposYML := "repos:\n  - " + repo + "\n  - " + other + "\n"
	if err := os.WriteFile(filepath.Join(tuiDir, "config.yaml"), []byte(reposYML), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()

	for _, tc := range []struct {
		name string
		cwd  string
		args []string
		want string
	}{
		{"missing id", outside, []string{"tui", repo, "--item", "nope"}, "item not found: nope"},
		{"partial id", outside, []string{"tui", repo, "--item", "chil"}, "item not found: chil"},
		{"partial full id", outside, []string{"tui", repo, "--item", "demo-chil"}, "item not found: demo-chil"},
		{"empty flag", outside, []string{"tui", repo, "--item="}, "--item requires a non-empty item ID"},
		{"empty arg", outside, []string{"tui", repo, "--item", ""}, "--item requires a non-empty item ID"},
		{"blank flag", outside, []string{"tui", repo, "--item", "  "}, "--item requires a non-empty item ID"},
		{"no repository, two configured: discovery error, no Lobby", outside, []string{"tui", "--item", "demo-child"}, "no .lif.yml found above"},
		{"explicit path beats cwd", other, []string{"tui", repo, "--item", "only-other"}, "item not found: only-other"},
		{"--lif-path selects the store", outside, []string{"tui", repo, "--lif-path", emptyStore, "--item", "demo-child"}, "item not found: demo-child"},
		{"--config selects the prefix", outside, []string{"tui", repo, "--config", noPrefixCfg, "--item", "child"}, "item not found: child"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exit := runTUIBinary(t, tc.cwd, home, tc.args...)
			if exit != 1 {
				t.Errorf("exit = %d, want 1\nstderr: %s", exit, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.want)
			}
			if strings.Contains(stdout+stderr, altScreen) {
				t.Error("AltScreen escape written: the terminal UI started")
			}
		})
	}
}
