<script lang="ts">
  import { onMount } from 'svelte';
  import { FpsMeter, type Samples } from '../grid/perf';
  import type { SheetMode } from '../input/keymap';
  import type { BlockCache } from '../stores/viewportCache';

  interface Props {
    sheetName: string;
    mode: SheetMode;
    cache?: BlockCache;
    drawTimes?: Samples;
    benchBusy: boolean;
    onbench: () => void;
  }

  let { sheetName, mode, cache, drawTimes, benchBusy, onbench }: Props = $props();

  const MODE_LABEL: Record<SheetMode, string> = { ready: '就緒', enter: '輸入', edit: '編輯' };

  let fps = $state(0);
  let drawP95 = $state(0);
  let fetchMs = $state(0);
  let blocks = $state(0);

  onMount(() => {
    const meter = new FpsMeter();
    meter.start();
    const timer = setInterval(() => {
      fps = meter.fps;
      drawP95 = drawTimes?.quantile(0.95) ?? 0;
      const last = cache?.fetches.at(-1);
      fetchMs = last?.ms ?? 0;
      blocks = cache?.size ?? 0;
    }, 500);
    return () => {
      meter.stop();
      clearInterval(timer);
    };
  });
</script>

<footer class="status">
  <div class="left">
    <span class="mode">{MODE_LABEL[mode]}</span>
    <span class="tab active">{sheetName}</span>
  </div>
  <div class="hud">
    <span title="Frames per second">{fps.toFixed(0)} fps</span>
    <span title="Grid draw time, p95">draw {drawP95.toFixed(1)} ms</span>
    <span title="Last GetBlocks round trip">fetch {fetchMs.toFixed(1)} ms</span>
    <span title="Blocks cached in the frontend">{blocks} blocks</span>
    <button tabindex="-1" onclick={onbench} disabled={benchBusy}>{benchBusy ? '量測中…' : '效能量測'}</button>
  </div>
</footer>

<style>
  .status {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 26px;
    padding: 0 8px;
    background: var(--chrome-bg);
    border-top: 1px solid var(--chrome-border);
    font-size: 11px;
    color: var(--chrome-muted);
  }
  .left {
    display: flex;
    align-items: center;
    height: 100%;
    gap: 8px;
  }
  .mode {
    min-width: 32px;
    color: var(--chrome-text);
  }
  .tab {
    display: flex;
    align-items: center;
    padding: 0 14px;
    color: var(--chrome-text);
  }
  .tab.active {
    color: var(--accent);
    font-weight: 600;
    border-bottom: 2px solid var(--accent);
  }
  .hud {
    display: flex;
    gap: 12px;
    align-items: center;
    font-variant-numeric: tabular-nums;
  }
  button {
    font: inherit;
    padding: 1px 8px;
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
    background: var(--app-bg);
    color: var(--chrome-text);
  }
</style>
