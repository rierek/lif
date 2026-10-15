package itemcore

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// AC3: the error and status texts the core returns name the noun as item and
// the product as lif. Each subtest drives the production function.
func TestUserFacingTextsNameItems(t *testing.T) {
	t.Run("ErrNotFound", func(t *testing.T) {
		if got := ErrNotFound.Error(); got != "item not found" {
			t.Errorf("ErrNotFound = %q, want %q", got, "item not found")
		}
	})

	t.Run("policy violation quotes lif complete", func(t *testing.T) {
		err := &PolicyViolationError{ItemID: "x-1", Status: "completed", Missing: []string{"commit"}}
		if got := err.Error(); !strings.Contains(got, "`lif complete x-1 --commit HEAD`") {
			t.Errorf("Error() = %q, want it to quote `lif complete x-1 --commit HEAD`", got)
		}
	})

	t.Run("parent not found", func(t *testing.T) {
		c, _ := setupTestCore(t)
		err := c.ValidateParent(&item.Item{ID: "x-1", Type: "task"}, "nope")
		if err == nil || err.Error() != "parent item not found: nope" {
			t.Errorf("ValidateParent() error = %v, want %q", err, "parent item not found: nope")
		}
	})

	t.Run("parent of a type that takes none", func(t *testing.T) {
		c, _ := setupTestCore(t)
		task := &item.Item{ID: "task-1", Slug: "t", Title: "T", Status: "todo", Type: "task"}
		if err := c.Create(task); err != nil {
			t.Fatal(err)
		}
		err := c.ValidateParent(&item.Item{ID: "x-1", Type: "milestone"}, "task-1")
		if want := "milestone items cannot have a parent"; err == nil || err.Error() != want {
			t.Errorf("ValidateParent() error = %v, want %q", err, want)
		}
	})

	t.Run("parent of the wrong type", func(t *testing.T) {
		c, _ := setupTestCore(t)
		task := &item.Item{ID: "task-1", Slug: "t", Title: "T", Status: "todo", Type: "task"}
		if err := c.Create(task); err != nil {
			t.Fatal(err)
		}
		err := c.ValidateParent(&item.Item{ID: "x-1", Type: "epic"}, "task-1")
		if want := "epic items can only have milestone as parent, not task"; err == nil || err.Error() != want {
			t.Errorf("ValidateParent() error = %v, want %q", err, want)
		}
	})

	t.Run("rename of an unknown ID", func(t *testing.T) {
		c := newTestCore(t, "t-", nil)
		_, err := c.PlanRenameID("t-nope", "t-other")
		if err == nil || err.Error() != `item "t-nope" not found` {
			t.Errorf("PlanRenameID() error = %v, want %q", err, `item "t-nope" not found`)
		}
	})

	t.Run("rebrand without a matching item", func(t *testing.T) {
		c := newTestCore(t, "t-", nil)
		_, err := c.PlanRebrand("u-")
		if err == nil || !strings.Contains(err.Error(), "no items matched prefix") {
			t.Errorf("PlanRebrand() error = %v, want it to contain %q", err, "no items matched prefix")
		}
	})

	t.Run("rebrand with a foreign-prefixed item", func(t *testing.T) {
		c := newTestCore(t, "t-", map[string]string{
			"t-aaaa--a.md": "---\n# t-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
			"z-bbbb--b.md": "---\n# z-bbbb\ntitle: B\nstatus: todo\ntype: task\n---\n",
		})
		want := `item "z-bbbb" does not start with the current prefix "t-"; refusing rebrand to avoid ID corruption`
		if _, err := c.PlanRebrand("u-"); err == nil || err.Error() != want {
			t.Errorf("PlanRebrand() error = %v, want %q", err, want)
		}
	})

	t.Run("slug rename of an unknown ID", func(t *testing.T) {
		c := newTestCore(t, "t-", nil)
		slug := "x"
		if _, err := c.PlanRenameSlug("t-nope", &slug, false); err == nil || err.Error() != `item "t-nope" not found` {
			t.Errorf("PlanRenameSlug() error = %v, want %q", err, `item "t-nope" not found`)
		}
	})

	t.Run("rebrand while the server runs", func(t *testing.T) {
		c := newTestCore(t, "t-", map[string]string{
			"t-aaaa--a.md": "---\n# t-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
		})
		port := c.Config().GetServerPort()
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Skipf("cannot bind configured port %d: %v", port, err)
		}
		defer ln.Close()

		want := fmt.Sprintf("lif-serve appears to be running on port %d; stop it before a prefix rebrand", port)
		if _, err := c.PlanRebrand("u-"); err == nil || err.Error() != want {
			t.Errorf("PlanRebrand() error = %v, want %q", err, want)
		}
	})

	t.Run("worktree watcher log lines", func(t *testing.T) {
		c, _ := setupTestCore(t)
		var log bytes.Buffer
		c.SetWarnWriter(&log)
		wt := t.TempDir()
		if err := os.MkdirAll(c.worktreeItemsDir(wt), 0755); err != nil {
			t.Fatal(err)
		}

		if err := c.WatchWorktreeItems(wt); err != nil {
			t.Fatal(err)
		}
		c.UnwatchWorktreeItems(wt)

		for _, want := range []string{"warning: watching worktree items: ", "warning: stopped watching worktree items: "} {
			if !strings.Contains(log.String(), want) {
				t.Errorf("log = %q, want it to contain %q", log.String(), want)
			}
		}
	})

	t.Run("prefix consistency", func(t *testing.T) {
		c := New(t.TempDir(), &config.Config{Items: config.ItemsConfig{Prefix: "a-"}})
		c.items["a-1"] = &item.Item{ID: "a-1"}
		c.items["b-1"] = &item.Item{ID: "b-1"}
		if got := c.ValidatePrefixConsistency(); !strings.Contains(got, "items have prefixes") {
			t.Errorf("mixed-prefix message = %q, want it to say %q", got, "items have prefixes")
		}

		c = New(t.TempDir(), &config.Config{Items: config.ItemsConfig{Prefix: "a-"}})
		c.items["b-1"] = &item.Item{ID: "b-1"}
		if got := c.ValidatePrefixConsistency(); !strings.Contains(got, "items have prefix ") {
			t.Errorf("prefix-mismatch message = %q, want it to say %q", got, "items have prefix ")
		}
	})
}

