<script lang="ts">
  import { browser } from '$app/env';
  import { ui } from '#lib/uiState.svelte.js';

  let inputEl = $state<HTMLInputElement | null>(null);
  // Guarded: without a browser there is no platform to detect, and reading
  // navigator at render time breaks SSR and prerendering.
  const isMac = browser && navigator.platform.startsWith('Mac');
  const shortcutHint = isMac ? '⌘/' : 'Ctrl+/';

  export function focus() {
    inputEl?.focus();
  }
</script>

<div class="relative">
  <input
    bind:this={inputEl}
    type="text"
    placeholder="Filter items… ({shortcutHint})"
    value={ui.filterText}
    oninput={(e) => ui.setFilterText(e.currentTarget.value)}
    class="w-full rounded-md border-none bg-surface px-3 py-1.5 pr-8 text-sm text-text placeholder:text-text-faint focus:outline-none"
    data-testid="filter-input"
  />
  {#if ui.filterText}
    <button
      onclick={() => {
        ui.setFilterText('');
        inputEl?.focus();
      }}
      class="absolute top-1/2 right-2 -translate-y-1/2 cursor-pointer text-text-muted hover:text-text"
      title="Clear filter"
      data-testid="filter-clear"
    >
      &#x2715;
    </button>
  {/if}
</div>
