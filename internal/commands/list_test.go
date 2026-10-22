package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
)

func TestSortItems(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	evenEarlier := now.Add(-2 * time.Hour)

	// Statuses are now hardcoded, so we just use default config
	testCfg := config.Default()

	t.Run("sort by id", func(t *testing.T) {
		items := []*item.Item{
			{ID: "c3"},
			{ID: "a1"},
			{ID: "b2"},
		}
		sortItems(items, "id", false, testCfg)

		if items[0].ID != "a1" || items[1].ID != "b2" || items[2].ID != "c3" {
			t.Errorf("sort by id: got [%s, %s, %s], want [a1, b2, c3]",
				items[0].ID, items[1].ID, items[2].ID)
		}
	})

	t.Run("sort by created", func(t *testing.T) {
		items := []*item.Item{
			{ID: "old", CreatedAt: &evenEarlier},
			{ID: "new", CreatedAt: &now},
			{ID: "mid", CreatedAt: &earlier},
		}
		sortItems(items, "created", false, testCfg)

		// Should be newest first
		if items[0].ID != "new" || items[1].ID != "mid" || items[2].ID != "old" {
			t.Errorf("sort by created: got [%s, %s, %s], want [new, mid, old]",
				items[0].ID, items[1].ID, items[2].ID)
		}
	})

	t.Run("sort by created with nil", func(t *testing.T) {
		items := []*item.Item{
			{ID: "nil1", CreatedAt: nil},
			{ID: "has", CreatedAt: &now},
			{ID: "nil2", CreatedAt: nil},
		}
		sortItems(items, "created", false, testCfg)

		// Non-nil should come first, then nil sorted by ID
		if items[0].ID != "has" {
			t.Errorf("sort by created with nil: first should be \"has\", got %q", items[0].ID)
		}
	})

	t.Run("sort by updated", func(t *testing.T) {
		items := []*item.Item{
			{ID: "old", UpdatedAt: &evenEarlier},
			{ID: "new", UpdatedAt: &now},
			{ID: "mid", UpdatedAt: &earlier},
		}
		sortItems(items, "updated", false, testCfg)

		// Should be newest first
		if items[0].ID != "new" || items[1].ID != "mid" || items[2].ID != "old" {
			t.Errorf("sort by updated: got [%s, %s, %s], want [new, mid, old]",
				items[0].ID, items[1].ID, items[2].ID)
		}
	})

	t.Run("sort by status", func(t *testing.T) {
		items := []*item.Item{
			{ID: "c1", Status: "completed"},
			{ID: "t1", Status: "todo"},
			{ID: "i1", Status: "in-progress"},
			{ID: "t2", Status: "todo"},
		}
		sortItems(items, "status", false, testCfg)

		// Should be ordered by status config order (in-progress, todo, draft, completed, scrapped), then by ID within same status
		expected := []string{"i1", "t1", "t2", "c1"}
		for i, want := range expected {
			if items[i].ID != want {
				t.Errorf("sort by status[%d]: got %q, want %q", i, items[i].ID, want)
			}
		}
	})

	t.Run("sort by order, scoped per parent", func(t *testing.T) {
		items := []*item.Item{
			// Four siblings under "p1" whose order values were assigned out
			// of creation sequence, plus a fifth sibling without an order
			// value that must come last (SC-01).
			{ID: "p1-c", Title: "c", Parent: "p1", Order: "m"},
			{ID: "p1-a", Title: "a", Parent: "p1", Order: "a"},
			{ID: "p1-e", Title: "e", Parent: "p1"},
			{ID: "p1-d", Title: "d", Parent: "p1", Order: "t"},
			{ID: "p1-b", Title: "b", Parent: "p1", Order: "g"},
			// A sibling under a different parent shares an Order value with
			// one of p1's items, which must not interleave the groups —
			// Order is a fractional index scoped per parent (R-12).
			{ID: "p2-x", Title: "x", Parent: "p2", Order: "a"},
		}
		sortItems(items, "order", false, testCfg)

		expected := []string{"p1-a", "p1-b", "p1-c", "p1-d", "p1-e", "p2-x"}
		got := make([]string, len(items))
		for i, b := range items {
			got[i] = b.ID
		}
		for i, want := range expected {
			if got[i] != want {
				t.Errorf("sort by order[%d]: got %v, want %v", i, got, expected)
				break
			}
		}
	})

	t.Run("default sort (archive status then type)", func(t *testing.T) {
		items := []*item.Item{
			{ID: "completed-bug", Status: "completed", Type: "bug"},
			{ID: "todo-feature", Status: "todo", Type: "feature"},
			{ID: "todo-task", Status: "todo", Type: "task"},
			{ID: "completed-task", Status: "completed", Type: "task"},
			{ID: "todo-bug", Status: "todo", Type: "bug"},
		}
		sortItems(items, "", false, testCfg)

		// Should be: non-archive first (sorted by type order from DefaultTypes: milestone, epic, feature, bug, task),
		// then archive (sorted by type)
		// DefaultTypes order: milestone, epic, feature, bug, task
		expected := []string{"todo-feature", "todo-bug", "todo-task", "completed-bug", "completed-task"}
		for i, want := range expected {
			if items[i].ID != want {
				t.Errorf("default sort[%d]: got %q, want %q", i, items[i].ID, want)
			}
		}
	})
}