// logBuffer collects warnings written by a watcher goroutine.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// denyDir makes dir unwritable for the rest of the test; a process running as
// root ignores permission bits, so there is nothing to observe then.
func denyDir(t *testing.T, dir string, mode os.FileMode) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission bits do not bind root")
	}
	if err := os.Chmod(dir, mode); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0755) })
}

// AC3: the errors the core returns when the store cannot be written name the
// noun as item.
func TestStoreWriteFailuresNameItems(t *testing.T) {
	t.Run("archive", func(t *testing.T) {
		c, root := setupTestCore(t)
		createTestItem(t, c, "x-1", "One", "completed")
		archive := filepath.Join(root, ArchiveDir)
		if err := os.MkdirAll(archive, 0755); err != nil {
			t.Fatal(err)
		}
		denyDir(t, archive, 0555)

		if err := c.Archive("x-1"); err == nil || !strings.HasPrefix(err.Error(), "moving item to archive: ") {
			t.Errorf("Archive() error = %v, want it to start with %q", err, "moving item to archive: ")
		}
	})

	t.Run("unarchive", func(t *testing.T) {
		c, root := setupTestCore(t)
		createTestItem(t, c, "x-1", "One", "completed")
		if err := c.Archive("x-1"); err != nil {
			t.Fatal(err)
		}
		denyDir(t, root, 0555)

		if err := c.Unarchive("x-1"); err == nil || !strings.HasPrefix(err.Error(), "moving item from archive: ") {
			t.Errorf("Unarchive() error = %v, want it to start with %q", err, "moving item from archive: ")
		}
	})

	t.Run("load and unarchive", func(t *testing.T) {
		c, root := setupTestCore(t)
		createTestItem(t, c, "x-1", "One", "completed")
		if err := c.Archive("x-1"); err != nil {
			t.Fatal(err)
		}
		denyDir(t, root, 0555)

		if _, err := c.LoadAndUnarchive("x-1"); err == nil || !strings.HasPrefix(err.Error(), "moving item from archive: ") {
			t.Errorf("LoadAndUnarchive() error = %v, want it to start with %q", err, "moving item from archive: ")
		}
	})

	t.Run("save a dirty item", func(t *testing.T) {
		for name, save := range map[string]func(c *Core) error{
			"SaveItem":  func(c *Core) error { return c.SaveItem("x-1") },
			"SaveDirty": func(c *Core) error { _, err := c.SaveDirty(); return err },
		} {
			c, root := setupTestCore(t)
			b := createTestItem(t, c, "x-1", "One", "todo")
			c.dirty[b.ID] = true
			denyDir(t, filepath.Join(root, b.Path), 0444)

			if err := save(c); err == nil || !strings.HasPrefix(err.Error(), "saving item x-1: ") {
				t.Errorf("%s() error = %v, want it to start with %q", name, err, "saving item x-1: ")
			}
		}
	})

	t.Run("write into a worktree", func(t *testing.T) {
		c, _ := setupTestCore(t)
		b := &item.Item{ID: "x-1", Slug: "one", Title: "One", Status: "todo", Type: "task"}

		// The worktree path sits below a regular file, so its items directory
		// cannot be created.
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, 0644); err != nil {
			t.Fatal(err)
		}
		if err := c.saveToWorktree(b, filepath.Join(blocker, "wt")); err == nil || !strings.HasPrefix(err.Error(), "creating worktree items dir: ") {
			t.Errorf("saveToWorktree() error = %v, want it to start with %q", err, "creating worktree items dir: ")
		}

		wt := t.TempDir()
		dir := c.worktreeItemsDir(wt)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		denyDir(t, dir, 0555)
		if err := c.saveToWorktree(b, wt); err == nil || !strings.HasPrefix(err.Error(), "writing worktree item: ") {
			t.Errorf("saveToWorktree() error = %v, want it to start with %q", err, "writing worktree item: ")
		}
	})
}

