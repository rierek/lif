/**
 * Shared drag-and-drop ordering utilities.
 *
 * Used by both BoardView (kanban columns) and backlog (flat/hierarchical list)
 * to compute fractional-index order keys when items are reordered via drag.
 */

import type { Item } from '#lib/items.svelte.js';
import { itemsStore } from '#lib/items.svelte.js';
import { orderBetween } from '#lib/fractional.js';
import { client } from '#lib/graphqlClient.js';
import { UpdateItemOrderDocument } from './graphql/generated';

/**
 * Ensure all items in the list have order keys.
 * Assigns evenly-spaced keys to any items missing them,
 * preserving the relative positions of items that already have keys.
 * Returns the list with orders filled in. Updates the store optimistically.
 */
export function ensureOrdered(items: Item[]): Item[] {
  const needsOrder = items.filter((b) => !b.order);
  if (needsOrder.length === 0) return items;

  const result = [...items];
  let key = '';
  for (let i = 0; i < result.length; i++) {
    const nextKey = i < result.length - 1 && result[i + 1].order ? result[i + 1].order : '';
    if (!result[i].order) {
      const newOrder = orderBetween(key, nextKey);
      result[i] = { ...result[i], order: newOrder };
      itemsStore.optimisticUpdate(result[i].id, { order: newOrder });
      client.mutation(UpdateItemOrderDocument, { id: result[i].id, input: { order: newOrder } }).toPromise();
    }
    key = result[i].order;
  }
  return result;
}

/**
 * Compute the fractional-index order key for an item being dropped
 * at `targetIndex` within `items`. The dragged item (identified by
 * `draggedId`) is filtered out before computing neighbours.
 */
export function computeOrder(items: Item[], targetIndex: number, draggedId: string): string {
  const draggedIndex = items.findIndex((b) => b.id === draggedId);
  const filtered = items.filter((b) => b.id !== draggedId);

  if (filtered.length === 0) {
    return orderBetween('', '');
  }

  // Adjust target index when dragging downward in the same list
  let idx = targetIndex;
  if (draggedIndex >= 0 && targetIndex > draggedIndex) {
    idx--;
  }
  idx = Math.min(idx, filtered.length);

  if (idx === 0) {
    return orderBetween('', filtered[0].order);
  }
  if (idx >= filtered.length) {
    return orderBetween(filtered[filtered.length - 1].order, '');
  }

  return orderBetween(filtered[idx - 1].order, filtered[idx].order);
}

/**
 * Reparent an item: make it a child of newParentId (or top-level if null),
 * placing it at the end of the new parent's children.
 */
/** Valid parent types per item type (must match backend's ValidParentTypes) */
const VALID_PARENT_TYPES: Record<string, string[]> = {
  milestone: [],
  epic: ['milestone'],
  feature: ['milestone', 'epic'],
  task: ['milestone', 'epic', 'feature'],
  bug: ['milestone', 'epic', 'feature']
};

export function applyReparent(
  draggedId: string,
  newParentId: string | null,
  targetChildren: Item[]
): void {
  const item = itemsStore.get(draggedId);
  if (!item) return;

  // Don't reparent to self or to current parent
  if (newParentId === draggedId) return;
  if (item.parentId === newParentId) return;

  // Prevent creating cycles: newParentId must not be a descendant of draggedId
  if (newParentId && isDescendant(newParentId, draggedId)) return;

  // Validate type hierarchy client-side
  if (newParentId) {
    const parent = itemsStore.get(newParentId);
    if (!parent) return;
    const validTypes = VALID_PARENT_TYPES[item.type] ?? ['milestone', 'epic', 'feature'];
    if (!validTypes.includes(parent.type)) return;
  }

  // Compute order key at end of new sibling list
  const orderedChildren = ensureOrdered(targetChildren);
  const newOrder =
    orderedChildren.length > 0
      ? orderBetween(orderedChildren[orderedChildren.length - 1].order, '')
      : orderBetween('', '');

  // Save previous state for rollback
  const prevParentId = item.parentId;
  const prevOrder = item.order;

  itemsStore.optimisticUpdate(draggedId, { parentId: newParentId, order: newOrder });

  const input: Record<string, string | null> = { parent: newParentId ?? '', order: newOrder };
  client
    .mutation(UpdateItemOrderDocument, { id: draggedId, input })
    .toPromise()
    .then((result) => {
      if (result.error) {
        console.error('Failed to reparent item:', result.error);
        // Roll back optimistic update
        itemsStore.optimisticUpdate(draggedId, { parentId: prevParentId, order: prevOrder });
      }
    });
}

/** Check if candidateId is a descendant of ancestorId */
function isDescendant(candidateId: string, ancestorId: string): boolean {
  let current = itemsStore.get(candidateId);
  while (current?.parentId) {
    if (current.parentId === ancestorId) return true;
    current = itemsStore.get(current.parentId);
  }
  return false;
}

/**
 * Apply a drop: compute the new order, optimistically update the store,
 * and fire the GraphQL mutation. Optionally changes status (board) or
 * parent (backlog cross-group reorder).
 */
export function applyDrop(
  items: Item[],
  draggedId: string,
  targetIndex: number,
  opts?: { newStatus?: string; newParentId?: string | null }
): void {
  const item = itemsStore.get(draggedId);
  if (!item) return;

  const newStatus = opts?.newStatus;
  // undefined = don't change parent; null = move to top-level; string = reparent
  const newParentId = opts?.newParentId;
  const changingParent = newParentId !== undefined && item.parentId !== newParentId;

  // Validate type hierarchy if reparenting
  if (changingParent && newParentId) {
    const parent = itemsStore.get(newParentId);
    if (!parent) return;
    const validTypes = VALID_PARENT_TYPES[item.type] ?? ['milestone', 'epic', 'feature'];
    if (!validTypes.includes(parent.type)) return;
  }

  const orderedItems = ensureOrdered(items);
  const newOrder = computeOrder(orderedItems, targetIndex, draggedId);

  const sameStatus = !newStatus || item.status === newStatus;
  if (sameStatus && !changingParent && item.order === newOrder) return;

  // Save previous state for rollback
  const prevOrder = item.order;
  const prevStatus = item.status;
  const prevParentId = item.parentId;

  const optimistic: Partial<Item> = { order: newOrder };
  if (!sameStatus) optimistic.status = newStatus;
  if (changingParent) optimistic.parentId = newParentId;
  itemsStore.optimisticUpdate(draggedId, optimistic);

  const input: Record<string, string | null> = { order: newOrder };
  if (!sameStatus) input.status = newStatus!;
  if (changingParent) input.parent = newParentId ?? '';
  client
    .mutation(UpdateItemOrderDocument, { id: draggedId, input })
    .toPromise()
    .then((result) => {
      if (result.error) {
        console.error('Failed to update item:', result.error);
        itemsStore.writeError = result.error.message;
        itemsStore.optimisticUpdate(draggedId, {
          order: prevOrder,
          status: prevStatus,
          parentId: prevParentId
        });
      }
    });
}
