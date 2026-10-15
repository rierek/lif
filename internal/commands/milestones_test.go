package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// TestMilestonesListsEveryVisibleRank1Type verifies that collectMilestones
// selects by rank (D08), not by the hardcoded type name "milestone", and
// that a rank-1 type which opted out via Roadmap: false (D15) is excluded.
func TestMilestonesListsEveryVisibleRank1Type(t *testing.T) {
	rank := 1
	visible := false
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{
		{Name: "release", Rank: &rank},
		{Name: "bucket", Rank: &rank, Roadmap: &visible},
	}}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "r1", Type: "release", Status: "todo", Title: "v1"},
		{ID: "b1", Type: "bucket", Status: "todo", Title: "Parking lot"},
		{ID: "t1", Type: "task", Status: "todo", Title: "Work"},
	}

	got := collectMilestones(items, false)

	if len(got) != 1 {
		t.Fatalf("got %d entries, want 1 — release counts, bucket and task do not", len(got))
	}
	if got[0].ID != "r1" {
		t.Errorf("entry is %q, want \"r1\"", got[0].ID)
	}
}

// setupMilestonesTest installs a throwaway core and default config into the
// package globals milestonesCmd.RunE reads, mirroring setupUpdateTest.
func setupMilestonesTest(t *testing.T) {
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

// resetMilestonesFlags clears every milestones* package global the tests
// below touch and restores the pre-test values afterwards.
func resetMilestonesFlags(t *testing.T) {
	t.Helper()
	oldJSON, oldAll := milestonesJSON, milestonesAll
	oldView, oldMaxWidth, oldTags := milestonesView, milestonesMaxWidth, milestonesTags
	milestonesJSON, milestonesAll = false, false
	milestonesView, milestonesMaxWidth, milestonesTags = "table", 0, false
	t.Cleanup(func() {
		milestonesJSON, milestonesAll = oldJSON, oldAll
		milestonesView, milestonesMaxWidth, milestonesTags = oldView, oldMaxWidth, oldTags
	})
}

// milestonesCmdWithFlags returns a throwaway *cobra.Command carrying every
// flag RunE reads, bound to the same package-level vars as the real
// milestonesCmd singleton. milestonesCmd itself only gets its flags from
// RegisterMilestonesCmd, which production wiring calls but these tests
// don't, and RunE needs a cmd whose Flags().Changed("max-width") can be
// queried — mirrors createCmdWithOrderFlag.
func milestonesCmdWithFlags() *cobra.Command {
	c := &cobra.Command{Use: "milestones"}
	c.Flags().BoolVar(&milestonesJSON, "json", false, "")
	c.Flags().BoolVar(&milestonesAll, "all", false, "")
	c.Flags().StringVar(&milestonesView, "view", "table", "")
	c.Flags().IntVar(&milestonesMaxWidth, "max-width", 0, "")
	c.Flags().BoolVar(&milestonesTags, "tags", false, "")
	return c
}

// runMilestonesInTestStore seeds one active milestone with a descendant and
// one archived-status milestone, runs milestonesCmd with the given extra
// flags, and returns everything written to stdout.
func runMilestonesInTestStore(t *testing.T, args []string) string {
	t.Helper()
	setupMilestonesTest(t)
	resetMilestonesFlags(t)

	active := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Active milestone"), Title: "Active milestone", Status: "todo", Type: "milestone"}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Task"), Title: "Task", Status: "completed", Type: "task", Parent: "beans-mile1"}
	archived := &item.Item{ID: "beans-mile2", Slug: item.Slugify("an archived milestone"), Title: "an archived milestone", Status: "completed", Type: "milestone"}
	for _, b := range []*item.Item{active, task, archived} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	cmd := milestonesCmdWithFlags()
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatalf("parsing flags %v: %v", args, err)
	}

	return string(captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	}))
}

// runMilestonesWithProgress seeds a single milestone with total direct-child
// task items, done of which are completed, and returns the default (table)
// rendering. It exists to prove the progress counter's column width is
// measured from the actual data, not a hardcoded constant.
func runMilestonesWithProgress(t *testing.T, done, total int) string {
	t.Helper()
	setupMilestonesTest(t)
	resetMilestonesFlags(t)

	milestone := &item.Item{ID: "beans-milew", Slug: item.Slugify("Wide milestone"), Title: "Wide milestone", Status: "todo", Type: "milestone"}
	if err := core.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	for i := 0; i < total; i++ {
		status := "todo"
		if i < done {
			status = "completed"
		}
		task := &item.Item{
			ID:     fmt.Sprintf("beans-wt%d", i),
			Slug:   item.Slugify(fmt.Sprintf("wide task %d", i)),
			Title:  fmt.Sprintf("wide task %d", i),
			Status: status,
			Type:   "task",
			Parent: milestone.ID,
		}
		if err := core.Create(task); err != nil {
			t.Fatalf("core.Create(%s) error = %v", task.ID, err)
		}
	}

	cmd := milestonesCmdWithFlags()
	return string(captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(cmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	}))
}

