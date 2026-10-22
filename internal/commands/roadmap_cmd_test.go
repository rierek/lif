package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/roadmap"
	"github.com/spf13/cobra"
)

// setupRoadmapCmdTest installs a throwaway core+config into the package
// globals roadmapCmd.RunE reads and returns the core for item creation.
func setupRoadmapCmdTest(t *testing.T) *itemcore.Core {
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

	return testCore
}

// resetRoadmapFlags clears the roadmap* package globals these wiring tests
// touch, so a flag set in one test can't leak into the next.
func resetRoadmapFlags(t *testing.T) {
	t.Helper()
	oldStatus, oldNoStatus, oldJSON := roadmapStatus, roadmapNoStatus, roadmapJSON
	oldDepth, oldTags := roadmapDepth, roadmapTags
	oldView, oldFormat, oldWidth := roadmapView, roadmapFormat, roadmapWidthFlag
	roadmapStatus, roadmapNoStatus, roadmapJSON = nil, nil, false
	roadmapDepth, roadmapTags = 0, false
	// roadmapView's registered flag default is "tree", not the empty string
	// its zero value would give it -- seed that here so a RunE test doesn't
	// depend on some other test file having already called
	// RegisterRoadmapCmd (whose StringVar call has that side effect).
	roadmapView, roadmapFormat, roadmapWidthFlag = "tree", "", 0
	t.Cleanup(func() {
		roadmapStatus, roadmapNoStatus, roadmapJSON = oldStatus, oldNoStatus, oldJSON
		roadmapDepth, roadmapTags = oldDepth, oldTags
		roadmapView, roadmapFormat, roadmapWidthFlag = oldView, oldFormat, oldWidth
		roadmapCmd.SetOut(nil)
	})
}

func TestRoadmapCmdJSONScopesToEpicRoot(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	epic := &item.Item{ID: "beans-epic1", Slug: item.Slugify("Auth"), Title: "Auth", Status: "todo", Type: "epic"}
	if err := testCore.Create(epic); err != nil {
		t.Fatalf("core.Create(epic) error = %v", err)
	}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Login"), Title: "Login", Status: "todo", Type: "task", Parent: epic.ID}
	if err := testCore.Create(task); err != nil {
		t.Fatalf("core.Create(task) error = %v", err)
	}
	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("v1"), Title: "v1", Status: "todo", Type: "milestone"}
	if err := testCore.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}

	out := new(bytes.Buffer)
	roadmapCmd.SetOut(out)
	roadmapJSON = true

	if err := roadmapCmd.RunE(roadmapCmd, []string{epic.ID}); err != nil {
		t.Fatalf("roadmapCmd.RunE() error = %v", err)
	}

	var got roadmap.Data
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if got.Root == nil || got.Root.Epic == nil {
		t.Fatalf("expected Root.Epic to be set, got %+v", got.Root)
	}
	if got.Root.Epic.Epic.ID != epic.ID {
		t.Errorf("got epic %s, want %s", got.Root.Epic.Epic.ID, epic.ID)
	}
	if len(got.Milestones) != 0 {
		t.Errorf("expected no Milestones in scoped output, got %d", len(got.Milestones))
	}
}

func TestRoadmapCmdJSONScopesToMilestoneRoot(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("v1"), Title: "v1", Status: "todo", Type: "milestone"}
	if err := testCore.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Docs"), Title: "Docs", Status: "todo", Type: "task", Parent: milestone.ID}
	if err := testCore.Create(task); err != nil {
		t.Fatalf("core.Create(task) error = %v", err)
	}
	otherMilestone := &item.Item{ID: "beans-mile2", Slug: item.Slugify("v2"), Title: "v2", Status: "todo", Type: "milestone"}
	if err := testCore.Create(otherMilestone); err != nil {
		t.Fatalf("core.Create(otherMilestone) error = %v", err)
	}

	out := new(bytes.Buffer)
	roadmapCmd.SetOut(out)
	roadmapJSON = true

	if err := roadmapCmd.RunE(roadmapCmd, []string{milestone.ID}); err != nil {
		t.Fatalf("roadmapCmd.RunE() error = %v", err)
	}

	var got roadmap.Data
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(got.Milestones) != 1 || got.Milestones[0].Milestone.ID != milestone.ID {
		t.Fatalf("got Milestones = %+v, want exactly [%s]", got.Milestones, milestone.ID)
	}
	if got.Unscheduled != nil {
		t.Errorf("expected Unscheduled to be nil for a milestone scope, got %+v", got.Unscheduled)
	}
}

