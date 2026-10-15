package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// setupTagTest installs a throwaway core and default config into the package
// globals tagCmd.RunE reads, and returns an item already persisted in it.
func setupTagTest(t *testing.T) *item.Item {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	return mkTagItem(t, "beans-tgt1", "A test bean", []string{"existing"})
}

func mkTagItem(t *testing.T, id, title string, tags []string) *item.Item {
	t.Helper()
	b := &item.Item{
		ID:     id,
		Slug:   item.Slugify(title),
		Title:  title,
		Status: "todo",
		Type:   "task",
		Tags:   tags,
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create(%s) error = %v", id, err)
	}
	return b
}

// resetTagFlags clears the tag* package globals and restores them afterwards.
func resetTagFlags(t *testing.T) {
	t.Helper()
	oldAdd, oldRemove, oldJSON := tagAdd, tagRemove, tagJSON
	tagAdd, tagRemove, tagJSON = nil, nil, false
	t.Cleanup(func() {
		tagAdd, tagRemove, tagJSON = oldAdd, oldRemove, oldJSON
	})
}

func captureTagStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return data
}

func tagsOf(t *testing.T, id string) []string {
	t.Helper()
	got, err := core.Get(id)
	if err != nil {
		t.Fatalf("core.Get(%s) error = %v", id, err)
	}
	return got.Tags
}

// TestTagAddsTags verifies that --tag adds to the existing tags rather than
// replacing them.
func TestTagAddsTags(t *testing.T) {
	b := setupTagTest(t)
	resetTagFlags(t)

	tagAdd = []string{"cli", "beans"}
	if err := tagCmd.RunE(tagCmd, []string{b.ID}); err != nil {
		t.Fatalf("tagCmd.RunE() error = %v", err)
	}

	if got := strings.Join(tagsOf(t, b.ID), ","); got != "beans,cli,existing" {
		t.Errorf("tags = %q, want %q", got, "beans,cli,existing")
	}
}

// TestTagRemovesTags verifies the --remove-tag counterpart.
func TestTagRemovesTags(t *testing.T) {
	b := setupTagTest(t)
	resetTagFlags(t)

	tagRemove = []string{"existing"}
	if err := tagCmd.RunE(tagCmd, []string{b.ID}); err != nil {
		t.Fatalf("tagCmd.RunE() error = %v", err)
	}

	if got := tagsOf(t, b.ID); len(got) != 0 {
		t.Errorf("tags = %v, want none", got)
	}
}

// TestTagAddsAndRemovesInOneCall verifies both flags compose.
func TestTagAddsAndRemovesInOneCall(t *testing.T) {
	b := setupTagTest(t)
	resetTagFlags(t)

	tagAdd = []string{"cli"}
	tagRemove = []string{"existing"}
	if err := tagCmd.RunE(tagCmd, []string{b.ID}); err != nil {
		t.Fatalf("tagCmd.RunE() error = %v", err)
	}

	if got := strings.Join(tagsOf(t, b.ID), ","); got != "cli" {
		t.Errorf("tags = %q, want %q", got, "cli")
	}
}

// TestTagTakesMultipleIDs is the reason the verb exists: one call, n items.
func TestTagTakesMultipleIDs(t *testing.T) {
	first := setupTagTest(t)
	resetTagFlags(t)
	second := mkTagItem(t, "beans-tgt2", "Second bean", nil)

	tagAdd = []string{"cli"}
	if err := tagCmd.RunE(tagCmd, []string{first.ID, second.ID}); err != nil {
		t.Fatalf("tagCmd.RunE() error = %v", err)
	}

	for _, id := range []string{first.ID, second.ID} {
		found := false
		for _, tag := range tagsOf(t, id) {
			if tag == "cli" {
				found = true
			}
		}
		if !found {
			t.Errorf("bean %s did not receive the tag, tags = %v", id, tagsOf(t, id))
		}
	}
}

// TestTagRequiresAFlag verifies that a call which would change nothing is
// rejected instead of silently rewriting the items.
func TestTagRequiresAFlag(t *testing.T) {
	b := setupTagTest(t)
	resetTagFlags(t)

	if err := tagCmd.RunE(tagCmd, []string{b.ID}); err == nil {
		t.Fatal("tagCmd.RunE() error = nil, want an error when neither flag is given")
	}
}

// TestTagPreflightRejectsUnknownID verifies the batch preflight applies here
// as it does to the status verbs.
func TestTagPreflightRejectsUnknownID(t *testing.T) {
	first := setupTagTest(t)
	resetTagFlags(t)
	second := mkTagItem(t, "beans-tgt3", "Second bean", nil)

	tagAdd = []string{"cli"}
	if err := tagCmd.RunE(tagCmd, []string{first.ID, "beans-nope", second.ID}); err == nil {
		t.Fatal("tagCmd.RunE() error = nil, want an error for the unknown ID")
	}

	for _, id := range []string{first.ID, second.ID} {
		for _, tag := range tagsOf(t, id) {
			if tag == "cli" {
				t.Errorf("bean %s was tagged despite the preflight failure", id)
			}
		}
	}
}

// TestTagJSONShape verifies the verb starts in the target shape: a bare item
// for one ID, a bare array beyond that.
func TestTagJSONShape(t *testing.T) {
	t.Run("single gives a bare bean", func(t *testing.T) {
		b := setupTagTest(t)
		resetTagFlags(t)
		tagAdd = []string{"cli"}
		tagJSON = true

		out := captureTagStdout(t, func() {
			if err := tagCmd.RunE(tagCmd, []string{b.ID}); err != nil {
				t.Fatalf("tagCmd.RunE() error = %v", err)
			}
		})

		var got struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON: %v; output = %s", err, out)
		}
		if got.ID != b.ID {
			t.Errorf("bare bean = %s, want id %s", out, b.ID)
		}
	})

	t.Run("multiple gives a bare array", func(t *testing.T) {
		first := setupTagTest(t)
		resetTagFlags(t)
		second := mkTagItem(t, "beans-tgt4", "Second bean", nil)
		tagAdd = []string{"cli"}
		tagJSON = true

		out := captureTagStdout(t, func() {
			if err := tagCmd.RunE(tagCmd, []string{first.ID, second.ID}); err != nil {
				t.Fatalf("tagCmd.RunE() error = %v", err)
			}
		})

		var got []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON as a bare array: %v; output = %s", err, out)
		}
		if len(got) != 2 {
			t.Errorf("array = %s, want two entries", out)
		}
	})
}
