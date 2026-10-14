package candidates

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
	"github.com/xRiErOS/lif/pkg/itemgraph"
)

// fixture builds a small item tree:
//
//	beans-m1 "Milestone A" (milestone)                                tags: echo, delta
//	beans-m2 "Milestone B" (milestone)                                tags: echo, delta
//	beans-e1 "Epic A"      (epic, parent beans-m1)                    tags: echo, delta
//	beans-e2 "Epic B"      (epic)                                     tags: echo, delta
//	beans-f1 "Feature A"   (feature, parent beans-e1) tags: alpha, bravo, echo
//	beans-f2 "Feature B"   (feature, parent beans-e2) tags: alpha
//	beans-t1 "Task A"      (task, parent beans-f1)    tags: bravo, charlie
//	beans-m3 "Milestone C" (milestone, parent beans-t1; a descendant of
//	                        beans-f1 whose type would otherwise be a valid
//	                        parent candidate, so it can only be kept out of
//	                        ParentCandidates by the descendant exclusion)
//	                        tags: bravo
//
// Tag counts: echo=5, delta=4, bravo=3, alpha=2, charlie=1 — five
// pairwise-distinct counts, so the expected tag order [echo, delta,
// bravo, alpha, charlie] can only be produced by a comparator that
// actually orders by count; see the rationale on TestTagCandidates.
func fixture(t *testing.T) (*itemgraph.CoreResolver, *config.Config) {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test .beans dir: %v", err)
	}
	cfg := config.Default()
	core := itemcore.New(itemsDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatalf("loading core: %v", err)
	}

	items := []*item.Item{
		{ID: "beans-m1", Slug: item.Slugify("Milestone A"), Title: "Milestone A", Status: "todo", Type: "milestone", Tags: []string{"echo", "delta"}},
		{ID: "beans-m2", Slug: item.Slugify("Milestone B"), Title: "Milestone B", Status: "todo", Type: "milestone", Tags: []string{"echo", "delta"}},
		{ID: "beans-e1", Slug: item.Slugify("Epic A"), Title: "Epic A", Status: "todo", Type: "epic", Parent: "beans-m1", Tags: []string{"echo", "delta"}},
		{ID: "beans-e2", Slug: item.Slugify("Epic B"), Title: "Epic B", Status: "todo", Type: "epic", Tags: []string{"echo", "delta"}},
		{ID: "beans-f1", Slug: item.Slugify("Feature A"), Title: "Feature A", Status: "todo", Type: "feature", Parent: "beans-e1", Tags: []string{"alpha", "bravo", "echo"}},
		{ID: "beans-f2", Slug: item.Slugify("Feature B"), Title: "Feature B", Status: "todo", Type: "feature", Parent: "beans-e2", Tags: []string{"alpha"}},
		{ID: "beans-t1", Slug: item.Slugify("Task A"), Title: "Task A", Status: "todo", Type: "task", Parent: "beans-f1", Tags: []string{"bravo", "charlie"}},
		{ID: "beans-m3", Slug: item.Slugify("Milestone C"), Title: "Milestone C", Status: "todo", Type: "milestone", Parent: "beans-t1", Tags: []string{"bravo"}},
	}
	for _, b := range items {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s): %v", b.ID, err)
		}
	}

	return &itemgraph.CoreResolver{Core: core}, cfg
}

func ids(items []*item.Item) []string {
	out := make([]string, len(items))
	for i, b := range items {
		out[i] = b.ID
	}
	return out
}

func mustEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestParentCandidates_ExcludesDescendants pins ParentCandidates'
// content, order and exclusions: only valid parent types, sorted by type
// rank then title, excluding the edited item's descendants.
//
// Self-exclusion is not independently observable here: under the strict
// type-rank hierarchy an item's own type is never a valid parent type for
// itself, so the selected-item skip never changes the result and cannot
// be pinned by a passing/failing assertion.
func TestParentCandidates_ExcludesDescendants(t *testing.T) {
	resolver, cfg := fixture(t)

	got, err := ParentCandidates(context.Background(), resolver, cfg, []string{"beans-f1"}, []string{"feature"})
	if err != nil {
		t.Fatalf("ParentCandidates: %v", err)
	}

	mustEqual(t, ids(got), []string{"beans-m1", "beans-m2", "beans-e1", "beans-e2"})

	for _, excluded := range []string{"beans-f1", "beans-t1", "beans-f2", "beans-m3"} {
		for _, id := range ids(got) {
			if id == excluded {
				t.Fatalf("expected %s to be excluded from parent candidates, got %v", excluded, ids(got))
			}
		}
	}
}

