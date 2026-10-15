package itemcore

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// newTestCore builds a Core over a temp .beans dir with one item file.
func newTestCore(t *testing.T, prefix string, files map[string]string) *Core {
	t.Helper()
	repo := t.TempDir()
	itemsDir := filepath.Join(repo, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		full := filepath.Join(itemsDir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.DefaultWithPrefix(prefix)
	cfg.SetConfigDir(repo) // ensure Save/ConfigDir target repo root
	c := New(itemsDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestApplyRenameSlug_setsSlug(t *testing.T) {
	// I02 (T05-Review prelude): correct fixture form is `---\n# id\n<yaml>\n---`
	// (the "# id" comment INSIDE the frontmatter block). The previous "# id"
	// -before-"---" form parses to zero-value fields silently; assert on a
	// parsed field (Title) below to prove this fixture actually parses.
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--old-slug.md": "---\n# tp-aaaa\ntitle: Test Bean\nstatus: todo\ntype: task\n---\nBody.\n",
	})
	b, err := c.Get("tp-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Test Bean" {
		t.Fatalf("fixture did not parse: Title = %q, want %q", b.Title, "Test Bean")
	}
	plan, err := c.PlanRenameSlug("tp-aaaa", strPtr("new-slug"), false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "slug" || len(plan.Changes) != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Changes[0].NewPath != "tp-aaaa--new-slug.md" { // .beans-relative
		t.Errorf("NewPath = %q", plan.Changes[0].NewPath)
	}
	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}
	// old gone, new present
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--old-slug.md")); !os.IsNotExist(err) {
		t.Error("old file still present")
	}
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--new-slug.md")); err != nil {
		t.Errorf("new file missing: %v", err)
	}
}

func TestApplyRenameSlug_idAndRefsUnchanged(t *testing.T) {
	// I02 (T05-Review prelude): corrected fixture form, see comment above.
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--parent.md": "---\n# tp-aaaa\ntitle: Parent\nstatus: todo\ntype: epic\n---\n",
		"tp-bbbb--child.md":  "---\n# tp-bbbb\ntitle: Child\nstatus: todo\ntype: task\nparent: tp-aaaa\n---\n",
	})
	plan, err := c.PlanRenameSlug("tp-aaaa", strPtr("renamed"), false)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}
	// ID unchanged, in-memory Path/Slug updated
	b, err := c.Get("tp-aaaa")
	if err != nil {
		t.Fatalf("bean still resolvable under same ID: %v", err)
	}
	if b.Slug != "renamed" {
		t.Errorf("Slug = %q, want renamed", b.Slug)
	}
	if b.Path != "tp-aaaa--renamed.md" {
		t.Errorf("Path = %q, want tp-aaaa--renamed.md", b.Path)
	}
	// child's ref to parent is untouched (still references tp-aaaa, the ID)
	content, err := os.ReadFile(filepath.Join(c.Root(), "tp-bbbb--child.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "parent: tp-aaaa") {
		t.Errorf("child ref rewritten unexpectedly: %s", content)
	}
	// on-disk id comment for the renamed item is unchanged
	renamedContent, err := os.ReadFile(filepath.Join(c.Root(), "tp-aaaa--renamed.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(renamedContent), "# tp-aaaa") {
		t.Errorf("id comment changed unexpectedly: %s", renamedContent)
	}
}

func TestPlanRenameSlug_noopWhenPathsIdentical(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--same.md": "# tp-aaaa\n---\ntitle: Test Bean\nstatus: todo\ntype: task\n---\n",
	})
	plan, err := c.PlanRenameSlug("tp-aaaa", strPtr("same"), false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Changes[0].OldPath != plan.Changes[0].NewPath {
		t.Fatalf("expected identical paths, got %q vs %q", plan.Changes[0].OldPath, plan.Changes[0].NewPath)
	}
	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--same.md")); err != nil {
		t.Errorf("file should still be present: %v", err)
	}
}

