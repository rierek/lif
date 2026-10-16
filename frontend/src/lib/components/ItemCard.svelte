<script lang="ts">
  import type { Item } from '#lib/items.svelte.js';
  import { itemsStore } from '#lib/items.svelte.js';
  import { worktreeStore } from '#lib/worktrees.svelte.js';
  import { agentStatusesStore } from '#lib/agentStatuses.svelte.js';
  import { ui } from '#lib/uiState.svelte.js';
  import { statusColors, typeColors, typeBorders, priorityIndicators } from '#lib/styles.js';
  import { client } from '#lib/graphqlClient.js';
  import { ArchiveItemDocument } from '#lib/graphql/generated.js';

  interface Props {
    item: Item;
    variant?: 'list' | 'board' | 'compact';
    selected?: boolean;
    onclick?: () => void;
  }

  let { item, variant = 'list', selected = false, onclick }: Props = $props();

  const linkedWorktreeId = $derived(worktreeStore.worktreeForItem(item.id));
  const hasWorktree = $derived(variant !== 'compact' && !!linkedWorktreeId);
  const agentRunning = $derived(hasWorktree && linkedWorktreeId != null && agentStatusesStore.isRunning(linkedWorktreeId));

  const worktreeLabel = $derived.by(() => {
    if (!linkedWorktreeId) return '';
    const wt = worktreeStore.worktrees.find((w) => w.id === linkedWorktreeId);
    return wt?.name ?? linkedWorktreeId;
  });

  function handleWorktreeClick(e: MouseEvent) {
    e.stopPropagation();
    if (linkedWorktreeId) {
      ui.navigateTo(linkedWorktreeId);
    }
  }
  const isArchivable = $derived(item.status === 'completed' || item.status === 'scrapped');

  let archiving = $state(false);

  async function handleArchive(e: MouseEvent) {
    e.stopPropagation();
    archiving = true;
    await client.mutation(ArchiveItemDocument, { id: item.id }).toPromise();
    archiving = false;
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onclick?.();
    }
  }
</script>

<!-- Using div instead of button so we can nest the archive <button> inside (HTML forbids button-in-button) -->
<div
  {onclick}
  onkeydown={handleKeydown}
  role="button"
  tabindex="0"
  class={[
    'relative w-full cursor-pointer overflow-hidden text-left transition-all',
    variant === 'board'
      ? 'p-3'
      : [
          'rounded-xs p-2',
          variant === 'compact' ? 'border-l-2' : 'border-l-3',
          typeBorders[item.type] ?? 'border-l-type-task-border',
          selected ? 'bg-accent/10 ring-1 ring-accent' : 'bg-surface hover:bg-surface-alt'
        ]
  ]}
>
  {#if variant === 'board'}
    <!-- Board: two-row layout -->
    <div class="flex min-w-0 items-start gap-2">
      <span class="flex-1 text-sm leading-snug text-text">{item.title}</span>
      {#if item.priority && item.priority !== 'normal' && priorityIndicators[item.priority]}
        <span class={['shrink-0 text-xs', priorityIndicators[item.priority]]}>
          {item.priority}
        </span>
      {/if}
    </div>
    <div class="mt-1 flex items-center gap-2">
      <code class="text-[10px] text-text-faint">{item.id.slice(-4)}</code>
      <span
        class={[
          'badge-sm',
          typeColors[item.type] ?? 'bg-type-task-bg text-type-task-text'
        ]}
      >
        {item.type}
      </span>
      {#each item.tags as tag}
        <span class="badge-sm bg-surface-alt text-text-muted">{tag}</span>
      {/each}
      {#if hasWorktree}
        <button
          class="ml-auto flex cursor-pointer items-center gap-1 rounded-sm px-1.5 py-0.5 text-[10px] text-success transition-colors hover:bg-success/10"
          title="Go to workspace: {worktreeLabel}"
          onclick={handleWorktreeClick}
        >
          {#if agentRunning}
            <span class="loader inline-block !size-2.5"></span>
          {/if}
          <span class="icon-[uil--code-branch] size-3"></span>
        </button>
      {/if}
      {#if isArchivable}
        <button
          class={['cursor-pointer icon-[uil--archive] size-3.5 text-text-faint transition-colors hover:text-text-muted disabled:opacity-50', !hasWorktree && 'ml-auto']}
          title="Archive"
          onclick={handleArchive}
          disabled={archiving}
        ></button>
      {/if}
    </div>
  {:else}
    <!-- List / Compact: single-row layout -->
    <div class="flex min-w-0 items-center gap-2">
      <code
        class={['shrink-0 text-text-faint', variant === 'compact' ? 'text-[9px]' : 'text-[10px]']}
        >{item.id.slice(-4)}</code
      >
      <span class={['flex-1 truncate text-text', variant === 'compact' ? 'text-xs' : 'text-sm']}
        >{item.title}</span
      >
      {#each item.tags as tag}
        <span class="shrink-0 badge-sm bg-surface-alt text-text-muted">{tag}</span>
      {/each}
      {#if hasWorktree}
        <button
          class="flex shrink-0 cursor-pointer items-center gap-0.5 rounded-sm px-1 py-0.5 text-success transition-colors hover:bg-success/10"
          title="Go to workspace: {worktreeLabel}"
          onclick={handleWorktreeClick}
        >
          {#if agentRunning}
            <span class="loader inline-block !size-2.5"></span>
          {/if}
          <span class="icon-[uil--code-branch] size-3"></span>
        </button>
      {/if}
      <span
        class={[
          'shrink-0 badge-sm',
          statusColors[item.status] ?? 'bg-status-todo-bg text-status-todo-text'
        ]}
      >
        {item.status}
      </span>
      {#if isArchivable}
        <button
          class="cursor-pointer icon-[uil--archive] size-3.5 shrink-0 text-text-faint transition-colors hover:text-text-muted disabled:opacity-50"
          title="Archive"
          onclick={handleArchive}
          disabled={archiving}
        ></button>
      {/if}
    </div>
  {/if}
</div>

