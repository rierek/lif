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

// setupNextTest installs a throwaway core and default config into the
// package globals nextCmd.RunE reads, mirroring setupStartTest.
func setupNextTest(t *testing.T) {
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
}

// resetNextFlags clears every next* package global the tests below touch
// and restores the pre-test values afterwards.
func resetNextFlags(t *testing.T) {
	t.Helper()
	oldType, oldTag, oldParent := nextType, nextTag, nextParent
	oldSort, oldDesc, oldJSON := nextSort, nextDesc, nextJSON
	nextType, nextTag, nextParent = nil, nil, ""
	nextSort, nextDesc, nextJSON = "", false, false
	t.Cleanup(func() {
		nextType, nextTag, nextParent = oldType, oldTag, oldParent
		nextSort, nextDesc, nextJSON = oldSort, oldDesc, oldJSON
	})
}

// captureNextStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureNextStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
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

// TestNextReturnsHighestPriorityReady verifies that lif next picks the
// highest-priority ready (todo, not blocked) item and displays it.
func TestNextReturnsHighestPriorityReady(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	low := &item.Item{
		ID:       "beans-low1",
		Slug:     item.Slugify("Low priority task"),
		Title:    "Low priority task",
		Status:   "todo",
		Type:     "task",
		Priority: "low",
	}
	high := &item.Item{
		ID:       "beans-high1",
		Slug:     item.Slugify("High priority task"),
		Title:    "High priority task",
		Status:   "todo",
		Type:     "task",
		Priority: "high",
	}
	if err := core.Create(low); err != nil {
		t.Fatalf("core.Create(low) error = %v", err)
	}
	if err := core.Create(high); err != nil {
		t.Fatalf("core.Create(high) error = %v", err)
	}

	out := captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	if !strings.Contains(got, high.ID) {
		t.Errorf("output does not contain highest-priority bean ID %q, output = %q", high.ID, got)
	}
	if strings.Contains(got, low.ID) {
		t.Errorf("output unexpectedly contains lower-priority bean ID %q, output = %q", low.ID, got)
	}
}

// TestNextRespectsBlockedExclusion verifies that a blocked todo item is
// skipped in favor of an unblocked one.
func TestNextRespectsBlockedExclusion(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	// blocker is itself a valid ready item (low priority), so it must lose
	// the pick to the higher-priority unblocked item below — this also
	// proves the winner isn't just an artifact of ID tie-breaking.
	blocker := &item.Item{
		ID:       "beans-blk1",
		Slug:     item.Slugify("Blocker"),
		Title:    "Blocker",
		Status:   "todo",
		Type:     "task",
		Priority: "low",
	}
	if err := core.Create(blocker); err != nil {
		t.Fatalf("core.Create(blocker) error = %v", err)
	}

	// blocked carries the highest priority of the three but must be
	// excluded entirely because it is blocked by an active (todo) blocker.
	blocked := &item.Item{
		ID:        "beans-blkd1",
		Slug:      item.Slugify("Blocked high priority"),
		Title:     "Blocked high priority",
		Status:    "todo",
		Type:      "task",
		Priority:  "high",
		BlockedBy: []string{blocker.ID},
	}
	if err := core.Create(blocked); err != nil {
		t.Fatalf("core.Create(blocked) error = %v", err)
	}

	unblocked := &item.Item{
		ID:       "beans-unbl1",
		Slug:     item.Slugify("Unblocked high priority"),
		Title:    "Unblocked high priority",
		Status:   "todo",
		Type:     "task",
		Priority: "high",
	}
	if err := core.Create(unblocked); err != nil {
		t.Fatalf("core.Create(unblocked) error = %v", err)
	}

	out := captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	if strings.Contains(got, blocked.ID) {
		t.Errorf("output unexpectedly contains blocked bean ID %q, output = %q", blocked.ID, got)
	}
	if strings.Contains(got, blocker.ID) {
		t.Errorf("output unexpectedly contains lower-priority blocker bean ID %q, output = %q", blocker.ID, got)
	}
	if !strings.Contains(got, unblocked.ID) {
		t.Errorf("output does not contain unblocked bean ID %q, output = %q", unblocked.ID, got)
	}
}

// TestNextReportsNoneReady verifies that with no ready items, next reports
// this without erroring, both in plain and --json mode.
func TestNextReportsNoneReady(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		setupNextTest(t)
		resetNextFlags(t)

		out := captureNextStdout(t, func() {
			if err := nextCmd.RunE(nextCmd, nil); err != nil {
				t.Fatalf("nextCmd.RunE() error = %v", err)
			}
		})

		got := string(out)
		for _, want := range []string{"No ready items found.", "lif list --is-blocked", "lif list for what's outstanding."} {
			if !strings.Contains(got, want) {
				t.Errorf("empty-result message lacks %q, output = %q", want, got)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		setupNextTest(t)
		resetNextFlags(t)
		nextJSON = true

		out := captureNextStdout(t, func() {
			if err := nextCmd.RunE(nextCmd, nil); err != nil {
				t.Fatalf("nextCmd.RunE() error = %v", err)
			}
		})

		var resp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Fatalf("decoding JSON output: %v; output = %s", err, out)
		}
		if !resp.Success {
			t.Errorf("expected success=true for empty-result JSON, got %+v; output = %s", resp, out)
		}
		if resp.Message != "no ready items found" {
			t.Errorf("message = %q, want %q", resp.Message, "no ready items found")
		}
	})
}

