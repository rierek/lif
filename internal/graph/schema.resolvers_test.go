package graph

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rierek/lif/internal/agent"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

func setupTestResolver(t *testing.T) (*Resolver, *itemcore.Core) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	cfg := config.Default()
	core := itemcore.New(itemsDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return &Resolver{CoreResolver: &itemgraph.CoreResolver{Core: core}}, core
}

func createTestItem(t *testing.T, core *itemcore.Core, id, title, status string) *item.Item {
	t.Helper()
	b := &item.Item{
		ID:     id,
		Slug:   item.Slugify(title),
		Title:  title,
		Status: status,
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}
	return b
}

func TestQueryItem(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test item
	createTestItem(t, core, "test-1", "Test Bean", "todo")

	// Test exact match
	t.Run("exact match", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Item(ctx, "test-1")
		if err != nil {
			t.Fatalf("Item() error = %v", err)
		}
		if got == nil {
			t.Fatal("Item() returned nil")
		}
		if got.ID != "test-1" {
			t.Errorf("Bean().ID = %q, want %q", got.ID, "test-1")
		}
	})

	// Test partial ID not found (no prefix matching)
	t.Run("partial ID not found", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Item(ctx, "test")
		if err != nil {
			t.Fatalf("Item() error = %v", err)
		}
		if got != nil {
			t.Errorf("Item() = %v, want nil (partial IDs should not match)", got)
		}
	})

	// Test not found
	t.Run("not found", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Item(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Item() error = %v", err)
		}
		if got != nil {
			t.Errorf("Item() = %v, want nil", got)
		}
	})
}

func TestQueryItems(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items
	createTestItem(t, core, "bean-1", "First Bean", "todo")
	createTestItem(t, core, "bean-2", "Second Bean", "in-progress")
	createTestItem(t, core, "bean-3", "Third Bean", "completed")

	t.Run("no filter", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Items(ctx, nil)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("Items() count = %d, want 3", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Status: []string{"todo"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "bean-1" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "bean-1")
		}
	})

	t.Run("filter by multiple statuses", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Status: []string{"todo", "in-progress"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude status", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})
}

func TestQueryItemsWithTags(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items with tags
	b1 := &item.Item{ID: "tag-1", Title: "Tagged 1", Status: "todo", Tags: []string{"frontend", "urgent"}}
	b2 := &item.Item{ID: "tag-2", Title: "Tagged 2", Status: "todo", Tags: []string{"backend"}}
	b3 := &item.Item{ID: "tag-3", Title: "No Tags", Status: "todo"}
	core.Create(b1)
	core.Create(b2)
	core.Create(b3)

	t.Run("filter by tag", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Tags: []string{"frontend"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
	})

	t.Run("filter by multiple tags (OR)", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Tags: []string{"frontend", "backend"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude by tag", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			ExcludeTags: []string{"urgent"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})
}

func TestQueryItemsWithPriority(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items with various priorities
	// Empty priority should be treated as "normal"
	b1 := &item.Item{ID: "pri-1", Title: "Critical", Status: "todo", Priority: "critical"}
	b2 := &item.Item{ID: "pri-2", Title: "High", Status: "todo", Priority: "high"}
	b3 := &item.Item{ID: "pri-3", Title: "Normal Explicit", Status: "todo", Priority: "normal"}
	b4 := &item.Item{ID: "pri-4", Title: "Normal Implicit", Status: "todo", Priority: ""} // empty = normal
	b5 := &item.Item{ID: "pri-5", Title: "Low", Status: "todo", Priority: "low"}
	core.Create(b1)
	core.Create(b2)
	core.Create(b3)
	core.Create(b4)
	core.Create(b5)

	t.Run("filter by normal includes empty priority", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Priority: []string{"normal"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		// Should include both explicit "normal" and implicit (empty) priority
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}
		if !ids["pri-3"] || !ids["pri-4"] {
			t.Errorf("Items() should include pri-3 and pri-4, got %v", ids)
		}
	})

	t.Run("filter by critical", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Priority: []string{"critical"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "pri-1" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "pri-1")
		}
	})

	t.Run("filter by multiple priorities", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			Priority: []string{"critical", "high"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude normal excludes empty priority", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.ItemFilter{
			ExcludePriority: []string{"normal"},
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		// Should exclude both explicit "normal" and implicit (empty) priority
		if len(got) != 3 {
			t.Errorf("Items() count = %d, want 3", len(got))
		}
		for _, b := range got {
			if b.ID == "pri-3" || b.ID == "pri-4" {
				t.Errorf("Items() should not include %s", b.ID)
			}
		}
	})
}

func TestItemRelationships(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create items with relationships
	parent := &item.Item{ID: "parent-1", Title: "Parent", Status: "todo"}
	child1 := &item.Item{
		ID:     "child-1",
		Title:  "Child 1",
		Status: "todo",
		Parent: "parent-1",
	}
	child2 := &item.Item{
		ID:     "child-2",
		Title:  "Child 2",
		Status: "todo",
		Parent: "parent-1",
	}
	blocker := &item.Item{
		ID:       "blocker-1",
		Title:    "Blocker",
		Status:   "todo",
		Blocking: []string{"child-1"},
	}

	core.Create(parent)
	core.Create(child1)
	core.Create(child2)
	core.Create(blocker)

	t.Run("parent resolver", func(t *testing.T) {
		br := resolver.Item()
		got, err := br.Parent(ctx, child1)
		if err != nil {
			t.Fatalf("Parent() error = %v", err)
		}
		if got == nil {
			t.Fatal("Parent() returned nil")
		}
		if got.ID != "parent-1" {
			t.Errorf("Parent().ID = %q, want %q", got.ID, "parent-1")
		}
	})

	t.Run("children resolver", func(t *testing.T) {
		br := resolver.Item()
		got, err := br.Children(ctx, parent, nil)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Children() count = %d, want 2", len(got))
		}
	})

	t.Run("blockedBy resolver", func(t *testing.T) {
		br := resolver.Item()
		got, err := br.BlockedBy(ctx, child1, nil)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy() count = %d, want 1", len(got))
		}
		if got[0].ID != "blocker-1" {
			t.Errorf("BlockedBy()[0].ID = %q, want %q", got[0].ID, "blocker-1")
		}
	})

	t.Run("blocks resolver", func(t *testing.T) {
		br := resolver.Item()
		got, err := br.Blocking(ctx, blocker, nil)
		if err != nil {
			t.Fatalf("Blocks() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Blocks() count = %d, want 1", len(got))
		}
		if got[0].ID != "child-1" {
			t.Errorf("Blocks()[0].ID = %q, want %q", got[0].ID, "child-1")
		}
	})
}

func TestBrokenLinksFiltered(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create item with broken link
	b := &item.Item{
		ID:     "orphan-1",
		Title:  "Orphan",
		Status: "todo",
		Parent: "nonexistent",
	}
	core.Create(b)

	t.Run("broken parent link returns nil", func(t *testing.T) {
		br := resolver.Item()
		got, err := br.Parent(ctx, b)
		if err != nil {
			t.Fatalf("Parent() error = %v", err)
		}
		if got != nil {
			t.Errorf("Parent() = %v, want nil for broken link", got)
		}
	})
}