func TestRoadmapCmdRejectsNonContainerRootType(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	task := &item.Item{ID: "beans-task1", Slug: item.Slugify("Just a task"), Title: "Just a task", Status: "todo", Type: "task"}
	if err := testCore.Create(task); err != nil {
		t.Fatalf("core.Create(task) error = %v", err)
	}

	err := roadmapCmd.RunE(roadmapCmd, []string{task.ID})
	if err == nil {
		t.Fatal("expected an error for a task-typed root")
	}
	if !strings.Contains(err.Error(), "milestone, epic, feature") {
		t.Errorf("expected error to name the allowed types, got %q", err.Error())
	}
}

func TestRoadmapCmdRejectsUnknownID(t *testing.T) {
	setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	err := roadmapCmd.RunE(roadmapCmd, []string{"beans-doesnotexist"})
	if err == nil {
		t.Fatal("expected an error for an unknown item ID")
	}
	if !strings.Contains(err.Error(), "unknown item: beans-doesnotexist") {
		t.Errorf("expected error to be 'unknown item: ...', got %q", err.Error())
	}
}

func TestRoadmapCmdRejectsStatusFlagWithRootID(t *testing.T) {
	testCore := setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)

	milestone := &item.Item{ID: "beans-mile1", Slug: item.Slugify("v1"), Title: "v1", Status: "todo", Type: "milestone"}
	if err := testCore.Create(milestone); err != nil {
		t.Fatalf("core.Create(milestone) error = %v", err)
	}
	roadmapStatus = []string{"todo"}

	err := roadmapCmd.RunE(roadmapCmd, []string{milestone.ID})
	if err == nil {
		t.Fatal("expected an error combining a root ID with --status")
	}
	if !strings.Contains(err.Error(), "--status") {
		t.Errorf("expected error to mention --status, got %q", err.Error())
	}
}

// -- --format override (beans-dbph): roadmapOutput stays a pure function of
// its arguments -- these call it directly with an explicit isTTY bool, the
// same pattern roadmap_test.go already uses, rather than trying to fake a
// real terminal through RunE.

func TestRoadmapOutputFormatMarkdownOverridesTTY(t *testing.T) {
	data := roadmapOutputFixture()

	got := roadmapOutput(data, true, roadmapFormatMarkdown, 80, true, "", false, ui.FormTree, config.Default())
	if !strings.HasPrefix(got, "# Roadmap") {
		t.Errorf("got prefix %q, want %q (--format markdown must win over isTTY=true)", got[:min(len(got), 20)], "# Roadmap")
	}
	if !strings.Contains(got, "img.shields.io") {
		t.Error("expected markdown badges even though isTTY was true")
	}
}

func TestRoadmapOutputFormatTTYOverridesNonTTY(t *testing.T) {
	data := roadmapOutputFixture()

	got := roadmapOutput(data, false, roadmapFormatTTY, 80, true, "", false, ui.FormTree, config.Default())
	if !strings.HasPrefix(got, "Roadmap") {
		t.Errorf("got prefix %q, want %q (--format tty must win over isTTY=false)", got[:min(len(got), 20)], "Roadmap")
	}
	if strings.Contains(got, "img.shields.io") {
		t.Error("--format tty must not render markdown badges even though isTTY was false")
	}
}