// captureMilestonesStdout redirects os.Stdout for the duration of fn and
// returns everything written to it.
func captureMilestonesStdout(t *testing.T, fn func()) []byte {
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

// TestMilestonesListsByType verifies that only Type == "milestone" items
// appear in the output, not epics/tasks/features.
func TestMilestonesListsByType(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("A milestone"), Title: "A milestone", Status: "todo", Type: "milestone"}
	epic := &item.Item{ID: "beans-epic1", Slug: item.Slugify("An epic"), Title: "An epic", Status: "todo", Type: "epic"}
	if err := core.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	if err := core.Create(epic); err != nil {
		t.Fatalf("core.Create(epic) error = %v", err)
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	if !strings.Contains(got, "A milestone") {
		t.Errorf("expected output to contain milestone title, got %q", got)
	}
	if strings.Contains(got, "An epic") {
		t.Errorf("expected output to NOT contain epic title, got %q", got)
	}
}

// TestMilestonesShowsProgress verifies the "N/M completed" figure counts
// descendants (not just direct children), per the descendantProgress
// formula.
func TestMilestonesShowsProgress(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Milestone"), Title: "Milestone", Status: "todo", Type: "milestone"}
	epic := &item.Item{ID: "beans-epic1", Slug: item.Slugify("Epic"), Title: "Epic", Status: "todo", Type: "epic", Parent: "beans-mile1"}
	doneTask := &item.Item{ID: "beans-task1", Slug: item.Slugify("Done task"), Title: "Done task", Status: "completed", Type: "task", Parent: "beans-epic1"}
	todoTask := &item.Item{ID: "beans-task2", Slug: item.Slugify("Todo task"), Title: "Todo task", Status: "todo", Type: "task", Parent: "beans-epic1"}
	for _, b := range []*item.Item{milestone, epic, doneTask, todoTask} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	// 3 descendants total (epic + 2 tasks), 1 completed.
	if !strings.Contains(got, "1/3") {
		t.Errorf("expected output to contain 1/3 progress, got %q", got)
	}
}

// TestMilestonesExcludesCompletedScrappedByDefault verifies that completed
// and scrapped milestones are hidden unless --all is passed.
func TestMilestonesExcludesCompletedScrappedByDefault(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)

	active := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Active milestone"), Title: "Active milestone", Status: "todo", Type: "milestone"}
	done := &item.Item{ID: "beans-mile2", Slug: item.Slugify("Done milestone"), Title: "Done milestone", Status: "completed", Type: "milestone"}
	scrapped := &item.Item{ID: "beans-mile3", Slug: item.Slugify("Scrapped milestone"), Title: "Scrapped milestone", Status: "scrapped", Type: "milestone"}
	for _, b := range []*item.Item{active, done, scrapped} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	if !strings.Contains(got, "Active milestone") {
		t.Errorf("expected output to contain active milestone, got %q", got)
	}
	if strings.Contains(got, "Done milestone") {
		t.Errorf("expected output to NOT contain completed milestone by default, got %q", got)
	}
	if strings.Contains(got, "Scrapped milestone") {
		t.Errorf("expected output to NOT contain scrapped milestone by default, got %q", got)
	}
}

// TestMilestonesAllFlagIncludesThem verifies that --all includes
// completed/scrapped milestones.
func TestMilestonesAllFlagIncludesThem(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)
	milestonesAll = true

	active := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Active milestone"), Title: "Active milestone", Status: "todo", Type: "milestone"}
	done := &item.Item{ID: "beans-mile2", Slug: item.Slugify("Done milestone"), Title: "Done milestone", Status: "completed", Type: "milestone"}
	scrapped := &item.Item{ID: "beans-mile3", Slug: item.Slugify("Scrapped milestone"), Title: "Scrapped milestone", Status: "scrapped", Type: "milestone"}
	for _, b := range []*item.Item{active, done, scrapped} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	got := string(out)
	for _, want := range []string{"Active milestone", "Done milestone", "Scrapped milestone"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected output to contain %q with --all, got %q", want, got)
		}
	}
}