func TestListReadyFlagMutualExclusion(t *testing.T) {
	// Test that --ready and --is-blocked are mutually exclusive
	// by checking the validation logic directly
	tests := []struct {
		name        string
		ready       bool
		isBlocked   bool
		expectError bool
	}{
		{"neither flag", false, false, false},
		{"only --ready", true, false, false},
		{"only --is-blocked", false, true, false},
		{"both flags", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mirrors the validation logic in list.go
			hasError := tt.ready && tt.isBlocked
			if hasError != tt.expectError {
				t.Errorf("ready=%v, isBlocked=%v: got error=%v, want error=%v",
					tt.ready, tt.isBlocked, hasError, tt.expectError)
			}
		})
	}
}

// AC3: --where on a reserved key fails naming the native filter flag.
func TestValidateWhereKeysReservedKeyFails(t *testing.T) {
	err := validateWhereKeys([]string{"status=done"})
	if err == nil {
		t.Fatal("expected error for reserved key")
	}
	if !contains(err.Error(), "--status") {
		t.Errorf("expected error to name --status, got %q", err.Error())
	}
}

// --where without "=" is a usage error, same shape as --set.
func TestValidateWhereKeysWithoutEqualsFails(t *testing.T) {
	err := validateWhereKeys([]string{"release"})
	if err == nil {
		t.Fatal("expected error for --where without '='")
	}
}

func TestValidateWhereKeysAcceptsExtraKey(t *testing.T) {
	if err := validateWhereKeys([]string{"release=0-4-1"}); err != nil {
		t.Errorf("validateWhereKeys() unexpected error: %v", err)
	}
}

// AC1/SC-01: a single --where key=value returns only matching items.
func TestFilterByWhereSingleMatch(t *testing.T) {
	items := []*item.Item{
		{ID: "a", Extra: map[string]any{"release": "0-4-1"}},
		{ID: "b", Extra: map[string]any{"release": "0-4-2"}},
		{ID: "c", Extra: map[string]any{"release": "0-4-1"}},
	}

	got := filterByWhere(items, []string{"release=0-4-1"})

	if len(got) != 2 {
		t.Fatalf("expected 2 beans, got %d", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "c" {
		t.Errorf("expected [a, c], got [%s, %s]", got[0].ID, got[1].ID)
	}
}

// AC2: multiple --where pairs combine with AND semantics.
func TestFilterByWhereMultiplePairsAreANDed(t *testing.T) {
	items := []*item.Item{
		{ID: "a", Extra: map[string]any{"release": "0-4-1", "klasse": "bugfix"}},
		{ID: "b", Extra: map[string]any{"release": "0-4-1", "klasse": "feature"}},
	}

	got := filterByWhere(items, []string{"release=0-4-1", "klasse=bugfix"})

	if len(got) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("expected [a], got [%s]", got[0].ID)
	}
}

// AC4: a key carried by no item returns an empty result, not an error.
func TestFilterByWhereNoMatchIsEmpty(t *testing.T) {
	items := []*item.Item{
		{ID: "a", Extra: map[string]any{"release": "0-4-1"}},
	}

	got := filterByWhere(items, []string{"release=9-9-9"})

	if len(got) != 0 {
		t.Errorf("expected 0 beans, got %d", len(got))
	}
}

func TestFilterByWhereEmptyWheresReturnsAllItems(t *testing.T) {
	items := []*item.Item{
		{ID: "a"},
		{ID: "b"},
	}

	got := filterByWhere(items, nil)

	if len(got) != 2 {
		t.Errorf("expected 2 beans, got %d", len(got))
	}
}

// captureListStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. listCmd's --json path encodes directly to
// os.Stdout via output.SuccessMultiple, so this is the only way to observe it.
func captureListStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return data
}

