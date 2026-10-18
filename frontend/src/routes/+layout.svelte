<script lang="ts">
  import './layout.css';
  import favicon from '#lib/assets/favicon.svg';
  import { preloadHighlighter } from '#lib/markdown.js';
  import { page } from '$app/state';
  import { onMount, onDestroy, untrack } from 'svelte';
  import { itemsStore } from '#lib/items.svelte.js';
  import { worktreeStore, MAIN_WORKSPACE_ID } from '#lib/worktrees.svelte.js';
  import { agentStatusesStore } from '#lib/agentStatuses.svelte.js';
  import { configStore } from '#lib/config.svelte.js';
  import { ui } from '#lib/uiState.svelte.js';
  import { theme } from '#lib/theme.svelte.js';
  import ItemForm from '#lib/components/ItemForm.svelte';
  import Sidebar from '#lib/components/Sidebar.svelte';
  import SplitPane from '#lib/components/SplitPane.svelte';

  preloadHighlighter();

  let { data, children } = $props();

  // Initialize UI state from load function data (runs before first render)
  $effect.pre(() => {
    ui.showPlanningChat = data.showPlanningChat;
    ui.showChanges = data.showChanges;
    ui.filterText = data.filterText;
  });

  // Sync UIState from URL path on every navigation
  $effect(() => {
    ui.syncFromUrl(page.url.pathname);
  });

  // Hydrate initial item selection from URL ?item= param (once, after activeView is set)
  let initialItemApplied = false;
  $effect(() => {
    if (!initialItemApplied && data.selectedItemId) {
      initialItemApplied = true;
      ui.selectItemById(data.selectedItemId);
    }
  });

  // Fall back to planning view if agents are disabled or the workspace's worktree is removed.
  // Wait for worktreeStore to initialize before checking — otherwise a cold page load
  // to a workspace route would redirect before the subscription delivers data.
  $effect(() => {
    if (
      !ui.isPlanning &&
      worktreeStore.initialized &&
      (!configStore.agentEnabled || (ui.activeView !== MAIN_WORKSPACE_ID && !worktreeStore.hasWorktree(ui.activeView)))
    ) {
      ui.navigateTo('planning');
    }
  });

  // Follow display.theme once the config has loaded (it picks the dark flavour)
  $effect(() => {
    const name = configStore.theme;
    if (name) untrack(() => theme.setConfigured(name));
  });

  onMount(() => {
    theme.init();
    configStore.load();
    itemsStore.subscribe();
    worktreeStore.subscribe();
    agentStatusesStore.subscribe();
  });

  onDestroy(() => {
    itemsStore.unsubscribe();
    worktreeStore.unsubscribe();
    agentStatusesStore.unsubscribe();
  });
</script>

<svelte:head>
  <title>{configStore.projectName ? `${configStore.projectName} — Lif` : 'Lif'}</title>
  <link rel="icon" href={favicon} />
</svelte:head>

<div class="flex h-screen flex-col bg-surface-alt">
  {#if itemsStore.error}
    <div class="m-4">
      <div class="rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
        Error: {itemsStore.error}
      </div>
    </div>
  {:else}
    {#snippet sidebarPanel()}
      <Sidebar />
    {/snippet}

    {#snippet mainContent()}
      {@render children()}
    {/snippet}

    <SplitPane
      direction="horizontal"
      panels={[
        { content: sidebarPanel, size: 224, minSize: 150, maxSize: 400, persistKey: 'sidebar' },
        { content: mainContent }
      ]}
    />
  {/if}
</div>

{#if ui.showForm}
  <ItemForm
    item={ui.editingItem}
    onClose={() => ui.closeForm()}
    onSaved={(item) => ui.selectItem(item)}
  />
{/if}