func TestRewriteRefs(t *testing.T) {
	tests := []struct {
		name          string
		parent        string
		blocking      []string
		blockedBy     []string
		m             map[string]string
		wantN         int
		wantParent    string
		wantBlocking  []string
		wantBlockedBy []string
	}{
		{
			name:          "mixed map: parent + one blocking entry changed, rest untouched",
			parent:        "old-1",
			blocking:      []string{"old-1", "keep-9"},
			blockedBy:     []string{"other-5"},
			m:             map[string]string{"old-1": "new-1"},
			wantN:         2,
			wantParent:    "new-1",
			wantBlocking:  []string{"new-1", "keep-9"},
			wantBlockedBy: []string{"other-5"},
		},
		{
			name:          "ref not in map left unchanged",
			parent:        "untouched-1",
			blocking:      nil,
			blockedBy:     []string{"untouched-2"},
			m:             map[string]string{"old-1": "new-1"},
			wantN:         0,
			wantParent:    "untouched-1",
			wantBlocking:  nil,
			wantBlockedBy: []string{"untouched-2"},
		},
		{
			name:          "empty parent never matched",
			parent:        "",
			blocking:      []string{"old-1"},
			blockedBy:     nil,
			m:             map[string]string{"": "should-not-apply", "old-1": "new-1"},
			wantN:         1,
			wantParent:    "",
			wantBlocking:  []string{"new-1"},
			wantBlockedBy: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &item.Item{
				ID:        "x-2",
				Parent:    tt.parent,
				Blocking:  append([]string(nil), tt.blocking...),
				BlockedBy: append([]string(nil), tt.blockedBy...),
			}
			n := rewriteRefs(b, tt.m)
			if n != tt.wantN {
				t.Errorf("changed count = %d, want %d", n, tt.wantN)
			}
			if b.Parent != tt.wantParent {
				t.Errorf("Parent = %q, want %q", b.Parent, tt.wantParent)
			}
			if !equalStrSlices(b.Blocking, tt.wantBlocking) {
				t.Errorf("Blocking = %v, want %v", b.Blocking, tt.wantBlocking)
			}
			if !equalStrSlices(b.BlockedBy, tt.wantBlockedBy) {
				t.Errorf("BlockedBy = %v, want %v", b.BlockedBy, tt.wantBlockedBy)
			}
		})
	}
}

func TestStageAndSwap_atomicOnPreSwapFailure(t *testing.T) {
	tests := []struct {
		name    string
		writes  map[string][]byte
		removes []string
	}{
		{
			name: "write path with NUL byte fails MkdirAll during staging",
			writes: map[string][]byte{
				filepath.Join("\x00bad", "x.md"): []byte("nope"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCore(t, "tp-", map[string]string{
				"tp-aaaa--a.md": "content-a",
				"tp-bbbb--b.md": "content-b",
			})
			err := c.stageAndSwap(tt.writes, tt.removes)
			if err == nil {
				t.Fatal("expected staging failure, got nil")
			}
			// original tree intact, byte-for-byte
			got, readErr := os.ReadFile(filepath.Join(c.Root(), "tp-aaaa--a.md"))
			if readErr != nil || string(got) != "content-a" {
				t.Errorf("original file corrupted: content=%q err=%v", got, readErr)
			}
			got2, readErr2 := os.ReadFile(filepath.Join(c.Root(), "tp-bbbb--b.md"))
			if readErr2 != nil || string(got2) != "content-b" {
				t.Errorf("original file corrupted: content=%q err=%v", got2, readErr2)
			}
			// no staging/backup siblings left behind
			entries, err := os.ReadDir(c.repoRoot())
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				name := e.Name()
				if name != ".beans" && strings.HasPrefix(name, ".beans") {
					t.Errorf("leftover sibling: %s", name)
				}
			}
		})
	}
}