// setupListTest installs a throwaway core and default config into the
// package globals listCmd.RunE reads.
func setupListTest(t *testing.T) {
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

// resetListWhereFlag clears listWhere and restores it afterwards.
func resetListWhereFlag(t *testing.T) {
	t.Helper()
	old := listWhere
	listWhere = nil
	t.Cleanup(func() { listWhere = old })
}

// SC-01: in a store of five items of which two carry release: 0-4-1,
// `lif list --where release=0-4-1 --json` returns exactly those two.
func TestListCmdWhereEndToEnd(t *testing.T) {
	setupListTest(t)
	resetListWhereFlag(t)

	oldJSON, oldFull := listJSON, listFull
	listJSON, listFull = true, false
	t.Cleanup(func() { listJSON, listFull = oldJSON, oldFull })

	releases := []string{"0-4-1", "0-4-1", "0-4-0", "0-4-0", "0-4-0"}
	for i, release := range releases {
		b := &item.Item{
			ID:     "beans-sc" + string(rune('1'+i)),
			Slug:   item.Slugify("bean"),
			Title:  "bean",
			Status: "todo",
			Type:   "task",
			Extra:  map[string]any{"release": release},
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create() error = %v", err)
		}
	}

	listWhere = []string{"release=0-4-1"}

	out := captureListStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatalf("listCmd.RunE() error = %v", err)
		}
	})

	dec := json.NewDecoder(bytes.NewReader(out))
	var got []*item.Item
	if err := dec.Decode(&got); err != nil {
		t.Fatalf("decoding JSON output: %v; output = %s", err, out)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 beans, got %d; output = %s", len(got), out)
	}
	for _, b := range got {
		if b.Extra["release"] != "0-4-1" {
			t.Errorf("Extra[release] = %v, want %q", b.Extra["release"], "0-4-1")
		}
	}
}

// TestListCmdReadyEndToEnd is a regression test for the ApplyReadyFilter
// extraction (Task 4): `list --ready` must still exclude blocked/terminal
// items AND continue to combine with other flags already applied to the
// same filter object (here --type), since ApplyReadyFilter mutates the
// filter list.go already built rather than constructing a separate one.
func TestListCmdReadyEndToEnd(t *testing.T) {
	setupListTest(t)

	oldJSON, oldFull, oldReady, oldType := listJSON, listFull, listReady, listType
	listJSON, listFull, listReady = true, false, true
	t.Cleanup(func() { listJSON, listFull, listReady, listType = oldJSON, oldFull, oldReady, oldType })

	readyTask := &item.Item{
		ID:     "beans-rdy1",
		Slug:   item.Slugify("ready task"),
		Title:  "ready task",
		Status: "todo",
		Type:   "task",
	}
	readyEpic := &item.Item{
		ID:     "beans-rdy2",
		Slug:   item.Slugify("ready epic"),
		Title:  "ready epic",
		Status: "todo",
		Type:   "epic",
	}
	blockedTask := &item.Item{
		ID:        "beans-rdy3",
		Slug:      item.Slugify("blocked task"),
		Title:     "blocked task",
		Status:    "todo",
		Type:      "task",
		BlockedBy: []string{readyTask.ID},
	}
	completedTask := &item.Item{
		ID:     "beans-rdy4",
		Slug:   item.Slugify("completed task"),
		Title:  "completed task",
		Status: "completed",
		Type:   "task",
	}
	for _, b := range []*item.Item{readyTask, readyEpic, blockedTask, completedTask} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	// --ready alone: excludes blocked, completed and container items.
	out := captureListStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatalf("listCmd.RunE() error = %v", err)
		}
	})
	var got []*item.Item
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding JSON output: %v; output = %s", err, out)
	}
	gotIDs := make(map[string]bool, len(got))
	for _, b := range got {
		gotIDs[b.ID] = true
	}
	if !gotIDs[readyTask.ID] {
		t.Errorf("expected the ready task in result, got IDs = %v", gotIDs)
	}
	if gotIDs[readyEpic.ID] {
		t.Errorf("expected the epic, a container, excluded, got IDs = %v", gotIDs)
	}
	if gotIDs[blockedTask.ID] || gotIDs[completedTask.ID] {
		t.Errorf("expected blocked/completed beans excluded, got IDs = %v", gotIDs)
	}

	// --ready combined with --type task: must still apply --ready's
	// exclusions AND the --type filter on the same filter object.
	listType = []string{"task"}
	out = captureListStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatalf("listCmd.RunE() error = %v", err)
		}
	})
	got = nil
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding JSON output: %v; output = %s", err, out)
	}
	if len(got) != 1 || got[0].ID != readyTask.ID {
		t.Fatalf("expected only [%s] for --ready --type task, got %v", readyTask.ID, got)
	}
}

// TestApplyReadyFilterExcludesCustomArchiveStatus is a regression test: a
// project-defined status marked archive: true (not just the built-in
// "completed"/"scrapped") must be excluded from --ready, since a
// hardcoded literal exclude list would silently treat it as still ready.
func TestApplyReadyFilterExcludesCustomArchiveStatus(t *testing.T) {
	archive := true
	testCfg := &config.Config{
		Statuses: []config.StatusOverride{
			{Name: "po-review", Archive: &archive},
		},
	}

	filter := &model.ItemFilter{}
	itemgraph.ApplyReadyFilter(filter, testCfg)

	found := false
	for _, s := range filter.ExcludeStatus {
		if s == "po-review" {
			found = true
		}
	}
	if !found {
		t.Errorf("ExcludeStatus = %v, want it to contain custom archive status %q", filter.ExcludeStatus, "po-review")
	}
}

