package itemgraph

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
	"github.com/xRiErOS/lif/pkg/itemgraph/model"
)

// CreateItem creates a new item from the given input.
func (r *CoreResolver) CreateItem(ctx context.Context, input model.CreateItemInput, opts ...itemcore.UpdateOption) (*item.Item, error) {
	if err := r.validateCreateInput(input); err != nil {
		return nil, err
	}

	b := &item.Item{
		Slug:     item.Slugify(input.Title),
		Title:    input.Title,
		Type:     "task", // default
		Blocking: []string{},
	}

	// Optional fields with defaults documented in schema
	if input.Type != nil {
		b.Type = *input.Type
	}
	b.Status = r.Core.Config().GetDefaultStatus()
	if input.Status != nil {
		b.Status = *input.Status
	}
	if input.Priority != nil {
		b.Priority = *input.Priority
	}
	if input.Body != nil {
		b.Body = *input.Body
	}
	if len(input.Tags) > 0 {
		b.Tags = input.Tags
	}

	// Handle parent (with validation)
	if input.Parent != nil && *input.Parent != "" {
		// Normalise short ID to full ID
		parentID, _ := r.Core.NormalizeID(*input.Parent)
		if err := r.Core.ValidateParent(b, parentID); err != nil {
			return nil, err
		}
		b.Parent = parentID
	}

	// Handle blocking (with validation)
	if len(input.Blocking) > 0 {
		// Normalise short IDs to full IDs
		normalizedBlocking := make([]string, len(input.Blocking))
		for i, id := range input.Blocking {
			var ok bool
			normalizedBlocking[i], ok = r.Core.NormalizeID(id)
			if !ok {
				return nil, fmt.Errorf("target item not found: %s", id)
			}
		}
		b.Blocking = normalizedBlocking
	}

	// Handle blocked_by (with cycle validation)
	if len(input.BlockedBy) > 0 {
		// Normalise short IDs to full IDs
		normalizedBlockedBy := make([]string, len(input.BlockedBy))
		for i, id := range input.BlockedBy {
			var ok bool
			normalizedBlockedBy[i], ok = r.Core.NormalizeID(id)
			if !ok {
				return nil, fmt.Errorf("blocker item not found: %s", id)
			}
		}
		// Check for cycles with blocking relationships
		// (new item being blocked_by X means X→newItem, check if newItem→X exists via blocking)
		for _, blockerID := range normalizedBlockedBy {
			for _, blockingID := range b.Blocking {
				if blockerID == blockingID {
					return nil, fmt.Errorf("would create cycle: new item both blocks and is blocked by %s", blockerID)
				}
			}
		}
		b.BlockedBy = normalizedBlockedBy
	}

	// Handle custom prefix - pre-generate ID if prefix is provided
	if input.Prefix != nil && *input.Prefix != "" {
		idLength := 4 // default
		if cfg := r.Core.Config(); cfg != nil && cfg.Items.IDLength > 0 {
			idLength = cfg.Items.IDLength
		}
		id, err := item.NewID(*input.Prefix, idLength)
		if err != nil {
			return nil, fmt.Errorf("generating item ID: %w", err)
		}
		b.ID = id
	}

	if err := r.Core.Create(b, opts...); err != nil {
		return nil, err
	}

	return b, nil
}

