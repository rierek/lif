package itemcore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// beans-xqsq AC1/AC2: the etag `list`, `show` and the write path each expose
// for an item must be the same value, and that value must be the one
// core.Update's own optimistic-concurrency check accepts. Each of the three
// "commands" below is its own freshly reloaded Core, exactly as `lif list`,
// `lif show` and `lif update` are three separate OS processes against the
// same on-disk store -- the scenario the measured SPF-34aa/SPF-ukwd
// divergence came from, not the same in-memory Core serving all three, which
// would hide the defaulting-on-load discrepancy entirely.
func TestETagConsistentAcrossListShowAndWrite(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	cfg := config.Default()

	// "lif create": no -p/-t, so priority/type are absent on disk -- the
	// exact shape that gets defaulted in memory on every subsequent load.
	writer := New(itemsDir, cfg)
	writer.SetWarnWriter(nil)
	if err := writer.Load(); err != nil {
		t.Fatalf("writer.Load() error = %v", err)
	}
	b := &item.Item{ID: "beans-etagx", Slug: item.Slugify("Etag test"), Title: "Etag test", Status: "todo"}
	if err := writer.Create(b); err != nil {
		t.Fatalf("writer.Create() error = %v", err)
	}

	// "lif list": a fresh process, reads the whole store.
	listCore := New(itemsDir, cfg)
	listCore.SetWarnWriter(nil)
	if err := listCore.Load(); err != nil {
		t.Fatalf("listCore.Load() error = %v", err)
	}
	listItem, err := listCore.Get(b.ID)
	if err != nil {
		t.Fatalf("listCore.Get() error = %v", err)
	}
	listETag := listItem.ETag()

	// "lif show": another fresh process, reads one item.
	showCore := New(itemsDir, cfg)
	showCore.SetWarnWriter(nil)
	if err := showCore.Load(); err != nil {
		t.Fatalf("showCore.Load() error = %v", err)
	}
	showItem, err := showCore.Get(b.ID)
	if err != nil {
		t.Fatalf("showCore.Get() error = %v", err)
	}
	showETag := showItem.ETag()

	if listETag != showETag {
		t.Fatalf("list etag %q != show etag %q -- same on-disk bean must report the same etag from every read path", listETag, showETag)
	}

	// "lif update ... --if-match <list's etag>": a third fresh process.
	// The write path must accept the exact token the read paths handed out.
	updateCore := New(itemsDir, cfg)
	updateCore.SetWarnWriter(nil)
	if err := updateCore.Load(); err != nil {
		t.Fatalf("updateCore.Load() error = %v", err)
	}
	updateItem, err := updateCore.Get(b.ID)
	if err != nil {
		t.Fatalf("updateCore.Get() error = %v", err)
	}
	updateItem.Priority = "high"
	if err := updateCore.Update(updateItem, &listETag); err != nil {
		t.Fatalf("Update() with the list/show etag as --if-match was rejected: %v (an etag a caller can only read after a write has already failed is not a usable lock)", err)
	}
}

// beans-xqsq AC3: the fix must not turn --if-match into a no-op. A caller
// that reads an etag, then loses a race to a genuinely concurrent write
// before its own write lands, must have that write rejected.
func TestETagIfMatchRejectsGenuineConcurrentModification(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	cfg := config.Default()

	writer := New(itemsDir, cfg)
	writer.SetWarnWriter(nil)
	if err := writer.Load(); err != nil {
		t.Fatalf("writer.Load() error = %v", err)
	}
	b := &item.Item{ID: "beans-etagy", Slug: item.Slugify("Etag race"), Title: "Etag race", Status: "todo"}
	if err := writer.Create(b); err != nil {
		t.Fatalf("writer.Create() error = %v", err)
	}

	// Caller A reads the item and its etag (its own fresh process).
	callerA := New(itemsDir, cfg)
	callerA.SetWarnWriter(nil)
	if err := callerA.Load(); err != nil {
		t.Fatalf("callerA.Load() error = %v", err)
	}
	itemA, err := callerA.Get(b.ID)
	if err != nil {
		t.Fatalf("callerA.Get() error = %v", err)
	}
	staleETag := itemA.ETag()

	// Caller B (a genuinely concurrent process) writes first.
	callerB := New(itemsDir, cfg)
	callerB.SetWarnWriter(nil)
	if err := callerB.Load(); err != nil {
		t.Fatalf("callerB.Load() error = %v", err)
	}
	itemB, err := callerB.Get(b.ID)
	if err != nil {
		t.Fatalf("callerB.Get() error = %v", err)
	}
	itemB.Priority = "critical"
	if err := callerB.Update(itemB, nil); err != nil {
		t.Fatalf("callerB.Update() error = %v", err)
	}

	// Caller A's write, using the etag it read before B's write landed,
	// must now be rejected -- not silently accepted.
	itemA.Priority = "low"
	err = callerA.Update(itemA, &staleETag)
	if err == nil {
		t.Fatal("expected an etag-mismatch error for a write racing a genuine concurrent modification, got nil")
	}
	var mismatchErr *ETagMismatchError
	if !errors.As(err, &mismatchErr) {
		t.Fatalf("expected *ETagMismatchError, got %T: %v", err, err)
	}
}