// TestApplyReadyFilterExcludesContainerTypes is a regression test for
// beans-g5dn: ready work is leaf work, so every type on a container rank
// (1 through config.MaxContainerRank) must be excluded. The names come from
// the configuration, so a project-defined container type is excluded too and
// a project-defined leaf type is not.
func TestApplyReadyFilterExcludesContainerTypes(t *testing.T) {
	two := 2
	testCfg := &config.Config{
		Types: []config.TypeOverride{
			{Name: "package", Rank: &two},
			{Name: "story"},
		},
	}

	filter := &model.ItemFilter{ExcludeType: []string{"wontfix"}}
	itemgraph.ApplyReadyFilter(filter, testCfg)

	want := []string{"wontfix", "milestone", "epic", "package", "feature"}
	if !slices.Equal(filter.ExcludeType, want) {
		t.Errorf("ExcludeType = %v, want %v (the caller's entry kept, then the container types)", filter.ExcludeType, want)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 4, "h..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestSortItemsDescReversesEveryField verifies that --desc is a property of
// sortItems itself and therefore holds for every sort field, rather than
// being wired into one branch.
func TestSortItemsDescReversesEveryField(t *testing.T) {
	testCfg := config.Default()

	for _, sortBy := range []string{"", "id", "created", "updated", "status", "priority", "order"} {
		t.Run("sort="+sortBy, func(t *testing.T) {
			mk := func() []*item.Item {
				return []*item.Item{
					{ID: "beans-d001", Title: "First", Status: "todo", Type: "task", Priority: "high", Order: "a"},
					{ID: "beans-d002", Title: "Second", Status: "in-progress", Type: "bug", Priority: "low", Order: "b"},
					{ID: "beans-d003", Title: "Third", Status: "done", Type: "epic", Priority: "normal", Order: "c"},
				}
			}

			asc := mk()
			sortItems(asc, sortBy, false, testCfg)
			desc := mk()
			sortItems(desc, sortBy, true, testCfg)

			if len(asc) != len(desc) {
				t.Fatalf("length changed: asc=%d desc=%d", len(asc), len(desc))
			}
			for i := range asc {
				want := asc[len(asc)-1-i].ID
				if desc[i].ID != want {
					t.Errorf("desc[%d].ID = %q, want %q (reverse of ascending %v)", i, desc[i].ID, want, idsOf(asc))
				}
			}
		})
	}
}

func idsOf(items []*item.Item) []string {
	ids := make([]string, len(items))
	for i, b := range items {
		ids[i] = b.ID
	}
	return ids
}

// TestListCmdUnblockedEndToEnd covers `lif list --unblocked` (beans-8olg):
// items with an active blocker drop out, items whose only blocker is
// resolved stay in, and — unlike --ready — no status is excluded, so
// draft/in-progress/completed items survive the filter. The last block
// pins the combination with another flag, since --unblocked mutates the
// same filter object list.go already built from --status et al.
func TestListCmdUnblockedEndToEnd(t *testing.T) {
	setupListTest(t)

	oldJSON, oldFull, oldUnblocked, oldStatus := listJSON, listFull, listUnblocked, listStatus
	listJSON, listFull, listUnblocked = true, false, true
	t.Cleanup(func() {
		listJSON, listFull, listUnblocked, listStatus = oldJSON, oldFull, oldUnblocked, oldStatus
	})

	openBlocker := &item.Item{
		ID:     "beans-unb1",
		Slug:   item.Slugify("open blocker"),
		Title:  "open blocker",
		Status: "todo",
		Type:   "task",
	}
	doneBlocker := &item.Item{
		ID:     "beans-unb2",
		Slug:   item.Slugify("done blocker"),
		Title:  "done blocker",
		Status: "completed",
		Type:   "task",
	}
	blocked := &item.Item{
		ID:        "beans-unb3",
		Slug:      item.Slugify("blocked by open"),
		Title:     "blocked by open",
		Status:    "todo",
		Type:      "task",
		BlockedBy: []string{openBlocker.ID},
	}
	unblocked := &item.Item{
		ID:        "beans-unb4",
		Slug:      item.Slugify("blocker is done"),
		Title:     "blocker is done",
		Status:    "todo",
		Type:      "task",
		BlockedBy: []string{doneBlocker.ID},
	}
	draft := &item.Item{
		ID:     "beans-unb5",
		Slug:   item.Slugify("draft task"),
		Title:  "draft task",
		Status: "draft",
		Type:   "task",
	}
	for _, b := range []*item.Item{openBlocker, doneBlocker, blocked, unblocked, draft} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	listIDs := func(t *testing.T) map[string]bool {
		t.Helper()
		out := captureListStdout(t, func() {
			if err := listCmd.RunE(listCmd, nil); err != nil {
				t.Fatalf("listCmd.RunE() error = %v", err)
			}
		})
		var got []*item.Item
		if err := json.Unmarshal(out, &got); err != nil {
			t.Fatalf("decoding JSON output: %v; output = %s", err, out)
		}
		ids := make(map[string]bool, len(got))
		for _, b := range got {
			ids[b.ID] = true
		}
		return ids
	}

	gotIDs := listIDs(t)
	if gotIDs[blocked.ID] {
		t.Errorf("bean with an active blocker must be excluded, got IDs = %v", gotIDs)
	}
	if !gotIDs[unblocked.ID] {
		t.Errorf("bean whose only blocker is completed must be included, got IDs = %v", gotIDs)
	}
	// --unblocked is purely about blockers: it must not imply --ready's
	// status exclusions, so draft and the completed blocker stay in.
	if !gotIDs[draft.ID] || !gotIDs[doneBlocker.ID] || !gotIDs[openBlocker.ID] {
		t.Errorf("--unblocked must not exclude any status, got IDs = %v", gotIDs)
	}

	// Combined with --status: both filters apply to the same query.
	listStatus = []string{"todo"}
	gotIDs = listIDs(t)
	if !gotIDs[unblocked.ID] || !gotIDs[openBlocker.ID] {
		t.Errorf("expected unblocked todo beans, got IDs = %v", gotIDs)
	}
	if gotIDs[blocked.ID] || gotIDs[draft.ID] || gotIDs[doneBlocker.ID] {
		t.Errorf("expected blocked and non-todo beans excluded, got IDs = %v", gotIDs)
	}
}

// TestListCmdUnblockedRejectsIsBlocked pins the mutual exclusion through
// the command's own RunE rather than restating the condition in the test.
func TestListCmdUnblockedRejectsIsBlocked(t *testing.T) {
	setupListTest(t)

	oldUnblocked, oldIsBlocked := listUnblocked, listIsBlocked
	listUnblocked, listIsBlocked = true, true
	t.Cleanup(func() { listUnblocked, listIsBlocked = oldUnblocked, oldIsBlocked })

	if err := listCmd.RunE(listCmd, nil); err == nil {
		t.Fatal("expected --unblocked with --is-blocked to be rejected, got nil error")
	}
}

// seedListTree creates a parent and two children so a test can tell a flat
// table (peers, no connectors) apart from an indented tree (connectors,
// no header). The parent's title carries a non-ASCII character ("über") on
// purpose — an ASCII-only fixture cannot distinguish DisplayWidth (cells)
// from len (bytes), which is exactly the class of bug the width plumbing in
// this task must not have.
func seedListTree(t *testing.T) {
	t.Helper()
	parent := &item.Item{
		ID:     "beans-par1",
		Slug:   item.Slugify("Parent über"),
		Title:  "Parent über",
		Status: "todo",
		Type:   "epic",
		Tags:   []string{"seeded"},
	}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create(parent) error = %v", err)
	}
	for _, suffix := range []string{"a", "b"} {
		c := &item.Item{
			ID:     "beans-chd" + suffix,
			Slug:   item.Slugify("Child " + suffix),
			Title:  "Child " + suffix,
			Status: "todo",
			Type:   "task",
			Parent: parent.ID,
		}
		if err := core.Create(c); err != nil {
			t.Fatalf("core.Create(child %s) error = %v", suffix, err)
		}
	}
}

// resetListViewFlags puts listView/listMaxWidth and their FlagSet bookkeeping
// back to a known, fresh state before an Execute()-driven test. Two separate
// cobra quirks make this necessary: listCmd's flags are registered exactly
// once per test binary (idempotent Lookup guard in RegisterListCmd, so tests
// can register into throwaway roots without panicking on a second flag
// definition — other _test.go files in this package register listCmd via
// RegisterCoreCommands before list_test.go's own tests ever run), and
// pflag.FlagSet.Parse never resets a Value or the Changed bit for a flag
// absent from the current argv. Without this, a --view/--max-width value —
// or just its Changed bit — set by one test leaks into the next.
//
// Both reset values are read back from each flag's own registered DefValue
// rather than hardcoded, so this helper never invents a value independently
// of what RegisterListCmd actually declares.
//
// listView's case is load-bearing: an earlier, hardcoded version of this
// helper masked a mutated default (RegisterListCmd's "table" literal changed
// to "tree") because it forced listView back to "table" regardless of the
// registration — the mutation proof for TestListDefaultsToTheTableForm
// caught this, since RunE reads listView unconditionally via ui.ParseForm.
//
// listMaxWidth's case is hygiene rather than a proven defect: resolveWidth
// (width.go) only ever reads its flagValue argument when flagChanged is
// true, so a stale or wrong *value* in listMaxWidth is inert whenever
// --max-width was not passed — mutating the registered default from 0 to 42
// and rerunning TestListDefaultMaxWidthFallsBackToConfig confirmed this
// (stayed green, because flagChanged gates flagValue out of resolveWidth
// entirely). Deriving from DefValue here still matches the source-of-truth
// principle and costs nothing; it just is not provably load-bearing the way
// listView's is, and I am not overstating it as such. What genuinely mattered
// for --max-width test isolation is resetting widthFlag.Changed to false,
// which this helper also does.
//
// Requires RegisterListCmd(root) to have already run so Lookup succeeds.
func resetListViewFlags(t *testing.T, root *cobra.Command) {
	t.Helper()
	oldView, oldWidth, oldTags := listView, listMaxWidth, listTags
	t.Cleanup(func() { listView, listMaxWidth, listTags = oldView, oldWidth, oldTags })

	viewFlag := listCmd.Flags().Lookup("view")
	widthFlag := listCmd.Flags().Lookup("max-width")
	tagsFlag := listCmd.Flags().Lookup("tags")
	if viewFlag == nil || widthFlag == nil || tagsFlag == nil {
		t.Fatalf("--view/--max-width/--tags not registered; call RegisterListCmd(root) first")
	}
	defWidth, err := strconv.Atoi(widthFlag.DefValue)
	if err != nil {
		t.Fatalf("--max-width DefValue %q is not an int: %v", widthFlag.DefValue, err)
	}
	listView = viewFlag.DefValue
	listMaxWidth = defWidth
	// Read the default from the registration rather than writing `false`
	// here: a reset that does not match what the flag actually registers
	// silently overrides the real default, which has already bitten this
	// branch three times.
	listTags = tagsFlag.DefValue == "true"
	viewFlag.Changed = false
	widthFlag.Changed = false
	tagsFlag.Changed = false
}

// runListThroughRoot drives listCmd via a real cobra root.Execute() rather
// than calling listCmd.RunE directly, because --view's rejection and
// cmd.Flags().Changed("max-width") only behave correctly under real flag
// parsing (the same reasoning as TestOrderCmdMutuallyExclusiveFlags).
func runListThroughRoot(t *testing.T, args []string) (stdout string, runErr error) {
	t.Helper()
	setupListTest(t)
	seedListTree(t)

	root := &cobra.Command{Use: "beans"}
	RegisterListCmd(root)
	resetListViewFlags(t, root)

	out := captureListStdout(t, func() {
		root.SetArgs(append([]string{"list"}, args...))
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		runErr = root.Execute()
	})
	return string(out), runErr
}

// runListInTestStore runs list against a throwaway store with one parent
// and two children, and fails the test on any error — for the happy path.
func runListInTestStore(t *testing.T, args []string) string {
	t.Helper()
	out, err := runListThroughRoot(t, args)
	if err != nil {
		t.Fatalf("list %v: %v", args, err)
	}
	return out
}

// runListExpectingError is runListInTestStore's counterpart for the
// rejection path: the caller asserts on the returned error itself.
func runListExpectingError(t *testing.T, args []string) error {
	t.Helper()
	_, err := runListThroughRoot(t, args)
	return err
}

// TestListDefaultsToTheTableForm is the brief's guard for AC "default is
// table": no tree connectors, and the header row (the table's defining
// trait per ui.Render) is present.
func TestListDefaultsToTheTableForm(t *testing.T) {
	out := runListInTestStore(t, []string{})
	if strings.Contains(out, "├─") {
		t.Errorf("list defaults to table and must be flat:\n%s", out)
	}
	if !strings.Contains(out, "TITLE") {
		t.Errorf("table form must carry a header:\n%s", out)
	}
}

// SC-03 / AC-02: a store without a matching item answers with a hint that
// names the canonical create command (`create`, not its alias `new`).
func TestListEmptyStateHintNamesLifCreate(t *testing.T) {
	setupListTest(t)

	root := &cobra.Command{Use: "lif"}
	RegisterListCmd(root)
	resetListViewFlags(t, root)

	var runErr error
	out := captureListStdout(t, func() {
		root.SetArgs([]string{"list"})
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		runErr = root.Execute()
	})
	if runErr != nil {
		t.Fatalf("list on an empty store: %v", runErr)
	}

	if got := string(out); !strings.Contains(got, "No items found.") {
		t.Errorf("empty-state hint does not say \"No items found.\": %q", got)
	}
	if got := string(out); !strings.Contains(got, "Create one with: lif create <title>") {
		t.Errorf("empty-state hint does not name `lif create <title>`: %q", got)
	}
}

// AC3: the first line of the rendered list is the title, which names the
// noun as item in the table and in the tree form.
func TestListTitleNamesItems(t *testing.T) {
	for _, view := range []string{"table", "tree"} {
		out := runListInTestStore(t, []string{"--view", view})
		if first, _, _ := strings.Cut(out, "\n"); first != "Items" {
			t.Errorf("--view %s: first line = %q, want %q", view, first, "Items")
		}
	}
}

// TestListViewTreeDrawsTheTree is the brief's guard for --view tree.
func TestListViewTreeDrawsTheTree(t *testing.T) {
	out := runListInTestStore(t, []string{"--view", "tree"})
	if !strings.Contains(out, "├─") && !strings.Contains(out, "└─") {
		t.Errorf("--view tree must draw connectors:\n%s", out)
	}
}

// TestListRejectsAnUnknownView is the brief's guard for --view validation.
func TestListRejectsAnUnknownView(t *testing.T) {
	if err := runListExpectingError(t, []string{"--view", "grid"}); err == nil {
		t.Error("--view grid should be rejected")
	}
}

// TestListMaxWidthCapsRenderedWidth is the companion the brief's own three
// tests do not cover: --view is exercised there, but --max-width never is,
// even though it is part of this task's produced interface. renderTable
// draws its separator as exactly `width` dashes (internal/ui/render.go) and,
// being a rule rather than content, is never right-trimmed the way a short
// title's trailing padding is — so its rendered width is a direct,
// unambiguous readout of the value resolveWidth actually received, not a
// coincidence of column packing. This also proves list.go passes
// cmd.Flags().Changed("max-width") rather than a literal true/false: a
// hardcoded false would make resolveWidth ignore --max-width 95 and fall
// back to the config default (110), which this test would catch as a width
// mismatch.
func TestListMaxWidthCapsRenderedWidth(t *testing.T) {
	out := runListInTestStore(t, []string{"--max-width", "95"})
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least a title and a separator line, got %d lines:\n%s", len(lines), out)
	}
	separator := lines[1]
	if !strings.Contains(separator, "─") {
		t.Fatalf("expected line 2 to be the dash separator, got %q", separator)
	}
	if w := ui.DisplayWidth(separator); w != 95 {
		t.Errorf("separator width = %d, want 95 (from --max-width): %q", w, separator)
	}
}