func TestBlockingCandidates_ExcludesDescendants(t *testing.T) {
	resolver, cfg := fixture(t)

	got, err := BlockingCandidates(context.Background(), resolver, cfg, "beans-f1")
	if err != nil {
		t.Fatalf("BlockingCandidates: %v", err)
	}

	mustEqual(t, ids(got), []string{"beans-m1", "beans-m2", "beans-e1", "beans-e2", "beans-f2"})

	for _, excluded := range []string{"beans-t1", "beans-m3"} {
		for _, id := range ids(got) {
			if id == excluded {
				t.Fatalf("expected %s (descendant of beans-f1) to be excluded from blocking candidates, got %v", excluded, ids(got))
			}
		}
	}
}

// TestStatusCandidates_ReflectsConfigOverride builds a *config.Config
// carrying a status override the built-in defaults do not know about
// (mirrors pkg/config/config_test.go's TestAnUnknownNameIsAppended
// pattern) and asserts it surfaces through StatusCandidates. A
// StatusCandidates that returned a hardcoded copy of config.DefaultStatuses
// instead of delegating to cfg.StatusList() would pass a same-length,
// same-names-as-defaults check but never see this override -- so the
// assertion is specifically that the override is present, not merely that
// the length matches the defaults.
func TestStatusCandidates_ReflectsConfigOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Statuses = []config.StatusOverride{{Name: "blocked", Color: "red"}}
	got := StatusCandidates(cfg)
	found := false
	for _, s := range got {
		if s.Name == "blocked" {
			found = true
		}
	}
	if !found {
		t.Errorf("StatusCandidates(cfg) = %v, want it to include the configured override %q", got, "blocked")
	}
	if len(got) != len(config.DefaultStatuses)+1 {
		t.Errorf("StatusCandidates(cfg) has %d entries, want defaults plus the override", len(got))
	}
}

// TestTypeCandidates_ReflectsConfigOverride is StatusCandidates_
// ReflectsConfigOverride's --type counterpart.
func TestTypeCandidates_ReflectsConfigOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Types = []config.TypeOverride{{Name: "spike", Color: "red"}}
	got := TypeCandidates(cfg)
	found := false
	for _, ty := range got {
		if ty.Name == "spike" {
			found = true
		}
	}
	if !found {
		t.Errorf("TypeCandidates(cfg) = %v, want it to include the configured override %q", got, "spike")
	}
	if len(got) != len(config.DefaultTypes)+1 {
		t.Errorf("TypeCandidates(cfg) has %d entries, want defaults plus the override", len(got))
	}
}

// TestPriorityCandidates_ReflectsConfigOverride is StatusCandidates_
// ReflectsConfigOverride's --priority counterpart.
func TestPriorityCandidates_ReflectsConfigOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Priorities = []config.PriorityOverride{{Name: "critical", Color: "#ff00ff"}}
	got := PriorityCandidates(cfg)
	found := false
	for _, p := range got {
		if p.Name == "critical" && p.Color == "#ff00ff" {
			found = true
		}
	}
	if !found {
		t.Errorf("PriorityCandidates(cfg) = %v, want it to include the configured override %q", got, "critical")
	}
}

// TestTagCandidates pins TagCandidates' content and order: every tag in
// use across all items with an accurate usage count, sorted by count
// descending then tag ascending. The fixture gives five tags five
// pairwise-distinct counts (echo=5, delta=4, bravo=3, alpha=2,
// charlie=1), so the expected order [echo, delta, bravo, alpha,
// charlie] is neither alphabetical nor any other structurally simple
// permutation of the tag set: it is reachable only by a comparator that
// actually orders by count. A comparator that dropped the count
// comparison (falling back to pure tag order) would deterministically
// produce [alpha, bravo, charlie, delta, echo] instead, differing on a
// single run. A comparator that ordered nothing at all leaves the
// result at the mercy of Go's per-call-randomized map iteration, whose
// reachable orders (rotations of one process-wide hash-bucket layout)
// are a small, fixed subset of all 5! permutations; the 10x repeated
// -call loop requires the exact expected order on all 10 independently
// randomized iterations, which the widened, non-trivial target makes an
// accidental full pass vanishingly unlikely without claiming it is
// mathematically impossible.
func TestTagCandidates(t *testing.T) {
	resolver, _ := fixture(t)

	wantTags := []string{"echo", "delta", "bravo", "alpha", "charlie"}
	wantCounts := map[string]int{"echo": 5, "delta": 4, "bravo": 3, "alpha": 2, "charlie": 1}

	for run := range 10 {
		got, err := TagCandidates(context.Background(), resolver)
		if err != nil {
			t.Fatalf("run %d: TagCandidates: %v", run, err)
		}
		if len(got) != len(wantTags) {
			t.Fatalf("run %d: got %v, want tags %v", run, got, wantTags)
		}
		for i, tc := range got {
			if tc.Tag != wantTags[i] {
				t.Fatalf("run %d: tag %d = %q, want %q (got order %v)", run, i, tc.Tag, wantTags[i], got)
			}
			if tc.Count != wantCounts[tc.Tag] {
				t.Fatalf("run %d: tag %q count = %d, want %d", run, tc.Tag, tc.Count, wantCounts[tc.Tag])
			}
		}
	}
}

