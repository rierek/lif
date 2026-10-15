package itemgraph

import (
	"fmt"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// CoreResolver implements the core item GraphQL operations (CRUD, relationships,
// filtering). It depends only on itemcore.Core and has no UI-specific dependencies
// (agents, worktrees, terminals, etc.).
//
// CLI commands use CoreResolver directly. The serve command's full GraphQL resolver
// embeds CoreResolver and adds UI-specific operations on top.
type CoreResolver struct {
	Core *itemcore.Core
}

// validateETag checks if the provided ifMatch etag matches the item's current etag.
// Returns an error if validation fails or if require_if_match is enabled and no etag provided.
func (r *CoreResolver) validateETag(b *item.Item, ifMatch *string) error {
	cfg := r.Core.Config()
	requireIfMatch := cfg != nil && cfg.Items.RequireIfMatch

	// If require_if_match is enabled and no etag provided, reject
	if requireIfMatch && (ifMatch == nil || *ifMatch == "") {
		return &itemcore.ETagRequiredError{}
	}

	// If ifMatch provided, validate it
	if ifMatch != nil && *ifMatch != "" {
		currentETag := b.ETag()
		if currentETag != *ifMatch {
			return &itemcore.ETagMismatchError{Provided: *ifMatch, Current: currentETag}
		}
	}

	return nil
}

// cycleError words the refusal of a link that would close cycle. what names
// the refused change and ends in "would create cycle". A cycle over blocking
// and parent links carries a note, because its path shows no cycle to a reader
// who does not know how the hierarchy takes part.
func cycleError(what string, cycle *itemcore.Cycle) error {
	if note := cycle.Note(); note != "" {
		return fmt.Errorf("%s: %v (%s)", what, cycle.Path, note)
	}
	return fmt.Errorf("%s: %v", what, cycle.Path)
}

// ValidateAndSetParent validates and sets the parent relationship.
func (r *CoreResolver) ValidateAndSetParent(b *item.Item, parentID string) error {
	if parentID == "" {
		b.Parent = ""
		return nil
	}

	// Normalise short ID to full ID
	normalizedParent, _ := r.Core.NormalizeID(parentID)

	// Validate parent type hierarchy
	if err := r.Core.ValidateParent(b, normalizedParent); err != nil {
		return err
	}

	// Check for cycles
	if cycle := r.Core.DetectCycle(b, "parent", normalizedParent); cycle != nil {
		return cycleError("setting parent would create cycle", cycle)
	}

	b.Parent = normalizedParent
	return nil
}

// ValidateTypeChange checks b, already carrying its new type, against the
// type hierarchy: its parent must allow the new type and every child must be
// allowed under it. The error names the violating item.
func (r *CoreResolver) ValidateTypeChange(b *item.Item) error {
	if err := r.Core.ValidateParent(b, b.Parent); err != nil {
		return fmt.Errorf("%s: %w", b.ID, err)
	}
	for _, link := range r.Core.FindIncomingLinks(b.ID) {
		child := link.FromItem
		if link.LinkType != "parent" || itemcore.IsValidParentRank(r.Core.Config(), child.Type, b.Type) {
			continue
		}
		return fmt.Errorf("%s: %s items cannot be a child of %s items (child %s)", b.ID, child.Type, b.Type, child.ID)
	}
	return nil
}

// ValidateAndAddBlocking validates and adds blocking relationships.
func (r *CoreResolver) ValidateAndAddBlocking(b *item.Item, targetIDs []string) error {
	for _, targetID := range targetIDs {
		// Normalise short ID to full ID
		normalizedTargetID, ok := r.Core.NormalizeID(targetID)

		// Validate: cannot block itself
		if normalizedTargetID == b.ID {
			return fmt.Errorf("item cannot block itself")
		}

		// Validate: target must exist
		if !ok {
			return fmt.Errorf("blocking target item not found: %s", targetID)
		}

		// Check for cycles
		if cycle := r.Core.DetectCycle(b, "blocking", normalizedTargetID); cycle != nil {
			return cycleError("adding blocking relationship would create cycle", cycle)
		}

		b.AddBlocking(normalizedTargetID)
	}
	return nil
}

// RemoveBlockingRelationships removes blocking relationships.
func (r *CoreResolver) RemoveBlockingRelationships(b *item.Item, targetIDs []string) {
	for _, targetID := range targetIDs {
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)
		b.RemoveBlocking(normalizedTargetID)
	}
}

// ValidateAndAddBlockedBy validates and adds blocked-by relationships.
func (r *CoreResolver) ValidateAndAddBlockedBy(b *item.Item, targetIDs []string) error {
	for _, targetID := range targetIDs {
		// Normalise short ID to full ID
		normalizedTargetID, ok := r.Core.NormalizeID(targetID)

		// Validate: cannot be blocked by itself
		if normalizedTargetID == b.ID {
			return fmt.Errorf("item cannot be blocked by itself")
		}

		// Validate: blocker must exist
		if !ok {
			return fmt.Errorf("blocker item not found: %s", targetID)
		}

		// Check for cycles
		if cycle := r.Core.DetectCycle(b, "blocked_by", normalizedTargetID); cycle != nil {
			return cycleError("adding blocked-by relationship would create cycle", cycle)
		}

		b.AddBlockedBy(normalizedTargetID)
	}
	return nil
}

// RemoveBlockedByRelationships removes blocked-by relationships.
func (r *CoreResolver) RemoveBlockedByRelationships(b *item.Item, targetIDs []string) {
	for _, targetID := range targetIDs {
		normalizedTargetID, _ := r.Core.NormalizeID(targetID)
		b.RemoveBlockedBy(normalizedTargetID)
	}
}