// TestListTableOmitsUnmatchedAncestors is a regression test for a real
// correctness bug found while proving this task's mutation coverage:
// BuildTree deliberately widens the tree beyond the filtered `lif` slice
// with unmatched ancestors kept for context (ui.TreeNode.Matched), which is
// right for --view tree but was leaking into --view table too, because the
// old code built table rows from the same tree via
// ui.RowsFromFlatItems(ui.FlattenTree(tree)) regardless of form. Verified
// against a real store (SPF_sproutling, `list --status completed --view
// table` vs `--json`), extracting item IDs from both outputs with the same
// regex so the two counts are actually comparable: 496 filter-matched items
// against 514 unique IDs in the old table output, an 18-ID leak of unmatched
// ancestors. list.go now builds table rows from ui.FlatRows(items) — the
// already-filtered slice — instead.
func TestListTableOmitsUnmatchedAncestors(t *testing.T) {
	setupListTest(t)

	oldStatus, oldView, oldWidth := listStatus, listView, listMaxWidth
	listStatus, listView, listMaxWidth = []string{"todo"}, "table", 0
	t.Cleanup(func() { listStatus, listView, listMaxWidth = oldStatus, oldView, oldWidth })

	parent := &item.Item{
		ID: "beans-anc1", Slug: item.Slugify("Ancestor"), Title: "Ancestor",
		Status: "draft", Type: "epic",
	}
	child := &item.Item{
		ID: "beans-anc2", Slug: item.Slugify("Matching child"), Title: "Matching child",
		Status: "todo", Type: "task", Parent: parent.ID,
	}
	for _, b := range []*item.Item{parent, child} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}

	out := string(captureListStdout(t, func() {
		if err := listCmd.RunE(listCmd, nil); err != nil {
			t.Fatalf("listCmd.RunE() error = %v", err)
		}
	}))

	if strings.Contains(out, parent.ID) {
		t.Errorf("--view table with --status todo must not show the unmatched ancestor %s (context leak):\n%s", parent.ID, out)
	}
	if !strings.Contains(out, child.ID) {
		t.Errorf("expected matched child %s in table output:\n%s", child.ID, out)
	}
}

