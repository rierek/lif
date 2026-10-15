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

// setupScrapTest installs a throwaway core and default config into the
// package globals scrapCmd.RunE reads, and returns an item already persisted
// in that core.
func setupScrapTest(t *testing.T) *item.Item {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
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

	b := &item.Item{
		ID:     "beans-test1",
		Slug:   item.Slugify("A test bean"),
		Title:  "A test bean",
		Status: "todo",
		Type:   "task",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}
	return b
}

// resetScrapFlags clears every scrap* package global the tests below touch
// and restores the pre-test values afterwards, so tests stay isolated despite
// sharing the scrapCmd package-level flag vars.
func resetScrapFlags(t *testing.T) {
	t.Helper()
	oldJSON, oldReason := scrapJSON, scrapReason
	scrapJSON, scrapReason = false, ""
	t.Cleanup(func() {
		scrapJSON, scrapReason = oldJSON, oldReason
	})
}

// TestScrapRequiresMissingReason verifies that omitting --reason errors without mutating the item.
func TestScrapRequiresMissingReason(t *testing.T) {
	b := setupScrapTest(t)
	resetScrapFlags(t)

	if err := scrapCmd.RunE(scrapCmd, []string{b.ID}); err == nil {
		t.Errorf("scrapCmd.RunE() expected error for missing reason, got nil")
	}

	// Verify item was not mutated
	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("bean status = %q, want %q (should not have changed)", got.Status, "todo")
	}
}

// TestScrapRequiresEmptyReason verifies that --reason "" errors without mutating the item.
func TestScrapRequiresEmptyReason(t *testing.T) {
	b := setupScrapTest(t)
	resetScrapFlags(t)

	scrapReason = ""
	if err := scrapCmd.RunE(scrapCmd, []string{b.ID}); err == nil {
		t.Errorf("scrapCmd.RunE() expected error for empty reason, got nil")
	}

	// Verify item was not mutated
	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("bean status = %q, want %q (should not have changed)", got.Status, "todo")
	}
}

// TestScrapSetsStatusAndAppendsReason verifies that --reason appends the reason section and sets status to scrapped.
func TestScrapSetsStatusAndAppendsReason(t *testing.T) {
	b := setupScrapTest(t)
	resetScrapFlags(t)

	scrapReason = "Superseded by beans-xyz"
	if err := scrapCmd.RunE(scrapCmd, []string{b.ID}); err != nil {
		t.Fatalf("scrapCmd.RunE() error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Status != "scrapped" {
		t.Errorf("bean status = %q, want %q", got.Status, "scrapped")
	}
	if !strings.Contains(got.Body, "## Reason for Scrapping") {
		t.Errorf("body does not contain '## Reason for Scrapping', body = %q", got.Body)
	}
	if !strings.Contains(got.Body, "Superseded by beans-xyz") {
		t.Errorf("body does not contain reason text, body = %q", got.Body)
	}
}

// TestScrapRejectsUnknownID verifies that unknown item ID returns an error.
func TestScrapRejectsUnknownID(t *testing.T) {
	setupScrapTest(t)
	resetScrapFlags(t)

	scrapReason = "Some reason"
	if err := scrapCmd.RunE(scrapCmd, []string{"beans-notfound"}); err == nil {
		t.Errorf("scrapCmd.RunE() expected error for unknown ID, got nil")
	}
}

// TestScrapJSONOutput verifies D05/D12: --json returns the bare item
// document directly, with no {success,item,message} envelope.
func TestScrapJSONOutput(t *testing.T) {
	b := setupScrapTest(t)
	resetScrapFlags(t)
	scrapJSON = true
	scrapReason = "Test reason for scrapping"

	out := captureScrapStdout(t, func() {
		if err := scrapCmd.RunE(scrapCmd, []string{b.ID}); err != nil {
			t.Fatalf("scrapCmd.RunE() error = %v", err)
		}
	})

	var got struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	if got.ID != b.ID {
		t.Errorf("bare bean id = %q, want %q", got.ID, b.ID)
	}
	if got.Status != "scrapped" {
		t.Errorf("bare bean status = %q, want %q", got.Status, "scrapped")
	}

	// Also verify the item was actually persisted with correct status
	persisted, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if persisted.Status != "scrapped" {
		t.Errorf("persisted bean status = %q, want %q", persisted.Status, "scrapped")
	}
}

// captureScrapStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureScrapStdout(t *testing.T, fn func()) []byte {
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

// mkScrapItem adds another item to the store set up by setupScrapTest.
func mkScrapItem(t *testing.T, id, title string) *item.Item {
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

// TestScrapTakesMultipleIDs verifies the batch signature scraps every named
// item and gives each the shared reason section.
func TestScrapTakesMultipleIDs(t *testing.T) {
	first := setupScrapTest(t)
	resetScrapFlags(t)
	second := mkScrapItem(t, "beans-bts2", "Second bean")

	scrapReason = "Superseded by the new approach"
	if err := scrapCmd.RunE(scrapCmd, []string{first.ID, second.ID}); err != nil {
		t.Fatalf("scrapCmd.RunE() error = %v", err)
	}

	for _, id := range []string{first.ID, second.ID} {
		got, err := core.Get(id)
		if err != nil {
			t.Fatalf("core.Get(%s) error = %v", id, err)
		}
		if got.Status != "scrapped" {
			t.Errorf("bean %s status = %q, want %q", id, got.Status, "scrapped")
		}
		if !strings.Contains(got.Body, "Superseded by the new approach") {
			t.Errorf("bean %s body does not carry the shared reason, body = %q", id, got.Body)
		}
	}
}

// TestScrapPreflightRejectsUnknownID verifies nothing is written when any ID
// in the call cannot be resolved.
func TestScrapPreflightRejectsUnknownID(t *testing.T) {
	first := setupScrapTest(t)
	resetScrapFlags(t)
	second := mkScrapItem(t, "beans-brs2", "Second bean")

	scrapReason = "Not going to happen"
	if err := scrapCmd.RunE(scrapCmd, []string{first.ID, "beans-nope", second.ID}); err == nil {
		t.Fatal("scrapCmd.RunE() error = nil, want an error for the unknown ID")
	}

	for _, id := range []string{first.ID, second.ID} {
		got, err := core.Get(id)
		if err != nil {
			t.Fatalf("core.Get(%s) error = %v", id, err)
		}
		if got.Status != "todo" {
			t.Errorf("bean %s status = %q, want it untouched at %q", id, got.Status, "todo")
		}
	}
}