func TestStageAndSwap_appliesWritesAndRemoves(t *testing.T) {
	tests := []struct {
		name          string
		writes        map[string][]byte
		removes       []string
		wantPresent   map[string]string // .beans-relative path -> expected content
		wantAbsent    []string
		wantUntouched map[string]string
	}{
		{
			name: "write applied, remove applied, untouched file survives",
			writes: map[string][]byte{
				"tp-cccc--c.md": []byte("content-c"),
			},
			removes:       []string{"tp-bbbb--b.md"},
			wantPresent:   map[string]string{"tp-cccc--c.md": "content-c"},
			wantAbsent:    []string{"tp-bbbb--b.md"},
			wantUntouched: map[string]string{"tp-aaaa--a.md": "content-a"},
		},
		{
			name:          "no writes/removes leaves tree unchanged",
			writes:        nil,
			removes:       nil,
			wantUntouched: map[string]string{"tp-aaaa--a.md": "content-a", "tp-bbbb--b.md": "content-b"},
		},
		{
			name: "non-bean file (e.g. archive/) survives the swap",
			writes: map[string][]byte{
				"tp-cccc--c.md": []byte("content-c"),
			},
			wantUntouched: map[string]string{
				"tp-aaaa--a.md":       "content-a",
				"archive/old-note.md": "kept",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCore(t, "tp-", map[string]string{
				"tp-aaaa--a.md": "content-a",
				"tp-bbbb--b.md": "content-b",
			})
			if _, ok := tt.wantUntouched["archive/old-note.md"]; ok {
				archiveDir := filepath.Join(c.Root(), "archive")
				if err := os.MkdirAll(archiveDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(archiveDir, "old-note.md"), []byte("kept"), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if err := c.stageAndSwap(tt.writes, tt.removes); err != nil {
				t.Fatal(err)
			}
			for relPath := range tt.wantPresent {
				wantContent := tt.wantPresent[relPath]
				got, err := os.ReadFile(filepath.Join(c.Root(), relPath))
				if err != nil {
					t.Errorf("write not applied at %q: %v", relPath, err)
					continue
				}
				if string(got) != wantContent {
					t.Errorf("write %q = %q, want %q", relPath, got, wantContent)
				}
			}
			for _, relPath := range tt.wantAbsent {
				if _, err := os.Stat(filepath.Join(c.Root(), relPath)); !os.IsNotExist(err) {
					t.Errorf("removed file %q still present", relPath)
				}
			}
			for relPath, wantContent := range tt.wantUntouched {
				got, err := os.ReadFile(filepath.Join(c.Root(), relPath))
				if err != nil {
					t.Errorf("untouched file %q missing: %v", relPath, err)
					continue
				}
				if string(got) != wantContent {
					t.Errorf("untouched file %q changed: %q, want %q", relPath, got, wantContent)
				}
			}
			// no staging/backup siblings left behind after success
			entries, err := os.ReadDir(c.repoRoot())
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				name := e.Name()
				if name != ".beans" && strings.HasPrefix(name, ".beans") {
					t.Errorf("leftover sibling: %s", name)
				}
			}
		})
	}
}

// TestNewItemPath proves subdir preservation for newItemPath (Prelude I01
// from T02-Review): nested Item.Path values (e.g. "epic-auth/id--slug.md")
// must keep their directory across a rename, not just flat files.
func TestNewItemPath(t *testing.T) {
	tests := []struct {
		name           string
		oldPath        string
		newID, newSlug string
		want           string
	}{
		{
			name: "flat file with slug", oldPath: "tp-aaaa--slug.md",
			newID: "tp-zzzz", newSlug: "slug", want: "tp-zzzz--slug.md",
		},
		{
			name: "flat file without slug", oldPath: "tp-aaaa.md",
			newID: "tp-zzzz", newSlug: "", want: "tp-zzzz--.md",
		},
		{
			name: "nested subdir preserved", oldPath: "epic-auth/tp-aaaa--slug.md",
			newID: "tp-zzzz", newSlug: "slug", want: filepath.Join("epic-auth", "tp-zzzz--slug.md"),
		},
		{
			name: "deeply nested subdir preserved", oldPath: "epic-auth/sub/tp-aaaa--slug.md",
			newID: "tp-zzzz", newSlug: "newslug", want: filepath.Join("epic-auth", "sub", "tp-zzzz--newslug.md"),
		},
		{
			name: "nested subdir, slug cleared", oldPath: "epic-auth/tp-aaaa--slug.md",
			newID: "tp-zzzz", newSlug: "", want: filepath.Join("epic-auth", "tp-zzzz--.md"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newItemPath(tt.oldPath, tt.newID, tt.newSlug)
			if got != tt.want {
				t.Errorf("newBeanPath(%q,%q,%q) = %q, want %q", tt.oldPath, tt.newID, tt.newSlug, got, tt.want)
			}
		})
	}
}

func TestRenameID_cascadesRefs(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--parent.md": "---\n# tp-aaaa\ntitle: Parent\nstatus: todo\ntype: epic\n---\n",
		"tp-bbbb--child.md":  "---\n# tp-bbbb\ntitle: Child\nstatus: todo\ntype: task\nparent: tp-aaaa\nblocked_by:\n  - tp-aaaa\n---\n",
	})
	plan, err := c.PlanRenameID("tp-aaaa", "tp-zzzz")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "id" {
		t.Fatalf("mode = %q", plan.Mode)
	}
	if plan.RefUpdates["tp-bbbb"] != 2 { // parent + blocked_by
		t.Errorf("RefUpdates[tp-bbbb] = %d, want 2", plan.RefUpdates["tp-bbbb"])
	}
	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}
	// I01 (T04-Review): the applied disk state matches the plan exactly.
	for _, ch := range plan.Changes {
		if _, err := os.Stat(filepath.Join(c.Root(), ch.NewPath)); err != nil {
			t.Errorf("planned NewPath %q missing after apply: %v", ch.NewPath, err)
		}
		if ch.OldPath != ch.NewPath {
			if _, err := os.Stat(filepath.Join(c.Root(), ch.OldPath)); !os.IsNotExist(err) {
				t.Errorf("planned OldPath %q still present after apply", ch.OldPath)
			}
		}
	}
	// B01: the SAME core must reflect the rename in-memory after apply.
	if _, err := c.Get("tp-zzzz"); err != nil {
		t.Errorf("same-core Get(new id) failed — in-memory state not refreshed: %v", err)
	}
	if _, err := c.Get("tp-aaaa"); err == nil {
		t.Error("same-core still resolves the old ID after rename")
	}
	// also verify what landed on disk via a fresh core
	c2 := New(c.Root(), c.Config())
	if err := c2.Load(); err != nil {
		t.Fatal(err)
	}
	child, err := c2.Get("tp-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "tp-zzzz" {
		t.Errorf("child.Parent = %q, want tp-zzzz", child.Parent)
	}
	if len(child.BlockedBy) != 1 || child.BlockedBy[0] != "tp-zzzz" {
		t.Errorf("child.BlockedBy = %v", child.BlockedBy)
	}
	if _, err := c2.Get("tp-zzzz"); err != nil {
		t.Errorf("renamed bean not found under new ID: %v", err)
	}
	if _, err := c2.Get("tp-aaaa"); err == nil {
		t.Error("old ID still resolvable")
	}
}

