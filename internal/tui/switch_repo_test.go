package tui

// switch_repo_test.go — E5 Task 6 (bean bt-zhwl): the Kernschwierigkeit,
// isolated from bubbletea entirely. switchRepoCmd (messages.go) is a plain
// func(oldStop, newRepoDir, notify) tea.Cmd -- calling the returned tea.Cmd
// directly (it's just a func() tea.Msg) exercises the FULL watcher-lifecycle
// switch with two real fsnotify watchers on two real temp directories, no
// tea.Program required.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/tui/data"
)

// fakeLifEmptyList installs a fake "lif" for a repo made by newTestRepoTUI:
// `lif path` names its store dir (what the watcher watches), every other call
// lists nothing.
func fakeLifEmptyList(t *testing.T) {
	t.Helper()
	fakeBeansOnPath(t, "#!/bin/sh\nif [ \"$1\" = path ]; then echo \"$(pwd)/.lif\"; else echo '[]'; fi\n")
}

// newTestRepoTUI creates a minimal beans repo dir (just the .beans/ subdir
// fsnotify watches -- no fixture bean files needed, every test in this file
// installs fakeBeansOnPath so `beans list --json --full` never touches real
// repo content). A package-LOCAL duplicate of internal/data's own unexported
// newTestRepo (testrepo_test.go): Go _test.go helpers are package-private
// even when capitalized (excluded from the package archive other packages
// import), so that helper is invisible from here -- bt-zhwl Task 6 Step 1's
// own "PRÜFEN, ob paketübergreifend exportiert" note, resolved by duplicating
// this one small piece rather than exporting a _test.go helper (which Go
// does not support anyway).
func newTestRepoTUI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".lif"), 0o755); err != nil {
		t.Fatalf("mkdir .lif: %v", err)
	}
	return dir
}

// TestSwitchRepoCmdStopsOldWatcherStartsNew is the bean's own named
// acceptance test: a file touch in the NEW repo after the switch must fire
// notify; a file touch in the OLD repo after the switch must NOT -- the old
// watcher is dead (switchRepoCmd already called oldStop() on it, since the
// new repo validated successfully here).
func TestSwitchRepoCmdStopsOldWatcherStartsNew(t *testing.T) {
	fakeLifEmptyList(t)

	oldRepo := newTestRepoTUI(t)
	newRepo := newTestRepoTUI(t)

	oldNotified := make(chan struct{}, 10)
	oldStop, err := data.Watch(filepath.Join(oldRepo, ".lif"), func() { oldNotified <- struct{}{} })
	if err != nil {
		t.Fatalf("Watch(oldRepo) error = %v", err)
	}
	t.Cleanup(oldStop)

	newNotified := make(chan struct{}, 10)
	cmd := switchRepoCmd(oldStop, newRepo, "", func() { newNotified <- struct{}{} })
	msg := cmd()
	rs, ok := msg.(repoSwitchedMsg)
	if !ok {
		t.Fatalf("switchRepoCmd() returned %T, want repoSwitchedMsg", msg)
	}
	if rs.err != nil {
		t.Fatalf("repoSwitchedMsg.err = %v, want nil", rs.err)
	}
	if rs.repoDir != newRepo {
		t.Fatalf("repoSwitchedMsg.repoDir = %q, want %q", rs.repoDir, newRepo)
	}
	if rs.client == nil {
		t.Fatal("repoSwitchedMsg.client is nil, want a live client")
	}
	if rs.watchStop == nil {
		t.Fatal("repoSwitchedMsg.watchStop is nil, want a live stop func")
	}
	t.Cleanup(rs.watchStop)

	// NEW repo: a file touch AFTER the switch must fire notify.
	if err := os.WriteFile(filepath.Join(newRepo, ".lif", "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write new repo file: %v", err)
	}
	select {
	case <-newNotified:
	case <-time.After(2 * time.Second):
		t.Fatal("new watcher's notify not called within 2s of a file touch")
	}

	// OLD repo: a file touch AFTER the switch must NOT fire notify -- the
	// old watcher is dead.
	if err := os.WriteFile(filepath.Join(oldRepo, ".lif", "y.md"), []byte("y"), 0o644); err != nil {
		t.Fatalf("write old repo file: %v", err)
	}
	select {
	case <-oldNotified:
		t.Fatal("old watcher's notify fired after switch -- old watcher was not stopped")
	case <-time.After(500 * time.Millisecond):
	}
}

