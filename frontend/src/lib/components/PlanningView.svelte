<script lang="ts">
  import { pointerClick } from '#lib/actions/pointerClick.js';
  import { itemsStore } from '#lib/items.svelte.js';
  import { ui } from '#lib/uiState.svelte.js';

  let { planningView }: { planningView: 'backlog' | 'board' } = $props();
  import { backlogDrag } from '#lib/backlogDrag.svelte.js';
  import { matchesFilter } from '#lib/filter.js';
  import ItemRow from '#lib/components/ItemRow.svelte';
  import BoardView from '#lib/components/BoardView.svelte';
  import ItemPane from '#lib/components/ItemPane.svelte';
  import SplitPane from '#lib/components/SplitPane.svelte';
  import FilterInput from '#lib/components/FilterInput.svelte';
  import ViewToolbar from '#lib/components/ViewToolbar.svelte';

  let filterInput = $state<FilterInput | null>(null);

  const topLevelItems = $derived(itemsStore.all.filter((b) => !b.parentId));

  function filterItems(items: typeof topLevelItems) {
    const text = ui.filterText;
    if (!text) return items;
    return items.filter((item) => {
      if (matchesFilter(item, text)) return true;
      return itemsStore.children(item.id).some((child) => matchesFilter(child, text));
    });
  }

  const filteredTodoItems = $derived(filterItems(topLevelItems.filter((b) => b.status === 'todo')));
  const filteredDraftItems = $derived(
    filterItems(topLevelItems.filter((b) => b.status === 'draft'))
  );

  function handleKeydown(e: KeyboardEvent) {
    if ((e.metaKey || e.ctrlKey) && (e.key === 'f' || e.key === '/')) {
      e.preventDefault();
      filterInput?.focus();
      return;
    }
    if (e.key === 'Escape' && ui.currentItem && !ui.showForm) {
      ui.clearSelection();
    }
  }

  function handlePlanningClick(e: MouseEvent) {
    if (e.target === e.currentTarget) {
      ui.clearSelection();
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="flex h-full flex-col">
  <ViewToolbar showPanelToggles={false}>
    <div class="flex">
      <button
        onclick={() => ui.navigateToPlanningView('backlog')}
        class={[
          'btn-tab rounded-l-md',
          planningView === 'backlog' ? 'btn-tab-active' : 'btn-tab-inactive'
        ]}
      >
        Backlog
      </button>
      <button
        onclick={() => ui.navigateToPlanningView('board')}
        class={[
          'btn-tab rounded-r-md border-l-0',
          planningView === 'board' ? 'btn-tab-active' : 'btn-tab-inactive'
        ]}
      >
        Board
      </button>
    </div>
    <div class="mx-3 w-60">
      <FilterInput bind:this={filterInput} />
    </div>
    {#snippet right()}
      <button class="btn-primary" onclick={() => ui.openCreateForm()}>+ New Item</button>
    {/snippet}
  </ViewToolbar>

  <div class="flex min-h-0 flex-1 overflow-hidden">
    {#snippet backlogBoard()}
        <div class="flex h-full flex-col bg-surface">
          {#if planningView === 'backlog'}
            <!-- Clicking empty space clears the selection; Escape does the same. -->
            <div class="min-h-0 flex-1 overflow-auto bg-surface-alt" use:pointerClick={handlePlanningClick}>
              {#snippet backlogSection(items: typeof filteredTodoItems, status: string, label: string)}
                <div
                  class="p-3"
                  ondragover={(e) => backlogDrag.hoverList(e, null, items.length, status)}
                  ondragleave={(e) => backlogDrag.leaveList(e, e.currentTarget, null)}
                  ondrop={(e) => backlogDrag.drop(e, null, items)}
                  role="list"
                >
                  <h3 class="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-text-faint">
                    {label}
                    <span class="font-normal">{items.length}</span>
                  </h3>
                  {#each items as item, i (item.id)}
                    <ItemRow
                      {item}
                      parentId={null}
                      index={i}
                      selectedId={ui.currentItem?.id}
                      onSelect={(b) => ui.selectItem(b)}
                      filterText={ui.filterText}
                      sectionStatus={status}
                    />
                  {:else}
                    {#if !itemsStore.loading}
                      <p class="text-center py-4 text-sm text-text-muted">No items</p>
                    {/if}
                  {/each}

                  <div
                    class={[
                      'mx-1 rounded-full transition-colors',
                      backlogDrag.showEndIndicator(null, items.length, status)
                        ? 'h-0.5 bg-accent'
                        : 'h-0'
                    ]}
                  ></div>
                </div>
              {/snippet}

              {@render backlogSection(filteredTodoItems, 'todo', 'Todo')}
              {@render backlogSection(filteredDraftItems, 'draft', 'Draft')}
            </div>
          {:else}
            <BoardView onSelect={(b) => ui.selectItem(b)} selectedId={ui.currentItem?.id} />
          {/if}
        </div>
      {/snippet}

      {#snippet detailPanel()}
        {#if ui.currentItem}
          <ItemPane
            item={ui.currentItem}
            onSelect={(b) => ui.selectItem(b)}
            onEdit={(b) => ui.openEditForm(b)}
            onClose={() => ui.clearSelection()}
          />
        {/if}
      {/snippet}

      <SplitPane
        direction="horizontal"
        panels={[
          { content: backlogBoard },
          {
            content: detailPanel,
            size: 480,
            collapsed: !ui.currentItem,
            persistKey: 'detail-width'
          }
        ]}
      />
  </div>
</div>