// TestRoadmapOutputFormatAutoPreservesDetection guards against this step
// shifting the no-flag behaviour: roadmapFormatAuto (the zero value, what
// RunE passes when --format was never given) must produce byte-identical
// output to explicitly requesting the branch isTTY alone would pick -- the
// TTY branch via ui.Render (roadmap.go's TTY branch since beans-dbph Step B,
// which replaced the bespoke renderer this test used to compare against),
// the non-TTY branch via renderRoadmapMarkdown unchanged.
func TestRoadmapOutputFormatAutoPreservesDetection(t *testing.T) {
	data := roadmapOutputFixture()
	cfg := config.Default()

	if got, want := roadmapOutput(data, true, roadmapFormatAuto, 80, true, "", false, ui.FormTree, cfg),
		ui.Render(roadmapRows(data), ui.FormTree, "Roadmap", 80, false, cfg); got != want {
		t.Errorf("roadmapFormatAuto with isTTY=true diverged from ui.Render:\ngot:  %q\nwant: %q", got, want)
	}
	if got, want := roadmapOutput(data, false, roadmapFormatAuto, 0, true, "", false, ui.FormTree, cfg),
		renderRoadmapMarkdown(data, true, "", false); got != want {
		t.Errorf("roadmapFormatAuto with isTTY=false diverged from renderRoadmapMarkdown:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestRoadmapCmdRejectsInvalidView(t *testing.T) {
	setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)
	roadmapView = "bogus"

	err := roadmapCmd.RunE(roadmapCmd, nil)
	if err == nil {
		t.Fatal("expected an error for an invalid --view value")
	}
	if !strings.Contains(err.Error(), "tree") || !strings.Contains(err.Error(), "table") {
		t.Errorf("expected error to name both valid --view values (tree, table), got %q", err.Error())
	}
}

func TestRoadmapCmdRejectsInvalidFormat(t *testing.T) {
	setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)
	roadmapFormat = "bogus"

	err := roadmapCmd.RunE(roadmapCmd, nil)
	if err == nil {
		t.Fatal("expected an error for an invalid --format value")
	}
	if !strings.Contains(err.Error(), "tty") || !strings.Contains(err.Error(), "markdown") {
		t.Errorf("expected error to name both valid --format values (tty, markdown), got %q", err.Error())
	}
}

// -- TTY branch through the shared layout engine (beans-dbph Step B):
// roadmapOutput's TTY branch now calls ui.Render(roadmapRows(data), ...)
// instead of a bespoke roadmap-only renderer. These guard the bridge itself
// (roadmapRows) and the two ui.Form outcomes it feeds.

func TestRoadmapCmdTreeViewRendersConnectors(t *testing.T) {
	data := roadmapOutputFixture()

	got := roadmapOutput(data, true, roadmapFormatTTY, 90, true, "", false, ui.FormTree, config.Default())
	if !strings.Contains(got, "└─") {
		t.Errorf("expected tree form to draw a connector (└─) for the nested task, got %q", got)
	}
}

func TestRoadmapCmdTableViewRendersHeaderNoConnectors(t *testing.T) {
	data := roadmapOutputFixture()

	got := roadmapOutput(data, true, roadmapFormatTTY, 90, true, "", false, ui.FormTable, config.Default())
	if !strings.Contains(got, "TITLE") {
		t.Errorf("expected table form to render a column header (TITLE), got %q", got)
	}
	if strings.Contains(got, "└─") || strings.Contains(got, "├─") {
		t.Errorf("table form must not draw tree connectors, got %q", got)
	}
}

// roadmapGroupingFixture builds two milestones plus an unscheduled bucket, so
// a test can check that the milestone/unscheduled grouping survives being
// flattened into ui.FormTable rows (which drop Depth entirely -- Section is
// what carries the grouping across that flattening, per ui/columns.go).
func roadmapGroupingFixture() *roadmap.Data {
	now := time.Now()
	m1 := &item.Item{ID: "beans-m1", Type: "milestone", Title: "v1.0", Status: "todo", CreatedAt: &now, Path: "m1--v10.md"}
	m2 := &item.Item{ID: "beans-m2", Type: "milestone", Title: "v2.0", Status: "todo", CreatedAt: &now, Path: "m2--v20.md"}
	t1 := &item.Item{ID: "beans-t1", Type: "task", Title: "In v1", Status: "todo", Path: "t1--in-v1.md"}
	t2 := &item.Item{ID: "beans-t2", Type: "task", Title: "In v2", Status: "todo", Path: "t2--in-v2.md"}
	orphan := &item.Item{ID: "beans-o1", Type: "task", Title: "Orphan", Status: "todo", Path: "o1--orphan.md"}
	return &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{
			{Milestone: m1, Other: []*item.Item{t1}},
			{Milestone: m2, Other: []*item.Item{t2}},
		},
		Unscheduled: &roadmap.UnscheduledGroup{Other: []*item.Item{orphan}},
	}
}

