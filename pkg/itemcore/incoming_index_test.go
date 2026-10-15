package itemcore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// incomingFingerprint renders the incoming links of id as a comparable value.
func incomingFingerprint(links []IncomingLink) []string {
	out := make([]string, 0, len(links))
	for _, l := range links {
		out = append(out, fmt.Sprintf("%s/%s", l.FromItem.ID, l.LinkType))
	}
	sort.Strings(out)
	return out
}

// assertIncomingConsistent compares what FindIncomingLinks answers from its
// index against what the same production code computes from a forced rebuild.
// A mutation that forgets to invalidate the index shows up here.
func assertIncomingConsistent(t *testing.T, c *Core, ids ...string) {
	t.Helper()
	for _, id := range ids {
		cached := incomingFingerprint(c.FindIncomingLinks(id))

		c.mu.Lock()
		c.incomingValid = false
		c.mu.Unlock()
		fresh := incomingFingerprint(c.FindIncomingLinks(id))

		if len(cached) != len(fresh) {
			t.Fatalf("incoming links for %s: index has %v, rebuild has %v", id, cached, fresh)
		}
		for i := range fresh {
			if cached[i] != fresh[i] {
				t.Fatalf("incoming links for %s: index has %v, rebuild has %v", id, cached, fresh)
			}
		}
	}
}

// warmIncoming builds the index so a stale entry can survive the next mutation.
func warmIncoming(c *Core, ids ...string) {
	for _, id := range ids {
		c.FindIncomingLinks(id)
	}
}