// TestParentCandidates_IntersectsValidTypesAcrossMultipleSelectedItems
// pins intersectStrings' role in a multi-select ParentCandidates call:
// with beans-e1 (epic) and beans-f1 (feature) selected together, the
// valid parent types must be the intersection of each item's own valid
// parent types (epic -> {milestone}, feature -> {milestone, epic}),
// leaving only milestone. Without the intersection, epic-typed items
// (e.g. beans-e2) would leak in as candidates too. Checked in both
// argument orders since intersection is commutative.
func TestParentCandidates_IntersectsValidTypesAcrossMultipleSelectedItems(t *testing.T) {
	resolver, cfg := fixture(t)

	for _, itemIDs := range [][]string{
		{"beans-e1", "beans-f1"},
		{"beans-f1", "beans-e1"},
	} {
		itemTypes := []string{"epic", "feature"}
		if itemIDs[0] == "beans-f1" {
			itemTypes = []string{"feature", "epic"}
		}

		got, err := ParentCandidates(context.Background(), resolver, cfg, itemIDs, itemTypes)
		if err != nil {
			t.Fatalf("ParentCandidates(%v, %v): %v", itemIDs, itemTypes, err)
		}

		mustEqual(t, ids(got), []string{"beans-m1", "beans-m2"})

		for _, id := range ids(got) {
			if id == "beans-e2" {
				t.Fatalf("beanIDs=%v: expected beans-e2 (epic, outside the type intersection) to be excluded, got %v", itemIDs, ids(got))
			}
		}
	}
}

// addArchivedEpics adds a completed and a scrapped epic to the fixture's
// store; the fixture's open epics stay as they are.
func addArchivedEpics(t *testing.T, resolver *itemgraph.CoreResolver) {
	t.Helper()
	for _, b := range []*item.Item{
		{ID: "beans-ec", Slug: item.Slugify("Epic Done"), Title: "Epic Done", Status: "completed", Type: "epic", Tags: []string{"zulu"}},
		{ID: "beans-es", Slug: item.Slugify("Epic Scrapped"), Title: "Epic Scrapped", Status: "scrapped", Type: "epic"},
	} {
		if err := resolver.Core.Create(b); err != nil {
			t.Fatalf("core.Create(%s): %v", b.ID, err)
		}
	}
}

func TestParentCandidates_ExcludesArchivedItems(t *testing.T) {
	resolver, cfg := fixture(t)
	addArchivedEpics(t, resolver)

	got, err := ParentCandidates(context.Background(), resolver, cfg, []string{"beans-f1"}, []string{"feature"})
	if err != nil {
		t.Fatalf("ParentCandidates: %v", err)
	}
	mustEqual(t, ids(got), []string{"beans-m1", "beans-m2", "beans-e1", "beans-e2"})
}

// TestParentCandidates_ArchiveFollowsConfig proves the predicate is
// Config.IsArchiveStatus and not a status literal: with archive: false on
// completed, the completed epic is a candidate again.
func TestParentCandidates_ArchiveFollowsConfig(t *testing.T) {
	resolver, cfg := fixture(t)
	addArchivedEpics(t, resolver)
	off := false
	cfg.Statuses = []config.StatusOverride{{Name: "completed", Archive: &off}}

	got, err := ParentCandidates(context.Background(), resolver, cfg, []string{"beans-f1"}, []string{"feature"})
	if err != nil {
		t.Fatalf("ParentCandidates: %v", err)
	}
	mustEqual(t, ids(got), []string{"beans-m1", "beans-m2", "beans-e1", "beans-e2", "beans-ec"})
}

// TestBlockingAndTagCandidates_KeepArchivedItems pins that the archive
// filter is ParentCandidates-only (R-14 forbidden side effect).
func TestBlockingAndTagCandidates_KeepArchivedItems(t *testing.T) {
	resolver, cfg := fixture(t)
	addArchivedEpics(t, resolver)

	blocking, err := BlockingCandidates(context.Background(), resolver, cfg, "beans-f1")
	if err != nil {
		t.Fatalf("BlockingCandidates: %v", err)
	}
	have := map[string]bool{}
	for _, id := range ids(blocking) {
		have[id] = true
	}
	if !have["beans-ec"] || !have["beans-es"] {
		t.Fatalf("BlockingCandidates must keep archived items, got %v", ids(blocking))
	}

	tags, err := TagCandidates(context.Background(), resolver)
	if err != nil {
		t.Fatalf("TagCandidates: %v", err)
	}
	found := false
	for _, tc := range tags {
		if tc.Tag == "zulu" {
			found = true
		}
	}
	if !found {
		t.Fatalf("TagCandidates must count tags of archived items, got %v", tags)
	}
}