func TestRoadmapCmdMilestoneGroupingSurvivesTableFlattening(t *testing.T) {
	data := roadmapGroupingFixture()

	got := roadmapOutput(data, true, roadmapFormatTTY, 90, true, "", false, ui.FormTable, config.Default())
	for _, want := range []string{"beans-m1", "beans-m2", "beans-o1", "Unscheduled"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected table output to contain %q, got %q", want, got)
		}
	}
	// The unscheduled bucket's heading must come after both milestones --
	// otherwise "Unscheduled" is floating free of the group it labels.
	m2Idx := strings.Index(got, "beans-m2")
	headingIdx := strings.Index(got, "Unscheduled")
	if m2Idx < 0 || headingIdx < 0 || headingIdx < m2Idx {
		t.Errorf("expected \"Unscheduled\" heading after both milestone rows, got %q", got)
	}
}

// TestRoadmapOutputNoLineExceedsWidth sweeps cols across the range
// resolveWidth can actually produce -- at and around its 80-column floor,
// and well above the old 110-column default cap -- rather than only trying
// comfortably-wide values, which is exactly the kind of guard-never-fired
// false pass this plan has already had to remove once (see the task
// report). It stops at 79 rather than sweeping further below 80: cols under
// resolveWidth's floor are not a case roadmapOutput itself is responsible
// for any more (see TestRoadmapOutputMaxWidthAboveDefaultCapIsHonoured --
// roadmapOutput is a pure pass-through, resolveWidth is the one place the
// width is decided), and pathologically narrow widths are the one case
// ui.Render documents itself as accepting overflow on (minRenderableTitle),
// which is ui's own contract to test, not roadmap.go's wiring. The fixture
// nests three levels deep (milestone -> epic -> feature -> task) with long
// titles and tags so the column budget is actually under pressure here.
func TestRoadmapOutputNoLineExceedsWidth(t *testing.T) {
	now := time.Now()
	m := &item.Item{ID: "beans-m1", Type: "milestone", Title: "A moderately long milestone title", Status: "todo", CreatedAt: &now, Tags: []string{"backend", "urgent"}, Path: "m1.md"}
	e := &item.Item{ID: "beans-e1", Type: "epic", Title: "A moderately long epic title", Status: "todo", Tags: []string{"auth"}, Path: "e1.md"}
	f := &item.Item{ID: "beans-f1", Type: "feature", Title: "A moderately long feature title", Status: "todo", Priority: "high", Path: "f1.md"}
	leaf := &item.Item{ID: "beans-t1", Type: "task", Title: "A moderately long leaf title", Status: "in-progress", Priority: "critical", Tags: []string{"blocked"}, Path: "t1.md"}
	data := &roadmap.Data{
		Milestones: []roadmap.MilestoneGroup{
			{
				Milestone: m,
				Containers: []roadmap.Container{
					{Epic: &roadmap.EpicGroup{Epic: e, Features: []roadmap.FeatureGroup{{Feature: f, Items: []*item.Item{leaf}}}}},
				},
			},
		},
	}

	cols := []int{79, 80, 81, 95, 109, 110, 111, 150, 200, 500}
	for _, form := range []ui.Form{ui.FormTree, ui.FormTable} {
		for _, c := range cols {
			got := roadmapOutput(data, true, roadmapFormatTTY, c, true, "", true, form, config.Default())
			lines := strings.Split(got, "\n")
			for _, line := range lines {
				if w := ui.DisplayWidth(line); w > c {
					t.Errorf("form=%v cols=%d: line exceeds width (%d): %q", form, c, w, line)
				}
			}
			// The divider (second line, both forms) is exactly `width`
			// dashes -- pinning it to the raw `c` (not a clamped value)
			// proves roadmapOutput passes cols straight through to
			// ui.Render rather than re-capping what resolveWidth already
			// decided.
			if len(lines) < 2 {
				t.Fatalf("form=%v cols=%d: expected at least 2 lines, got %d", form, c, len(lines))
			}
			if w := ui.DisplayWidth(lines[1]); w != c {
				t.Errorf("form=%v cols=%d: divider width = %d, want %d", form, c, w, c)
			}
		}
	}
}