// TestRenameID_cascadesRefs_nestedSubdir extends the cascade proof to an item
// living in a subdirectory (Prelude I01, T02-Review): the renamed file must
// land back in the same subdir, not at .beans root.
func TestRenameID_cascadesRefs_nestedSubdir(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"epic-auth/tp-aaaa--parent.md": "---\n# tp-aaaa\ntitle: Parent\nstatus: todo\ntype: epic\n---\n",
		"tp-bbbb--child.md":            "---\n# tp-bbbb\ntitle: Child\nstatus: todo\ntype: task\nparent: tp-aaaa\n---\n",
	})
	plan, err := c.PlanRenameID("tp-aaaa", "tp-zzzz")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join("epic-auth", "tp-zzzz--parent.md")
	if len(plan.Changes) != 1 || plan.Changes[0].NewPath != wantPath {
		t.Fatalf("Changes = %+v, want NewPath %q", plan.Changes, wantPath)
	}
	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(c.Root(), wantPath)); err != nil {
		t.Errorf("renamed file missing at nested path %q: %v", wantPath, err)
	}
	if _, err := os.Stat(filepath.Join(c.Root(), "epic-auth", "tp-aaaa--parent.md")); !os.IsNotExist(err) {
		t.Error("old nested file still present")
	}
}

func TestRenameID_collisionRejected(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
		"tp-bbbb--b.md": "---\n# tp-bbbb\ntitle: B\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRenameID("tp-aaaa", "tp-bbbb"); err == nil {
		t.Fatal("expected collision error")
	}
	// no mutation on refusal
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--a.md")); err != nil {
		t.Errorf("original file disturbed by refused rename: %v", err)
	}
}

func TestRenameID_sameIDRejected(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRenameID("tp-aaaa", "tp-aaaa"); err == nil {
		t.Fatal("expected error when newID equals oldID")
	}
}

func TestRenameID_unknownOldIDRejected(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRenameID("tp-nope", "tp-zzzz"); err == nil {
		t.Fatal("expected error for unknown oldID")
	}
}