func TestQueryItemsWithParentAndBlocks(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create items with various relationship configurations
	noRels := &item.Item{ID: "no-rels", Title: "No Relationships", Status: "todo"}
	hasParent := &item.Item{
		ID:     "has-parent",
		Title:  "Has Parent",
		Status: "todo",
		Parent: "no-rels",
	}
	hasBlocks := &item.Item{
		ID:       "has-blocks",
		Title:    "Has Blocks",
		Status:   "todo",
		Blocking: []string{"has-parent"},
	}

	core.Create(noRels)
	core.Create(hasParent)
	core.Create(hasBlocks)

	t.Run("filter hasParent", func(t *testing.T) {
		qr := resolver.Query()
		hasParentBool := true
		filter := &model.ItemFilter{
			HasParent: &hasParentBool,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})

	t.Run("filter noParent", func(t *testing.T) {
		qr := resolver.Query()
		noParentBool := true
		filter := &model.ItemFilter{
			NoParent: &noParentBool,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
	})

	t.Run("filter hasBlocks", func(t *testing.T) {
		qr := resolver.Query()
		hasBlocksBool := true
		filter := &model.ItemFilter{
			HasBlocking: &hasBlocksBool,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-blocks" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "has-blocks")
		}
	})

	t.Run("filter isBlocked true", func(t *testing.T) {
		qr := resolver.Query()
		isBlockedBool := true
		filter := &model.ItemFilter{
			IsBlocked: &isBlockedBool,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})

	t.Run("filter isBlocked false", func(t *testing.T) {
		qr := resolver.Query()
		isBlockedBool := false
		filter := &model.ItemFilter{
			IsBlocked: &isBlockedBool,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		// Should return all items except "has-parent" (which is blocked by "has-blocks")
		if len(got) != 2 {
			t.Errorf("Items() count = %d, want 2", len(got))
		}
		// Verify "has-parent" is not in results
		for _, b := range got {
			if b.ID == "has-parent" {
				t.Errorf("Items() should not contain blocked bean 'has-parent'")
			}
		}
	})

	t.Run("filter by parentId", func(t *testing.T) {
		qr := resolver.Query()
		parentID := "no-rels"
		filter := &model.ItemFilter{
			ParentID: &parentID,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Items() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Items()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})
}

func TestIsBlockedFilterWithResolvedBlockers(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create items to test blocking with various blocker statuses
	activeBlocker := &item.Item{
		ID:       "active-blocker",
		Title:    "Active Blocker",
		Status:   "todo",
		Blocking: []string{"blocked-by-active"},
	}
	completedBlocker := &item.Item{
		ID:       "completed-blocker",
		Title:    "Completed Blocker",
		Status:   "completed",
		Blocking: []string{"blocked-by-completed"},
	}
	scrappedBlocker := &item.Item{
		ID:       "scrapped-blocker",
		Title:    "Scrapped Blocker",
		Status:   "scrapped",
		Blocking: []string{"blocked-by-scrapped"},
	}
	blockedByActive := &item.Item{
		ID:     "blocked-by-active",
		Title:  "Blocked by Active",
		Status: "todo",
	}
	blockedByCompleted := &item.Item{
		ID:     "blocked-by-completed",
		Title:  "Blocked by Completed",
		Status: "todo",
	}
	blockedByScrapped := &item.Item{
		ID:     "blocked-by-scrapped",
		Title:  "Blocked by Scrapped",
		Status: "todo",
	}
	notBlocked := &item.Item{
		ID:     "not-blocked",
		Title:  "Not Blocked",
		Status: "todo",
	}
	// Item with mixed blockers (one active, one completed)
	mixedBlocker := &item.Item{
		ID:       "mixed-blocker",
		Title:    "Mixed Blocker (active)",
		Status:   "in-progress",
		Blocking: []string{"mixed-blocked"},
	}
	mixedBlockerCompleted := &item.Item{
		ID:       "mixed-blocker-completed",
		Title:    "Mixed Blocker (completed)",
		Status:   "completed",
		Blocking: []string{"mixed-blocked"},
	}
	mixedBlocked := &item.Item{
		ID:     "mixed-blocked",
		Title:  "Mixed Blocked",
		Status: "todo",
	}

	items := []*item.Item{
		activeBlocker, completedBlocker, scrappedBlocker,
		blockedByActive, blockedByCompleted, blockedByScrapped,
		notBlocked, mixedBlocker, mixedBlockerCompleted, mixedBlocked,
	}
	for _, b := range items {
		if err := core.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	t.Run("isBlocked true returns only beans with active blockers", func(t *testing.T) {
		qr := resolver.Query()
		isBlocked := true
		filter := &model.ItemFilter{
			IsBlocked: &isBlocked,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}

		// Should only return items blocked by active blockers
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}

		if !ids["blocked-by-active"] {
			t.Error("expected blocked-by-active in results (has active blocker)")
		}
		if !ids["mixed-blocked"] {
			t.Error("expected mixed-blocked in results (has one active blocker)")
		}
		if ids["blocked-by-completed"] {
			t.Error("blocked-by-completed should NOT be in results (blocker is completed)")
		}
		if ids["blocked-by-scrapped"] {
			t.Error("blocked-by-scrapped should NOT be in results (blocker is scrapped)")
		}
		if ids["not-blocked"] {
			t.Error("not-blocked should NOT be in results (no blockers)")
		}
	})

	t.Run("isBlocked false excludes beans with active blockers", func(t *testing.T) {
		qr := resolver.Query()
		isBlocked := false
		filter := &model.ItemFilter{
			IsBlocked: &isBlocked,
		}
		got, err := qr.Items(ctx, filter)
		if err != nil {
			t.Fatalf("Items() error = %v", err)
		}

		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}

		// Should include items with no active blockers
		if !ids["blocked-by-completed"] {
			t.Error("expected blocked-by-completed in results (blocker is completed)")
		}
		if !ids["blocked-by-scrapped"] {
			t.Error("expected blocked-by-scrapped in results (blocker is scrapped)")
		}
		if !ids["not-blocked"] {
			t.Error("expected not-blocked in results (no blockers)")
		}
		// Should exclude items with active blockers
		if ids["blocked-by-active"] {
			t.Error("blocked-by-active should NOT be in results (has active blocker)")
		}
		if ids["mixed-blocked"] {
			t.Error("mixed-blocked should NOT be in results (has active blocker)")
		}
	})
}

func TestMutationCreateItem(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with required fields only", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title: "New Bean",
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if got == nil {
			t.Fatal("CreateItem() returned nil")
		}
		if got.Title != "New Bean" {
			t.Errorf("CreateItem().Title = %q, want %q", got.Title, "New Bean")
		}
		// Type defaults to "task"
		if got.Type != "task" {
			t.Errorf("CreateItem().Type = %q, want %q (default)", got.Type, "task")
		}
		if got.ID == "" {
			t.Error("CreateItem().ID is empty")
		}
	})

	t.Run("create with all fields", func(t *testing.T) {
		// Create parent and target items first
		parentItem := &item.Item{
			ID:     "some-parent",
			Title:  "Parent Bean",
			Status: "todo",
			Type:   "epic",
		}
		targetItem := &item.Item{
			ID:     "some-target",
			Title:  "Target Bean",
			Status: "todo",
			Type:   "task",
		}
		core.Create(parentItem)
		core.Create(targetItem)

		mr := resolver.Mutation()
		itemType := "feature"
		status := "in-progress"
		priority := "high"
		body := "Test body content"
		parent := "some-parent"
		input := model.CreateItemInput{
			Title:    "Full Bean",
			Type:     &itemType,
			Status:   &status,
			Priority: &priority,
			Body:     &body,
			Tags:     []string{"tag1", "tag2"},
			Parent:   &parent,
			Blocking: []string{"some-target"},
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if got.Type != "feature" {
			t.Errorf("CreateItem().Type = %q, want %q", got.Type, "feature")
		}
		if got.Status != "in-progress" {
			t.Errorf("CreateItem().Status = %q, want %q", got.Status, "in-progress")
		}
		if got.Priority != "high" {
			t.Errorf("CreateItem().Priority = %q, want %q", got.Priority, "high")
		}
		if got.Body != "Test body content" {
			t.Errorf("CreateItem().Body = %q, want %q", got.Body, "Test body content")
		}
		if len(got.Tags) != 2 {
			t.Errorf("CreateItem().Tags count = %d, want 2", len(got.Tags))
		}
		if got.Parent != "some-parent" {
			t.Errorf("CreateItem().Parent = %q, want %q", got.Parent, "some-parent")
		}
		if len(got.Blocking) != 1 {
			t.Errorf("CreateItem().Blocking count = %d, want 1", len(got.Blocking))
		}
	})
}

func TestMutationCreateItemWithCustomPrefix(t *testing.T) {
	resolver, _ := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with custom prefix", func(t *testing.T) {
		mr := resolver.Mutation()
		customPrefix := "SYNC-TASK-"
		input := model.CreateItemInput{
			Title:  "Custom Prefix Bean",
			Prefix: &customPrefix,
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if got == nil {
			t.Fatal("CreateItem() returned nil")
		}
		// ID should start with the custom prefix
		if !strings.HasPrefix(got.ID, "SYNC-TASK-") {
			t.Errorf("CreateItem().ID = %q, want prefix %q", got.ID, "SYNC-TASK-")
		}
		// ID should be prefix + 4 chars (default length)
		if len(got.ID) != len("SYNC-TASK-")+4 {
			t.Errorf("CreateItem().ID length = %d, want %d", len(got.ID), len("SYNC-TASK-")+4)
		}
	})

	t.Run("create without prefix uses config default", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title: "No Custom Prefix Bean",
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		// Without custom prefix, should use config default (empty in test setup)
		// ID should just be 4 chars
		if len(got.ID) != 4 {
			t.Errorf("CreateItem().ID length = %d, want 4", len(got.ID))
		}
	})

	t.Run("create with empty prefix string uses config default", func(t *testing.T) {
		mr := resolver.Mutation()
		emptyPrefix := ""
		input := model.CreateItemInput{
			Title:  "Empty Prefix Bean",
			Prefix: &emptyPrefix,
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		// Empty string prefix should fall back to config default
		if len(got.ID) != 4 {
			t.Errorf("CreateItem().ID length = %d, want 4", len(got.ID))
		}
	})
}

func TestMutationUpdateItem(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create a test item
	b := &item.Item{
		ID:       "update-test",
		Title:    "Original Title",
		Status:   "todo",
		Type:     "task",
		Priority: "normal",
		Body:     "Original body",
		Tags:     []string{"original"},
	}
	core.Create(b)

	t.Run("update single field", func(t *testing.T) {
		mr := resolver.Mutation()
		newStatus := "in-progress"
		input := model.UpdateItemInput{
			Status: &newStatus,
		}
		got, err := mr.UpdateItem(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		if got.Status != "in-progress" {
			t.Errorf("UpdateItem().Status = %q, want %q", got.Status, "in-progress")
		}
		// Other fields unchanged
		if got.Title != "Original Title" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Original Title")
		}
	})

	t.Run("update multiple fields", func(t *testing.T) {
		mr := resolver.Mutation()
		newTitle := "Updated Title"
		newPriority := "high"
		newBody := "Updated body"
		input := model.UpdateItemInput{
			Title:    &newTitle,
			Priority: &newPriority,
			Body:     &newBody,
		}
		got, err := mr.UpdateItem(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		if got.Title != "Updated Title" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Updated Title")
		}
		if got.Priority != "high" {
			t.Errorf("UpdateItem().Priority = %q, want %q", got.Priority, "high")
		}
		if got.Body != "Updated body" {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, "Updated body")
		}
	})

	t.Run("replace tags", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.UpdateItemInput{
			Tags: []string{"new-tag-1", "new-tag-2"},
		}
		got, err := mr.UpdateItem(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		if len(got.Tags) != 2 {
			t.Errorf("UpdateItem().Tags count = %d, want 2", len(got.Tags))
		}
	})

	t.Run("update nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		newTitle := "Whatever"
		input := model.UpdateItemInput{
			Title: &newTitle,
		}
		_, err := mr.UpdateItem(ctx, "nonexistent", input)
		if err == nil {
			t.Error("UpdateItem() expected error for nonexistent bean")
		}
	})
}

func TestMutationSetParent(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items
	parent := &item.Item{ID: "parent-1", Title: "Parent", Status: "todo", Type: "epic"}
	child := &item.Item{ID: "child-1", Title: "Child", Status: "todo", Type: "task"}
	core.Create(parent)
	core.Create(child)

	t.Run("set parent", func(t *testing.T) {
		mr := resolver.Mutation()
		parentID := "parent-1"
		got, err := mr.SetParent(ctx, "child-1", &parentID, nil)
		if err != nil {
			t.Fatalf("SetParent() error = %v", err)
		}
		if got.Parent != "parent-1" {
			t.Errorf("SetParent().Parent = %q, want %q", got.Parent, "parent-1")
		}
	})

	t.Run("clear parent", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.SetParent(ctx, "child-1", nil, nil)
		if err != nil {
			t.Fatalf("SetParent() error = %v", err)
		}
		if got.Parent != "" {
			t.Errorf("SetParent().Parent = %q, want empty", got.Parent)
		}
	})

	t.Run("set parent on nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		parentID := "parent-1"
		_, err := mr.SetParent(ctx, "nonexistent", &parentID, nil)
		if err == nil {
			t.Error("SetParent() expected error for nonexistent bean")
		}
	})
}