// TestRoadmapOutputMaxWidthAboveDefaultCapIsHonoured guards the removal of
// roadmapOutput's own clamp (beans-dbph Step B follow-up): resolveWidth is
// the one place the render width is decided -- it floors at 80 but has no
// upper cap, so --max-width can push a caller past the 110-column default.
// Before this fix roadmapOutput ran cols through a roadmap-only clamp on
// top of resolveWidth's own decision, silently
// re-capping roadmap at 110 while every other width-aware command (list,
// show, ...) honoured --max-width up to whatever the user asked for -- two
// commands given the same flag, quietly disagreeing.
func TestRoadmapOutputMaxWidthAboveDefaultCapIsHonoured(t *testing.T) {
	data := roadmapOutputFixture()

	// 200 > the 110-column default cap: this is exactly the case resolveWidth's
	// own doc comment describes -- a caller explicitly asking for more than
	// the default cap via --max-width, which resolveWidth grants unless a
	// narrower terminal is detected (none is, under go test).
	got := roadmapOutput(data, true, roadmapFormatTTY, 200, true, "", false, ui.FormTree, config.Default())

	lines := strings.SplitN(got, "\n", 3)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %q", len(lines), got)
	}
	if w := ui.DisplayWidth(lines[1]); w != 200 {
		t.Errorf("divider width = %d, want 200 -- roadmap must not silently re-cap what resolveWidth already decided", w)
	}
}

// captureRoadmapJSONError runs roadmapCmd.RunE(cmd, args) with roadmapJSON
// true, capturing os.Stdout. cmdError's JSON branch writes through
// output.Error -> output.JSON, which encodes straight to os.Stdout
// (internal/output/output.go), not through cmd.OutOrStdout() -- unlike the
// success path (roadmap.go's json.NewEncoder(cmd.OutOrStdout())), so a
// cmd.SetOut buffer would see nothing on an error path and this swap is the
// only way to observe the envelope.
func captureRoadmapJSONError(t *testing.T, cmd *cobra.Command, args []string) (runErr error, resp output.Response) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr = roadmapCmd.RunE(cmd, args)

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}

	if runErr == nil {
		return runErr, resp
	}
	dec := json.NewDecoder(bytes.NewReader(captured))
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decoding JSON output error = %v; output = %s", err, captured)
	}
	return runErr, resp
}

// depthOnlyCmd builds a throwaway *cobra.Command carrying only a "depth"
// int flag, set to 0 and marked Changed -- the one shape RunE needs to
// exercise validateRoadmapDepth's "changed && depth < 1" branch. A fresh
// command is used instead of roadmapCmd itself because pflag's Changed bit
// is sticky: setting it on the shared roadmapCmd would leak into every
// later test in this file, whereas a command built and discarded inside a
// single test case cannot leak anywhere.
func depthOnlyCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().IntVar(&roadmapDepth, "depth", 0, "")
	if err := cmd.Flags().Set("depth", "0"); err != nil {
		t.Fatalf(`cmd.Flags().Set("depth", "0") error = %v`, err)
	}
	return cmd
}

