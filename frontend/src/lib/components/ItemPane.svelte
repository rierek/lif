<script lang="ts">
  import type { Item } from '#lib/items.svelte.js';
  import ItemDetail from './ItemDetail.svelte';

  interface Props {
    item: Item;
    onSelect?: (item: Item) => void;
    onEdit?: (item: Item) => void;
    onClose?: () => void;
  }

  let { item, onSelect, onEdit, onClose }: Props = $props();
</script>

<div class="flex h-full flex-col bg-surface">
  <div class="pane-toolbar">
    <span>Item</span>
    <div class="flex-1"></div>
    {#if onClose}
      <button onclick={onClose} class="btn-icon" title="Close"> &#x2715; </button>
    {/if}
  </div>

  <div class="min-h-0 flex-1">
    <ItemDetail {item} {onSelect} {onEdit} />
  </div>
</div>