func TestMutationAddRemoveBlocking(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items
	blocker := &item.Item{ID: "blocker-1", Title: "Blocker", Status: "todo", Type: "task"}
	target := &item.Item{ID: "target-1", Title: "Target", Status: "todo", Type: "task"}
	core.Create(blocker)
	core.Create(target)

	t.Run("add block", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.AddBlocking(ctx, "blocker-1", "target-1", nil)
		if err != nil {
			t.Fatalf("AddBlocking() error = %v", err)
		}
		if len(got.Blocking) != 1 {
			t.Errorf("AddBlocking().Blocking count = %d, want 1", len(got.Blocking))
		}
		if got.Blocking[0] != "target-1" {
			t.Errorf("AddBlocking().Blocking[0] = %q, want %q", got.Blocking[0], "target-1")
		}
	})

	t.Run("remove block", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.RemoveBlocking(ctx, "blocker-1", "target-1", nil)
		if err != nil {
			t.Fatalf("RemoveBlocking() error = %v", err)
		}
		if len(got.Blocking) != 0 {
			t.Errorf("RemoveBlocking().Blocking count = %d, want 0", len(got.Blocking))
		}
	})

	t.Run("add block to nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.AddBlocking(ctx, "nonexistent", "target-1", nil)
		if err == nil {
			t.Error("AddBlocking() expected error for nonexistent bean")
		}
	})
}

func TestMutationDeleteItem(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("delete existing bean", func(t *testing.T) {
		// Create an item to delete
		b := &item.Item{ID: "delete-me", Title: "Delete Me", Status: "todo", Type: "task"}
		core.Create(b)

		mr := resolver.Mutation()
		got, err := mr.DeleteItem(ctx, "delete-me")
		if err != nil {
			t.Fatalf("DeleteItem() error = %v", err)
		}
		if !got {
			t.Error("DeleteItem() = false, want true")
		}

		// Verify it's gone
		qr := resolver.Query()
		item, _ := qr.Item(ctx, "delete-me")
		if item != nil {
			t.Error("Bean still exists after delete")
		}
	})

	t.Run("delete removes incoming links", func(t *testing.T) {
		// Create target item
		target := &item.Item{ID: "target-bean", Title: "Target", Status: "todo", Type: "task"}
		core.Create(target)

		// Create item that links to target
		linker := &item.Item{
			ID:       "linker-bean",
			Title:    "Linker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-bean"},
		}
		core.Create(linker)

		// Delete target - should remove the link from linker
		mr := resolver.Mutation()
		_, err := mr.DeleteItem(ctx, "target-bean")
		if err != nil {
			t.Fatalf("DeleteItem() error = %v", err)
		}

		// Verify linker no longer has the link
		qr := resolver.Query()
		updated, _ := qr.Item(ctx, "linker-bean")
		if updated == nil {
			t.Fatal("Linker bean was deleted unexpectedly")
		}
		if len(updated.Blocking) != 0 {
			t.Errorf("Linker still has %d blocks, want 0", len(updated.Blocking))
		}
	})

	t.Run("delete nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.DeleteItem(ctx, "nonexistent")
		if err == nil {
			t.Error("DeleteItem() expected error for nonexistent bean")
		}
	})
}

func TestSubscriptionItemChanged(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the file watcher
	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	// Create some test items before subscribing
	createTestItem(t, core, "existing-1", "Existing Bean 1", "todo")
	createTestItem(t, core, "existing-2", "Existing Bean 2", "in-progress")

	t.Run("includeInitial=true sends INITIAL_SNAPSHOT with all beans", func(t *testing.T) {
		sr := resolver.Subscription()
		includeInitial := true
		ch, err := sr.ItemChanged(ctx, &includeInitial)
		if err != nil {
			t.Fatalf("ItemChanged() error = %v", err)
		}

		// Should receive a single INITIAL_SNAPSHOT event with all items
		select {
		case event := <-ch:
			if event.Type != model.ChangeTypeInitialSnapshot {
				t.Errorf("Expected INITIAL_SNAPSHOT event, got %v", event.Type)
			}
			if len(event.Items) != 2 {
				t.Fatalf("Expected 2 beans in snapshot, got %d", len(event.Items))
			}
			received := make(map[string]bool)
			for _, b := range event.Items {
				received[b.ID] = true
			}
			if !received["existing-1"] || !received["existing-2"] {
				t.Errorf("Did not receive all expected beans: %v", received)
			}
		case <-ctx.Done():
			t.Fatal("Context cancelled before receiving snapshot")
		}
	})

	t.Run("includeInitial=false skips initial beans", func(t *testing.T) {
		sr := resolver.Subscription()
		includeInitial := false
		ch, err := sr.ItemChanged(ctx, &includeInitial)
		if err != nil {
			t.Fatalf("ItemChanged() error = %v", err)
		}

		// Channel should be waiting for real events, not sending anything immediately
		select {
		case event := <-ch:
			t.Errorf("Should not receive any events, got %v", event)
		default:
			// Expected: no events ready
		}
	})
}

func TestAgentSessionSubscription_DeliversLatestState(t *testing.T) {
	// Verify that when multiple rapid notifications occur, the subscriber
	// eventually receives the latest state (not a stale intermediate).
	mgr := agent.NewManager("", nil)

	itemID := "test-latest-value"
	resolver := &Resolver{AgentMgr: mgr}
	sr := resolver.Subscription()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := sr.AgentSessionChanged(ctx, itemID)
	if err != nil {
		t.Fatalf("AgentSessionChanged() error = %v", err)
	}

	// No session exists yet, so no initial emission. Fire rapid
	// notifications by adding multiple info messages without reading.
	for i := range 10 {
		mgr.AddInfoMessage(itemID, fmt.Sprintf("msg-%d", i))
	}

	// Drain the channel. Use a short idle timeout: once we stop receiving
	// updates for 100ms, assume all coalesced notifications have been delivered.
	var lastSession *model.AgentSession
	for {
		select {
		case s := <-ch:
			lastSession = s
		case <-time.After(100 * time.Millisecond):
			goto done
		}
	}
done:
	if lastSession == nil {
		t.Fatal("received no updates")
	}
	// The final state must include all info messages
	if len(lastSession.Messages) != 10 {
		t.Errorf("last update had %d messages, want 10", len(lastSession.Messages))
	}
}

func TestRelationshipFieldsWithFilter(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create a parent (milestone) with multiple children (tasks) of different statuses
	parent := &item.Item{
		ID:     "parent-filter-test",
		Title:  "Parent Milestone",
		Type:   "milestone",
		Status: "in-progress",
	}
	child1 := &item.Item{
		ID:     "child-todo",
		Title:  "Todo Task",
		Type:   "task",
		Status: "todo",
		Parent: "parent-filter-test",
	}
	child2 := &item.Item{
		ID:     "child-completed",
		Title:  "Completed Task",
		Type:   "task",
		Status: "completed",
		Parent: "parent-filter-test",
	}
	child3 := &item.Item{
		ID:       "child-inprogress",
		Title:    "In Progress Task",
		Type:     "task",
		Status:   "in-progress",
		Parent:   "parent-filter-test",
		Priority: "high",
	}

	// Create blocking relationships with different types
	blocker1 := &item.Item{
		ID:       "blocker-bug",
		Title:    "Blocking Bug",
		Type:     "bug",
		Status:   "todo",
		Blocking: []string{"child-todo"},
	}
	blocker2 := &item.Item{
		ID:       "blocker-task",
		Title:    "Blocking Task",
		Type:     "task",
		Status:   "completed",
		Blocking: []string{"child-todo"},
	}

	for _, b := range []*item.Item{parent, child1, child2, child3, blocker1, blocker2} {
		if err := core.Create(b); err != nil {
			t.Fatalf("Failed to create bean %s: %v", b.ID, err)
		}
	}

	br := resolver.Item()

	t.Run("children with status filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			Status: []string{"todo"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Children(filter status=todo) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "child-todo" {
			t.Errorf("Children(filter status=todo)[0].ID = %q, want %q", got[0].ID, "child-todo")
		}
	})

	t.Run("children with excludeStatus filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Children(filter excludeStatus=completed) count = %d, want 2", len(got))
		}
	})

	t.Run("children with priority filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			Priority: []string{"high"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Children(filter priority=high) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "child-inprogress" {
			t.Errorf("Children(filter priority=high)[0].ID = %q, want %q", got[0].ID, "child-inprogress")
		}
	})

	t.Run("children with nil filter returns all", func(t *testing.T) {
		got, err := br.Children(ctx, parent, nil)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("Children(nil filter) count = %d, want 3", len(got))
		}
	})

	t.Run("blockedBy with type filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			Type: []string{"bug"},
		}
		got, err := br.BlockedBy(ctx, child1, filter)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy(filter type=bug) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "blocker-bug" {
			t.Errorf("BlockedBy(filter type=bug)[0].ID = %q, want %q", got[0].ID, "blocker-bug")
		}
	})

	t.Run("blockedBy with excludeStatus filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := br.BlockedBy(ctx, child1, filter)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy(filter excludeStatus=completed) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "blocker-bug" {
			t.Errorf("BlockedBy(filter excludeStatus=completed)[0].ID = %q, want %q", got[0].ID, "blocker-bug")
		}
	})

	t.Run("blocking with status filter", func(t *testing.T) {
		filter := &model.ItemFilter{
			Status: []string{"todo"},
		}
		got, err := br.Blocking(ctx, blocker1, filter)
		if err != nil {
			t.Fatalf("Blocking() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Blocking(filter status=todo) count = %d, want 1", len(got))
		}
	})

	t.Run("blocking filter excludes all", func(t *testing.T) {
		filter := &model.ItemFilter{
			Status: []string{"completed"},
		}
		got, err := br.Blocking(ctx, blocker1, filter)
		if err != nil {
			t.Fatalf("Blocking() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("Blocking(filter status=completed) count = %d, want 0", len(got))
		}
	})
}

// setupTestResolverWithPrefix creates a test resolver with a configured prefix.
func setupTestResolverWithPrefix(t *testing.T, prefix string) (*Resolver, *itemcore.Core) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	cfg := config.DefaultWithPrefix(prefix)
	core := itemcore.New(itemsDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return &Resolver{CoreResolver: &itemgraph.CoreResolver{Core: core}}, core
}

// setupTestResolverWithRequireIfMatch creates a test resolver with require_if_match enabled.
func setupTestResolverWithRequireIfMatch(t *testing.T) (*Resolver, *itemcore.Core) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	cfg := config.Default()
	cfg.Items.RequireIfMatch = true
	core := itemcore.New(itemsDir, cfg)
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return &Resolver{CoreResolver: &itemgraph.CoreResolver{Core: core}}, core
}

