package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/storeroot"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// AC3: the tag completion describes each tag's usage count in items.
func TestTagCompletionCountsItems(t *testing.T) {
	setupListTest(t)
	for _, b := range []*item.Item{
		{ID: "t-1", Slug: "a", Title: "A", Status: "todo", Type: "task", Tags: []string{"echo"}},
		{ID: "t-2", Slug: "b", Title: "B", Status: "todo", Type: "task", Tags: []string{"echo"}},
	} {
		if err := core.Create(b); err != nil {
			t.Fatal(err)
		}
	}

	got, _ := tagFlagCompletion(nil, nil, "")
	if len(got) != 1 || got[0] != "echo\t2 item(s)" {
		t.Errorf("tagFlagCompletion() = %q, want [%q]", got, "echo\t2 item(s)")
	}
}

// AC3: update refuses --if-match for several IDs and names the noun as item.
func TestUpdateIfMatchWithSeveralIDsNamesItem(t *testing.T) {
	first := setupUpdateTest(t)
	oldIfMatch := updateIfMatch
	updateIfMatch = "deadbeefdeadbeef"
	t.Cleanup(func() { updateIfMatch = oldIfMatch })

	err := updateCmd.RunE(updateCmd, []string{first.ID, "beans-other"})
	want := "--if-match takes one item's etag, got 2 ids"
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

// AC3: archive reports its outcome in items, in plain and in JSON mode.
func TestArchiveTextsNameItems(t *testing.T) {
	run := func(t *testing.T, asJSON, withCompleted bool) string {
		t.Helper()
		setupListTest(t)
		if withCompleted {
			done := &item.Item{ID: "t-1", Slug: "a", Title: "A", Status: "completed", Type: "task"}
			if err := core.Create(done); err != nil {
				t.Fatal(err)
			}
		}
		old := archiveJSON
		archiveJSON = asJSON
		t.Cleanup(func() { archiveJSON = old })

		return string(captureListStdout(t, func() {
			if err := archiveCmd.RunE(archiveCmd, nil); err != nil {
				t.Fatalf("archive error = %v", err)
			}
		}))
	}

	for _, tc := range []struct {
		name                  string
		asJSON, withCompleted bool
		want                  string
	}{
		{"nothing to archive", false, false, "No items with archive status to archive."},
		{"nothing to archive as json", true, false, "No items to archive"},
		{"archived", false, true, "Archived 1 item(s) to "},
		{"archived as json", true, true, "Archived 1 item(s) to "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if out := run(t, tc.asJSON, tc.withCompleted); !strings.Contains(out, tc.want) {
				t.Errorf("output = %q, want it to contain %q", out, tc.want)
			}
		})
	}
}

// AC1/AC3: create's failure, its flag help and its completion hint name the
// noun as item and the product as lif.
func TestCreateTextsNameLifAndItems(t *testing.T) {
	t.Run("failure", func(t *testing.T) {
		setupListTest(t)
		t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

		_, _, err := runRootInDir(t, core.Root(), "create", "Some title", "--blocking", "nope")
		want := "failed to create item: target item not found: nope"
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %q", err, want)
		}
	})

	t.Run("hint", func(t *testing.T) {
		if want := `lif create "Fix the login bug"`; !strings.Contains(createHint(), want) {
			t.Errorf("createHint() = %q, want it to contain %q", createHint(), want)
		}
	})

	t.Run("type flag", func(t *testing.T) {
		sharedTestRoot(t)
		if got := createCmd.Flags().Lookup("type").Usage; !strings.HasPrefix(got, "Item type (") {
			t.Errorf("--type usage = %q, want it to start with %q", got, "Item type (")
		}
	})
}

// AC3: a plain `lif init` confirms in the product's name.
func TestInitConfirmationNamesLif(t *testing.T) {
	var err error
	out := captureListStdout(t, func() { _, err = runInitCmd(t, "", false) })
	if err != nil {
		t.Fatalf("init error = %v", err)
	}
	if want := "Initialized Lif project\n"; string(out) != want {
		t.Errorf("init output = %q, want %q", out, want)
	}
}

// AC3: the shared batch resolution (start/complete/scrap/tag/update/...)
// names the noun as item when an ID repeats or does not resolve.
func TestBatchTargetErrorsNameItems(t *testing.T) {
	first := setupStartTest(t)
	resetStartFlags(t)

	dup := startCmd.RunE(startCmd, []string{first.ID, first.ID})
	if want := "item " + first.ID + " named more than once"; dup == nil || !strings.Contains(dup.Error(), want) {
		t.Errorf("duplicate-ID error = %v, want it to contain %q", dup, want)
	}

	unknown := startCmd.RunE(startCmd, []string{"beans-notfound"})
	if want := "item not found: beans-notfound"; unknown == nil || unknown.Error() != want {
		t.Errorf("unknown-ID error = %v, want %q", unknown, want)
	}
}

