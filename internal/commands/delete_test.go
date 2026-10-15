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

// setupDeleteTest installs a throwaway core and default config into the
// package globals deleteCmd.RunE reads. delete had no command-level test at
// all before the batch verbs were unified on one JSON shape.
func setupDeleteTest(t *testing.T) {
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
}

func resetDeleteFlags(t *testing.T) {
	t.Helper()
	oldForce, oldJSON := forceDelete, deleteJSON
	forceDelete, deleteJSON = false, false
	t.Cleanup(func() { forceDelete, deleteJSON = oldForce, oldJSON })
}

func mkDeleteItem(t *testing.T, id, title string) *item.Item {
	t.Helper()
	b := &item.Item{
		ID:     id,
		Slug:   item.Slugify(title),
		Title:  title,
		Status: "todo",
		Type:   "task",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create(%s) error = %v", id, err)
	}
	return b
}

func captureDeleteStdout(t *testing.T, fn func()) []byte {
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

// TestDeleteJSONShape pins delete to the same D05 contract as the other
// batch verbs: one ID gives a bare item document, several IDs give a bare
// array instead of the hand-built response delete used to construct.
func TestDeleteJSONShape(t *testing.T) {
	t.Run("single gives a bare bean", func(t *testing.T) {
		setupDeleteTest(t)
		resetDeleteFlags(t)
		b := mkDeleteItem(t, "beans-del1", "First bean")
		deleteJSON = true

		out := captureDeleteStdout(t, func() {
			if err := deleteCmd.RunE(deleteCmd, []string{b.ID}); err != nil {
				t.Fatalf("deleteCmd.RunE() error = %v", err)
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
		setupDeleteTest(t)
		resetDeleteFlags(t)
		first := mkDeleteItem(t, "beans-del2", "First bean")
		second := mkDeleteItem(t, "beans-del3", "Second bean")
		deleteJSON = true

		out := captureDeleteStdout(t, func() {
			if err := deleteCmd.RunE(deleteCmd, []string{first.ID, second.ID}); err != nil {
				t.Fatalf("deleteCmd.RunE() error = %v", err)
			}
		})

		var got []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON as a bare array: %v; output = %s", err, out)
		}
		if len(got) != 2 || got[0].ID != first.ID || got[1].ID != second.ID {
			t.Errorf("array = %s, want [%s %s]", out, first.ID, second.ID)
		}
	})
}

// TestDeleteRemovesEveryNamedItem guards the behaviour the shape change sits
// on top of.
func TestDeleteRemovesEveryNamedItem(t *testing.T) {
	setupDeleteTest(t)
	resetDeleteFlags(t)
	first := mkDeleteItem(t, "beans-del4", "First bean")
	second := mkDeleteItem(t, "beans-del5", "Second bean")
	forceDelete = true

	captureDeleteStdout(t, func() {
		if err := deleteCmd.RunE(deleteCmd, []string{first.ID, second.ID}); err != nil {
			t.Fatalf("deleteCmd.RunE() error = %v", err)
		}
	})

	for _, id := range []string{first.ID, second.ID} {
		if _, err := core.Get(id); err == nil {
			t.Errorf("bean %s still resolves after delete", id)
		}
	}
}

// answerDeletePrompt runs fn with os.Stdin replaced by answer and returns
// what the prompt wrote.
func answerDeletePrompt(t *testing.T, answer string, fn func()) string {
	t.Helper()
	in, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.WriteString(answer); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = in
	t.Cleanup(func() { os.Stdin = old })
	return string(captureDeleteStdout(t, fn))
}

// Delete takes the attachment directory with the item, so the prompt has to
// say which files that is before the operator consents. Under the current
// sink those are the container's design documents, and a confirmation that
// names only the item understates what it destroys -- the same reason
// rename's --dry-run names the attachment move.
func TestDeletePromptNamesAttachments(t *testing.T) {
	setupDeleteTest(t)
	resetDeleteFlags(t)
	b := mkDeleteItem(t, "beans-del6", "Host bean")
	dir := filepath.Join(core.Root(), itemcore.AttachmentsDir, b.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "DESIGN.md"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out := answerDeletePrompt(t, "n\n", func() {
		if confirmDeleteMultiple([]itemWithLinks{{item: b, attachments: []string{"DESIGN.md"}}}) {
			t.Fatal("prompt accepted a declined deletion")
		}
	})

	if !strings.Contains(out, "DESIGN.md") {
		t.Fatalf("prompt does not name the attachment:\n%s", out)
	}
}

// The command has to collect the names itself; a prompt that is only fed by
// its caller's test would pass while the real run says nothing.
func TestDeleteRunNamesAttachments(t *testing.T) {
	setupDeleteTest(t)
	resetDeleteFlags(t)
	b := mkDeleteItem(t, "beans-del7", "Host bean")
	dir := filepath.Join(core.Root(), itemcore.AttachmentsDir, b.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "REQUIREMENTS.md"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out := answerDeletePrompt(t, "n\n", func() {
		if err := deleteCmd.RunE(deleteCmd, []string{b.ID}); err != nil {
			t.Fatalf("deleteCmd.RunE() error = %v", err)
		}
	})

	if !strings.Contains(out, "REQUIREMENTS.md") {
		t.Fatalf("delete run does not name the attachment:\n%s", out)
	}
	if _, err := core.Get(b.ID); err != nil {
		t.Fatalf("declined deletion still removed the bean: %v", err)
	}
}

// AC3: delete's lookup error and both confirmation prompts name the noun as
// item.
func TestDeleteTextsNameItems(t *testing.T) {
	t.Run("unknown id", func(t *testing.T) {
		setupDeleteTest(t)
		resetDeleteFlags(t)
		err := deleteCmd.RunE(deleteCmd, []string{"beans-nope"})
		if err == nil || err.Error() != "item not found: beans-nope" {
			t.Errorf("error = %v, want %q", err, "item not found: beans-nope")
		}
	})

	linked := func(t *testing.T) (*item.Item, []itemWithLinks) {
		t.Helper()
		setupDeleteTest(t)
		resetDeleteFlags(t)
		parent := mkDeleteItem(t, "beans-delp", "Parent")
		child := &item.Item{ID: "beans-delc", Slug: "child", Title: "Child", Status: "todo", Type: "task", Parent: parent.ID}
		if err := core.Create(child); err != nil {
			t.Fatal(err)
		}
		return parent, []itemWithLinks{{item: parent, links: core.FindIncomingLinks(parent.ID)}}
	}

	t.Run("single prompt warns about linking items", func(t *testing.T) {
		_, targets := linked(t)
		out := answerDeletePrompt(t, "n\n", func() { confirmDeleteMultiple(targets) })
		if want := "Warning: 1 item(s) link to 'Parent':"; !strings.Contains(out, want) {
			t.Errorf("prompt = %q, want it to contain %q", out, want)
		}
	})

	t.Run("batch prompt", func(t *testing.T) {
		_, targets := linked(t)
		other := mkDeleteItem(t, "beans-delo", "Other")
		targets = append(targets, itemWithLinks{item: other})
		out := answerDeletePrompt(t, "n\n", func() { confirmDeleteMultiple(targets) })
		for _, want := range []string{
			"About to delete 2 item(s):",
			"Warning: 1 item(s) have incoming references (1 total) that will be removed.",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("prompt = %q, want it to contain %q", out, want)
			}
		}
	})
}
