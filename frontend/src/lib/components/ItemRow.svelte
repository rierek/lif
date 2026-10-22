<script lang="ts">
  import type { Item } from '#lib/items.svelte.js';
  import { itemsStore } from '#lib/items.svelte.js';
  import { backlogDrag } from '#lib/backlogDrag.svelte.js';
  import { matchesFilter } from '#lib/filter.js';
  import ItemCard from './ItemCard.svelte';
  import ItemRow from './ItemRow.svelte';

  interface Props {
    item: Item;
    /** Parent ID of this item's sibling group (null = top-level) */
    parentId?: string | null;
    index?: number;
    depth?: number;
    selectedId?: string | null;
    onSelect?: (item: Item) => void;
    filterText?: string;
    /** Status of the backlog section this item is in (for cross-section drag) */
    sectionStatus?: string;
  }

  let {
    item,
    parentId = null,
    index = 0,
    depth = 0,
    selectedId = null,
    onSelect,
    filterText = '',
    sectionStatus
  }: Props = $props();

  const children = $derived(itemsStore.children(item.id));
  const filteredChildren = $derived(
    filterText ? children.filter((child) => matchesFilter(child, filterText)) : children
  );

  function handleClick(e: MouseEvent) {
    e.stopPropagation();
    onSelect?.(item);
  }
</script>

<div class="item-row my-1" role="listitem" data-item-id={item.id}>
  <!-- Drop indicator before this card -->
  <div
    class={[
      'mx-1 rounded-full transition-colors',
      backlogDrag.showIndicator(parentId, index, item.id, sectionStatus) ? 'h-0.5 bg-accent' : 'h-0'
    ]}
  ></div>

  <!-- Drag wrapper; the card inside it is the focusable control. -->
  <div
    class={[
      'rounded transition-all',
      backlogDrag.draggedItemId === item.id && 'opacity-40',
      backlogDrag.isReparentTarget(item.id) && 'ring-2 ring-accent ring-offset-1'
    ]}
    draggable="true"
    ondragstart={(e) => backlogDrag.startDrag(e, item)}
    ondragend={() => backlogDrag.endDrag()}
    ondragover={(e) => backlogDrag.hoverCard(e, parentId, index, item.id, sectionStatus)}
    onclick={handleClick}
    role="presentation"
  >
    <ItemCard
      {item}
      variant="list"
      selected={selectedId === item.id}
      onclick={() => onSelect?.(item)}
    />
  </div>

  {#if filteredChildren.length > 0}
    <div
      class="ml-6"
      ondragover={(e) => backlogDrag.hoverList(e, item.id, filteredChildren.length)}
      ondragleave={(e) => backlogDrag.leaveList(e, e.currentTarget, item.id)}
      ondrop={(e) => backlogDrag.drop(e, item.id, filteredChildren)}
      role="list"
    >
      {#each filteredChildren as child, i (child.id)}
        <ItemRow
          item={child}
          parentId={item.id}
          index={i}
          depth={depth + 1}
          {selectedId}
          {onSelect}
          {filterText}
          {sectionStatus}
        />
      {/each}

      <!-- Drop indicator at end of children -->
      <div
        class={[
          'mx-1 rounded-full transition-colors',
          backlogDrag.showEndIndicator(item.id, filteredChildren.length)
            ? 'h-0.5 bg-accent'
            : 'h-0'
        ]}
      ></div>
    </div>
  {/if}
</div>
