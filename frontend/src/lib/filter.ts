import type { Item } from '#lib/items.svelte.js';

/**
 * Returns true if the item matches the filter text.
 * Case-insensitive substring match against title, type, status, tags, and ID.
 */
export function matchesFilter(item: Item, text: string): boolean {
  if (!text) return true;
  const lower = text.toLowerCase();
  return (
    item.title.toLowerCase().includes(lower) ||
    item.type.toLowerCase().includes(lower) ||
    item.status.toLowerCase().includes(lower) ||
    item.id.toLowerCase().includes(lower) ||
    item.tags.some((tag) => tag.toLowerCase().includes(lower))
  );
}
