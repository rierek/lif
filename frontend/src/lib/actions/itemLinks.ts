import type { Action } from 'svelte/action';
import { itemsStore } from '#lib/items.svelte.js';
import { ui } from '#lib/uiState.svelte.js';

/**
 * Svelte action that turns item references inside rendered content into in-app
 * navigation. The references are real links (`<a href="?item=…">`), so keyboard
 * users reach and activate them without help; this action only intercepts the
 * resulting activation to select the item in place instead of reloading.
 *
 * Modified clicks (new tab, new window, download) are left to the browser.
 *
 * Usage:
 *   <div use:itemLinks>{@html renderedHtml}</div>
 */
export const itemLinks: Action<HTMLElement> = (node) => {
  function handleClick(e: MouseEvent) {
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) {
      return;
    }
    const target = (e.target as HTMLElement).closest<HTMLElement>('[data-item-id]');
    if (!target || !node.contains(target)) return;

    // Prevent the default before the store lookup, not after: a reference whose
    // item is not in the store (archived, deleted, another store, a false
    // positive of the beans-xxxx matcher) must be inert. Letting the browser
    // follow `?item=<id>` there pushes a history entry for an item that is
    // neither selected nor existent.
    e.preventDefault();

    const linkedItem = itemsStore.get(target.dataset.itemId!);
    if (!linkedItem) return;

    ui.selectItem(linkedItem);
  }

  node.addEventListener('click', handleClick);

  return {
    destroy() {
      node.removeEventListener('click', handleClick);
    }
  };
};