// TestStageAndSwap_rollsBackOnSwapFailure exercises the rollback branch
// (rename.go, second os.Rename after the backup rename succeeds) — Prelude
// I01 from T04-Review. swapRename is a package-level indirection over the
// second os.Rename call specifically so this failure mode is deterministically
// injectable in tests (no OS-level race required).
func TestStageAndSwap_rollsBackOnSwapFailure(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "content-a",
		"tp-bbbb--b.md": "content-b",
	})
	orig := swapRename
	swapRename = func(oldpath, newpath string) error {
		return fmt.Errorf("simulated swap failure")
	}
	defer func() { swapRename = orig }()

	err := c.stageAndSwap(map[string][]byte{"tp-cccc--c.md": []byte("content-c")}, []string{"tp-bbbb--b.md"})
	if err == nil {
		t.Fatal("expected swap failure error")
	}
	// original tree restored byte-for-byte
	got, readErr := os.ReadFile(filepath.Join(c.Root(), "tp-aaaa--a.md"))
	if readErr != nil || string(got) != "content-a" {
		t.Errorf("original file not restored after rollback: content=%q err=%v", got, readErr)
	}
	got2, readErr2 := os.ReadFile(filepath.Join(c.Root(), "tp-bbbb--b.md"))
	if readErr2 != nil || string(got2) != "content-b" {
		t.Errorf("removed-then-rolled-back file not restored: content=%q err=%v", got2, readErr2)
	}
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-cccc--c.md")); !os.IsNotExist(err) {
		t.Error("staged write should not be visible after rollback")
	}
	// staging dir cleaned up (best-effort rollback still tears down staging)
	entries, err := os.ReadDir(c.repoRoot())
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".lif-staging") || strings.HasPrefix(name, ".beans-staging") {
			t.Errorf("leftover staging sibling: %s", name)
		}
	}
}

// TestPlanRebrand_countsBlockingRefs closes I01 (T05-Review prelude):
// countRefHits' Blocking loop was 0%-covered — no existing PlanRenameID/
// PlanRebrand fixture used a `blocking:` field (TestRenameID_cascadesRefs
// only exercises parent + blocked_by). Prefix-rebrand drives countRefHits
// project-wide via planCascade, so assert the ref count here.
func TestRebrand_countsBlockingRefs(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: epic\n---\n",
		"tp-bbbb--b.md": "---\n# tp-bbbb\ntitle: B\nstatus: todo\ntype: task\nblocking:\n  - tp-aaaa\n---\n",
	})
	plan, err := c.PlanRebrand("op-")
	if err != nil {
		t.Fatal(err)
	}
	if plan.RefUpdates["tp-bbbb"] != 1 {
		t.Errorf("RefUpdates[tp-bbbb] = %d, want 1 (blocking ref counted)", plan.RefUpdates["tp-bbbb"])
	}
}

func TestRebrand_remapsAllAndWritesConfig(t *testing.T) {
	c := newTestCore(t, "old_Long-Prefix-", map[string]string{
		"old_Long-Prefix-aaaa--a.md": "---\n# old_Long-Prefix-aaaa\ntitle: A\nstatus: todo\ntype: epic\n---\n",
		"old_Long-Prefix-bbbb--b.md": "---\n# old_Long-Prefix-bbbb\ntitle: B\nstatus: todo\ntype: task\nparent: old_Long-Prefix-aaaa\nblocked_by:\n  - old_Long-Prefix-aaaa\n---\n",
	})
	plan, err := c.PlanRebrand("op-")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Mode != "prefix" || !plan.ConfigWrite || plan.NewPrefix != "op-" {
		t.Fatalf("unexpected plan: mode=%q configWrite=%v newPrefix=%q", plan.Mode, plan.ConfigWrite, plan.NewPrefix)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("Changes = %d, want 2", len(plan.Changes))
	}
	if plan.RefUpdates["op-bbbb"] == 0 && plan.RefUpdates["old_Long-Prefix-bbbb"] == 0 {
		t.Errorf("RefUpdates missing entry for the child bean: %+v", plan.RefUpdates)
	}

	if err := c.ApplyRename(plan); err != nil {
		t.Fatal(err)
	}

	// .beans.yml now carries the new prefix.
	cfg2, err := config.LoadFromDirectory(c.repoRoot())
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Items.Prefix != "op-" {
		t.Errorf("config prefix = %q, want op-", cfg2.Items.Prefix)
	}

	// Same-core in-memory state already reflects the new IDs.
	if _, err := c.Get("op-aaaa"); err != nil {
		t.Errorf("same-core Get(op-aaaa) failed: %v", err)
	}
	if _, err := c.Get("old_Long-Prefix-aaaa"); err == nil {
		t.Error("same-core still resolves an old-prefix ID after rebrand")
	}

	// Disk state is consistent for a fresh Core load, refs intact.
	c2 := New(c.Root(), cfg2)
	if err := c2.Load(); err != nil {
		t.Fatal(err)
	}
	child, err := c2.Get("op-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "op-aaaa" {
		t.Errorf("child.Parent = %q, want op-aaaa", child.Parent)
	}
	if len(child.BlockedBy) != 1 || child.BlockedBy[0] != "op-aaaa" {
		t.Errorf("child.BlockedBy = %v, want [op-aaaa]", child.BlockedBy)
	}
	if _, err := c2.Get("op-aaaa"); err != nil {
		t.Errorf("rebranded parent not found under new ID: %v", err)
	}
	if _, err := c2.Get("old_Long-Prefix-aaaa"); err == nil {
		t.Error("old-prefix ID still resolvable after rebrand")
	}
}

