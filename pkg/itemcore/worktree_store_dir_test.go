package itemcore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// Inside a worktree an item store lives in the directory that carries the same
// name as the main project's store directory: .beans for a legacy store, .lif
// for a new one (AC-9). The unused sibling name must never be read or written.

func storeDirNames() []struct{ own, other string } {
	return []struct{ own, other string }{
		{config.LegacyDefaultStoreDir, config.DefaultStoreDir},
		{config.DefaultStoreDir, config.LegacyDefaultStoreDir},
	}
}

func newCoreNamed(t *testing.T, name string) *Core {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	itemsDir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	core := New(itemsDir, config.Default())
	core.SetWarnWriter(nil)
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return core
}

func TestWorktreeSave_UsesTheMainStoreDirName(t *testing.T) {
	for _, tc := range storeDirNames() {
		t.Run(tc.own, func(t *testing.T) {
			core := newCoreNamed(t, tc.own)
			b := createTestItem(t, core, "wt-save-1", "Saved", "todo")
			wtDir := t.TempDir()

			b.Status = "in-progress"
			if err := core.Update(b, nil, WithWorktreePath(wtDir)); err != nil {
				t.Fatalf("Update() error = %v", err)
			}

			file := item.BuildFilename(b.ID, b.Slug)
			if _, err := os.Stat(filepath.Join(wtDir, tc.own, file)); err != nil {
				t.Errorf("item not written to %s inside the worktree: %v", tc.own, err)
			}
			if _, err := os.Stat(filepath.Join(wtDir, tc.other)); !os.IsNotExist(err) {
				t.Errorf("worktree gained a %s directory (err = %v)", tc.other, err)
			}
		})
	}
}

func TestWorktreeStatusOnDisk_ReadsTheMainStoreDirName(t *testing.T) {
	for _, tc := range storeDirNames() {
		t.Run(tc.own, func(t *testing.T) {
			core := newCoreNamed(t, tc.own)
			wtDir := t.TempDir()
			content := "---\ntitle: X\nstatus: in-progress\ntype: task\n---\n"
			if err := os.MkdirAll(filepath.Join(wtDir, tc.own), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(wtDir, tc.own, "x.md"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			// The sibling name carries a different status; reading it would be wrong.
			if err := os.MkdirAll(filepath.Join(wtDir, tc.other), 0o755); err != nil {
				t.Fatal(err)
			}
			wrong := "---\ntitle: X\nstatus: completed\ntype: task\n---\n"
			if err := os.WriteFile(filepath.Join(wtDir, tc.other, "x.md"), []byte(wrong), 0o644); err != nil {
				t.Fatal(err)
			}

			status, ok := core.statusOnDisk("x.md", wtDir)
			if !ok || status != "in-progress" {
				t.Errorf("statusOnDisk() = %q, %v; want in-progress from %s", status, ok, tc.own)
			}
		})
	}
}

func TestWorktreeWatch_WatchesTheMainStoreDirName(t *testing.T) {
	for _, tc := range storeDirNames() {
		t.Run(tc.own, func(t *testing.T) {
			core := newCoreNamed(t, tc.own)
			defer core.UnwatchAllWorktrees()
			wtDir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(wtDir, tc.own), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := core.WatchWorktreeItems(wtDir); err != nil {
				t.Fatalf("WatchWorktreeItems() error = %v", err)
			}
			core.mu.RLock()
			wt := core.worktreeWatchers[wtDir]
			core.mu.RUnlock()
			if wt == nil || wt.itemsDir != filepath.Join(wtDir, tc.own) {
				t.Errorf("watcher = %+v, want one on %s", wt, filepath.Join(wtDir, tc.own))
			}
		})
	}

	t.Run("a worktree with only the sibling name is not watched", func(t *testing.T) {
		core := newCoreNamed(t, config.DefaultStoreDir)
		wtDir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(wtDir, config.LegacyDefaultStoreDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := core.WatchWorktreeItems(wtDir); err != nil {
			t.Fatalf("WatchWorktreeItems() error = %v", err)
		}
		core.mu.RLock()
		defer core.mu.RUnlock()
		if len(core.worktreeWatchers) != 0 {
			t.Errorf("watched %d worktrees, want none", len(core.worktreeWatchers))
		}
	})
}
