package itemcore

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
)

// mainPath is the store-relative counterpart of findMainItemFile, for assertions.
func mainPathRel(t *testing.T, c *Core, id string) string {
	t.Helper()
	abs := func() string {
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.findMainItemFile(id)
	}()
	if abs == "" {
		return ""
	}
	rel, err := filepath.Rel(c.root, abs)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q): %v", c.root, abs, err)
	}
	return rel
}

func TestFindMainItemFile(t *testing.T) {
	t.Run("finds a bean stored in a subdirectory", func(t *testing.T) {
		core, itemsDir := setupTestCore(t)

		// Items may live in subdirectories — loadFromDisk walks the tree.
		// A flat directory scan misses those.
		sub := filepath.Join(itemsDir, "epic-auth")
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatalf("creating subdirectory: %v", err)
		}
		content := "---\ntitle: Nested\nstatus: todo\ntype: task\n---\n"
		if err := os.WriteFile(filepath.Join(sub, "nested-1--nested.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing nested bean: %v", err)
		}
		if err := core.Load(); err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got, want := mainPathRel(t, core, "nested-1"), filepath.Join("epic-auth", "nested-1--nested.md"); got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})

	t.Run("tracks create, archive, unarchive and delete", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "path-1", "Tracked", "todo")

		if got, want := mainPathRel(t, core, "path-1"), "path-1--tracked.md"; got != want {
			t.Errorf("after create: findMainBeanFile() = %q, want %q", got, want)
		}

		if err := core.Archive("path-1"); err != nil {
			t.Fatalf("Archive(): %v", err)
		}
		if got, want := mainPathRel(t, core, "path-1"), filepath.Join(ArchiveDir, "path-1--tracked.md"); got != want {
			t.Errorf("after archive: findMainBeanFile() = %q, want %q", got, want)
		}

		if err := core.Unarchive("path-1"); err != nil {
			t.Fatalf("Unarchive(): %v", err)
		}
		if got, want := mainPathRel(t, core, "path-1"), "path-1--tracked.md"; got != want {
			t.Errorf("after unarchive: findMainBeanFile() = %q, want %q", got, want)
		}

		if err := core.Delete("path-1"); err != nil {
			t.Fatalf("Delete(): %v", err)
		}
		if got := mainPathRel(t, core, "path-1"); got != "" {
			t.Errorf("after delete: findMainBeanFile() = %q, want %q", got, "")
		}
	})

	t.Run("follows a slug rename", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "slug-1", "Before", "todo")

		newSlug := "after"
		plan, err := core.PlanRenameSlug("slug-1", &newSlug, false)
		if err != nil {
			t.Fatalf("PlanRenameSlug(): %v", err)
		}
		if err := core.ApplyRename(plan); err != nil {
			t.Fatalf("ApplyRename(): %v", err)
		}

		if got, want := mainPathRel(t, core, "slug-1"), "slug-1--after.md"; got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})

	t.Run("follows an ID rename", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "old-1", "Renamed", "todo")

		plan, err := core.PlanRenameID("old-1", "new-1")
		if err != nil {
			t.Fatalf("PlanRenameID(): %v", err)
		}
		if err := core.ApplyRename(plan); err != nil {
			t.Fatalf("ApplyRename(): %v", err)
		}

		if got := mainPathRel(t, core, "old-1"); got != "" {
			t.Errorf("the old ID still maps to %q, want %q", got, "")
		}
		if got, want := mainPathRel(t, core, "new-1"), "new-1--renamed.md"; got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})

	t.Run("follows LoadAndUnarchive moving the file out of the archive", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "load-1", "Reopened", "todo")
		if err := core.Archive("load-1"); err != nil {
			t.Fatalf("Archive(): %v", err)
		}

		if _, err := core.LoadAndUnarchive("load-1"); err != nil {
			t.Fatalf("LoadAndUnarchive(): %v", err)
		}

		// LoadAndUnarchive moved the file; a stale mainPaths entry pointing at
		// the now-empty .archive/ location makes findMainItemFile drop the
		// entry and answer "", which handleWorktreeChanges reads as
		// "worktree-only" and turns into a phantom EventDeleted.
		if got, want := mainPathRel(t, core, "load-1"), "load-1--reopened.md"; got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})

	t.Run("returns nothing for a bean that never existed", func(t *testing.T) {
		core, _ := setupTestCore(t)

		if got := mainPathRel(t, core, "ghost-1"); got != "" {
			t.Errorf("findMainBeanFile() = %q, want %q", got, "")
		}
	})

	t.Run("survives a worktree version taking over in memory", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "wt-path-1", "Main Title", "todo")

		wtDir := t.TempDir()
		wtItemsDir := filepath.Join(wtDir, config.DefaultStoreDir)
		if err := os.MkdirAll(wtItemsDir, 0755); err != nil {
			t.Fatalf("creating worktree beans dir: %v", err)
		}
		// Different slug in the worktree — the filename alone no longer
		// identifies the main-store file.
		content := "---\ntitle: Renamed In Worktree\nstatus: in-progress\ntype: task\n---\n"
		if err := os.WriteFile(filepath.Join(wtItemsDir, "wt-path-1--renamed-in-worktree.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing worktree bean: %v", err)
		}

		if err := core.WatchWorktreeItems(wtDir); err != nil {
			t.Fatalf("WatchWorktreeBeans(): %v", err)
		}
		defer core.UnwatchWorktreeItems(wtDir)
		time.Sleep(100 * time.Millisecond)

		got, err := core.Get("wt-path-1")
		if err != nil {
			t.Fatalf("Get(): %v", err)
		}
		if got.Title != "Renamed In Worktree" {
			t.Fatalf("Title = %q, want the worktree version to be active", got.Title)
		}

		if got, want := mainPathRel(t, core, "wt-path-1"), "wt-path-1--main-title.md"; got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})

	t.Run("picks up a file the main watcher reports", func(t *testing.T) {
		core, itemsDir := setupTestCore(t)
		if err := core.StartWatching(); err != nil {
			t.Fatalf("StartWatching(): %v", err)
		}
		defer core.Unwatch()

		events, unsub := core.Subscribe()
		defer unsub()

		content := "---\ntitle: Dropped In\nstatus: todo\ntype: task\n---\n"
		if err := os.WriteFile(filepath.Join(itemsDir, "watched-1--dropped-in.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing bean: %v", err)
		}

		select {
		case <-events:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the watcher to pick up the new file")
		}

		if got, want := mainPathRel(t, core, "watched-1"), "watched-1--dropped-in.md"; got != want {
			t.Errorf("findMainBeanFile() = %q, want %q", got, want)
		}
	})
}