// TestSwitchRepoCmdHonorsExplicitStorePath is switchRepoCmd's own coverage
// of Client.StorePath (internal/data/client_test.go's
// TestClientHonorsExplicitStorePath already covers the Client layer
// directly): the Lobby must forward config.Settings.RepoStorePaths[repo],
// not silently clear it, for the one repo dir it names.
func TestSwitchRepoCmdHonorsExplicitStorePath(t *testing.T) {
	var gotArgs string
	fakeBeansOnPath(t, `#!/bin/sh
echo "ARGS=$@" >&2
exit 1
`)

	newRepo := newTestRepoTUI(t)
	cmd := switchRepoCmd(nil, newRepo, "/explicit/store/.lif", func() {})
	msg := cmd()
	rs, ok := msg.(repoSwitchedMsg)
	if !ok {
		t.Fatalf("switchRepoCmd() returned %T, want repoSwitchedMsg", msg)
	}
	if rs.err == nil {
		t.Fatal("repoSwitchedMsg.err = nil, want an error (fake lif always fails)")
	}
	gotArgs = rs.err.Error()
	if !strings.Contains(gotArgs, "--lif-path /explicit/store/.lif") {
		t.Fatalf("switchRepoCmd did not forward the explicit StorePath, error = %q", gotArgs)
	}
}

// TestSwitchRepoCmdKeepsOldWatcherAliveOnValidationFailure is the regression
// guard for the "Fehlerpfad darf die laufende Session NICHT zerstören"
// constraint (bean bt-zhwl): switching to a broken/empty target must leave
// the OLD watcher completely untouched -- still running, still live -- since
// switchRepoCmd validates the NEW repo BEFORE ever calling oldStop
// (design note, messages.go's own switchRepoCmd doc-stamp).
func TestSwitchRepoCmdKeepsOldWatcherAliveOnValidationFailure(t *testing.T) {
	fakeBeansOnPath(t, "#!/bin/sh\necho 'boom' 1>&2\nexit 1\n")

	oldRepo := newTestRepoTUI(t)
	brokenRepo := t.TempDir() // no .beans dir -- List() fails first regardless

	oldNotified := make(chan struct{}, 10)
	oldStop, err := data.Watch(filepath.Join(oldRepo, ".lif"), func() { oldNotified <- struct{}{} })
	if err != nil {
		t.Fatalf("Watch(oldRepo) error = %v", err)
	}
	t.Cleanup(oldStop)

	cmd := switchRepoCmd(oldStop, brokenRepo, "", func() {})
	msg := cmd()
	rs, ok := msg.(repoSwitchedMsg)
	if !ok {
		t.Fatalf("switchRepoCmd() returned %T, want repoSwitchedMsg", msg)
	}
	if rs.err == nil {
		t.Fatal("repoSwitchedMsg.err = nil, want an error for a broken target repo")
	}
	if rs.client != nil {
		t.Fatal("repoSwitchedMsg.client is set on a failed switch, want nil (caller must not apply it)")
	}

	// OLD watcher must still be ALIVE -- oldStop was never called.
	if err := os.WriteFile(filepath.Join(oldRepo, ".lif", "z.md"), []byte("z"), 0o644); err != nil {
		t.Fatalf("write old repo file: %v", err)
	}
	select {
	case <-oldNotified:
	case <-time.After(2 * time.Second):
		t.Fatal("old watcher did not fire after a validation-failed switch -- it was wrongly stopped")
	}
}
