<script lang="ts">
  import type { Item } from '#lib/items.svelte.js';
  import { itemsStore, sortItems } from '#lib/items.svelte.js';
  import { applyDrop } from '#lib/dragOrder.js';
  import { matchesFilter } from '#lib/filter.js';
  import { client } from '#lib/graphqlClient.js';
  import { ui } from '#lib/uiState.svelte.js';
  import { typeBorders } from '#lib/styles.js';
  import { fade } from 'svelte/transition';
  import { ArchiveItemDocument } from '#lib/graphql/generated.js';
  import ItemCard from './ItemCard.svelte';
  import ConfirmModal from './ConfirmModal.svelte';

  interface Props {
    onSelect?: (item: Item) => void;
    selectedId?: string | null;
  }

  let { onSelect, selectedId = null }: Props = $props();

  let confirmingArchiveAll = $state(false);
  let archivingAll = $state(false);

  async function archiveAll() {
    archivingAll = true;
    const completedItems = itemsForStatus('completed');
    for (const item of completedItems) {
      await client.mutation(ArchiveItemDocument, { id: item.id }).toPromise();
    }
    archivingAll = false;
    confirmingArchiveAll = false;
  }

  const columns = [
    { status: 'todo', label: 'Todo', color: 'bg-status-todo-bg text-status-todo-text' },
    {
      status: 'in-progress',
      label: 'In Progress',
      color: 'bg-status-in-progress-bg text-status-in-progress-text'
    },
    {
      status: 'completed',
      label: 'Completed',
      color: 'bg-status-completed-bg text-status-completed-text'
    }
  ];

  function itemsForStatus(status: string): Item[] {
    // sortItems already handles order → priority → type → title sorting
    return sortItems(
      itemsStore.all.filter(
        (b) => b.status === status && b.status !== 'scrapped' && matchesFilter(b, ui.filterText)
      )
    );
  }

  // Drag and drop
  let draggedItemId = $state<string | null>(null);
  let dropTargetStatus = $state<string | null>(null);
  let dropIndex = $state<number | null>(null);

  function onDragStart(e: DragEvent, item: Item) {
    draggedItemId = item.id;
    e.dataTransfer!.effectAllowed = 'move';
    e.dataTransfer!.setData('text/plain', item.id);
  }

  function onDragEnd() {
    draggedItemId = null;
    dropTargetStatus = null;
    dropIndex = null;
  }

  function onCardDragOver(e: DragEvent, status: string, index: number) {
    e.preventDefault();
    e.stopPropagation();
    e.dataTransfer!.dropEffect = 'move';
    dropTargetStatus = status;

    // Determine if we're in the top or bottom half of the card
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
    const midY = rect.top + rect.height / 2;
    dropIndex = e.clientY < midY ? index : index + 1;
  }

  function onColumnDragOver(e: DragEvent, status: string, itemCount: number) {
    e.preventDefault();
    e.dataTransfer!.dropEffect = 'move';
    dropTargetStatus = status;
    // If dragging over empty space at the bottom, drop at end
    if (dropIndex === null || dropTargetStatus !== status) {
      dropIndex = itemCount;
    }
  }

  function onDragLeave(e: DragEvent, columnEl: HTMLElement) {
    if (!columnEl.contains(e.relatedTarget as Node)) {
      dropTargetStatus = null;
      dropIndex = null;
    }
  }

  function onDrop(e: DragEvent, targetStatus: string, items: Item[]) {
    e.preventDefault();
    const targetIdx = dropIndex;
    dropTargetStatus = null;
    dropIndex = null;

    const itemId = draggedItemId;
    draggedItemId = null;

    if (!itemId) return;

    applyDrop(items, itemId, targetIdx ?? items.length, { newStatus: targetStatus });
  }
</script>

<div class="min-h-0 flex-1 overflow-auto bg-surface-alt px-4 pt-4">
  <div class="flex w-full max-w-4xl min-w-0 items-start">
  {#each columns as col (col.status)}
    {@const items = itemsForStatus(col.status)}
    <div class="min-w-50 flex-1" data-status={col.status}>
      <!-- Column header -->
      <div class="sticky top-0 z-10 mb-3 flex items-center gap-2 bg-surface-alt px-1 pb-1">
        <span class={['badge', col.color]}
          >{col.label}</span
        >
        <span class="text-xs text-text-faint">{items.length}</span>
        {#if col.status === 'completed' && items.length > 0}
          <button
            class="cursor-pointer text-text-faint transition-colors hover:text-text-muted"
            title="Archive all completed items"
            onclick={() => (confirmingArchiveAll = true)}
            disabled={archivingAll}
          >
            <span class="icon-[uil--archive] size-3.5"></span>
          </button>
        {/if}
      </div>

      <!-- Cards (drop zone) -->
      <div
        class={[
          'rounded-xl p-2 transition-colors',
          dropTargetStatus === col.status && draggedItemId && 'bg-accent/10 ring-2 ring-accent/30'
        ]}
        role="list"
        ondragover={(e) => onColumnDragOver(e, col.status, items.length)}
        ondragleave={(e) => onDragLeave(e, e.currentTarget)}
        ondrop={(e) => onDrop(e, col.status, items)}
      >
        {#each items as item, index (item.id)}
          <!-- Drop indicator (always present, transparent unless active) -->
          <div
            class={[
              'mx-1 my-1 h-0.5 rounded-full transition-colors',
              dropTargetStatus === col.status &&
              draggedItemId &&
              draggedItemId !== item.id &&
              dropIndex === index
                ? 'bg-accent'
                : 'bg-transparent'
            ]}
          ></div>

          <div
            class={[
              'overflow-hidden rounded border border-l-5 border-border bg-surface shadow transition-all',
              typeBorders[item.type] ?? 'border-l-type-task-border',
              draggedItemId === item.id ? 'opacity-40' : 'hover:shadow-md',
              selectedId === item.id && 'bg-accent/5 ring-1 ring-accent'
            ]}
            draggable="true"
            ondragstart={(e) => onDragStart(e, item)}
            ondragend={onDragEnd}
            ondragover={(e) => onCardDragOver(e, col.status, index)}
            role="listitem"
            transition:fade={{ duration: 150 }}
          >
            <ItemCard {item} variant="board" onclick={() => onSelect?.(item)} />
          </div>
        {:else}
          <div class="text-center text-text-faint text-sm py-8">No items</div>
        {/each}

        <!-- Drop indicator at end (always present) -->
        <div
          class={[
            'mx-1 my-1 h-0.5 rounded-full transition-colors',
            dropTargetStatus === col.status && draggedItemId && dropIndex === items.length
              ? 'bg-accent'
              : 'bg-transparent'
          ]}
        ></div>
      </div>
    </div>
  {/each}
  </div>
</div>

{#if confirmingArchiveAll}
  {@const completedCount = itemsForStatus('completed').length}
  <ConfirmModal
    title="Archive All Completed"
    message="Are you sure you want to archive all {completedCount} completed items? This will move them to the archive directory."
    confirmLabel="Archive All"
    danger={false}
    onConfirm={archiveAll}
    onCancel={() => (confirmingArchiveAll = false)}
  />
{/if}
