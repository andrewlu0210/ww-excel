<script lang="ts">
  import type { Snippet } from 'svelte';

  let { children, onclose }: { children: Snippet; onclose: () => void } = $props();

  // Render outside the scrolling ribbon so palettes can extend over the sheet.
  function portal(node: HTMLDivElement) {
    const anchor = node.parentElement!;
    document.body.appendChild(node);
    const bounds = anchor.getBoundingClientRect();
    const popup = node.getBoundingClientRect();
    node.style.left = `${Math.max(4, Math.min(bounds.left, window.innerWidth - popup.width - 4))}px`;
    node.style.top = `${Math.max(4, Math.min(bounds.bottom + 4, window.innerHeight - popup.height - 4))}px`;
    const scroll = (event: Event) => {
      // Scrolling within the popup must not dismiss it.
      if (event.target instanceof Element && event.target.contains(anchor)) onclose();
    };
    const close = () => onclose();
    const keydown = (event: KeyboardEvent) => { if (event.key === 'Escape') close(); };
    document.addEventListener('scroll', scroll, true);
    window.addEventListener('resize', close);
    window.addEventListener('keydown', keydown);
    return { destroy() {
      document.removeEventListener('scroll', scroll, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('keydown', keydown);
      node.remove();
    } };
  }
</script>

<div class="toolbar-popup menu-host" use:portal>{@render children()}</div>

<style>
  .toolbar-popup { position:fixed; z-index:100; width:max-content; max-width:calc(100vw - 8px); max-height:calc(100vh - 8px); overflow:auto; color:var(--chrome-text); font-size:12px; }
  .toolbar-popup :global(.picker) { position:static; margin:0; }
</style>
