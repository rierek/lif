<script lang="ts">
  import type { Item } from '#lib/items.svelte.js';
  import { itemsStore } from '#lib/items.svelte.js';
  import { client } from '#lib/graphqlClient.js';
  import { CreateItemDocument, UpdateItemDocument, type CreateItemInput, type UpdateItemInput } from '#lib/graphql/generated.js';

  interface Props {
    item?: Item | null;
    onClose: () => void;
    onSaved?: (item: Item) => void;
  }

  let { item = null, onClose, onSaved }: Props = $props();

  const isEdit = $derived(!!item);

  // Form fields — intentionally capture initial prop values for local editing
  /* eslint-disable svelte/valid-compile */
  // svelte-ignore state_referenced_locally
  let title = $state(item?.title ?? '');
  // svelte-ignore state_referenced_locally
  let type = $state(item?.type ?? 'task');
  // svelte-ignore state_referenced_locally
  let status = $state(item?.status ?? 'todo');
  // svelte-ignore state_referenced_locally
  let priority = $state(item?.priority ?? 'normal');
  // svelte-ignore state_referenced_locally
  let tags = $state(item?.tags.join(', ') ?? '');
  // svelte-ignore state_referenced_locally
  let body = $state(item?.body ?? '');
  // svelte-ignore state_referenced_locally
  let parentId = $state(item?.parentId ?? '');
  /* eslint-enable svelte/valid-compile */

  let submitting = $state(false);
  let error = $state<string | null>(null);

  const types = ['task', 'bug', 'feature', 'epic', 'milestone'];
  const statuses = ['draft', 'todo', 'in-progress', 'completed', 'scrapped'];
  const priorities = ['critical', 'high', 'normal', 'low', 'deferred'];

  // Available parents (all items except current item and its descendants)
  const availableParents = $derived(
    itemsStore.all.filter((b) => {
      if (!item) return true;
      if (b.id === item.id) return false;
      // Simple cycle check: don't allow own children as parent
      let current: Item | undefined = b;
      while (current) {
        if (current.parentId === item.id) return false;
        current = current.parentId ? itemsStore.get(current.parentId) : undefined;
      }
      return true;
    })
  );

  function parseTags(raw: string): string[] {
    return raw
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
  }

  async function handleSubmit() {
    if (!title.trim()) {
      error = 'Title is required';
      return;
    }

    submitting = true;
    error = null;

    const fields = {
      title: title.trim(),
      type,
      status,
      priority,
      body: body || null,
      tags: parseTags(tags),
      parent: parentId || null
    };

    let saved: Item | null;
    if (isEdit && item) {
      const input: UpdateItemInput = fields;
      const result = await client.mutation(UpdateItemDocument, { id: item.id, input }).toPromise();
      submitting = false;
      if (result.error) { error = result.error.message; return; }
      saved = result.data?.updateItem ?? null;
    } else {
      const input: CreateItemInput = fields;
      const result = await client.mutation(CreateItemDocument, { input }).toPromise();
      submitting = false;
      if (result.error) { error = result.error.message; return; }
      saved = result.data?.createItem ?? null;
    }
    if (saved) {
      onSaved?.(saved);
    }
    onClose();
  }
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
  <div class="w-11/12 max-w-2xl rounded-xl bg-surface p-6 shadow-xl">
    <h3 class="text-lg font-bold text-text">{isEdit ? 'Edit Item' : 'New Item'}</h3>

    {#if error}
      <div
        class="mt-4 rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger"
      >
        {error}
      </div>
    {/if}

    <form
      onsubmit={(e) => {
        e.preventDefault();
        handleSubmit();
      }}
      class="mt-4 space-y-4"
    >
      <!-- Title -->
      <div>
        <label class="mb-1 block text-sm font-medium text-text-muted" for="item-title">Title</label>
        <input
          id="item-title"
          type="text"
          class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
          bind:value={title}
          placeholder="What needs to be done?"
        />
      </div>

      <!-- Type / Status / Priority row -->
      <div class="grid grid-cols-3 gap-3">
        <div>
          <label class="mb-1 block text-sm font-medium text-text-muted" for="item-type">Type</label>
          <select
            id="item-type"
            class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
            bind:value={type}
          >
            {#each types as t}
              <option value={t}>{t}</option>
            {/each}
          </select>
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-text-muted" for="item-status"
            >Status</label
          >
          <select
            id="item-status"
            class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
            bind:value={status}
          >
            {#each statuses as s}
              <option value={s}>{s}</option>
            {/each}
          </select>
        </div>

        <div>
          <label class="mb-1 block text-sm font-medium text-text-muted" for="item-priority"
            >Priority</label
          >
          <select
            id="item-priority"
            class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
            bind:value={priority}
          >
            {#each priorities as p}
              <option value={p}>{p}</option>
            {/each}
          </select>
        </div>
      </div>

      <!-- Parent -->
      <div>
        <label class="mb-1 block text-sm font-medium text-text-muted" for="item-parent"
          >Parent</label
        >
        <select
          id="item-parent"
          class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
          bind:value={parentId}
        >
          <option value="">None</option>
          {#each availableParents as p}
            <option value={p.id}>{p.title} ({p.type})</option>
          {/each}
        </select>
      </div>

      <!-- Tags -->
      <div>
        <label class="mb-1 block text-sm font-medium text-text-muted" for="item-tags">Tags</label>
        <input
          id="item-tags"
          type="text"
          class="w-full rounded-md border border-border bg-surface px-3 py-2 text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
          bind:value={tags}
          placeholder="Comma-separated tags"
        />
      </div>

      <!-- Body -->
      <div>
        <label class="mb-1 block text-sm font-medium text-text-muted" for="item-body"
          >Description (Markdown)</label
        >
        <textarea
          id="item-body"
          class="h-40 w-full resize-y rounded-md border border-border bg-surface px-3 py-2 font-mono text-sm text-text focus:border-accent focus:ring-2 focus:ring-accent/50 focus:outline-none"
          bind:value={body}
          placeholder="Markdown content..."
        ></textarea>
      </div>

      <!-- Actions -->
      <div class="flex justify-end gap-2 pt-2">
        <button
          type="button"
          class="cursor-pointer rounded-md border border-border px-4 py-2 text-sm font-medium text-text-muted transition-colors hover:bg-surface-alt"
          onclick={onClose}
          disabled={submitting}>Cancel</button
        >
        <button
          type="submit"
          class="flex cursor-pointer items-center gap-2 rounded-md bg-accent px-4 py-2 text-sm font-medium text-accent-text transition-opacity hover:opacity-90 disabled:opacity-50"
          disabled={submitting || !title.trim()}
        >
          {#if submitting}
            <span
              class="inline-block h-4 w-4 animate-spin rounded-full border-2 border-accent-text/30 border-t-accent-text"
            ></span>
          {/if}
          {isEdit ? 'Save Changes' : 'Create Item'}
        </button>
      </div>
    </form>
  </div>
  <!-- Backdrop -->
  <button class="fixed inset-0 -z-10" onclick={onClose} tabindex="-1" aria-label="Close"></button>
</div>