// TestRebrand_mixedPrefixRefused proves the B04 guard: a repo where not
// every item starts with the configured prefix is refused outright (no
// staging, no mutation) rather than emitting a double-prefixed ID.
func TestRebrand_mixedPrefixRefused(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md":    "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
		"other-bbbb--b.md": "---\n# other-bbbb\ntitle: B\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRebrand("op-"); err == nil {
		t.Fatal("expected refusal on a mixed-prefix repo (B04 guard)")
	}
	// no mutation on refusal
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--a.md")); err != nil {
		t.Errorf("original file disturbed by refused rebrand: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.Root(), "other-bbbb--b.md")); err != nil {
		t.Errorf("original file disturbed by refused rebrand: %v", err)
	}
}

func TestRebrand_samePrefixRejected(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRebrand("tp-"); err == nil {
		t.Fatal("expected error when newPrefix equals current prefix")
	}
}

// TestGuard_activeWorktreeRefusesRebrand proves the D05/T07 worktree guard:
// a rebrand is refused (SC-002: no mutation) while an active worktree marker
// (*.meta.json) exists under the resolved worktree directory, and the
// project-name resolution mirrors serve.go:115-117 (GetProjectName() first,
// basename(ConfigDir()) only as fallback).
func TestGuard_activeWorktreeRefusesRebrand(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the default worktree root lives under HOME
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	projectName := c.config.GetProjectName()
	if projectName == "" {
		projectName = filepath.Base(c.config.ConfigDir())
	}
	wtPath, _, err := c.config.ResolveWorktreePath(projectName)
	if err != nil {
		t.Skipf("cannot resolve worktree path: %v", err)
	}
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wtPath)
	if err := os.WriteFile(filepath.Join(wtPath, "tp-aaaa.meta.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := c.PlanRebrand("op-"); err == nil {
		t.Fatal("expected refusal due to active worktree")
	}
	// SC-002: no mutation on refusal.
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--a.md")); err != nil {
		t.Errorf("original file disturbed by refused rebrand: %v", err)
	}
}