func TestETagValidation(t *testing.T) {
	t.Run("update with correct etag succeeds", func(t *testing.T) {
		resolver, core := setupTestResolver(t)
		ctx := context.Background()

		b := &item.Item{ID: "etag-test-1", Title: "Test", Status: "todo"}
		core.Create(b)

		// Get current etag
		currentETag := b.ETag()

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateItemInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}
		got, err := mr.UpdateItem(ctx, "etag-test-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() with correct etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("update with incorrect etag fails", func(t *testing.T) {
		resolver, core := setupTestResolver(t)
		ctx := context.Background()

		b := &item.Item{ID: "etag-test-2", Title: "Test", Status: "todo"}
		core.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		wrongETag := "wrongetagvalue1"
		input := model.UpdateItemInput{
			Title:   &newTitle,
			IfMatch: &wrongETag,
		}
		_, err := mr.UpdateItem(ctx, "etag-test-2", input)
		if err == nil {
			t.Error("UpdateItem() with wrong etag should fail")
		}
		if !strings.Contains(err.Error(), "etag mismatch") {
			t.Errorf("Error should mention etag mismatch, got: %v", err)
		}
	})

	t.Run("update without etag succeeds when not required", func(t *testing.T) {
		resolver, core := setupTestResolver(t)
		ctx := context.Background()

		b := &item.Item{ID: "etag-test-3", Title: "Test", Status: "todo"}
		core.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateItemInput{
			Title: &newTitle,
		}
		got, err := mr.UpdateItem(ctx, "etag-test-3", input)
		if err != nil {
			t.Fatalf("UpdateItem() without etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Updated")
		}
	})
}

func TestRequireIfMatchConfig(t *testing.T) {
	t.Run("update without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, core := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b := &item.Item{ID: "require-etag-1", Title: "Test", Status: "todo"}
		core.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateItemInput{
			Title: &newTitle,
		}
		_, err := mr.UpdateItem(ctx, "require-etag-1", input)
		if err == nil {
			t.Error("UpdateItem() without etag should fail when require_if_match is true")
		}
		if !strings.Contains(err.Error(), "if-match etag is required") {
			t.Errorf("Error should mention etag is required, got: %v", err)
		}
	})

	t.Run("update with correct etag succeeds when require_if_match is true", func(t *testing.T) {
		resolver, core := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b := &item.Item{ID: "require-etag-2", Title: "Test", Status: "todo"}
		core.Create(b)

		currentETag := b.ETag()

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateItemInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}
		got, err := mr.UpdateItem(ctx, "require-etag-2", input)
		if err != nil {
			t.Fatalf("UpdateItem() with correct etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("setParent without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, core := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		parent := &item.Item{ID: "req-parent", Title: "Parent", Status: "todo", Type: "epic"}
		child := &item.Item{ID: "req-child", Title: "Child", Status: "todo", Type: "task"}
		core.Create(parent)
		core.Create(child)

		mr := resolver.Mutation()
		parentID := "req-parent"
		_, err := mr.SetParent(ctx, "req-child", &parentID, nil)
		if err == nil {
			t.Error("SetParent() without etag should fail when require_if_match is true")
		}
	})

	t.Run("addBlocking without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, core := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b1 := &item.Item{ID: "req-blocker", Title: "Blocker", Status: "todo"}
		b2 := &item.Item{ID: "req-target", Title: "Target", Status: "todo"}
		core.Create(b1)
		core.Create(b2)

		mr := resolver.Mutation()
		_, err := mr.AddBlocking(ctx, "req-blocker", "req-target", nil)
		if err == nil {
			t.Error("AddBlocking() without etag should fail when require_if_match is true")
		}
	})

	t.Run("removeBlocking without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, core := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b1 := &item.Item{ID: "req-blocker2", Title: "Blocker", Status: "todo", Blocking: []string{"req-target2"}}
		b2 := &item.Item{ID: "req-target2", Title: "Target", Status: "todo"}
		core.Create(b1)
		core.Create(b2)

		mr := resolver.Mutation()
		_, err := mr.RemoveBlocking(ctx, "req-blocker2", "req-target2", nil)
		if err == nil {
			t.Error("RemoveBlocking() without etag should fail when require_if_match is true")
		}
	})
}

func TestShortIDNormalization(t *testing.T) {
	// Use a prefix so we can test short ID resolution
	resolver, core := setupTestResolverWithPrefix(t, "beans-")
	ctx := context.Background()

	// Create test items with full IDs (prefix + short ID)
	parent := &item.Item{ID: "beans-parent1", Title: "Parent", Status: "todo", Type: "epic"}
	child := &item.Item{ID: "beans-child1", Title: "Child", Status: "todo", Type: "task"}
	target := &item.Item{ID: "beans-target1", Title: "Target", Status: "todo", Type: "task"}
	core.Create(parent)
	core.Create(child)
	core.Create(target)

	t.Run("SetParent normalizes short ID", func(t *testing.T) {
		mr := resolver.Mutation()
		// Use short ID (without prefix)
		shortParentID := "parent1"
		got, err := mr.SetParent(ctx, "beans-child1", &shortParentID, nil)
		if err != nil {
			t.Fatalf("SetParent() error = %v", err)
		}
		// Should store the full ID, not the short one
		if got.Parent != "beans-parent1" {
			t.Errorf("SetParent().Parent = %q, want %q", got.Parent, "beans-parent1")
		}
	})

	t.Run("AddBlocking normalizes short ID", func(t *testing.T) {
		mr := resolver.Mutation()
		// Use short ID (without prefix)
		got, err := mr.AddBlocking(ctx, "beans-child1", "target1", nil)
		if err != nil {
			t.Fatalf("AddBlocking() error = %v", err)
		}
		// Should store the full ID, not the short one
		if len(got.Blocking) != 1 {
			t.Fatalf("AddBlocking().Blocking count = %d, want 1", len(got.Blocking))
		}
		if got.Blocking[0] != "beans-target1" {
			t.Errorf("AddBlocking().Blocking[0] = %q, want %q", got.Blocking[0], "beans-target1")
		}
	})

	t.Run("RemoveBlocking normalizes short ID", func(t *testing.T) {
		mr := resolver.Mutation()
		// Remove using short ID
		got, err := mr.RemoveBlocking(ctx, "beans-child1", "target1", nil)
		if err != nil {
			t.Fatalf("RemoveBlocking() error = %v", err)
		}
		if len(got.Blocking) != 0 {
			t.Errorf("RemoveBlocking().Blocking count = %d, want 0", len(got.Blocking))
		}
	})

	t.Run("CreateItem normalizes parent short ID", func(t *testing.T) {
		mr := resolver.Mutation()
		itemType := "task"
		shortParentID := "parent1"
		input := model.CreateItemInput{
			Title:  "New Child",
			Type:   &itemType,
			Parent: &shortParentID,
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if got.Parent != "beans-parent1" {
			t.Errorf("CreateItem().Parent = %q, want %q", got.Parent, "beans-parent1")
		}
	})

	t.Run("CreateItem normalizes blocking short IDs", func(t *testing.T) {
		mr := resolver.Mutation()
		itemType := "task"
		input := model.CreateItemInput{
			Title:    "Blocker Bean",
			Type:     &itemType,
			Blocking: []string{"target1"},
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if len(got.Blocking) != 1 {
			t.Fatalf("CreateItem().Blocking count = %d, want 1", len(got.Blocking))
		}
		if got.Blocking[0] != "beans-target1" {
			t.Errorf("CreateItem().Blocking[0] = %q, want %q", got.Blocking[0], "beans-target1")
		}
	})
}

func TestUpdateItemWithBodyMod(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("bodyMod with single replacement only", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-1",
			Title:  "Test",
			Status: "todo",
			Body:   "## Tasks\n- [ ] Task 1\n- [ ] Task 2",
		}
		core.Create(b)

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task 1", New: "- [x] Task 1"},
				},
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		want := "## Tasks\n- [x] Task 1\n- [ ] Task 2"
		if got.Body != want {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with append only", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-2",
			Title:  "Test",
			Status: "todo",
			Body:   "Existing content",
		}
		core.Create(b)

		appendText := "## Notes\n\nNew section"
		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-2", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		want := "Existing content\n\n## Notes\n\nNew section"
		if got.Body != want {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with replacement and append combined", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-3",
			Title:  "Test",
			Status: "todo",
			Body:   "## Tasks\n- [ ] Deploy",
		}
		core.Create(b)

		appendText := "## Summary\n\nCompleted"
		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Deploy", New: "- [x] Deploy"},
				},
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-3", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		want := "## Tasks\n- [x] Deploy\n\n## Summary\n\nCompleted"
		if got.Body != want {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with multiple replacements sequential", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-4",
			Title:  "Test",
			Status: "todo",
			Body:   "- [ ] Task 1\n- [ ] Task 2\n- [ ] Task 3",
		}
		core.Create(b)

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task 1", New: "- [x] Task 1"},
					{Old: "- [ ] Task 2", New: "- [x] Task 2"},
					{Old: "- [ ] Task 3", New: "- [x] Task 3"},
				},
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-4", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		want := "- [x] Task 1\n- [x] Task 2\n- [x] Task 3"
		if got.Body != want {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with metadata update", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-5",
			Title:  "Test",
			Status: "todo",
			Body:   "- [ ] Task",
		}
		core.Create(b)

		status := "completed"
		appendText := "## Done"
		input := model.UpdateItemInput{
			Status: &status,
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task", New: "- [x] Task"},
				},
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-5", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		if got.Status != "completed" {
			t.Errorf("UpdateItem().Status = %q, want %q", got.Status, "completed")
		}
		want := "- [x] Task\n\n## Done"
		if got.Body != want {
			t.Errorf("UpdateItem().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("error when both body and bodyMod provided", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-6",
			Title:  "Test",
			Status: "todo",
			Body:   "Original",
		}
		core.Create(b)

		bodyText := "New body"
		appendText := "Append"
		input := model.UpdateItemInput{
			Body: &bodyText,
			BodyMod: &model.BodyModification{
				Append: &appendText,
			},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-6", input)
		if err == nil {
			t.Error("UpdateItem() expected error when both body and bodyMod provided")
		}
		if !strings.Contains(err.Error(), "cannot specify both body and bodyMod") {
			t.Errorf("Error should mention mutual exclusivity, got: %v", err)
		}
	})

	t.Run("error when replacement text not found", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-7",
			Title:  "Test",
			Status: "todo",
			Body:   "Hello world",
		}
		core.Create(b)

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "nonexistent", New: "fail"},
				},
			},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-7", input)
		if err == nil {
			t.Error("UpdateItem() expected error when replacement text not found")
		}
		if !strings.Contains(err.Error(), "text not found") {
			t.Errorf("Error should mention text not found, got: %v", err)
		}
	})

	t.Run("error when replacement text found multiple times", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-8",
			Title:  "Test",
			Status: "todo",
			Body:   "foo foo foo",
		}
		core.Create(b)

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "foo", New: "bar"},
				},
			},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-8", input)
		if err == nil {
			t.Error("UpdateItem() expected error when replacement text found multiple times")
		}
		if !strings.Contains(err.Error(), "found 3 times") {
			t.Errorf("Error should mention multiple matches, got: %v", err)
		}
	})

	t.Run("transactional: later replacement fails, nothing saved", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-9",
			Title:  "Test",
			Status: "todo",
			Body:   "Task 1\nTask 2",
		}
		core.Create(b)
		originalBody := b.Body

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "Task 1", New: "Done 1"},    // This should succeed
					{Old: "nonexistent", New: "fail"}, // This should fail
				},
			},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-9", input)
		if err == nil {
			t.Error("UpdateItem() expected error")
		}

		// Verify item wasn't modified
		updated, _ := core.Get("bodymod-test-9")
		if updated.Body != originalBody {
			t.Errorf("Bean body was modified despite error. Got %q, want %q", updated.Body, originalBody)
		}
	})

	t.Run("empty append is no-op", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-10",
			Title:  "Test",
			Status: "todo",
			Body:   "Original content",
		}
		core.Create(b)

		emptyAppend := ""
		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Append: &emptyAppend,
			},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-10", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}
		if got.Body != "Original content" {
			t.Errorf("UpdateItem().Body = %q, want %q (no-op for empty append)", got.Body, "Original content")
		}
	})

	t.Run("transactional: later replacement fails, nothing saved", func(t *testing.T) {
		b := &item.Item{
			ID:     "bodymod-test-9",
			Title:  "Test",
			Status: "todo",
			Body:   "Task 1\nTask 2",
		}
		core.Create(b)
		originalBody := b.Body

		input := model.UpdateItemInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "Task 1", New: "Done 1"},    // This should succeed
					{Old: "nonexistent", New: "fail"}, // This should fail
				},
			},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "bodymod-test-9", input)
		if err == nil {
			t.Error("UpdateItem() expected error")
		}

		// Verify item wasn't modified
		updated, _ := core.Get("bodymod-test-9")
		if updated.Body != originalBody {
			t.Errorf("Bean body was modified despite error. Got %q, want %q", updated.Body, originalBody)
		}
	})
}

