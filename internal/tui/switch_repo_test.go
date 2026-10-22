package tui

// switch_repo_test.go — E5 Task 6 (bean bt-zhwl): the Kernschwierigkeit,
// isolated from bubbletea entirely. switchRepoCmd (messages.go) is a plain
// func(oldStore, newRepoDir, storePath) tea.Cmd -- calling the returned
// tea.Cmd directly (it's just a func() tea.Msg) exercises the FULL store
// switch with two real stores on two real temp directories, no tea.Program
// required.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rierek/lif/internal/tui/data"
)

// newTestRepoTUI creates a minimal lif repo dir: .lif.yml plus an empty store
// dir, so data.Open succeeds and the store watcher has a directory to watch.
func newTestRepoTUI(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte("project:\n    name: tui-test\nlif:\n    path: .lif\n"), 0o644); err != nil {
		t.Fatalf("write .lif.yml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".lif"), 0o755); err != nil {
		t.Fatalf("mkdir .lif: %v", err)
	}
	return dir
}

// writeItemFile drops a minimal item file into repo's store.
func writeItemFile(t *testing.T, repo, name string) {
	t.Helper()
	body := "---\ntitle: " + name + "\nstatus: todo\ntype: task\n---\n"
	if err := os.WriteFile(filepath.Join(repo, ".lif", name+".md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write item file: %v", err)
	}
}

// TestSwitchRepoCmdStopsOldWatcherStartsNew is the bean's own named
// acceptance test: a file touch in the NEW repo after the switch must reach
// the new events channel; the OLD store's channel is closed (the old watcher
// is dead) -- switchRepoCmd closed it since the new repo opened successfully.
func TestSwitchRepoCmdStopsOldWatcherStartsNew(t *testing.T) {
	oldRepo := newTestRepoTUI(t)
	newRepo := newTestRepoTUI(t)

	oldStore, err := data.Open(oldRepo, "", "")
	if err != nil {
		t.Fatalf("Open(oldRepo): %v", err)
	}
	t.Cleanup(func() { oldStore.Close() })
	oldEvents, err := oldStore.Watch()
	if err != nil {
		t.Fatalf("Watch(oldRepo): %v", err)
	}

	msg := switchRepoCmd(oldStore, newRepo, "")()
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
	if rs.store == nil {
		t.Fatal("repoSwitchedMsg.store is nil, want a live store")
	}
	t.Cleanup(func() { rs.store.Close() })
	if rs.events == nil {
		t.Fatal("repoSwitchedMsg.events is nil, want a live event channel")
	}

	// OLD repo: its store was closed, so its event channel is closed too.
	select {
	case _, ok := <-oldEvents:
		if ok {
			t.Fatal("old store delivered an event after the switch")
		}
	case <-time.After(time.Second):
		t.Fatal("old store's event channel was not closed by the switch")
	}

	// NEW repo: a file touch AFTER the switch must reach the new channel.
	writeItemFile(t, newRepo, "x")
	select {
	case <-rs.events:
	case <-time.After(5 * time.Second):
		t.Fatal("new store reported no event within 5s of a file touch")
	}
}

// TestSwitchRepoCmdHonorsExplicitStorePath is switchRepoCmd's own coverage
// of the store path: the Lobby must forward config.Settings.RepoStorePaths
// [repo], not silently clear it, for the one repo dir it names.
func TestSwitchRepoCmdHonorsExplicitStorePath(t *testing.T) {
	newRepo := newTestRepoTUI(t)
	explicit := filepath.Join(t.TempDir(), "explicit-store")
	if err := os.MkdirAll(explicit, 0o755); err != nil {
		t.Fatal(err)
	}
	writeItemFile(t, newRepo, "in-repo-store") // only the default store holds this

	rs, ok := switchRepoCmd(nil, newRepo, explicit)().(repoSwitchedMsg)
	if !ok || rs.err != nil {
		t.Fatalf("switchRepoCmd() = %+v, want a successful repoSwitchedMsg", rs)
	}
	t.Cleanup(func() { rs.store.Close() })
	if len(rs.beans) != 0 {
		t.Fatalf("beans = %d items, want 0 from the explicit (empty) store, not the repo's default one", len(rs.beans))
	}
}

// TestSwitchRepoCmdKeepsOldStoreAliveOnOpenFailure is the regression guard
// for the "Fehlerpfad darf die laufende Session NICHT zerstören" constraint
// (bean bt-zhwl): switching to a broken target must leave the OLD store
// completely untouched -- still open, still watching -- since switchRepoCmd
// opens the NEW repo BEFORE ever closing the old store (design note,
// messages.go's own switchRepoCmd doc-stamp).
func TestSwitchRepoCmdKeepsOldStoreAliveOnOpenFailure(t *testing.T) {
	oldRepo := newTestRepoTUI(t)
	brokenRepo := t.TempDir() // no .lif.yml, no store dir -- Open fails

	oldStore, err := data.Open(oldRepo, "", "")
	if err != nil {
		t.Fatalf("Open(oldRepo): %v", err)
	}
	t.Cleanup(func() { oldStore.Close() })
	oldEvents, err := oldStore.Watch()
	if err != nil {
		t.Fatalf("Watch(oldRepo): %v", err)
	}

	rs, ok := switchRepoCmd(oldStore, brokenRepo, "")().(repoSwitchedMsg)
	if !ok {
		t.Fatal("switchRepoCmd() did not return a repoSwitchedMsg")
	}
	if rs.err == nil {
		t.Fatal("repoSwitchedMsg.err = nil, want an error for a broken target repo")
	}
	if rs.store != nil {
		t.Fatal("repoSwitchedMsg.store is set on a failed switch, want nil (caller must not apply it)")
	}

	// OLD store must still be ALIVE -- never closed.
	writeItemFile(t, oldRepo, "z")
	select {
	case _, ok := <-oldEvents:
		if !ok {
			t.Fatal("old store's channel was closed by a failed switch")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old store reported nothing after a failed switch -- it was wrongly stopped")
	}
}