// TestGuard_legacyWorktreeRootRefusesRebrand proves the guard follows the
// resolver: with a legacy ~/.beans/worktrees/<project> root present, an active
// worktree marker there refuses the rebrand.
func TestGuard_legacyWorktreeRootRefusesRebrand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	projectName := c.config.GetProjectName()
	if projectName == "" {
		projectName = filepath.Base(c.config.ConfigDir())
	}
	legacyRoot := filepath.Join(home, ".beans", "worktrees", projectName)
	if err := os.MkdirAll(legacyRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PlanRebrand("op-"); err != nil {
		t.Fatalf("unexpected refusal with a marker-less legacy root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyRoot, "tp-aaaa.meta.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PlanRebrand("op-"); err == nil {
		t.Fatal("expected refusal due to an active worktree under the legacy root")
	}
}

// TestGuard_activeWorktreeAllowsRebrand_whenDirEmpty proves the guard does
// not false-positive: an existing (but empty, or marker-less) worktree
// directory must not block a rebrand.
func TestGuard_activeWorktreeAllowsRebrand_whenDirEmpty(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the default worktree root lives under HOME
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	projectName := c.config.GetProjectName()
	if projectName == "" {
		projectName = filepath.Base(c.config.ConfigDir())
	}
	wtPath, _, err := c.config.ResolveWorktreePath(projectName)
	if err != nil {
		t.Skipf("cannot resolve worktree path: %v", err)
	}
	if err := os.MkdirAll(wtPath, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wtPath)

	if _, err := c.PlanRebrand("op-"); err != nil {
		t.Fatalf("unexpected refusal with no worktree markers present: %v", err)
	}
}

// TestGuard_runningServerRefusesRebrand proves the D05/T07 server guard: a
// rebrand is refused (SC-002: no mutation) while a listener occupies the
// project's configured server port.
func TestGuard_runningServerRefusesRebrand(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	port := c.config.GetServerPort()
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Skipf("cannot bind configured port %d: %v", port, err)
	}
	defer ln.Close()

	if _, err := c.PlanRebrand("op-"); err == nil {
		t.Fatal("expected refusal while a listener occupies the configured port")
	}
	// SC-002: no mutation on refusal.
	if _, err := os.Stat(filepath.Join(c.Root(), "tp-aaaa--a.md")); err != nil {
		t.Errorf("original file disturbed by refused rebrand: %v", err)
	}
}

// TestGuard_noServerRunningAllowsRebrand proves the server guard does not
// false-positive: with nothing listening on the configured port, PlanRebrand
// must proceed normally.
func TestGuard_noServerRunningAllowsRebrand(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := c.PlanRebrand("op-"); err != nil {
		t.Fatalf("unexpected refusal with no server running: %v", err)
	}
}

func equalStrSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func strPtr(s string) *string { return &s }

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// TestStageAndSwap_rollbackOnSecondRenameFailure tests that when stageAndSwap
// performs its second os.Rename (swapping staging back to .beans) and fails,
// the original .beans is still in place (rollback branch). This validates that
// swapRename's failure handling correctly restores the original state.
func TestStageAndSwap_rollbackOnSecondRenameFailure(t *testing.T) {
	// Save the original swapRename function
	originalSwapRename := swapRename
	defer func() { swapRename = originalSwapRename }()

	// Track if swapRename was called
	swapRenameCalls := 0
	swapRename = func(old, new string) error {
		swapRenameCalls++
		// Fail on any call (will be first/only call in this test)
		return fmt.Errorf("injected failure")
	}

	repo := t.TempDir()
	itemsDir := filepath.Join(repo, ".beans")

	// Create initial .beans with an item
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(itemsDir, "tp-aaaa--test.md"), []byte("---\n# tp-aaaa\ntitle: Original\nstatus: todo\ntype: task\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultWithPrefix("tp-")
	cfg.SetConfigDir(repo)
	c := New(itemsDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}

	// Verify original item loaded
	b, err := c.Get("tp-aaaa")
	if err != nil {
		t.Fatalf("Failed to load original bean: %v", err)
	}
	if b.Title != "Original" {
		t.Errorf("Original bean title mismatch: got %q, want %q", b.Title, "Original")
	}

	// Try to stage and swap - this should fail and rollback
	err = c.stageAndSwap(map[string][]byte{
		"tp-bbbb--test.md": []byte("---\n# tp-bbbb\ntitle: New\nstatus: todo\ntype: task\n---\n"),
	}, nil)

	if err == nil {
		t.Fatalf("Expected stageAndSwap to fail (injected failure), but it succeeded. swapRename was called %d times", swapRenameCalls)
	}
	if !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("Expected injected failure, got: %v", err)
	}

	// Verify swapRename was actually called (at least once)
	if swapRenameCalls < 1 {
		t.Fatalf("swapRename was never called: %d calls", swapRenameCalls)
	}

	// After rollback, original .beans should be restored
	if _, err := os.Stat(itemsDir); os.IsNotExist(err) {
		t.Fatal("Original .beans dir was not restored after rollback")
	}

	// Verify no backup orphan left (they should be cleaned up after successful rollback)
	orphanDirs, _ := filepath.Glob(filepath.Join(repo, ".lif.bak-*"))
	legacyOrphanDirs, _ := filepath.Glob(filepath.Join(repo, ".beans.bak-*"))
	orphanDirs = append(orphanDirs, legacyOrphanDirs...)
	if len(orphanDirs) > 0 {
		t.Errorf("Found orphan backups after rollback: %v", orphanDirs)
	}

	// Verify original item is still there and NOT replaced
	c2 := New(itemsDir, cfg)
	if err := c2.Load(); err != nil {
		t.Fatalf("Failed to load after rollback: %v", err)
	}
	b2, err := c2.Get("tp-aaaa")
	if err != nil {
		t.Fatalf("Original bean lost after rollback: %v", err)
	}
	if b2.Title != "Original" {
		t.Errorf("Original bean corrupted: got %q, want %q", b2.Title, "Original")
	}
}

// TestDetectOrphanBackup_repairsExactlyOneBackup tests that when .beans is missing
// and exactly one .beans.bak-* exists with content, Load() detects and repairs it
// by renaming the backup back to .beans.
func TestDetectOrphanBackup_repairsExactlyOneBackup(t *testing.T) {
	repo := t.TempDir()
	itemsDir := filepath.Join(repo, ".beans")
	backupDir := filepath.Join(repo, ".beans.bak-1234567890")

	// Create a backup with an item inside
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "tp-xxbb--test.md"), []byte("---\n# tp-xxbb\ntitle: Recovered\nstatus: todo\ntype: task\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Core with missing .beans dir
	cfg := config.DefaultWithPrefix("tp-")
	cfg.SetConfigDir(repo)
	c := New(itemsDir, cfg)

	// Load should detect orphan and repair it
	if err := c.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// After repair, .beans should exist
	if _, err := os.Stat(itemsDir); os.IsNotExist(err) {
		t.Fatal(".beans was not restored from backup")
	}

	// Backup should no longer exist
	if _, err := os.Stat(backupDir); !os.IsNotExist(err) {
		t.Fatal("Backup was not removed after repair")
	}

	// Verify the item is now accessible
	b, err := c.Get("tp-xxbb")
	if err != nil {
		t.Fatalf("Bean not found after repair: %v", err)
	}
	if b.Title != "Recovered" {
		t.Errorf("Bean title mismatch: got %q, want %q", b.Title, "Recovered")
	}
}

// TestDetectOrphanBackup_warnsOnMultipleBackups tests that when .beans is missing
// and multiple .beans.bak-* exist, Load() warns but does not auto-repair
// (ambiguous which to use), allowing the user to manually recover.
func TestDetectOrphanBackup_warnsOnMultipleBackups(t *testing.T) {
	repo := t.TempDir()
	itemsDir := filepath.Join(repo, ".beans")

	// Create multiple backups
	for _, ts := range []string{"1000", "2000"} {
		backupDir := filepath.Join(repo, ".beans.bak-"+ts)
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, "tp-aaaa--test.md"), []byte("---\n# tp-aaaa\ntitle: Test\nstatus: todo\ntype: task\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.DefaultWithPrefix("tp-")
	cfg.SetConfigDir(repo)
	c := New(itemsDir, cfg)

	// Load should warn but not repair
	if err := c.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// .beans should exist (empty) but not repaired to any specific backup
	if _, err := os.Stat(itemsDir); os.IsNotExist(err) {
		t.Fatal(".beans should be created (empty) even when multiple backups present")
	}

	// Backups should still exist (not touched)
	for _, ts := range []string{"1000", "2000"} {
		backupDir := filepath.Join(repo, ".beans.bak-"+ts)
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			t.Fatalf("Backup .beans.bak-%s was removed", ts)
		}
	}
}

// TestDetectOrphanBackup_nothingToRepair tests that Load() succeeds normally
// when .beans exists and has content (no orphans).
func TestDetectOrphanBackup_nothingToRepair(t *testing.T) {
	c := newTestCore(t, "tp-", map[string]string{
		"tp-aaaa--test.md": "---\n# tp-aaaa\ntitle: Normal\nstatus: todo\ntype: task\n---\n",
	})

	// newTestCore already calls Load(), so the item should be loaded
	b, err := c.Get("tp-aaaa")
	if err != nil {
		t.Fatalf("Bean not found: %v", err)
	}
	if b.Title != "Normal" {
		t.Errorf("Bean title mismatch: got %q, want %q", b.Title, "Normal")
	}
}