func TestIncomingIndexStaysConsistent(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		warmIncoming(core, "target")

		child := &item.Item{ID: "child", Slug: "child", Title: "Child", Status: "todo", Parent: "target"}
		if err := core.Create(child); err != nil {
			t.Fatalf("Create(): %v", err)
		}

		if got := core.FindIncomingLinks("target"); len(got) != 1 {
			t.Errorf("FindIncomingLinks(target) = %d links, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("update adds and removes links", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		linker := createTestItem(t, core, "linker", "Linker", "todo")
		warmIncoming(core, "target")

		linker.Blocking = []string{"target"}
		if err := core.Update(linker, nil); err != nil {
			t.Fatalf("Update(): %v", err)
		}
		if got := core.FindIncomingLinks("target"); len(got) != 1 {
			t.Errorf("after adding a link: %d incoming, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target")

		linker.Blocking = nil
		if err := core.Update(linker, nil); err != nil {
			t.Fatalf("Update(): %v", err)
		}
		if got := core.FindIncomingLinks("target"); len(got) != 0 {
			t.Errorf("after removing the link: %d incoming, want 0", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("delete", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		linker := &item.Item{ID: "linker", Slug: "linker", Title: "Linker", Status: "todo", BlockedBy: []string{"target"}}
		if err := core.Create(linker); err != nil {
			t.Fatalf("Create(): %v", err)
		}
		warmIncoming(core, "target")

		if err := core.Delete("linker"); err != nil {
			t.Fatalf("Delete(): %v", err)
		}
		if got := core.FindIncomingLinks("target"); len(got) != 0 {
			t.Errorf("after deleting the linker: %d incoming, want 0", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("archive and unarchive", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		linker := &item.Item{ID: "linker", Slug: "linker", Title: "Linker", Status: "todo", Parent: "target"}
		if err := core.Create(linker); err != nil {
			t.Fatalf("Create(): %v", err)
		}
		warmIncoming(core, "target")

		if err := core.Archive("linker"); err != nil {
			t.Fatalf("Archive(): %v", err)
		}
		assertIncomingConsistent(t, core, "target")

		if err := core.Unarchive("linker"); err != nil {
			t.Fatalf("Unarchive(): %v", err)
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("remove links to", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		linker := &item.Item{ID: "linker", Slug: "linker", Title: "Linker", Status: "todo", Blocking: []string{"target"}}
		if err := core.Create(linker); err != nil {
			t.Fatalf("Create(): %v", err)
		}
		warmIncoming(core, "target")

		if _, err := core.RemoveLinksTo("target"); err != nil {
			t.Fatalf("RemoveLinksTo(): %v", err)
		}
		if got := core.FindIncomingLinks("target"); len(got) != 0 {
			t.Errorf("after RemoveLinksTo: %d incoming, want 0", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("fix broken links", func(t *testing.T) {
		core, _ := setupTestCore(t)
		linker := &item.Item{ID: "linker", Slug: "linker", Title: "Linker", Status: "todo", Blocking: []string{"ghost"}}
		if err := core.Create(linker); err != nil {
			t.Fatalf("Create(): %v", err)
		}
		warmIncoming(core, "ghost")

		if _, err := core.FixBrokenLinks(); err != nil {
			t.Fatalf("FixBrokenLinks(): %v", err)
		}
		if got := core.FindIncomingLinks("ghost"); len(got) != 0 {
			t.Errorf("after FixBrokenLinks: %d incoming, want 0", len(got))
		}
		assertIncomingConsistent(t, core, "ghost")
	})

	t.Run("id rename rewrites refs", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		linker := &item.Item{ID: "linker", Slug: "linker", Title: "Linker", Status: "todo", Parent: "target"}
		if err := core.Create(linker); err != nil {
			t.Fatalf("Create(): %v", err)
		}
		warmIncoming(core, "target", "renamed")

		plan, err := core.PlanRenameID("target", "renamed")
		if err != nil {
			t.Fatalf("PlanRenameID(): %v", err)
		}
		if err := core.ApplyRename(plan); err != nil {
			t.Fatalf("ApplyRename(): %v", err)
		}

		if got := core.FindIncomingLinks("renamed"); len(got) != 1 {
			t.Errorf("FindIncomingLinks(renamed) = %d links, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target", "renamed")
	})

	t.Run("reload from disk", func(t *testing.T) {
		core, itemsDir := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		warmIncoming(core, "target")

		content := "---\ntitle: Outside\nstatus: todo\ntype: task\nparent: target\n---\n"
		if err := os.WriteFile(filepath.Join(itemsDir, "outside--outside.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing bean: %v", err)
		}
		if err := core.Load(); err != nil {
			t.Fatalf("Load(): %v", err)
		}

		if got := core.FindIncomingLinks("target"); len(got) != 1 {
			t.Errorf("after reload: %d incoming, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("main watcher change", func(t *testing.T) {
		core, itemsDir := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		warmIncoming(core, "target")

		if err := core.StartWatching(); err != nil {
			t.Fatalf("StartWatching(): %v", err)
		}
		defer core.Unwatch()
		events, unsub := core.Subscribe()
		defer unsub()

		content := "---\ntitle: Watched\nstatus: todo\ntype: task\nparent: target\n---\n"
		if err := os.WriteFile(filepath.Join(itemsDir, "watched--watched.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing bean: %v", err)
		}
		select {
		case <-events:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the watcher")
		}

		if got := core.FindIncomingLinks("target"); len(got) != 1 {
			t.Errorf("after a watched write: %d incoming, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})

	t.Run("worktree overlay", func(t *testing.T) {
		core, _ := setupTestCore(t)
		createTestItem(t, core, "target", "Target", "todo")
		createTestItem(t, core, "linker", "Linker", "todo")
		warmIncoming(core, "target")

		wtDir := t.TempDir()
		wtItemsDir := filepath.Join(wtDir, config.DefaultStoreDir)
		if err := os.MkdirAll(wtItemsDir, 0755); err != nil {
			t.Fatalf("creating worktree beans dir: %v", err)
		}
		content := "---\ntitle: Linker\nstatus: in-progress\ntype: task\nparent: target\n---\n"
		if err := os.WriteFile(filepath.Join(wtItemsDir, "linker--linker.md"), []byte(content), 0644); err != nil {
			t.Fatalf("writing worktree bean: %v", err)
		}

		if err := core.WatchWorktreeItems(wtDir); err != nil {
			t.Fatalf("WatchWorktreeBeans(): %v", err)
		}
		defer core.UnwatchWorktreeItems(wtDir)
		time.Sleep(150 * time.Millisecond)

		if got := core.FindIncomingLinks("target"); len(got) != 1 {
			t.Errorf("with the worktree version active: %d incoming, want 1", len(got))
		}
		assertIncomingConsistent(t, core, "target")
	})
}

// BenchmarkFindIncomingLinks walks every item in a store and asks for its
// incoming links — the shape the children/blocking/blocked-by resolvers
// produce when a client loads a board.
func BenchmarkFindIncomingLinks(b *testing.B) {
	core, _ := setupTestCore(&testing.T{})
	const itemCount = 500
	for i := range itemCount {
		child := &item.Item{
			ID:     fmt.Sprintf("bench-%d", i),
			Slug:   fmt.Sprintf("bench-%d", i),
			Title:  fmt.Sprintf("Bench %d", i),
			Status: "todo",
		}
		if i > 0 {
			child.Parent = "bench-0"
		}
		if err := core.Create(child); err != nil {
			b.Fatalf("Create(): %v", err)
		}
	}

	b.ResetTimer()
	for b.Loop() {
		for i := range itemCount {
			core.FindIncomingLinks(fmt.Sprintf("bench-%d", i))
		}
	}
}
