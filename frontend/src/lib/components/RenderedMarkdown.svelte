<script lang="ts">
  import { itemLinks } from '#lib/actions/itemLinks.js';
  import { renderMarkdown } from '#lib/markdown.js';

  interface Props {
    content: string;
    class?: string;
  }

  let { content, class: className }: Props = $props();

  let renderedHtml = $state('');

  $effect(() => {
    if (content) {
      renderMarkdown(content).then((html) => {
        renderedHtml = html;
      });
    } else {
      renderedHtml = '';
    }
  });
</script>

{#if renderedHtml}
  <div class={className} use:itemLinks>
    {@html renderedHtml}
  </div>
{/if}