// UpdateItem updates an existing item.
func (r *CoreResolver) UpdateItem(ctx context.Context, id string, input model.UpdateItemInput, opts ...itemcore.UpdateOption) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}
	oldType := b.Type

	if err := r.validateUpdateInput(input); err != nil {
		return nil, err
	}

	// Validate body and bodyMod are mutually exclusive
	if input.Body != nil && input.BodyMod != nil {
		return nil, fmt.Errorf("cannot specify both body and bodyMod")
	}

	// Validate tags and addTags/removeTags are mutually exclusive
	if input.Tags != nil && (input.AddTags != nil || input.RemoveTags != nil) {
		return nil, fmt.Errorf("cannot specify both tags and addTags/removeTags")
	}

	// Update fields if provided
	// A slug still equal to the old title's slug was derived, not hand-set
	// (lif rename --slug), so it follows the title. Capture before overwrite.
	reslug := false
	if input.Title != nil && *input.Title != b.Title {
		reslug = b.Slug == item.Slugify(b.Title) && r.Core.WorktreeForItem(b.ID) == ""
		b.Title = *input.Title
	}
	if input.Status != nil {
		b.Status = *input.Status
	}
	if input.Type != nil {
		b.Type = *input.Type
	}
	if input.Priority != nil {
		b.Priority = *input.Priority
	}
	if input.Order != nil {
		b.Order = *input.Order
	}
	if input.Body != nil {
		b.Body = *input.Body
	} else if input.BodyMod != nil {
		// Apply body modifications
		workingBody := b.Body

		// Apply replacements sequentially
		if input.BodyMod.Replace != nil {
			for i, replaceOp := range input.BodyMod.Replace {
				newBody, err := item.ReplaceOnce(workingBody, replaceOp.Old, replaceOp.New)
				if err != nil {
					return nil, fmt.Errorf("replacement %d failed: %w", i, err)
				}
				workingBody = newBody
			}
		}

		// Apply append if provided
		if input.BodyMod.Append != nil && *input.BodyMod.Append != "" {
			workingBody = item.AppendWithSeparator(workingBody, *input.BodyMod.Append)
		}

		b.Body = workingBody
	}
	// The size limit applies to the body that is about to be written, not to
	// the input: bodyMod appends to what is already stored, so only the grown
	// result shows the true size. An item whose body this update does not touch
	// stays editable even if it was written before the limit existed.
	if input.Body != nil || input.BodyMod != nil {
		if err := ValidateBody(b.Body); err != nil {
			return nil, err
		}
	}
	// Handle tags
	if input.Tags != nil {
		b.Tags = input.Tags
	} else if input.AddTags != nil || input.RemoveTags != nil {
		// Build a set of current tags
		tagSet := make(map[string]bool)
		for _, tag := range b.Tags {
			tagSet[tag] = true
		}

		// Add new tags
		if input.AddTags != nil {
			for _, tag := range input.AddTags {
				tagSet[tag] = true
			}
		}

		// Remove tags
		if input.RemoveTags != nil {
			for _, tag := range input.RemoveTags {
				delete(tagSet, tag)
			}
		}

		// Convert back to slice. Map iteration order is random, so sort:
		// without this every tag write can reshuffle the tags: list in the
		// file, which produces diff noise and makes any assertion on the
		// written order flaky.
		newTags := make([]string, 0, len(tagSet))
		for tag := range tagSet {
			newTags = append(newTags, tag)
		}
		sort.Strings(newTags)
		b.Tags = newTags
	}

	// Handle parent relationship
	if input.Parent != nil {
		if err := r.ValidateAndSetParent(b, *input.Parent); err != nil {
			return nil, err
		}
	}

	// A type change re-checks the existing hierarchy: the parent only gets
	// validated above when it changes, and children never.
	if b.Type != oldType {
		if err := r.ValidateTypeChange(b); err != nil {
			return nil, err
		}
	}

	// Handle blocking relationships
	if input.AddBlocking != nil {
		if err := r.ValidateAndAddBlocking(b, input.AddBlocking); err != nil {
			return nil, err
		}
	}
	if input.RemoveBlocking != nil {
		r.RemoveBlockingRelationships(b, input.RemoveBlocking)
	}

	// Handle blocked-by relationships
	if input.AddBlockedBy != nil {
		if err := r.ValidateAndAddBlockedBy(b, input.AddBlockedBy); err != nil {
			return nil, err
		}
	}
	if input.RemoveBlockedBy != nil {
		r.RemoveBlockedByRelationships(b, input.RemoveBlockedBy)
	}

	// ETag validation now happens inside Update() under write lock.
	// If the item is linked to a worktree, Core auto-routes the write there.
	if err := r.Core.Update(b, input.IfMatch, opts...); err != nil {
		return nil, err
	}

	if reslug {
		// Content is written under the old name first; the rename then moves
		// that one file, so exactly one file with new content remains.
		plan, err := r.Core.PlanRenameSlug(b.ID, nil, true)
		if err != nil {
			return nil, err
		}
		if err := r.Core.ApplyRename(plan); err != nil {
			return nil, err
		}
		b.Path = plan.Changes[0].NewPath
		_, b.Slug = item.ParseFilename(filepath.Base(b.Path))
	}

	return b, nil
}