// AC3: a malformed item file noticed by the file watchers is logged as an
// item, for the main store and for a worktree.
func TestWatcherLogsAMalformedItemAsAnItem(t *testing.T) {
	const malformed = "---\ntitle: [unclosed\n---\n"
	waitFor := func(t *testing.T, log *logBuffer, want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for !strings.Contains(log.String(), want) {
			if time.Now().After(deadline) {
				t.Fatalf("log lacks %q after 5s:\n%s", want, log.String())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	t.Run("main store", func(t *testing.T) {
		c, root := setupTestCore(t)
		var log logBuffer
		c.SetWarnWriter(&log)
		if err := c.StartWatching(); err != nil {
			t.Fatal(err)
		}
		defer c.Unwatch()

		bad := filepath.Join(root, "bad-1--broken.md")
		if err := os.WriteFile(bad, []byte(malformed), 0644); err != nil {
			t.Fatal(err)
		}
		waitFor(t, &log, "warning: failed to load item from "+bad+": ")
	})

	t.Run("worktree", func(t *testing.T) {
		c, _ := setupTestCore(t)
		var log logBuffer
		c.SetWarnWriter(&log)
		wt := t.TempDir()
		dir := c.worktreeItemsDir(wt)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := c.WatchWorktreeItems(wt); err != nil {
			t.Fatal(err)
		}
		defer c.UnwatchWorktreeItems(wt)

		bad := filepath.Join(dir, "bad-1--broken.md")
		if err := os.WriteFile(bad, []byte(malformed), 0644); err != nil {
			t.Fatal(err)
		}
		waitFor(t, &log, "warning: failed to load worktree item from "+bad+": ")
	})
}
