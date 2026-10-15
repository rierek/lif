package commands

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// setupStartTest installs a throwaway core and default config into the
// package globals startCmd.RunE reads, and returns an item already persisted
// in that core.
func setupStartTest(t *testing.T) *item.Item {
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
		Body:   "Test body content",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}
	return b
}

// resetStartFlags clears every start* package global the tests below touch
// and restores the pre-test values afterwards, so tests stay isolated despite
// sharing the startCmd package-level flag vars.
func resetStartFlags(t *testing.T) {
	t.Helper()
	oldJSON := startJSON
	startJSON = false
	t.Cleanup(func() {
		startJSON = oldJSON
	})
}

// TestStartSetsStatusInProgress verifies that lif start <id> sets status to in-progress.
func TestStartSetsStatusInProgress(t *testing.T) {
	b := setupStartTest(t)
	resetStartFlags(t)

	if err := startCmd.RunE(startCmd, []string{b.ID}); err != nil {
		t.Fatalf("startCmd.RunE() error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Status != "in-progress" {
		t.Errorf("bean status = %q, want %q", got.Status, "in-progress")
	}
}

// TestStartDisplaysItemDetails verifies that start displays the item details
// by delegating to show command.
func TestStartDisplaysItemDetails(t *testing.T) {
	b := setupStartTest(t)
	resetStartFlags(t)

	// Capture stdout to verify display output
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	oldStdout := os.Stdout
	os.Stdout = w

	runErr := startCmd.RunE(startCmd, []string{b.ID})

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}

	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}

	if runErr != nil {
		t.Fatalf("startCmd.RunE() error = %v", runErr)
	}

	output := string(captured)

	// Verify output contains item ID and title (show command output includes these)
	if !strings.Contains(output, b.ID) {
		t.Errorf("output does not contain bean ID %q, output = %q", b.ID, output)
	}
	if !strings.Contains(output, b.Title) {
		t.Errorf("output does not contain bean title %q, output = %q", b.Title, output)
	}
	if !strings.Contains(output, "in-progress") {
		t.Errorf("output does not contain status 'in-progress', output = %q", output)
	}
}

// TestStartRejectsUnknownID verifies that unknown item ID returns an error.
func TestStartRejectsUnknownID(t *testing.T) {
	setupStartTest(t)
	resetStartFlags(t)

	if err := startCmd.RunE(startCmd, []string{"beans-notfound"}); err == nil {
		t.Errorf("startCmd.RunE() expected error for unknown ID, got nil")
	}
}

// captureStartStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureStartStdout(t *testing.T, fn func()) []byte {
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

// mkStartItem adds another item to the store set up by setupStartTest.
func mkStartItem(t *testing.T, id, title string) *item.Item {
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

// TestStartTakesMultipleIDs verifies the batch signature starts every named
// item. With more than one ID the output collapses to one line per item
// instead of the full show rendering, which would otherwise dump n bodies.
func TestStartTakesMultipleIDs(t *testing.T) {
	first := setupStartTest(t)
	resetStartFlags(t)
	second := mkStartItem(t, "beans-bst2", "Second bean")

	out := captureStartStdout(t, func() {
		if err := startCmd.RunE(startCmd, []string{first.ID, second.ID}); err != nil {
			t.Fatalf("startCmd.RunE() error = %v", err)
		}
	})

	for _, id := range []string{first.ID, second.ID} {
		got, err := core.Get(id)
		if err != nil {
			t.Fatalf("core.Get(%s) error = %v", id, err)
		}
		if got.Status != "in-progress" {
			t.Errorf("bean %s status = %q, want %q", id, got.Status, "in-progress")
		}
		if !strings.Contains(string(out), id) {
			t.Errorf("output does not mention %s, output = %q", id, out)
		}
	}
	if n := strings.Count(string(out), "\n"); n > 4 {
		t.Errorf("output has %d lines, want the compact per-bean form; output = %q", n, out)
	}
}

// TestStartPreflightRejectsUnknownID verifies nothing is written when any ID
// in the call cannot be resolved.
func TestStartPreflightRejectsUnknownID(t *testing.T) {
	first := setupStartTest(t)
	resetStartFlags(t)
	second := mkStartItem(t, "beans-brt2", "Second bean")

	if err := startCmd.RunE(startCmd, []string{first.ID, "beans-nope", second.ID}); err == nil {
		t.Fatal("startCmd.RunE() error = nil, want an error for the unknown ID")
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