// roadmapJSONErrorCases enumerates every RunE failure path (beans-9vqb
// Acceptance: "every failure path ... emits the standard JSON error
// envelope"), not only the two the item's Guard section names as the
// minimum. Each row's wantCode also fixes the error-code convention this
// fix must follow: ErrNotFound for a missing root item (matching
// progress.go/update.go/delete.go/order.go's "bean not found" handling),
// ErrValidation for every flag/shape validation (matching
// list.go/milestones.go/next.go/progress.go's "querying items" and
// unknown-flag-value handling). cmd defaults to roadmapCmd when nil; only
// the depth case needs an isolated command (see depthOnlyCmd).
func roadmapJSONErrorCases(t *testing.T) []struct {
	name     string
	cmd      func(t *testing.T) *cobra.Command
	args     func(core *itemcore.Core) []string
	wantCode string
} {
	t.Helper()
	return []struct {
		name     string
		cmd      func(t *testing.T) *cobra.Command
		args     func(core *itemcore.Core) []string
		wantCode string
	}{
		{
			name: "invalid view",
			args: func(core *itemcore.Core) []string {
				roadmapView = "bogus"
				return nil
			},
			wantCode: output.ErrValidation,
		},
		{
			name: "invalid format",
			args: func(core *itemcore.Core) []string {
				roadmapFormat = "bogus"
				return nil
			},
			wantCode: output.ErrValidation,
		},
		{
			name: "invalid depth",
			cmd:  depthOnlyCmd,
			args: func(core *itemcore.Core) []string {
				return nil
			},
			wantCode: output.ErrValidation,
		},
		{
			name: "status mutex with root id",
			args: func(core *itemcore.Core) []string {
				root := &item.Item{ID: "beans-mutex1", Slug: item.Slugify("Root"), Title: "Root", Status: "todo", Type: "milestone"}
				if err := core.Create(root); err != nil {
					t.Fatalf("core.Create(root) error = %v", err)
				}
				roadmapStatus = []string{"todo"}
				return []string{root.ID}
			},
			wantCode: output.ErrValidation,
		},
		{
			name: "unknown root id",
			args: func(core *itemcore.Core) []string {
				return []string{"beans-doesnotexist"}
			},
			wantCode: output.ErrNotFound,
		},
		{
			name: "invalid root type",
			args: func(core *itemcore.Core) []string {
				task := &item.Item{ID: "beans-badroot1", Slug: item.Slugify("Just a task"), Title: "Just a task", Status: "todo", Type: "task"}
				if err := core.Create(task); err != nil {
					t.Fatalf("core.Create(task) error = %v", err)
				}
				return []string{task.ID}
			},
			wantCode: output.ErrValidation,
		},
	}
}

func TestRoadmapCmdJSONErrorEnvelope(t *testing.T) {
	cases := roadmapJSONErrorCases(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testCore := setupRoadmapCmdTest(t)
			resetRoadmapFlags(t)
			roadmapJSON = true

			args := tc.args(testCore)
			cmd := roadmapCmd
			if tc.cmd != nil {
				cmd = tc.cmd(t)
			}

			runErr, resp := captureRoadmapJSONError(t, cmd, args)
			if runErr == nil {
				t.Fatal("roadmapCmd.RunE() expected error, got nil")
			}
			if resp.Success {
				t.Errorf("response success = true, want false")
			}
			if resp.Code != tc.wantCode {
				t.Errorf("response code = %q, want %q", resp.Code, tc.wantCode)
			}
			if resp.Error == "" {
				t.Errorf("response error message is empty")
			}
		})
	}
}

// TestRoadmapCmdInvalidViewWithoutJSONStaysPlainText is the Acceptance
// criterion's non-regression half: "the plain-text form is unchanged when
// --json is absent". It asserts the exact pre-fix message text and that
// nothing resembling the JSON envelope leaks onto stdout when --json was
// never set.
func TestRoadmapCmdInvalidViewWithoutJSONStaysPlainText(t *testing.T) {
	setupRoadmapCmdTest(t)
	resetRoadmapFlags(t)
	roadmapView = "bogus"

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := roadmapCmd.RunE(roadmapCmd, nil)

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}

	if runErr == nil {
		t.Fatal("expected an error for an invalid --view")
	}
	want := `invalid --view "bogus": must be one of "tree", "table"`
	if runErr.Error() != want {
		t.Errorf("error = %q, want %q", runErr.Error(), want)
	}
	if len(captured) != 0 {
		t.Errorf("expected nothing written to stdout without --json, got %q", captured)
	}
}