func TestUpdateItemWithRelationships(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("atomic update with parent and blocking", func(t *testing.T) {
		epic := &item.Item{ID: "epic-1", Title: "Epic", Type: "epic", Status: "todo"}
		task := &item.Item{ID: "task-1", Title: "Task", Type: "task", Status: "todo"}
		blocker := &item.Item{ID: "blocker-1", Title: "Blocker", Type: "task", Status: "todo"}
		core.Create(epic)
		core.Create(task)
		core.Create(blocker)

		input := model.UpdateItemInput{
			Status:      stringPtr("in-progress"),
			Parent:      stringPtr("epic-1"),
			AddBlocking: []string{"blocker-1"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if got.Status != "in-progress" {
			t.Errorf("UpdateItem().Status = %q, want %q", got.Status, "in-progress")
		}
		if got.Parent != "epic-1" {
			t.Errorf("UpdateItem().Parent = %q, want %q", got.Parent, "epic-1")
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "blocker-1" {
			t.Errorf("UpdateItem().Blocking = %v, want [blocker-1]", got.Blocking)
		}
	})

	t.Run("atomic update with bodyMod and relationships", func(t *testing.T) {
		epic := &item.Item{ID: "epic-2", Title: "Epic", Type: "epic", Status: "todo"}
		task := &item.Item{ID: "task-2", Title: "Task", Type: "task", Status: "todo", Body: "- [ ] Step 1"}
		blocker := &item.Item{ID: "blocker-2", Title: "Blocker", Type: "task", Status: "todo"}
		core.Create(epic)
		core.Create(task)
		core.Create(blocker)

		input := model.UpdateItemInput{
			Status: stringPtr("completed"),
			Parent: stringPtr("epic-2"),
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Step 1", New: "- [x] Step 1"},
				},
				Append: stringPtr("## Done"),
			},
			AddBlocking: []string{"blocker-2"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-2", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if got.Status != "completed" {
			t.Errorf("Status = %q, want completed", got.Status)
		}
		if got.Parent != "epic-2" {
			t.Errorf("Parent = %q, want epic-2", got.Parent)
		}
		if !strings.Contains(got.Body, "- [x] Step 1") {
			t.Errorf("Body missing completed task")
		}
		if !strings.Contains(got.Body, "## Done") {
			t.Errorf("Body missing appended content")
		}
		if len(got.Blocking) != 1 {
			t.Errorf("Blocking count = %d, want 1", len(got.Blocking))
		}
	})

	t.Run("parent validation fails for invalid type hierarchy", func(t *testing.T) {
		task1 := &item.Item{ID: "task-invalid-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &item.Item{ID: "task-invalid-2", Title: "Task 2", Type: "task", Status: "todo"}
		core.Create(task1)
		core.Create(task2)

		input := model.UpdateItemInput{
			Parent: stringPtr("task-invalid-2"),
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-invalid-1", input)
		if err == nil {
			t.Error("UpdateItem() should fail for invalid parent type")
		}
	})

	t.Run("blocking self-reference validation", func(t *testing.T) {
		task := &item.Item{ID: "task-self", Title: "Task", Type: "task", Status: "todo"}
		core.Create(task)

		input := model.UpdateItemInput{
			AddBlocking: []string{"task-self"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-self", input)
		if err == nil {
			t.Error("UpdateItem() should fail when bean blocks itself")
		}
		if !strings.Contains(err.Error(), "block itself") {
			t.Errorf("Error should mention self-blocking, got: %v", err)
		}
	})

	t.Run("blocking cycle detection", func(t *testing.T) {
		task1 := &item.Item{ID: "task-block-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &item.Item{ID: "task-block-2", Title: "Task 2", Type: "task", Status: "todo", Blocking: []string{"task-block-1"}}
		core.Create(task1)
		core.Create(task2)

		// Try to make task-1 block task-2 (would create cycle)
		input := model.UpdateItemInput{
			AddBlocking: []string{"task-block-2"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-block-1", input)
		if err == nil {
			t.Error("UpdateItem() should fail when creating blocking cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocking target not found", func(t *testing.T) {
		task := &item.Item{ID: "task-notfound", Title: "Task", Type: "task", Status: "todo"}
		core.Create(task)

		input := model.UpdateItemInput{
			AddBlocking: []string{"nonexistent"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-notfound", input)
		if err == nil {
			t.Error("UpdateItem() should fail when blocking target doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("remove blocking relationships", func(t *testing.T) {
		task := &item.Item{ID: "task-remove-1", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"other-1", "other-2"}}
		other1 := &item.Item{ID: "other-1", Title: "Other 1", Type: "task", Status: "todo"}
		other2 := &item.Item{ID: "other-2", Title: "Other 2", Type: "task", Status: "todo"}
		core.Create(task)
		core.Create(other1)
		core.Create(other2)

		input := model.UpdateItemInput{
			RemoveBlocking: []string{"other-1"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-remove-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Blocking) != 1 || got.Blocking[0] != "other-2" {
			t.Errorf("Blocking = %v, want [other-2]", got.Blocking)
		}
	})

	t.Run("blockedBy self-reference validation", func(t *testing.T) {
		task := &item.Item{ID: "task-blockedby-self", Title: "Task", Type: "task", Status: "todo"}
		core.Create(task)

		input := model.UpdateItemInput{
			AddBlockedBy: []string{"task-blockedby-self"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-blockedby-self", input)
		if err == nil {
			t.Error("UpdateItem() should fail when bean is blocked by itself")
		}
		if !strings.Contains(err.Error(), "blocked by itself") {
			t.Errorf("Error should mention self-blocking, got: %v", err)
		}
	})

	t.Run("blockedBy target not found", func(t *testing.T) {
		task := &item.Item{ID: "task-blockedby-notfound", Title: "Task", Type: "task", Status: "todo"}
		core.Create(task)

		input := model.UpdateItemInput{
			AddBlockedBy: []string{"nonexistent"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-blockedby-notfound", input)
		if err == nil {
			t.Error("UpdateItem() should fail when blocker doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("combined add and remove operations", func(t *testing.T) {
		task := &item.Item{ID: "task-combined", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"old-1"}}
		old1 := &item.Item{ID: "old-1", Title: "Old", Type: "task", Status: "todo"}
		new1 := &item.Item{ID: "new-1", Title: "New", Type: "task", Status: "todo"}
		core.Create(task)
		core.Create(old1)
		core.Create(new1)

		input := model.UpdateItemInput{
			RemoveBlocking: []string{"old-1"},
			AddBlocking:    []string{"new-1"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-combined", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Blocking) != 1 || got.Blocking[0] != "new-1" {
			t.Errorf("Blocking = %v, want [new-1]", got.Blocking)
		}
	})

	t.Run("blockedBy cycle detection", func(t *testing.T) {
		task1 := &item.Item{ID: "task-blockedby-cycle-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &item.Item{ID: "task-blockedby-cycle-2", Title: "Task 2", Type: "task", Status: "todo", BlockedBy: []string{"task-blockedby-cycle-1"}}
		core.Create(task1)
		core.Create(task2)

		// Try to make task-1 blocked by task-2 (would create cycle)
		input := model.UpdateItemInput{
			AddBlockedBy: []string{"task-blockedby-cycle-2"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-blockedby-cycle-1", input)
		if err == nil {
			t.Error("UpdateItem() should fail when creating blockedBy cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("remove parent", func(t *testing.T) {
		epic := &item.Item{ID: "epic-parent-remove", Title: "Epic", Type: "epic", Status: "todo"}
		task := &item.Item{ID: "task-parent-remove", Title: "Task", Type: "task", Status: "todo", Parent: "epic-parent-remove"}
		core.Create(epic)
		core.Create(task)

		// Remove parent by setting to empty string
		emptyParent := ""
		input := model.UpdateItemInput{
			Parent: &emptyParent,
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-parent-remove", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if got.Parent != "" {
			t.Errorf("Parent = %q, want empty string", got.Parent)
		}
	})

	t.Run("remove blockedBy relationships", func(t *testing.T) {
		task := &item.Item{ID: "task-remove-blockedby", Title: "Task", Type: "task", Status: "todo", BlockedBy: []string{"blocker-1", "blocker-2"}}
		blocker1 := &item.Item{ID: "blocker-1", Title: "Blocker 1", Type: "task", Status: "todo"}
		blocker2 := &item.Item{ID: "blocker-2", Title: "Blocker 2", Type: "task", Status: "todo"}
		core.Create(task)
		core.Create(blocker1)
		core.Create(blocker2)

		input := model.UpdateItemInput{
			RemoveBlockedBy: []string{"blocker-1"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-remove-blockedby", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.BlockedBy) != 1 || got.BlockedBy[0] != "blocker-2" {
			t.Errorf("BlockedBy = %v, want [blocker-2]", got.BlockedBy)
		}
	})

	t.Run("multiple blocking additions", func(t *testing.T) {
		task := &item.Item{ID: "task-multi-blocking", Title: "Task", Type: "task", Status: "todo"}
		target1 := &item.Item{ID: "target-1", Title: "Target 1", Type: "task", Status: "todo"}
		target2 := &item.Item{ID: "target-2", Title: "Target 2", Type: "task", Status: "todo"}
		core.Create(task)
		core.Create(target1)
		core.Create(target2)

		input := model.UpdateItemInput{
			AddBlocking: []string{"target-1", "target-2"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-multi-blocking", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Blocking) != 2 {
			t.Errorf("Blocking count = %d, want 2", len(got.Blocking))
		}
	})

	t.Run("all relationship types combined", func(t *testing.T) {
		epic := &item.Item{ID: "epic-all", Title: "Epic", Type: "epic", Status: "todo"}
		task := &item.Item{ID: "task-all", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"old-blocking"}}
		blocker := &item.Item{ID: "new-blocker", Title: "Blocker", Type: "task", Status: "todo"}
		blocked := &item.Item{ID: "new-blocked", Title: "Blocked", Type: "task", Status: "todo"}
		oldBlocking := &item.Item{ID: "old-blocking", Title: "Old Blocking", Type: "task", Status: "todo"}
		core.Create(epic)
		core.Create(task)
		core.Create(blocker)
		core.Create(blocked)
		core.Create(oldBlocking)

		input := model.UpdateItemInput{
			Status:         stringPtr("in-progress"),
			Parent:         stringPtr("epic-all"),
			AddBlocking:    []string{"new-blocked"},
			RemoveBlocking: []string{"old-blocking"},
			AddBlockedBy:   []string{"new-blocker"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-all", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if got.Status != "in-progress" {
			t.Errorf("Status = %q, want in-progress", got.Status)
		}
		if got.Parent != "epic-all" {
			t.Errorf("Parent = %q, want epic-all", got.Parent)
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "new-blocked" {
			t.Errorf("Blocking = %v, want [new-blocked]", got.Blocking)
		}
		if len(got.BlockedBy) != 1 || got.BlockedBy[0] != "new-blocker" {
			t.Errorf("BlockedBy = %v, want [new-blocker]", got.BlockedBy)
		}
	})

	t.Run("add tags", func(t *testing.T) {
		task := &item.Item{ID: "task-tags-1", Title: "Task", Type: "task", Status: "todo", Tags: []string{"existing"}}
		core.Create(task)

		input := model.UpdateItemInput{
			AddTags: []string{"new1", "new2"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-tags-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Tags) != 3 {
			t.Errorf("Tags count = %d, want 3", len(got.Tags))
		}
		tagSet := make(map[string]bool)
		for _, tag := range got.Tags {
			tagSet[tag] = true
		}
		if !tagSet["existing"] || !tagSet["new1"] || !tagSet["new2"] {
			t.Errorf("Tags = %v, want [existing new1 new2]", got.Tags)
		}
	})

	t.Run("remove tags", func(t *testing.T) {
		task := &item.Item{ID: "task-tags-2", Title: "Task", Type: "task", Status: "todo", Tags: []string{"tag1", "tag2", "tag3"}}
		core.Create(task)

		input := model.UpdateItemInput{
			RemoveTags: []string{"tag2"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-tags-2", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Tags) != 2 {
			t.Errorf("Tags count = %d, want 2", len(got.Tags))
		}
		for _, tag := range got.Tags {
			if tag == "tag2" {
				t.Error("Tag 'tag2' should have been removed")
			}
		}
	})

	t.Run("add and remove tags in one operation", func(t *testing.T) {
		task := &item.Item{ID: "task-tags-3", Title: "Task", Type: "task", Status: "todo", Tags: []string{"old1", "old2", "keep"}}
		core.Create(task)

		input := model.UpdateItemInput{
			AddTags:    []string{"new1", "new2"},
			RemoveTags: []string{"old1", "old2"},
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "task-tags-3", input)
		if err != nil {
			t.Fatalf("UpdateItem() error = %v", err)
		}

		if len(got.Tags) != 3 {
			t.Errorf("Tags count = %d, want 3", len(got.Tags))
		}
		tagSet := make(map[string]bool)
		for _, tag := range got.Tags {
			tagSet[tag] = true
		}
		if !tagSet["keep"] || !tagSet["new1"] || !tagSet["new2"] {
			t.Errorf("Tags = %v, want [keep new1 new2]", got.Tags)
		}
		if tagSet["old1"] || tagSet["old2"] {
			t.Errorf("Tags = %v, should not contain old1 or old2", got.Tags)
		}
	})

	t.Run("tags and addTags are mutually exclusive", func(t *testing.T) {
		task := &item.Item{ID: "task-tags-4", Title: "Task", Type: "task", Status: "todo"}
		core.Create(task)

		input := model.UpdateItemInput{
			Tags:    []string{"tag1"},
			AddTags: []string{"tag2"},
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "task-tags-4", input)
		if err == nil {
			t.Error("UpdateItem() should fail when both tags and addTags are specified")
		}
		if !strings.Contains(err.Error(), "cannot specify both") {
			t.Errorf("Error should mention conflict, got: %v", err)
		}
	})
}

// Helper function for tests
func stringPtr(s string) *string {
	return &s
}

func TestBlockedByCycleDetection(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("blocked_by self-reference fails", func(t *testing.T) {
		b := &item.Item{ID: "self-ref", Title: "Self Reference", Status: "todo"}
		core.Create(b)

		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "self-ref", "self-ref", nil)
		if err == nil {
			t.Error("AddBlockedBy() should fail for self-reference")
		}
		if !strings.Contains(err.Error(), "blocked by itself") {
			t.Errorf("Error should mention self-reference, got: %v", err)
		}
	})

	t.Run("blocked_by cycle via blocked_by only is detected", func(t *testing.T) {
		// This tests the scenario where cycles are created using only blocked_by
		a := &item.Item{ID: "cycle-a", Title: "Bean A", Status: "todo"}
		b := &item.Item{ID: "cycle-b", Title: "Bean B", Status: "todo"}
		core.Create(a)
		core.Create(b)

		mr := resolver.Mutation()

		// A is blocked by B (B → A)
		_, err := mr.AddBlockedBy(ctx, "cycle-a", "cycle-b", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy(A, B) error = %v", err)
		}

		// B is blocked by A (A → B) - should create cycle A → B → A
		_, err = mr.AddBlockedBy(ctx, "cycle-b", "cycle-a", nil)
		if err == nil {
			t.Error("AddBlockedBy(B, A) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocked_by cycle via blocking is detected", func(t *testing.T) {
		// A blocks B, then B is blocked_by A creates a conflict
		a := &item.Item{ID: "cross-a", Title: "Bean A", Status: "todo"}
		b := &item.Item{ID: "cross-b", Title: "Bean B", Status: "todo"}
		core.Create(a)
		core.Create(b)

		mr := resolver.Mutation()

		// A blocks B (A → B)
		_, err := mr.AddBlocking(ctx, "cross-a", "cross-b", nil)
		if err != nil {
			t.Fatalf("AddBlocking(A, B) error = %v", err)
		}

		// A is blocked by B (B → A) - should create cycle
		_, err = mr.AddBlockedBy(ctx, "cross-a", "cross-b", nil)
		if err == nil {
			t.Error("AddBlockedBy(A, B) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocking cycle via blocked_by is detected", func(t *testing.T) {
		// A is blocked_by B, then A blocking B creates a conflict
		a := &item.Item{ID: "cross2-a", Title: "Bean A", Status: "todo"}
		b := &item.Item{ID: "cross2-b", Title: "Bean B", Status: "todo"}
		core.Create(a)
		core.Create(b)

		mr := resolver.Mutation()

		// A is blocked by B (B → A)
		_, err := mr.AddBlockedBy(ctx, "cross2-a", "cross2-b", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy(A, B) error = %v", err)
		}

		// A blocks B (A → B) - should create cycle
		_, err = mr.AddBlocking(ctx, "cross2-a", "cross2-b", nil)
		if err == nil {
			t.Error("AddBlocking(A, B) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocker bean not found fails", func(t *testing.T) {
		a := &item.Item{ID: "exists-a", Title: "Bean A", Status: "todo"}
		core.Create(a)

		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "exists-a", "nonexistent", nil)
		if err == nil {
			t.Error("AddBlockedBy() should fail when blocker doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})
}

func TestCreateItemBlockedByValidation(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with blocked_by referencing nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title:     "New Bean",
			BlockedBy: []string{"nonexistent"},
		}
		_, err := mr.CreateItem(ctx, input)
		if err == nil {
			t.Error("CreateItem() should fail when blocked_by references nonexistent bean")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("create with blocking referencing nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title:    "New Bean",
			Blocking: []string{"nonexistent"},
		}
		_, err := mr.CreateItem(ctx, input)
		if err == nil {
			t.Error("CreateItem() should fail when blocking references nonexistent bean")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("create with same bean in both blocking and blocked_by fails", func(t *testing.T) {
		target := &item.Item{ID: "target-bean", Title: "Target", Status: "todo"}
		core.Create(target)

		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title:     "Cyclic Bean",
			Blocking:  []string{"target-bean"},
			BlockedBy: []string{"target-bean"},
		}
		_, err := mr.CreateItem(ctx, input)
		if err == nil {
			t.Error("CreateItem() should fail when same bean is in both blocking and blocked_by")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("create with valid blocked_by succeeds", func(t *testing.T) {
		blocker := &item.Item{ID: "valid-blocker", Title: "Blocker", Status: "todo"}
		core.Create(blocker)

		mr := resolver.Mutation()
		input := model.CreateItemInput{
			Title:     "Blocked Bean",
			BlockedBy: []string{"valid-blocker"},
		}
		got, err := mr.CreateItem(ctx, input)
		if err != nil {
			t.Fatalf("CreateItem() error = %v", err)
		}
		if len(got.BlockedBy) != 1 {
			t.Errorf("CreateItem().BlockedBy count = %d, want 1", len(got.BlockedBy))
		}
		if got.BlockedBy[0] != "valid-blocker" {
			t.Errorf("CreateItem().BlockedBy[0] = %q, want %q", got.BlockedBy[0], "valid-blocker")
		}
	})
}

func TestMutationAddRemoveBlockedBy(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create test items
	blocked := &item.Item{ID: "blocked-1", Title: "Blocked", Status: "todo"}
	blocker := &item.Item{ID: "blocker-1", Title: "Blocker", Status: "todo"}
	core.Create(blocked)
	core.Create(blocker)

	t.Run("add blocked_by", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.AddBlockedBy(ctx, "blocked-1", "blocker-1", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy() error = %v", err)
		}
		if len(got.BlockedBy) != 1 {
			t.Errorf("AddBlockedBy().BlockedBy count = %d, want 1", len(got.BlockedBy))
		}
		if got.BlockedBy[0] != "blocker-1" {
			t.Errorf("AddBlockedBy().BlockedBy[0] = %q, want %q", got.BlockedBy[0], "blocker-1")
		}
	})

	t.Run("remove blocked_by", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.RemoveBlockedBy(ctx, "blocked-1", "blocker-1", nil)
		if err != nil {
			t.Fatalf("RemoveBlockedBy() error = %v", err)
		}
		if len(got.BlockedBy) != 0 {
			t.Errorf("RemoveBlockedBy().BlockedBy count = %d, want 0", len(got.BlockedBy))
		}
	})

	t.Run("add blocked_by to nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "nonexistent", "blocker-1", nil)
		if err == nil {
			t.Error("AddBlockedBy() expected error for nonexistent bean")
		}
	})
}

func TestBlockedByResolverCombinesBothDirections(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("blocked_by field is resolved by blockedBy resolver", func(t *testing.T) {
		blocker := &item.Item{ID: "blocker-r1", Title: "Blocker", Status: "todo", Type: "task"}
		blocked := &item.Item{ID: "blocked-r1", Title: "Blocked", Status: "todo", Type: "task", BlockedBy: []string{"blocker-r1"}}
		core.Create(blocker)
		core.Create(blocked)

		br := resolver.Item()
		result, err := br.BlockedBy(ctx, blocked, nil)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(result) != 1 || result[0].ID != "blocker-r1" {
			t.Errorf("BlockedBy() = %v, want [blocker-r1]", result)
		}
	})

	t.Run("incoming blocking links are resolved by blockedBy resolver", func(t *testing.T) {
		blocker := &item.Item{ID: "blocker-r2", Title: "Blocker", Status: "todo", Type: "task", Blocking: []string{"blocked-r2"}}
		blocked := &item.Item{ID: "blocked-r2", Title: "Blocked", Status: "todo", Type: "task"}
		core.Create(blocker)
		core.Create(blocked)

		br := resolver.Item()
		result, err := br.BlockedBy(ctx, blocked, nil)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(result) != 1 || result[0].ID != "blocker-r2" {
			t.Errorf("BlockedBy() = %v, want [blocker-r2]", result)
		}
	})

	t.Run("both directions are combined and deduplicated", func(t *testing.T) {
		// blocker blocks target via both directions
		blocker := &item.Item{ID: "blocker-r3", Title: "Blocker", Status: "todo", Type: "task", Blocking: []string{"blocked-r3"}}
		blocked := &item.Item{ID: "blocked-r3", Title: "Blocked", Status: "todo", Type: "task", BlockedBy: []string{"blocker-r3"}}
		core.Create(blocker)
		core.Create(blocked)

		br := resolver.Item()
		result, err := br.BlockedBy(ctx, blocked, nil)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(result) != 1 {
			t.Errorf("BlockedBy() returned %d beans, want 1 (deduplicated)", len(result))
		}
	})
}

func TestUpdateItemWithETag(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	t.Run("update with correct etag succeeds", func(t *testing.T) {
		b := &item.Item{
			ID:     "etag-update-1",
			Title:  "Test",
			Status: "todo",
		}
		core.Create(b)

		currentETag := b.ETag()
		newTitle := "Updated"
		input := model.UpdateItemInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}

		got, err := resolver.Mutation().UpdateItem(ctx, "etag-update-1", input)
		if err != nil {
			t.Fatalf("UpdateItem() with correct etag failed: %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateItem().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("update with wrong etag fails", func(t *testing.T) {
		b := &item.Item{
			ID:     "etag-update-2",
			Title:  "Test",
			Status: "todo",
		}
		core.Create(b)

		wrongETag := "wrongetag123"
		newTitle := "Should Fail"
		input := model.UpdateItemInput{
			Title:   &newTitle,
			IfMatch: &wrongETag,
		}

		_, err := resolver.Mutation().UpdateItem(ctx, "etag-update-2", input)
		if err == nil {
			t.Error("UpdateItem() with wrong etag should fail")
		}

		var mismatchErr *itemcore.ETagMismatchError
		if !errors.As(err, &mismatchErr) {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestSetParentWithETag(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create parent
	parent := &item.Item{
		ID:     "parent-etag",
		Title:  "Parent",
		Status: "todo",
		Type:   "epic",
	}
	core.Create(parent)

	t.Run("setParent with correct etag succeeds", func(t *testing.T) {
		child := &item.Item{
			ID:     "child-etag-1",
			Title:  "Child",
			Status: "todo",
			Type:   "task",
		}
		core.Create(child)

		currentETag := child.ETag()
		parentID := "parent-etag"

		got, err := resolver.Mutation().SetParent(ctx, "child-etag-1", &parentID, &currentETag)
		if err != nil {
			t.Fatalf("SetParent() with correct etag failed: %v", err)
		}
		if got.Parent != "parent-etag" {
			t.Errorf("SetParent().Parent = %q, want %q", got.Parent, "parent-etag")
		}
	})

	t.Run("setParent with wrong etag fails", func(t *testing.T) {
		child := &item.Item{
			ID:     "child-etag-2",
			Title:  "Child",
			Status: "todo",
			Type:   "task",
		}
		core.Create(child)

		wrongETag := "wrongetag123"
		parentID := "parent-etag"

		_, err := resolver.Mutation().SetParent(ctx, "child-etag-2", &parentID, &wrongETag)
		if err == nil {
			t.Error("SetParent() with wrong etag should fail")
		}

		var mismatchErr *itemcore.ETagMismatchError
		if !errors.As(err, &mismatchErr) {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestAddBlockingWithETag(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create target item
	target := &item.Item{
		ID:     "target-etag",
		Title:  "Target",
		Status: "todo",
		Type:   "task",
	}
	core.Create(target)

	t.Run("addBlocking with correct etag succeeds", func(t *testing.T) {
		blocker := &item.Item{
			ID:     "blocker-etag-1",
			Title:  "Blocker",
			Status: "todo",
			Type:   "task",
		}
		core.Create(blocker)

		currentETag := blocker.ETag()

		got, err := resolver.Mutation().AddBlocking(ctx, "blocker-etag-1", "target-etag", &currentETag)
		if err != nil {
			t.Fatalf("AddBlocking() with correct etag failed: %v", err)
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "target-etag" {
			t.Errorf("AddBlocking().Blocking = %v, want [target-etag]", got.Blocking)
		}
	})

	t.Run("addBlocking with wrong etag fails", func(t *testing.T) {
		blocker := &item.Item{
			ID:     "blocker-etag-2",
			Title:  "Blocker",
			Status: "todo",
			Type:   "task",
		}
		core.Create(blocker)

		wrongETag := "wrongetag123"

		_, err := resolver.Mutation().AddBlocking(ctx, "blocker-etag-2", "target-etag", &wrongETag)
		if err == nil {
			t.Error("AddBlocking() with wrong etag should fail")
		}

		var mismatchErr *itemcore.ETagMismatchError
		if !errors.As(err, &mismatchErr) {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestQueryAgentActions(t *testing.T) {
	ctx := context.Background()

	t.Run("returns always-visible actions for unknown bean", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		qr := resolver.Query()
		actions, err := qr.AgentActions(ctx, "any-bean-id", nil)
		if err != nil {
			t.Fatalf("AgentActions() error = %v", err)
		}
		// start-work is hidden (no worktree), so only commit + review
		if len(actions) != 2 {
			t.Fatalf("AgentActions() count = %d, want 2", len(actions))
		}
		if actions[0].ID != "commit" {
			t.Errorf("actions[0].ID = %q, want %q", actions[0].ID, "commit")
		}
		if actions[1].ID != "review" {
			t.Errorf("actions[1].ID = %q, want %q", actions[1].ID, "review")
		}
	})
}

func TestConditionalActionVisibility(t *testing.T) {
	// All actions that are only visible when changes exist should share the same behavior
	conditionalActions := []string{"tests", "learn", "integrate"}

	for _, actionID := range conditionalActions {
		action := findAgentAction(actionID)
		if action == nil {
			t.Fatalf("%s action not found", actionID)
		}

		tests := []struct {
			name    string
			ctx     actionContext
			visible bool
		}{
			{"hidden when no changes and no new commits", actionContext{}, false},
			{"visible with uncommitted changes", actionContext{HasChanges: true}, true},
			{"visible with new commits", actionContext{HasNewCommits: true}, true},
			{"visible with both", actionContext{HasChanges: true, HasNewCommits: true}, true},
		}

		for _, tt := range tests {
			t.Run(actionID+"/"+tt.name, func(t *testing.T) {
				got := action.Visible(tt.ctx)
				if got != tt.visible {
					t.Errorf("Visible() = %v, want %v", got, tt.visible)
				}
			})
		}
	}
}

func TestIntegrateActionDisabledWhenMainHasChanges(t *testing.T) {
	action := findAgentAction("integrate")
	if action == nil {
		t.Fatal("integrate action not found")
	}
	if action.Disabled == nil {
		t.Fatal("integrate action should have a Disabled function")
	}

	t.Run("enabled when main repo is clean", func(t *testing.T) {
		reason := action.Disabled(actionContext{HasChanges: true})
		if reason != "" {
			t.Errorf("Disabled() = %q, want empty", reason)
		}
	})

	t.Run("disabled when main repo has changes", func(t *testing.T) {
		reason := action.Disabled(actionContext{HasChanges: true, MainRepoHasChanges: true})
		if reason == "" {
			t.Error("Disabled() = empty, want a reason string")
		}
	})
}

func TestExecuteAgentAction(t *testing.T) {
	t.Run("nil agent manager", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		mr := &mutationResolver{resolver}

		_, err := mr.ExecuteAgentAction(context.Background(), "__central__", "commit")
		if err == nil {
			t.Fatal("expected error when AgentMgr is nil")
		}
		if err.Error() != "agent manager not available" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		resolver.AgentMgr = agent.NewManager("", nil)
		resolver.ProjectRoot = t.TempDir()
		mr := &mutationResolver{resolver}

		_, err := mr.ExecuteAgentAction(context.Background(), "__central__", "nonexistent")
		if err == nil {
			t.Fatal("expected error for unknown action")
		}
		if err.Error() != "unknown agent action: nonexistent" {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("commit action", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		resolver.AgentMgr = agent.NewManager("", nil)
		resolver.ProjectRoot = t.TempDir()
		mr := &mutationResolver{resolver}

		result, err := mr.ExecuteAgentAction(context.Background(), CentralSessionID, "commit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Fatal("expected true result")
		}

		session := resolver.AgentMgr.GetSession(CentralSessionID)
		if session == nil {
			t.Fatal("expected session to exist")
		}
		if len(session.Messages) == 0 {
			t.Fatal("expected at least one message")
		}
		lastMsg := session.Messages[len(session.Messages)-1]
		if lastMsg.Role != agent.RoleUser {
			t.Fatalf("expected user role, got %s", lastMsg.Role)
		}
		if lastMsg.Content == "" {
			t.Fatal("expected non-empty commit prompt")
		}
	})

	t.Run("review action", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		resolver.AgentMgr = agent.NewManager("", nil)
		resolver.ProjectRoot = t.TempDir()
		mr := &mutationResolver{resolver}

		result, err := mr.ExecuteAgentAction(context.Background(), CentralSessionID, "review")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result {
			t.Fatal("expected true result")
		}

		session := resolver.AgentMgr.GetSession(CentralSessionID)
		if session == nil {
			t.Fatal("expected session to exist")
		}
		lastMsg := session.Messages[len(session.Messages)-1]
		if !strings.Contains(lastMsg.Content, "Do NOT fix anything automatically") {
			t.Fatalf("expected review prompt with no-auto-fix instruction, got: %s", lastMsg.Content)
		}
	})

	t.Run("non-central without worktree", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		resolver.AgentMgr = agent.NewManager("", nil)
		mr := &mutationResolver{resolver}

		_, err := mr.ExecuteAgentAction(context.Background(), "some-bean", "commit")
		if err == nil {
			t.Fatal("expected error for non-central bean without worktree")
		}
	})
}

// Without an agent manager, which lif-serve leaves out beyond loopback unless
// --allow-remote-agent is given, the server refuses every agent mutation and
// tells the UI to hide the agent.
func TestAgentRefusedWithoutManager(t *testing.T) {
	ctx := context.Background()

	t.Run("no manager", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)

		enabled, err := (&queryResolver{resolver}).AgentEnabled(ctx)
		if err != nil || enabled {
			t.Errorf("AgentEnabled() = %v, %v; want false, nil", enabled, err)
		}

		sent, err := (&mutationResolver{resolver}).SendAgentMessage(ctx, CentralSessionID, "run ls", nil, nil)
		if err == nil || sent || !strings.Contains(err.Error(), "agent manager not available") {
			t.Errorf("SendAgentMessage() = %v, %v; want false and the error \"agent manager not available\"", sent, err)
		}
	})

	t.Run("manager present", func(t *testing.T) {
		resolver, _ := setupTestResolver(t)
		resolver.AgentMgr = agent.NewManager("", nil)

		enabled, err := (&queryResolver{resolver}).AgentEnabled(ctx)
		if err != nil || !enabled {
			t.Errorf("AgentEnabled() = %v, %v; want true, nil", enabled, err)
		}
	})
}

func TestAgentActionRegistry(t *testing.T) {
	expectedActions := []string{"commit", "review", "integrate"}
	for _, id := range expectedActions {
		action := findAgentAction(id)
		if action == nil {
			t.Errorf("missing expected action: %s", id)
			continue
		}
		if action.Label == "" {
			t.Errorf("empty label for action: %s", id)
		}
		if action.Description == "" {
			t.Errorf("empty description for action: %s", id)
		}
		if action.PromptFunc == nil {
			t.Errorf("nil PromptFunc for action: %s", id)
		} else if action.PromptFunc(actionContext{WorktreeID: "test-wt"}) == "" {
			t.Errorf("empty prompt for action: %s", id)
		}
	}

	if findAgentAction("nonexistent") != nil {
		t.Error("expected nil for unknown action")
	}
}

func TestRemoveBlockingWithETag(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	// Create target item
	target := &item.Item{
		ID:     "target-rm-etag",
		Title:  "Target",
		Status: "todo",
		Type:   "task",
	}
	core.Create(target)

	t.Run("removeBlocking with correct etag succeeds", func(t *testing.T) {
		blocker := &item.Item{
			ID:       "blocker-rm-etag-1",
			Title:    "Blocker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-rm-etag"},
		}
		core.Create(blocker)

		currentETag := blocker.ETag()

		got, err := resolver.Mutation().RemoveBlocking(ctx, "blocker-rm-etag-1", "target-rm-etag", &currentETag)
		if err != nil {
			t.Fatalf("RemoveBlocking() with correct etag failed: %v", err)
		}
		if len(got.Blocking) != 0 {
			t.Errorf("RemoveBlocking().Blocking = %v, want []", got.Blocking)
		}
	})

	t.Run("removeBlocking with wrong etag fails", func(t *testing.T) {
		blocker := &item.Item{
			ID:       "blocker-rm-etag-2",
			Title:    "Blocker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-rm-etag"},
		}
		core.Create(blocker)

		wrongETag := "wrongetag123"

		_, err := resolver.Mutation().RemoveBlocking(ctx, "blocker-rm-etag-2", "target-rm-etag", &wrongETag)
		if err == nil {
			t.Error("RemoveBlocking() with wrong etag should fail")
		}

		var mismatchErr *itemcore.ETagMismatchError
		if !errors.As(err, &mismatchErr) {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestListFiles(t *testing.T) {
	ctx := context.Background()

	// Create a temporary git repo with some files
	tmpDir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Create files
	for _, f := range []string{
		"README.md",
		"main.go",
		"pkg/bean/bean.go",
		"pkg/bean/sort.go",
		"pkg/beancore/core.go",
		"internal/graph/resolver.go",
		"internal/graph/schema.go",
		"internal/agent/manager.go",
	} {
		dir := filepath.Dir(filepath.Join(tmpDir, f))
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(tmpDir, f), []byte("package test"), 0644)
	}
	run("add", "-A")
	run("commit", "-m", "init")

	resolver := &Resolver{
		CoreResolver: &itemgraph.CoreResolver{},
		ProjectRoot:  tmpDir,
	}
	qr := resolver.Query().(*queryResolver)

	t.Run("empty query returns all files", func(t *testing.T) {
		results, err := qr.ListFiles(ctx, nil, "", nil)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) != 8 {
			t.Errorf("expected 8 results, got %d", len(results))
		}
	})

	t.Run("substring matches across full path", func(t *testing.T) {
		results, err := qr.ListFiles(ctx, nil, "resolver", nil)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != "internal/graph/resolver.go" {
			t.Errorf("expected internal/graph/resolver.go, got %s", results[0].Path)
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		results, err := qr.ListFiles(ctx, nil, "README", nil)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != "README.md" {
			t.Errorf("expected README.md, got %s", results[0].Path)
		}
	})

	t.Run("multiple terms all must match", func(t *testing.T) {
		results, err := qr.ListFiles(ctx, nil, "bean sort", nil)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Path != "pkg/bean/sort.go" {
			t.Errorf("expected pkg/bean/sort.go, got %s", results[0].Path)
		}
	})

	t.Run("limit caps results", func(t *testing.T) {
		limit := 2
		results, err := qr.ListFiles(ctx, nil, "", &limit)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) > 2 {
			t.Errorf("expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("non-matching query returns empty", func(t *testing.T) {
		results, err := qr.ListFiles(ctx, nil, "nonexistent", nil)
		if err != nil {
			t.Fatalf("ListFiles() error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestQueryItemExtra(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	b := &item.Item{
		ID:     "extra-test",
		Title:  "Bean With Extra",
		Status: "todo",
		Extra:  map[string]any{"release": "0-4-1"},
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}

	qr := resolver.Query()
	got, err := qr.Item(ctx, "extra-test")
	if err != nil {
		t.Fatalf("Item() error = %v", err)
	}
	if got == nil {
		t.Fatal("Item() returned nil")
	}
	if got.Extra["release"] != "0-4-1" {
		t.Errorf("Bean().Extra[release] = %v, want %q", got.Extra["release"], "0-4-1")
	}
}

func TestMutationUpdateItemPreservesExtra(t *testing.T) {
	resolver, core := setupTestResolver(t)
	ctx := context.Background()

	b := &item.Item{
		ID:       "extra-update-test",
		Title:    "Original Title",
		Status:   "todo",
		Priority: "normal",
		Extra:    map[string]any{"release": "0-4-1"},
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}

	mr := resolver.Mutation()
	newPriority := "high"
	input := model.UpdateItemInput{
		Priority: &newPriority,
	}
	got, err := mr.UpdateItem(ctx, "extra-update-test", input)
	if err != nil {
		t.Fatalf("UpdateItem() error = %v", err)
	}
	if got.Priority != "high" {
		t.Errorf("UpdateItem().Priority = %q, want %q", got.Priority, "high")
	}
	if got.Extra["release"] != "0-4-1" {
		t.Errorf("UpdateItem().Extra[release] = %v, want %q (extra keys must survive a mutation of unrelated fields)", got.Extra["release"], "0-4-1")
	}

	// Verify the extra key is preserved on disk
	itemsDir := filepath.Join(core.Root(), "")
	filename := item.BuildFilename("extra-update-test", got.Slug)
	filePath := filepath.Join(itemsDir, filename)
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read bean file from disk: %v", err)
	}
	if !strings.Contains(string(fileContent), "release: 0-4-1") {
		t.Errorf("bean file does not contain extra key on disk.\nFile content:\n%s", string(fileContent))
	}
}
