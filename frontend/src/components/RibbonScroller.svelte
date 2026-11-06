<script lang="ts">
  import { onMount, type Snippet } from 'svelte';
  import { t } from '../i18n';

  let { children }: { children: Snippet } = $props();
  let viewport: HTMLDivElement;
  let content: HTMLDivElement;
  let left = $state(false);
  let right = $state(false);

  function update(): void {
    left = viewport.scrollLeft > 1;
    right = viewport.scrollLeft + viewport.clientWidth < viewport.scrollWidth - 1;
  }

  function scroll(direction: number): void {
    viewport.scrollBy({ left: direction * Math.max(120, viewport.clientWidth * 0.7), behavior: 'smooth' });
  }

  onMount(() => {
    const observer = new ResizeObserver(update);
    observer.observe(viewport);
    observer.observe(content);
    const wheel = (event: WheelEvent) => {
      if (Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const before = viewport.scrollLeft;
      const delta = event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? viewport.clientWidth : 1);
      viewport.scrollLeft += delta;
      if (viewport.scrollLeft !== before) event.preventDefault();
    };
    viewport.addEventListener('wheel', wheel, { passive: false });
    update();
    return () => {
      observer.disconnect();
      viewport.removeEventListener('wheel', wheel);
    };
  });
</script>

<div class="ribbon-scroller">
  <div class="ribbon-viewport" bind:this={viewport} onscroll={update}>
    <div class="ribbon-content" bind:this={content}>{@render children()}</div>
  </div>
  {#if left}
    <button class="scroll-arrow left" aria-label={t('向左')} title={t('向左')}
      onpointerdown={(e) => e.preventDefault()} onclick={() => scroll(-1)}>‹</button>
  {/if}
  {#if right}
    <button class="scroll-arrow right" aria-label={t('向右')} title={t('向右')}
      onpointerdown={(e) => e.preventDefault()} onclick={() => scroll(1)}>›</button>
  {/if}
</div>

<style>
  .ribbon-scroller { isolation:isolate; position:relative; min-width:0; height:88px; }
  .ribbon-viewport { position:relative; z-index:0; height:100%; overflow-x:auto; overflow-y:hidden; scrollbar-width:none; }
  .ribbon-viewport::-webkit-scrollbar { display:none; }
  .ribbon-content { width:max-content; min-width:100%; height:100%; padding:0 24px; box-sizing:border-box; }
  .scroll-arrow { position:absolute; z-index:1; top:0; bottom:0; width:24px; border:0; padding:0; border-radius:0; font:26px/1 system-ui; color:var(--chrome-text); background:var(--chrome-bg); }
  .scroll-arrow:hover { background:var(--popup-active); }
  .scroll-arrow:focus-visible { outline:2px solid var(--accent); outline-offset:-2px; }
  .left { left:0; box-shadow:4px 0 6px rgb(0 0 0 / .12); }
  .right { right:0; box-shadow:-4px 0 6px rgb(0 0 0 / .12); }
</style>