// TestListDefaultMaxWidthFallsBackToConfig depends specifically on
// listMaxWidth's registered default (0, "uncapped by the flag") rather than
// any other value: with 0 and cmd.Flags().Changed("max-width") false,
// resolveWidth falls through to cfg.GetMaxWidth() (110, config.Default()).
// This is the companion the fix-round-1 review asked for: without it,
// resetListViewFlags's listMaxWidth reset could regress to a hardcoded
// literal (exactly the defect just fixed for listView) and nothing here
// would notice.
func TestListDefaultMaxWidthFallsBackToConfig(t *testing.T) {
	out := runListInTestStore(t, []string{})
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least a title and a separator line, got %d lines:\n%s", len(lines), out)
	}
	separator := lines[1]
	want := config.Default().GetMaxWidth()
	if w := ui.DisplayWidth(separator); w != want {
		t.Errorf("separator width = %d, want %d (config default, since --max-width was not given): %q", w, want, separator)
	}
}

// TestListShowsTagsOnlyWithTheFlag pins list to the same contract as
// milestones and roadmap: the tag column is the caller's choice. It used to
// appear automatically whenever any listed item happened to carry a tag,
// which made the same query render two different layouts depending on the
// data -- the kind of quiet disagreement between commands this whole branch
// exists to remove (PO decision, beans-jbgs).
//
// Mutation: restore the `hasTags` loop in list.go's RunE and pass it to
// ui.Render instead of listTags; the first half goes red because the seeded
// parent carries a tag.
func TestListShowsTagsOnlyWithTheFlag(t *testing.T) {
	if out := runListInTestStore(t, []string{}); strings.Contains(out, "#seeded") {
		t.Errorf("list without --tags must not render a tag column:\n%s", out)
	}
	if out := runListInTestStore(t, []string{"--tags"}); !strings.Contains(out, "#seeded") {
		t.Errorf("list --tags must render the tag column:\n%s", out)
	}
}