// DeleteItem removes an item and its incoming links.
func (r *CoreResolver) DeleteItem(ctx context.Context, id string) (bool, error) {
	// Verify item exists
	if _, ok := r.Core.NormalizeID(id); !ok {
		return false, itemcore.ErrNotFound
	}

	// Remove incoming links first
	if _, err := r.Core.RemoveLinksTo(id); err != nil {
		return false, err
	}

	// Delete the item
	if err := r.Core.Delete(id); err != nil {
		return false, err
	}

	return true, nil
}

// SetParent sets or clears the parent of an item.
func (r *CoreResolver) SetParent(ctx context.Context, id string, parentID *string, ifMatch *string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}

	newParent := ""
	if parentID != nil {
		// Normalise short ID to full ID
		newParent, _ = r.Core.NormalizeID(*parentID)
	}

	// Validate parent type hierarchy
	if newParent != "" {
		if err := r.Core.ValidateParent(b, newParent); err != nil {
			return nil, err
		}
		// Check for cycles
		if cycle := r.Core.DetectCycle(b.ID, "parent", newParent); cycle != nil {
			return nil, fmt.Errorf("would create cycle: %v", cycle)
		}
	}

	b.Parent = newParent
	// ETag validation now happens inside Update() under write lock
	if err := r.Core.Update(b, ifMatch); err != nil {
		return nil, err
	}
	return b, nil
}

// AddBlocking adds a blocking relationship.
func (r *CoreResolver) AddBlocking(ctx context.Context, id string, targetID string, ifMatch *string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}

	// Normalise short ID to full ID
	normalizedTargetID, ok := r.Core.NormalizeID(targetID)

	if normalizedTargetID == b.ID {
		return nil, fmt.Errorf("item cannot block itself")
	}

	// Check target exists
	if !ok {
		return nil, fmt.Errorf("target item not found: %s", targetID)
	}

	// Check for cycles in both directions
	if cycle := r.Core.DetectCycle(b.ID, "blocking", normalizedTargetID); cycle != nil {
		return nil, fmt.Errorf("would create cycle: %v", cycle)
	}
	if cycle := r.Core.DetectCycle(normalizedTargetID, "blocked_by", b.ID); cycle != nil {
		return nil, fmt.Errorf("would create cycle: %v", cycle)
	}

	b.AddBlocking(normalizedTargetID)
	if err := r.Core.Update(b, ifMatch); err != nil {
		return nil, err
	}
	return b, nil
}

// RemoveBlocking removes a blocking relationship.
func (r *CoreResolver) RemoveBlocking(ctx context.Context, id string, targetID string, ifMatch *string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}

	normalizedTargetID, _ := r.Core.NormalizeID(targetID)

	b.RemoveBlocking(normalizedTargetID)
	if err := r.Core.Update(b, ifMatch); err != nil {
		return nil, err
	}
	return b, nil
}

// AddBlockedBy adds a blocked-by relationship.
func (r *CoreResolver) AddBlockedBy(ctx context.Context, id string, targetID string, ifMatch *string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}

	normalizedTargetID, ok := r.Core.NormalizeID(targetID)

	if normalizedTargetID == b.ID {
		return nil, fmt.Errorf("item cannot be blocked by itself")
	}

	// Check target exists
	if !ok {
		return nil, fmt.Errorf("blocker item not found: %s", targetID)
	}

	// Check for cycles in both directions
	if cycle := r.Core.DetectCycle(normalizedTargetID, "blocking", b.ID); cycle != nil {
		return nil, fmt.Errorf("would create cycle: %v", cycle)
	}
	if cycle := r.Core.DetectCycle(b.ID, "blocked_by", normalizedTargetID); cycle != nil {
		return nil, fmt.Errorf("would create cycle: %v", cycle)
	}

	b.AddBlockedBy(normalizedTargetID)
	if err := r.Core.Update(b, ifMatch); err != nil {
		return nil, err
	}
	return b, nil
}

// RemoveBlockedBy removes a blocked-by relationship.
func (r *CoreResolver) RemoveBlockedBy(ctx context.Context, id string, targetID string, ifMatch *string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err != nil {
		return nil, err
	}

	normalizedTargetID, _ := r.Core.NormalizeID(targetID)

	b.RemoveBlockedBy(normalizedTargetID)
	if err := r.Core.Update(b, ifMatch); err != nil {
		return nil, err
	}
	return b, nil
}

// ArchiveItem archives an item.
func (r *CoreResolver) ArchiveItem(ctx context.Context, id string) (bool, error) {
	if err := r.Core.Archive(id); err != nil {
		return false, err
	}
	return true, nil
}