// TestMilestonesProgressExcludesHiddenSubtree verifies that a hidden
// container (D15, Roadmap: false) nested under a visible milestone does not
// leak its descendants into that milestone's completed/total count.
func TestMilestonesProgressExcludesHiddenSubtree(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)
	milestonesJSON = true

	hiddenRank := 2
	hidden := false
	cfg.Types = append(cfg.Types, config.TypeOverride{Name: "silo", Rank: &hiddenRank, Roadmap: &hidden})

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Milestone"), Title: "Milestone", Status: "todo", Type: "milestone"}
	silo := &item.Item{ID: "beans-silo1", Slug: item.Slugify("Silo"), Title: "Silo", Status: "todo", Type: "silo", Parent: "beans-mile1"}
	hiddenTask := &item.Item{ID: "beans-task1", Slug: item.Slugify("Hidden task"), Title: "Hidden task", Status: "completed", Type: "task", Parent: "beans-silo1"}
	visibleTask := &item.Item{ID: "beans-task2", Slug: item.Slugify("Visible task"), Title: "Visible task", Status: "todo", Type: "task", Parent: "beans-mile1"}
	for _, b := range []*item.Item{milestone, silo, hiddenTask, visibleTask} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	var entries []struct {
		Item      *item.Item `json:"item"`
		Completed int        `json:"completed"`
		Total     int        `json:"total"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("decoding JSON output: %v; output = %s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 milestone entry, got %d: %s", len(entries), out)
	}
	// Only the visible task counts; the hidden silo and its completed task
	// must not leak into the total or completed count.
	if entries[0].Total != 1 || entries[0].Completed != 0 {
		t.Errorf("expected completed=0 total=1 (hidden subtree excluded), got completed=%d total=%d", entries[0].Completed, entries[0].Total)
	}
}

// TestMilestonesJSONOutput verifies that --json emits a JSON array of
// {item, completed, total} entries, decodable via real stdout capture.
func TestMilestonesJSONOutput(t *testing.T) {
	setupMilestonesTest(t)
	resetMilestonesFlags(t)
	milestonesJSON = true

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("Milestone"), Title: "Milestone", Status: "todo", Type: "milestone"}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Task"), Title: "Task", Status: "completed", Type: "task", Parent: "beans-mile1"}
	for _, b := range []*item.Item{milestone, task} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := captureMilestonesStdout(t, func() {
		if err := milestonesCmd.RunE(milestonesCmd, nil); err != nil {
			t.Fatalf("milestonesCmd.RunE() error = %v", err)
		}
	})

	var entries []struct {
		Item      *item.Item `json:"item"`
		Completed int        `json:"completed"`
		Total     int        `json:"total"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatalf("decoding JSON output: %v; output = %s", err, out)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 milestone entry, got %d: %s", len(entries), out)
	}
	if entries[0].Item == nil || entries[0].Item.ID != milestone.ID {
		t.Errorf("expected bean %q in JSON entry, got %+v", milestone.ID, entries[0].Item)
	}
	if entries[0].Completed != 1 || entries[0].Total != 1 {
		t.Errorf("expected completed=1 total=1, got completed=%d total=%d", entries[0].Completed, entries[0].Total)
	}
}

// TestMilestonesShowsAProgressColumn verifies the shared layout engine
// renders a PROGRESS column with an n/m counter, not the old ad hoc
// "(n/m completed)" suffix.
func TestMilestonesShowsAProgressColumn(t *testing.T) {
	out := runMilestonesInTestStore(t, []string{})
	if !strings.Contains(out, "PROGRESS") {
		t.Errorf("milestones must carry a PROGRESS column:\n%s", out)
	}
	if !strings.Contains(out, "/") {
		t.Errorf("progress must show n/m:\n%s", out)
	}
}

// TestMilestonesStillHidesArchivedByDefault guards the one thing this task
// must not touch: the default filter already hid archive-status milestones
// before the engine migration, and it must survive it unchanged in both
// directions.
func TestMilestonesStillHidesArchivedByDefault(t *testing.T) {
	out := runMilestonesInTestStore(t, []string{})
	if strings.Contains(out, "an archived milestone") {
		t.Errorf("archived milestones must stay hidden without --all:\n%s", out)
	}
	all := runMilestonesInTestStore(t, []string{"--all"})
	if !strings.Contains(all, "an archived milestone") {
		t.Errorf("--all must bring archived milestones back:\n%s", all)
	}
}

// TestMilestonesCounterWidthFollowsTheData guards against the regression
// this plan already shipped once: a progress column hardcoded to a width of
// five tears a three-digit counter like 131/139 across the column boundary.
// The width must come from the widest n/m actually present in the rows.
func TestMilestonesCounterWidthFollowsTheData(t *testing.T) {
	out := runMilestonesWithProgress(t, 131, 139)
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "131/139") {
			return
		}
	}
	t.Errorf("the counter 131/139 did not survive on one line:\n%s", out)
}