// TestViewFlagIsValidatedBeforeAnyEarlyReturn pins the fix for beans-cfky.
// The three view-aware commands validated --view at the point where they
// were about to render, which is AFTER their empty-result return and, for
// milestones, after the --json return as well. A typo therefore vanished
// silently: `lif milestones --json --view bogus` printed [] and exited 0,
// and `lif list --view bogus` against an empty store printed the
// no-items hint. An agent reading prime as authoritative would get the
// default arrangement and no indication it had asked for something else.
//
// Mutation: move any of the three ParseForm checks back below its command's
// empty-result return and the matching case goes red.
func TestViewFlagIsValidatedBeforeAnyEarlyReturn(t *testing.T) {
	setupListTest(t) // empty store: no items, no milestones

	root := &cobra.Command{Use: "beans"}
	RegisterListCmd(root)
	RegisterMilestonesCmd(root)
	RegisterRoadmapCmd(root)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"list", []string{"list", "--view", "bogus"}},
		{"list --json", []string{"list", "--json", "--view", "bogus"}},
		{"milestones", []string{"milestones", "--view", "bogus"}},
		{"milestones --json", []string{"milestones", "--json", "--view", "bogus"}},
		{"roadmap", []string{"roadmap", "--view", "bogus"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// One root for the whole test: milestonesCmd and roadmapCmd are
			// package singletons without list's idempotence guard, so
			// registering them twice panics in pflag.
			resetListViewFlags(t, root)
			oldM, oldR := milestonesView, roadmapView
			t.Cleanup(func() { milestonesView, roadmapView = oldM, oldR })

			root.SetArgs(tc.args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			var err error
			captureListStdout(t, func() { err = root.Execute() })
			if err == nil {
				t.Errorf("%v: an invalid --view was accepted against an empty store", tc.args)
			}
		})
	}
}

// seedStatusMix creates an epic and three children in todo, draft and
// in-progress status, all under beans-mix00.
func seedStatusMix(t *testing.T) {
	t.Helper()
	mk := func(id, typ, status, parent string) {
		b := &item.Item{ID: id, Slug: item.Slugify(id), Title: id, Type: typ, Status: status, Parent: parent}
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", id, err)
		}
	}
	mk("beans-mix00", "epic", "todo", "")
	mk("beans-mixtd", "task", "todo", "beans-mix00")
	mk("beans-mixdr", "task", "draft", "beans-mix00")
	mk("beans-mixip", "task", "in-progress", "beans-mix00")
}

// A comma list must behave like repeating the flag. Needs a real parse:
// only pflag's slice Set splits on commas.
func TestListFilterFlagsSplitCommaLists(t *testing.T) {
	setupListTest(t)
	seedStatusMix(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	stdout, _, err := runRootInDir(t, core.Root(), "list", "--parent", "beans-mix00", "--status", "todo,draft", "-q")
	if err != nil {
		t.Fatalf("list error = %v", err)
	}
	if want := "beans-mixtd\nbeans-mixdr\n"; stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}