// AC3: an explicit store path that does not exist, and a missing store
// found by search, are reported without the former product name.
func TestStoreResolutionErrorsNameLif(t *testing.T) {
	neutralizeStoreEnv(t)
	c := config.Default()
	c.SetConfigDir(t.TempDir())

	missing := filepath.Join(t.TempDir(), "gone")
	_, _, err := storeroot.Resolve(missing, c)
	if want := "store path does not exist or is not a directory: " + missing; err == nil || err.Error() != want {
		t.Errorf("explicit path error = %v, want %q", err, want)
	}

	_, _, err = storeroot.Resolve("", c)
	if err == nil || !strings.Contains(err.Error(), "(run 'lif init' to create one)") {
		t.Errorf("missing store error = %v, want it to point at 'lif init'", err)
	}
}

// denyDir makes dir unwritable (or unreadable, for mode 0) for the rest of
// the test; a process running as root ignores permission bits, so there is
// nothing to observe then.
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

// AC3: a store that cannot be read is reported as items that could not be
// loaded, and a store that cannot be written reports the failed archive or
// delete as an item.
func TestIOFailuresNameItems(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".lif")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		denyDir(t, dir, 0)
		t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

		_, _, err := runRootInDir(t, dir, "list")
		if err == nil || !strings.HasPrefix(err.Error(), "loading items: ") {
			t.Errorf("error = %v, want it to start with %q", err, "loading items: ")
		}
	})

	archiveFixture := func(t *testing.T) {
		t.Helper()
		setupListTest(t)
		if err := core.Create(&item.Item{ID: "t-1", Slug: "a", Title: "A", Status: "completed", Type: "task"}); err != nil {
			t.Fatal(err)
		}
		archiveDir := filepath.Join(core.Root(), itemcore.ArchiveDir)
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			t.Fatal(err)
		}
		denyDir(t, archiveDir, 0555)
	}

	t.Run("archive", func(t *testing.T) {
		archiveFixture(t)
		err := archiveCmd.RunE(archiveCmd, nil)
		if err == nil || !strings.HasPrefix(err.Error(), "failed to archive item t-1: moving item to archive: ") {
			t.Errorf("error = %v, want it to start with %q", err, "failed to archive item t-1: moving item to archive: ")
		}
	})

	t.Run("archive as json", func(t *testing.T) {
		archiveFixture(t)
		old := archiveJSON
		archiveJSON = true
		t.Cleanup(func() { archiveJSON = old })

		var err error
		out := captureListStdout(t, func() { err = archiveCmd.RunE(archiveCmd, nil) })
		if err == nil || !strings.Contains(string(out), "failed to archive item t-1: ") {
			t.Errorf("error = %v, output = %q, want the JSON error to say %q", err, out, "failed to archive item t-1: ")
		}
	})

	t.Run("delete", func(t *testing.T) {
		setupDeleteTest(t)
		resetDeleteFlags(t)
		forceDelete = true
		b := mkDeleteItem(t, "beans-delx", "X")
		denyDir(t, core.Root(), 0555)

		err := deleteCmd.RunE(deleteCmd, []string{b.ID})
		if err == nil || !strings.HasPrefix(err.Error(), "failed to delete item beans-delx: ") {
			t.Errorf("error = %v, want it to start with %q", err, "failed to delete item beans-delx: ")
		}
	})
}

// AC3: a search query the index rejects is reported as a failed query for
// items.
func TestListSearchFailureNamesItems(t *testing.T) {
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) }) // --search must not leak into later list tests
	err := runListExpectingError(t, []string{"--search", `"unterminated`})
	if err == nil || !strings.HasPrefix(err.Error(), "querying items: ") {
		t.Errorf("error = %v, want it to start with %q", err, "querying items: ")
	}
}

// AC3: check reports an item whose type the configuration does not define as
// an item, and exits non-zero. The real binary runs it: check ends in
// os.Exit(1), which no in-process test can survive.
func TestCheckReportsAStrayTypeAsAnItem(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, ".lif")
	if out, err := exec.Command(completionTestBinary, "init", "--lif-path", store).CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	stray := "---\n# aaa2\ntitle: Stray\nstatus: todo\ntype: chore\n---\n"
	if err := os.WriteFile(filepath.Join(store, "aaa2--stray.md"), []byte(stray), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(completionTestBinary, "check", "--lif-path", store).CombinedOutput()
	if err == nil {
		t.Errorf("check exited 0, want a non-zero exit for the stray type:\n%s", out)
	}
	if want := "item aaa2 carries type 'chore', which the configuration does not define"; !strings.Contains(string(out), want) {
		t.Errorf("check output lacks %q:\n%s", want, out)
	}
}
