import { pipe, subscribe } from 'wonka';
import { SvelteMap } from 'svelte/reactivity';
import { client } from './graphqlClient';
import {
  ItemChangedDocument,
  type ItemFieldsFragment,
  type ItemChangedSubscription,
} from './graphql/generated';

/**
 * Item type matching the GraphQL schema (re-exported from codegen)
 */
export type Item = ItemFieldsFragment;

/**
 * Svelte 5 runes-style stateful store for items.
 * Frontend equivalent of itemcore on the backend.
 */
export class ItemsStore {
  /** All items indexed by ID */
  items = $state(new SvelteMap<string, Item>());

  /** Loading state (true until first non-initial event or subscription fully synced) */
  loading = $state(true);

  /** Error state */
  error = $state<string | null>(null);

  /**
   * Message of the last refused write (a gate block, an ETag conflict) that has
   * no form of its own to show it, such as a status button or a board drag.
   */
  writeError = $state<string | null>(null);

  /** Whether subscription is connected */
  connected = $state(false);

  /** Whether initial sync is complete */
  #initialSyncDone = false;

  /** Subscription teardown function */
  #unsubscribe: (() => void) | null = null;

  /** All non-archived items as a sorted array (derived) */
  get all(): Item[] {
    return sortItems(Array.from(this.items.values()).filter((b) => !b.path.startsWith('archive/')));
  }

  /** Count of items */
  get count(): number {
    return this.items.size;
  }

  /**
   * Start subscription to item changes with initial state.
   * This is the primary method to initialize the store - it subscribes to changes
   * and receives all current items as initial events, eliminating race conditions.
   */
  subscribe(): void {
    if (this.#unsubscribe) {
      return; // Already subscribed
    }

    this.loading = true;
    this.error = null;
    this.#initialSyncDone = false;

    const { unsubscribe } = pipe(
      client.subscription(ItemChangedDocument, { includeInitial: true }),
      subscribe((result) => {
        if (result.error) {
          console.error('Subscription error:', result.error);
          this.connected = false;
          this.error = result.error.message;
          this.loading = false;
          return;
        }

        this.connected = true;

        const event = result.data?.itemChanged;
        if (!event) return;

        switch (event.type) {
          case 'INITIAL_SNAPSHOT':
            if (event.items) {
              const fresh = new SvelteMap<string, Item>();
              for (const b of event.items) {
                fresh.set(b.id, b);
              }
              this.items = fresh;
            }
            this.#initialSyncDone = true;
            this.loading = false;
            break;
          case 'CREATED':
          case 'UPDATED':
            if (event.item) {
              this.items.set(event.item.id, event.item);
            }
            break;
          case 'DELETED':
            this.items.delete(event.itemId);
            break;
        }
      })
    );

    this.#unsubscribe = unsubscribe;
  }

  /**
   * Stop subscription to item changes.
   */
  unsubscribe(): void {
    if (this.#unsubscribe) {
      this.#unsubscribe();
      this.#unsubscribe = null;
      this.connected = false;
    }
  }

  /**
   * Get an item by ID
   */
  get(id: string): Item | undefined {
    return this.items.get(id);
  }

  /**
   * Get items filtered by status
   */
  byStatus(status: string): Item[] {
    return this.all.filter((b) => b.status === status);
  }

  /**
   * Get items filtered by type
   */
  byType(type: string): Item[] {
    return this.all.filter((b) => b.type === type);
  }

  /**
   * Get children of an item (items with this item as parent)
   */
  children(parentId: string): Item[] {
    return this.all.filter((b) => b.parentId === parentId);
  }

  /**
   * Get items that are blocking a given item
   */
  blockedBy(itemId: string): Item[] {
    return this.all.filter((b) => b.blockingIds.includes(itemId));
  }

  /**
   * Optimistically update an item's fields in the local store.
   * The subscription will eventually confirm or overwrite.
   */
  optimisticUpdate(id: string, fields: Partial<Item>): void {
    const item = this.items.get(id);
    if (item) {
      this.items.set(id, { ...item, ...fields });
    }
  }
}

/**
 * Sort order arrays matching the backend's config.DefaultStatuses/Priorities/Types.
 * These must stay in sync with internal/config/config.go.
 */
const STATUS_ORDER = ['in-progress', 'todo', 'draft', 'completed', 'scrapped'];
const PRIORITY_ORDER = ['critical', 'high', 'normal', 'low', 'deferred'];
const TYPE_ORDER = ['milestone', 'epic', 'bug', 'feature', 'task'];

function orderOf(value: string, order: string[], defaultTo?: string): number {
  if (!value && defaultTo) value = defaultTo;
  const idx = order.indexOf(value);
  return idx >= 0 ? idx : order.length;
}

/**
 * Sort items by status → priority → type → title, matching the backend's
 * SortByStatusPriorityAndType from internal/item/sort.go.
 */
export function sortItems(items: Item[]): Item[] {
  return items.toSorted((a, b) => {
    // Primary: status
    let d = orderOf(a.status, STATUS_ORDER) - orderOf(b.status, STATUS_ORDER);
    if (d !== 0) return d;

    // Secondary: manual order (fractional index) — items with order come first
    if (a.order && b.order) {
      if (a.order < b.order) return -1;
      if (a.order > b.order) return 1;
    } else if (a.order && !b.order) return -1;
    else if (!a.order && b.order) return 1;

    // Tertiary: priority (empty = normal)
    d =
      orderOf(a.priority, PRIORITY_ORDER, 'normal') - orderOf(b.priority, PRIORITY_ORDER, 'normal');
    if (d !== 0) return d;

    // Quaternary: type
    d = orderOf(a.type, TYPE_ORDER) - orderOf(b.type, TYPE_ORDER);
    if (d !== 0) return d;

    // Final: title (case-insensitive)
    return a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
  });
}

/**
 * Singleton instance of the items store
 */
export const itemsStore = new ItemsStore();
