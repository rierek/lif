package itemgraph

import (
	"context"
	"path/filepath"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// ItemIsDirty returns whether an item has unsaved runtime changes.
func (r *CoreResolver) ItemIsDirty(ctx context.Context, obj *item.Item) (bool, error) {
	return r.Core.IsDirty(obj.ID), nil
}

// ItemWorktreeID returns the worktree ID for an item, or nil if not linked.
func (r *CoreResolver) ItemWorktreeID(ctx context.Context, obj *item.Item) (*string, error) {
	wtPath := r.Core.WorktreeForItem(obj.ID)
	if wtPath == "" {
		return nil, nil
	}
	// Extract worktree ID from the path (last path component)
	id := filepath.Base(wtPath)
	return &id, nil
}

// ItemParentID returns the parent ID as a pointer, or nil if no parent.
func (r *CoreResolver) ItemParentID(ctx context.Context, obj *item.Item) (*string, error) {
	if obj.Parent == "" {
		return nil, nil
	}
	return &obj.Parent, nil
}

// ItemBlockingIds returns the blocking IDs slice.
func (r *CoreResolver) ItemBlockingIds(ctx context.Context, obj *item.Item) ([]string, error) {
	return obj.Blocking, nil
}

// ItemBlockedByIds returns the blocked-by IDs slice.
func (r *CoreResolver) ItemBlockedByIds(ctx context.Context, obj *item.Item) ([]string, error) {
	return obj.BlockedBy, nil
}

// ItemBlockedBy resolves the full list of items blocking this one.
// Combines both directions: the item's own blocked_by field AND incoming
// blocking links (other items that list this item in their blocking field).
func (r *CoreResolver) ItemBlockedBy(ctx context.Context, obj *item.Item, filter *model.ItemFilter) ([]*item.Item, error) {
	seen := make(map[string]bool)
	var result []*item.Item

	// 1. Resolve items from the direct blocked_by field
	for _, blockerID := range obj.BlockedBy {
		if !seen[blockerID] {
			seen[blockerID] = true
			if blocker, err := r.Core.Get(blockerID); err == nil {
				result = append(result, blocker)
			}
		}
	}

	// 2. Resolve items from incoming blocking links (other items blocking this one)
	incoming := r.Core.FindIncomingLinks(obj.ID)
	for _, link := range incoming {
		if link.LinkType == "blocking" && !seen[link.FromItem.ID] {
			seen[link.FromItem.ID] = true
			result = append(result, link.FromItem)
		}
	}

	filtered := ApplyFilter(result, filter, r.Core)
	cfg := r.Core.Config()
	item.SortByStatusPriorityAndType(filtered, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	return filtered, nil
}

// ItemBlocking resolves the items this item is blocking.
func (r *CoreResolver) ItemBlocking(ctx context.Context, obj *item.Item, filter *model.ItemFilter) ([]*item.Item, error) {
	var result []*item.Item
	for _, targetID := range obj.Blocking {
		// Filter out broken links
		if target, err := r.Core.Get(targetID); err == nil {
			result = append(result, target)
		}
	}
	filtered := ApplyFilter(result, filter, r.Core)
	cfg := r.Core.Config()
	item.SortByStatusPriorityAndType(filtered, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	return filtered, nil
}

// ItemParent resolves the parent item.
func (r *CoreResolver) ItemParent(ctx context.Context, obj *item.Item) (*item.Item, error) {
	if obj.Parent == "" {
		return nil, nil
	}
	// Filter out broken links
	parent, err := r.Core.Get(obj.Parent)
	if err == itemcore.ErrNotFound {
		return nil, nil
	}
	return parent, err
}

// ItemChildren resolves the child items.
func (r *CoreResolver) ItemChildren(ctx context.Context, obj *item.Item, filter *model.ItemFilter) ([]*item.Item, error) {
	incoming := r.Core.FindIncomingLinks(obj.ID)
	var result []*item.Item
	for _, link := range incoming {
		if link.LinkType == "parent" {
			result = append(result, link.FromItem)
		}
	}
	filtered := ApplyFilter(result, filter, r.Core)
	cfg := r.Core.Config()
	item.SortByStatusPriorityAndType(filtered, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())
	return filtered, nil
}

// ItemImplicitStatus returns the implicit status inherited from ancestors.
func (r *CoreResolver) ItemImplicitStatus(ctx context.Context, obj *item.Item) (*string, error) {
	status, _ := r.Core.ImplicitStatus(obj.ID)
	if status == "" {
		return nil, nil
	}
	return &status, nil
}

// ItemImplicitStatusFrom returns the ancestor ID from which implicit status is inherited.
func (r *CoreResolver) ItemImplicitStatusFrom(ctx context.Context, obj *item.Item) (*string, error) {
	_, fromID := r.Core.ImplicitStatus(obj.ID)
	if fromID == "" {
		return nil, nil
	}
	return &fromID, nil
}