// mkReadyItem creates a ready (todo) item in the test store and returns it.
func mkReadyItem(t *testing.T, id, title, itemType, priority string, tags []string, parent string) *item.Item {
	t.Helper()
	b := &item.Item{
		ID:       id,
		Slug:     item.Slugify(title),
		Title:    title,
		Status:   "todo",
		Type:     itemType,
		Priority: priority,
		Tags:     tags,
		Parent:   parent,
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create(%s) error = %v", id, err)
	}
	return b
}

// TestNextSkipsContainerItems is a regression test for beans-g5dn: the ready
// list is leaf work. A milestone, epic and feature that outrank the task by
// priority must not be handed back, and a type filter that names a container
// type finds nothing ready.
func TestNextSkipsContainerItems(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	milestone := mkReadyItem(t, "beans-gm01", "Release", "milestone", "critical", nil, "")
	epic := mkReadyItem(t, "beans-ge01", "Area", "epic", "critical", nil, milestone.ID)
	feature := mkReadyItem(t, "beans-gf01", "Capability", "feature", "critical", nil, epic.ID)
	task := mkReadyItem(t, "beans-gt01", "Do the work", "task", "low", nil, feature.ID)

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))
	// The shown item opens with its "# <id>" line; a mere ID match would also
	// hit the task's own "parent: <id>" line.
	if !strings.Contains(out, "\n# "+task.ID+"\n") {
		t.Errorf("output does not show the task %q, output = %q", task.ID, out)
	}
	for _, container := range []*item.Item{milestone, epic, feature} {
		if strings.Contains(out, "\n# "+container.ID+"\n") {
			t.Errorf("output shows the %s %q, output = %q", container.Type, container.ID, out)
		}
	}

	nextType = []string{"epic"}
	out = string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))
	if !strings.Contains(out, "No ready items found matching --type epic.") {
		t.Errorf("--type epic should find nothing ready, output = %q", out)
	}
}

// TestNextFiltersByType verifies that --type narrows the candidate set the
// ready filter is applied to, rather than being ignored.
func TestNextFiltersByType(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	// The bug carries lower priority, so without the filter the task would
	// win — the filter has to beat the sort order, not merely agree with it.
	task := mkReadyItem(t, "beans-ft01", "High priority task", "task", "high", nil, "")
	bug := mkReadyItem(t, "beans-fb01", "Low priority bug", "bug", "low", nil, "")

	nextType = []string{"bug"}

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, bug.ID) {
		t.Errorf("output does not contain filtered bean %q, output = %q", bug.ID, out)
	}
	if strings.Contains(out, task.ID) {
		t.Errorf("output unexpectedly contains excluded bean %q, output = %q", task.ID, out)
	}
}

// TestNextFiltersByTag verifies --tag narrows the candidate set.
func TestNextFiltersByTag(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	untagged := mkReadyItem(t, "beans-gu01", "High priority untagged", "task", "high", nil, "")
	tagged := mkReadyItem(t, "beans-gt01", "Low priority tagged", "task", "low", []string{"cli"}, "")

	nextTag = []string{"cli"}

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, tagged.ID) {
		t.Errorf("output does not contain tagged bean %q, output = %q", tagged.ID, out)
	}
	if strings.Contains(out, untagged.ID) {
		t.Errorf("output unexpectedly contains untagged bean %q, output = %q", untagged.ID, out)
	}
}

// TestNextFiltersByParent verifies --parent narrows the candidate set.
func TestNextFiltersByParent(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	epic := mkReadyItem(t, "beans-pe01", "Epic", "epic", "normal", nil, "")
	outside := mkReadyItem(t, "beans-po01", "High priority outside", "task", "high", nil, "")
	inside := mkReadyItem(t, "beans-pi01", "Low priority inside", "task", "low", nil, epic.ID)

	nextParent = epic.ID

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, inside.ID) {
		t.Errorf("output does not contain child bean %q, output = %q", inside.ID, out)
	}
	if strings.Contains(out, outside.ID) {
		t.Errorf("output unexpectedly contains bean outside the parent %q, output = %q", outside.ID, out)
	}
}