// TestUpdateWithIfMatchAndStatusPolicy covers the two on-disk checks Update
// runs together: the etag it validates If-Match against, and the previous
// status the field policy is measured from. Both read the same file, and they
// have to keep agreeing after that read was consolidated.
func TestUpdateWithIfMatchAndStatusPolicy(t *testing.T) {
	// Each subtest gets its own core and item: they all write to the same item,
	// so sharing one would make the outcome depend on which of them ran — and
	// on which of them happened to fail before writing.
	gated := func(t *testing.T) (*Core, *item.Item, string) {
		t.Helper()
		core, _ := setupTestCoreWithRequireFieldsOn(t, map[string][]string{"completed": {"commit"}})
		b := createTestItem(t, core, "test-etpol", "Gated bean", "todo")
		return core, b, b.ETag()
	}

	t.Run("policy blocks the transition even with a matching etag", func(t *testing.T) {
		core, b, etag := gated(t)
		b.Status = "completed"
		err := core.Update(b, &etag)

		var polErr *PolicyViolationError
		if !errors.As(err, &polErr) {
			t.Fatalf("Update() error = %v, want *PolicyViolationError", err)
		}
	})

	t.Run("a stale etag is rejected before the policy runs", func(t *testing.T) {
		core, b, _ := gated(t)
		stale := "0123456789abcdef"
		b.Status = "completed"
		err := core.Update(b, &stale)

		var mismatch *ETagMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("Update() error = %v, want *ETagMismatchError", err)
		}
	})

	t.Run("the transition goes through once the field is there", func(t *testing.T) {
		core, b, etag := gated(t)
		b.Status = "completed"
		b.Extra = map[string]any{"commit": "abc1234"}
		if err := core.Update(b, &etag); err != nil {
			t.Fatalf("Update() error = %v, want nil", err)
		}

		got, err := core.Get("test-etpol")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Status != "completed" {
			t.Errorf("Status = %q, want %q", got.Status, "completed")
		}
	})
}

// TestUpdateIfMatchUnderWorktreePath pins where the If-Match etag comes from
// when the write is routed to a worktree: the main store, not the in-memory
// item. A worktree write does not make the main-store file irrelevant — it is
// still the copy a later merge or a main-store writer works against, so a
// change behind the watcher's back has to be caught here as it is on the
// ordinary path.
func TestUpdateIfMatchUnderWorktreePath(t *testing.T) {
	core, itemsDir := setupTestCore(t)
	b := createTestItem(t, core, "test-wtetag", "Worktree gated", "todo")
	staleETag := b.ETag()

	// Someone replaced the main-store file behind the watcher's back.
	mainFile := filepath.Join(itemsDir, b.Path)
	replaced := "---\nid: test-wtetag\ntitle: Worktree gated\nstatus: in-progress\ntype: task\n---\n\nchanged\n"
	if err := os.WriteFile(mainFile, []byte(replaced), 0644); err != nil {
		t.Fatalf("replacing the main-store file: %v", err)
	}
	onDisk, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatalf("reading back the main-store file: %v", err)
	}
	freshETag := item.ETagOf(onDisk)
	if freshETag == staleETag {
		t.Fatal("the replacement did not change the file's etag — the test proves nothing")
	}

	wtDir := t.TempDir()

	t.Run("a stale etag is rejected", func(t *testing.T) {
		b.Priority = "low"
		err := core.Update(b, &staleETag, WithWorktreePath(wtDir))

		var mismatch *ETagMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("Update() error = %v, want *ETagMismatchError", err)
		}
		if mismatch.Current != freshETag {
			t.Errorf("Current = %q, want the main-store file's etag %q", mismatch.Current, freshETag)
		}
	})

	t.Run("the main-store etag is accepted", func(t *testing.T) {
		b.Priority = "high"
		if err := core.Update(b, &freshETag, WithWorktreePath(wtDir)); err != nil {
			t.Fatalf("Update() error = %v, want nil", err)
		}
		if _, err := os.Stat(filepath.Join(wtDir, config.DefaultStoreDir, item.BuildFilename(b.ID, b.Slug))); err != nil {
			t.Fatalf("the bean was not written to the worktree: %v", err)
		}
	})
}
