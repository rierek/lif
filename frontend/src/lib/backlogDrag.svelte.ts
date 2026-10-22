/**
 * Shared drag state for the backlog view.
 *
 * Since ItemRow is recursive, we need a single source of truth for
 * which item is being dragged and where the drop target is. This module
 * provides that shared reactive state.
 *
 * Card hover uses three zones:
 *   - Top 25%:    reorder above
 *   - Middle 50%: reparent onto this item
 *   - Bottom 25%: reorder below
 */

import type { Item } from '#lib/items.svelte.js';
import { itemsStore } from '#lib/items.svelte.js';
import { applyDrop, applyReparent } from '#lib/dragOrder.js';

export type DropMode = 'reorder' | 'reparent';

class BacklogDragState {
  draggedItemId = $state<string | null>(null);
  /** The parent ID of the sibling group being hovered (null = top-level) */
  dropTargetParent = $state<string | null | undefined>(undefined);
  dropIndex = $state<number | null>(null);
  /** The item ID being hovered for reparenting */
  reparentTargetId = $state<string | null>(null);
  dropMode = $state<DropMode>('reorder');
  /** The target status when dropping into a different section */
  targetStatus = $state<string | null>(null);

  get isDragging() {
    return this.draggedItemId !== null;
  }

  startDrag(e: DragEvent, item: Item) {
    this.draggedItemId = item.id;
    e.dataTransfer!.effectAllowed = 'move';
    e.dataTransfer!.setData('text/plain', item.id);
  }

  endDrag() {
    this.draggedItemId = null;
    this.dropTargetParent = undefined;
    this.dropIndex = null;
    this.reparentTargetId = null;
    this.dropMode = 'reorder';
    this.targetStatus = null;
  }

  hoverCard(e: DragEvent, parentId: string | null, index: number, itemId: string, status?: string) {
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer!.dropEffect = 'move';

    // Don't allow dropping on yourself
    if (itemId === this.draggedItemId) return;

    if (status) this.targetStatus = status;

    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const relativeY = (e.clientY - rect.top) / rect.height;

    if (relativeY < 0.25) {
      // Top zone — reorder above
      this.dropMode = 'reorder';
      this.dropTargetParent = parentId;
      this.dropIndex = index;
      this.reparentTargetId = null;
    } else if (relativeY > 0.75) {
      // Bottom zone — reorder below
      this.dropMode = 'reorder';
      this.dropTargetParent = parentId;
      this.dropIndex = index + 1;
      this.reparentTargetId = null;
    } else {
      // Middle zone — reparent onto this item
      this.dropMode = 'reparent';
      this.reparentTargetId = itemId;
      this.dropTargetParent = undefined;
      this.dropIndex = null;
    }
  }

  hoverList(e: DragEvent, parentId: string | null, itemCount: number, status?: string) {
    e.preventDefault();
    e.dataTransfer!.dropEffect = 'move';
    this.dropMode = 'reorder';
    this.reparentTargetId = null;
    this.dropTargetParent = parentId;
    this.targetStatus = status ?? null;
    if (this.dropIndex === null || this.dropTargetParent !== parentId) {
      this.dropIndex = itemCount;
    }
  }

  leaveList(e: DragEvent, listEl: HTMLElement, parentId: string | null) {
    if (!listEl.contains(e.relatedTarget as Node)) {
      if (this.dropTargetParent === parentId) {
        this.dropTargetParent = undefined;
        this.dropIndex = null;
      }
      if (this.reparentTargetId) {
        this.reparentTargetId = null;
      }
    }
  }

  drop(e: DragEvent, parentId: string | null, items: Item[]) {
    e.preventDefault();
    e.stopPropagation();

    const mode = this.dropMode;
    const targetIdx = this.dropIndex;
    const itemId = this.draggedItemId;
    const reparentTarget = this.reparentTargetId;
    const newStatus = this.targetStatus;

    this.dropTargetParent = undefined;
    this.dropIndex = null;
    this.draggedItemId = null;
    this.reparentTargetId = null;
    this.dropMode = 'reorder';
    this.targetStatus = null;

    if (!itemId) return;

    if (mode === 'reparent' && reparentTarget) {
      const targetChildren = itemsStore.children(reparentTarget);
      applyReparent(itemId, reparentTarget, targetChildren);
    } else {
      applyDrop(items, itemId, targetIdx ?? items.length, {
        newParentId: parentId,
        newStatus: newStatus ?? undefined
      });
    }
  }

  /** Check if a drop indicator should show at this position */
  showIndicator(parentId: string | null, index: number, itemId: string, status?: string): boolean {
    return (
      this.dropMode === 'reorder' &&
      this.dropTargetParent === parentId &&
      (!status || this.targetStatus === status) &&
      this.draggedItemId !== null &&
      this.draggedItemId !== itemId &&
      this.dropIndex === index
    );
  }

  showEndIndicator(parentId: string | null, count: number, status?: string): boolean {
    return (
      this.dropMode === 'reorder' &&
      this.dropTargetParent === parentId &&
      (!status || this.targetStatus === status) &&
      this.draggedItemId !== null &&
      this.dropIndex === count
    );
  }

  /** Check if this item is the reparent target */
  isReparentTarget(itemId: string): boolean {
    return this.dropMode === 'reparent' && this.reparentTargetId === itemId;
  }
}

export const backlogDrag = new BacklogDragState();
