package itemgraph

import (
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// ApplyFilter applies BeanFilter to a slice of items and returns filtered results.
// This is used by both the top-level items query and relationship field resolvers.
func ApplyFilter(items []*item.Item, filter *model.ItemFilter, core *itemcore.Core) []*item.Item {
	if filter == nil {
		return items
	}

	result := items

	// Status filters
	if len(filter.Status) > 0 {
		result = filterByField(result, filter.Status, func(b *item.Item) string { return b.Status })
	}
	if len(filter.ExcludeStatus) > 0 {
		result = excludeByField(result, filter.ExcludeStatus, func(b *item.Item) string { return b.Status })
	}

	// Type filters
	if len(filter.Type) > 0 {
		result = filterByField(result, filter.Type, func(b *item.Item) string { return b.Type })
	}
	if len(filter.ExcludeType) > 0 {
		result = excludeByField(result, filter.ExcludeType, func(b *item.Item) string { return b.Type })
	}

	// Priority filters (empty priority treated as "normal")
	if len(filter.Priority) > 0 {
		result = filterByPriority(result, filter.Priority)
	}
	if len(filter.ExcludePriority) > 0 {
		result = excludeByPriority(result, filter.ExcludePriority)
	}

	// Tag filters
	if len(filter.Tags) > 0 {
		result = filterByTags(result, filter.Tags)
	}
	if len(filter.ExcludeTags) > 0 {
		result = excludeByTags(result, filter.ExcludeTags)
	}

	// Parent filters
	if filter.HasParent != nil && *filter.HasParent {
		result = filterByHasParent(result)
	}
	if filter.NoParent != nil && *filter.NoParent {
		result = filterByNoParent(result)
	}
	if filter.ParentID != nil && *filter.ParentID != "" {
		result = filterByParentID(result, *filter.ParentID)
	}

	// Blocking filters
	if filter.HasBlocking != nil && *filter.HasBlocking {
		result = filterByHasBlocking(result)
	}
	if filter.BlockingID != nil && *filter.BlockingID != "" {
		result = filterByBlockingID(result, *filter.BlockingID)
	}
	if filter.NoBlocking != nil && *filter.NoBlocking {
		result = filterByNoBlocking(result)
	}
	if filter.IsBlocked != nil {
		if *filter.IsBlocked {
			result = filterByIsBlocked(result, core)
		} else {
			result = filterByNotBlocked(result, core)
		}
	}
	if filter.IsExplicitlyBlocked != nil && *filter.IsExplicitlyBlocked {
		result = filterByIsExplicitlyBlocked(result, core)
	}
	if filter.IsImplicitlyBlocked != nil && *filter.IsImplicitlyBlocked {
		result = filterByIsImplicitlyBlocked(result, core)
	}

	// Blocked-by filters (for direct blocked_by field)
	if filter.HasBlockedBy != nil && *filter.HasBlockedBy {
		result = filterByHasBlockedBy(result)
	}
	if filter.BlockedByID != nil && *filter.BlockedByID != "" {
		result = filterByBlockedByID(result, *filter.BlockedByID)
	}
	if filter.NoBlockedBy != nil && *filter.NoBlockedBy {
		result = filterByNoBlockedBy(result)
	}

	// Implicit status filter
	if filter.ExcludeImplicitTerminal != nil && *filter.ExcludeImplicitTerminal {
		result = filterByNoImplicitTerminal(result, core)
	}

	return result
}

// filterByField filters items to include only those where getter returns a value in values (OR logic).
func filterByField(items []*item.Item, values []string, getter func(*item.Item) string) []*item.Item {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*item.Item
	for _, b := range items {
		if valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByField filters items to exclude those where getter returns a value in values.
func excludeByField(items []*item.Item, values []string, getter func(*item.Item) string) []*item.Item {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*item.Item
	for _, b := range items {
		if !valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// filterByPriority filters items to include only those with matching priorities (OR logic).
// Empty priority in the item is treated as "normal" for matching purposes.
func filterByPriority(items []*item.Item, priorities []string) []*item.Item {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*item.Item
	for _, b := range items {
		priority := b.Priority
		if priority == "" {
			priority = "normal"
		}
		if prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByPriority filters items to exclude those with matching priorities.
// Empty priority in the item is treated as "normal" for matching purposes.
func excludeByPriority(items []*item.Item, priorities []string) []*item.Item {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*item.Item
	for _, b := range items {
		priority := b.Priority
		if priority == "" {
			priority = "normal"
		}
		if !prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// filterByTags filters items to include only those with any of the given tags (OR logic).
func filterByTags(items []*item.Item, tags []string) []*item.Item {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*item.Item
	for _, b := range items {
		for _, t := range b.Tags {
			if tagSet[t] {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// excludeByTags filters items to exclude those with any of the given tags.
func excludeByTags(items []*item.Item, tags []string) []*item.Item {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*item.Item
outer:
	for _, b := range items {
		for _, t := range b.Tags {
			if tagSet[t] {
				continue outer
			}
		}
		result = append(result, b)
	}
	return result
}

// filterByHasParent filters items to include only those with a parent.
func filterByHasParent(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if b.Parent != "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoParent filters items to include only those without a parent.
func filterByNoParent(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if b.Parent == "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByParentID filters items with specific parent ID.
func filterByParentID(items []*item.Item, parentID string) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if b.Parent == parentID {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlocking filters items that are blocking other items.
func filterByHasBlocking(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if len(b.Blocking) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockingID filters items that are blocking a specific item ID.
func filterByBlockingID(items []*item.Item, targetID string) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		for _, blocked := range b.Blocking {
			if blocked == targetID {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// filterByNoBlocking filters items that aren't blocking other items.
func filterByNoBlocking(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if len(b.Blocking) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByIsBlocked filters items that are blocked by others.
// An item is considered blocked only if it has active (non-completed, non-scrapped) blockers.
func filterByIsBlocked(items []*item.Item, core *itemcore.Core) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByIsExplicitlyBlocked filters items that have direct active blockers.
func filterByIsExplicitlyBlocked(items []*item.Item, core *itemcore.Core) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if core.IsExplicitlyBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByIsImplicitlyBlocked filters items where an ancestor is blocked.
func filterByIsImplicitlyBlocked(items []*item.Item, core *itemcore.Core) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if core.IsImplicitlyBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNotBlocked filters items that are NOT blocked by others.
// An item is considered not blocked if it has no active (non-completed, non-scrapped) blockers.
func filterByNotBlocked(items []*item.Item, core *itemcore.Core) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if !core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlockedBy filters items that have explicit blocked_by entries.
func filterByHasBlockedBy(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if len(b.BlockedBy) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockedByID filters items that are blocked by a specific item ID (via blocked_by field).
func filterByBlockedByID(items []*item.Item, blockerID string) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		for _, blocker := range b.BlockedBy {
			if blocker == blockerID {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// filterByNoBlockedBy filters items that have no explicit blocked_by entries.
func filterByNoBlockedBy(items []*item.Item) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		if len(b.BlockedBy) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoImplicitTerminal excludes items that inherit a terminal status
// (scrapped or completed) from an ancestor.
func filterByNoImplicitTerminal(items []*item.Item, core *itemcore.Core) []*item.Item {
	var result []*item.Item
	for _, b := range items {
		status, _ := core.ImplicitStatus(b.ID)
		if status == "" {
			result = append(result, b)
		}
	}
	return result
}