// TestNextCombinesFiltersWithReadySemantics verifies that the new filters
// compose with each other and do not disable the ready filter.
func TestNextCombinesFiltersWithReadySemantics(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	epic := mkReadyItem(t, "beans-ce01", "Epic", "epic", "normal", nil, "")
	// Matches every filter and outranks the winner on priority, but is
	// in-progress and therefore not ready.
	inProgress := &item.Item{
		ID:       "beans-cp01",
		Slug:     item.Slugify("In progress match"),
		Title:    "In progress match",
		Status:   "in-progress",
		Type:     "bug",
		Priority: "high",
		Tags:     []string{"cli"},
		Parent:   epic.ID,
	}
	if err := core.Create(inProgress); err != nil {
		t.Fatalf("core.Create(inProgress) error = %v", err)
	}
	// Matches type and tag but hangs under no parent.
	wrongParent := mkReadyItem(t, "beans-cw01", "Wrong parent", "bug", "high", []string{"cli"}, "")
	want := mkReadyItem(t, "beans-cm01", "Full match", "bug", "low", []string{"cli"}, epic.ID)

	nextType = []string{"bug"}
	nextTag = []string{"cli"}
	nextParent = epic.ID

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, want.ID) {
		t.Errorf("output does not contain the fully matching bean %q, output = %q", want.ID, out)
	}
	for _, unwanted := range []*item.Item{inProgress, wrongParent} {
		if strings.Contains(out, unwanted.ID) {
			t.Errorf("output unexpectedly contains %q, output = %q", unwanted.ID, out)
		}
	}
}

// TestNextSortPicksByRequestedField verifies that --sort changes which item
// next hands back, rather than being accepted and ignored.
func TestNextSortPicksByRequestedField(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	// Priority order and ID order disagree, so whichever item comes back
	// identifies which ordering actually ran.
	first := mkReadyItem(t, "beans-aaa1", "Alphabetically first, low priority", "task", "low", nil, "")
	high := mkReadyItem(t, "beans-zzz1", "Alphabetically last, high priority", "task", "high", nil, "")

	nextSort = "id"

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, first.ID) {
		t.Errorf("--sort id did not pick the lowest ID %q, output = %q", first.ID, out)
	}
	if strings.Contains(out, high.ID) {
		t.Errorf("--sort id unexpectedly picked %q, output = %q", high.ID, out)
	}
}

// TestNextDescReversesSelection verifies --desc flips which end of the sorted
// list next takes its answer from.
func TestNextDescReversesSelection(t *testing.T) {
	setupNextTest(t)
	resetNextFlags(t)

	first := mkReadyItem(t, "beans-aaa2", "Alphabetically first", "task", "normal", nil, "")
	last := mkReadyItem(t, "beans-zzz2", "Alphabetically last", "task", "normal", nil, "")

	nextSort = "id"
	nextDesc = true

	out := string(captureNextStdout(t, func() {
		if err := nextCmd.RunE(nextCmd, nil); err != nil {
			t.Fatalf("nextCmd.RunE() error = %v", err)
		}
	}))

	if !strings.Contains(out, last.ID) {
		t.Errorf("--sort id --desc did not pick the highest ID %q, output = %q", last.ID, out)
	}
	if strings.Contains(out, first.ID) {
		t.Errorf("--sort id --desc unexpectedly picked %q, output = %q", first.ID, out)
	}
}

// TestNextReportsNoneReadyWithFilters verifies that an empty result caused by
// a filter says so and names the filter, instead of reading like "there is no
// work left".
func TestNextReportsNoneReadyWithFilters(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		setupNextTest(t)
		resetNextFlags(t)
		mkReadyItem(t, "beans-nf01", "A ready task", "task", "high", nil, "")

		nextType = []string{"bug"}

		out := string(captureNextStdout(t, func() {
			if err := nextCmd.RunE(nextCmd, nil); err != nil {
				t.Fatalf("nextCmd.RunE() error = %v", err)
			}
		}))

		for _, want := range []string{"No ready items found matching --type bug.", "lif list --type bug"} {
			if !strings.Contains(out, want) {
				t.Errorf("empty-result message lacks %q, output = %q", want, out)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		setupNextTest(t)
		resetNextFlags(t)
		mkReadyItem(t, "beans-nf02", "A ready task", "task", "high", nil, "")

		nextType = []string{"bug"}
		nextJSON = true

		out := captureNextStdout(t, func() {
			if err := nextCmd.RunE(nextCmd, nil); err != nil {
				t.Fatalf("nextCmd.RunE() error = %v", err)
			}
		})

		var resp struct {
			Success bool   `json:"success"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			t.Fatalf("decoding JSON output: %v; output = %s", err, out)
		}
		if !resp.Success {
			t.Errorf("expected success=true, got %+v; output = %s", resp, out)
		}
		if want := "no ready items found matching --type bug"; resp.Message != want {
			t.Errorf("JSON message = %q, want %q", resp.Message, want)
		}
	})
}
