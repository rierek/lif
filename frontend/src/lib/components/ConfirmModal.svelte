<script lang="ts">
  import { pointerClick } from '#lib/actions/pointerClick.js';
  interface Props {
    title: string;
    message: string;
    confirmLabel?: string;
    cancelLabel?: string;
    danger?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  }

  let {
    title,
    message,
    confirmLabel = 'Confirm',
    cancelLabel = 'Cancel',
    danger = false,
    onConfirm,
    onCancel
  }: Props = $props();

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onCancel();
  }

  function handleBackdropClick(e: MouseEvent) {
    if (e.target === e.currentTarget) onCancel();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Clicking the backdrop cancels; Escape does the same. -->
<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-black/40"
  use:pointerClick={handleBackdropClick}
>
  <div
    class="mx-4 w-full max-w-sm rounded-lg border border-border bg-surface p-5 shadow-xl"
    role="dialog"
    aria-modal="true"
    aria-labelledby="confirm-modal-title"
    aria-describedby="confirm-modal-message"
  >
    <h2 id="confirm-modal-title" class="mb-2 text-base font-semibold text-text">{title}</h2>
    <p id="confirm-modal-message" class="mb-5 text-sm text-text-muted">{message}</p>
    <div class="flex justify-end gap-2">
      <button
        class="cursor-pointer rounded-md border border-border px-3 py-1.5 text-sm font-medium text-text-muted transition-colors hover:bg-surface-alt"
        onclick={onCancel}
      >
        {cancelLabel}
      </button>
      <button
        class={[
          'cursor-pointer rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
          danger
            ? 'bg-danger text-white hover:opacity-90'
            : 'bg-accent text-accent-text hover:opacity-90'
        ]}
        onclick={onConfirm}
      >
        {confirmLabel}
      </button>
    </div>
  </div>
</div>
