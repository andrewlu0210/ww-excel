<script lang="ts">
  import { onMount } from 'svelte';
  import { runBench } from './bench/runBench';
  import { benchApi, sheetApi, type SheetInfo } from './bridge/sheet';
  import Grid from './components/Grid.svelte';
  import StatusBar from './components/StatusBar.svelte';
  import { cellAddress } from './grid/layout';
  import type { CellPos } from './grid/renderer';

  const SHEET = 0;

  let info = $state<SheetInfo>();
  let active = $state<CellPos>({ row: 0, col: 0 });
  let grid = $state<ReturnType<typeof Grid>>();
  let benchBusy = $state(false);
  let report = $state('');

  let activeText = $derived(grid?.cache.cell(active.row, active.col)?.text ?? '');

  async function bench(): Promise<string> {
    if (!grid || !info) return '';
    benchBusy = true;
    try {
      const data = `${info.usedRows.toLocaleString()} used rows × ${info.usedCols.toLocaleString()} used cols（含邊界標記）`;
      return await runBench(grid, grid.cache, data);
    } finally {
      benchBusy = false;
    }
  }

  async function onBench(): Promise<void> {
    report = await bench();
  }

  const nextFrames = (n: number) =>
    new Promise<void>((resolve) => {
      const step = () => (--n <= 0 ? resolve() : requestAnimationFrame(step));
      requestAnimationFrame(step);
    });

  // Visual checks for bench mode: top-left with a selection, and the
  // bottom-right corner of the grid.
  async function saveSnapshots(): Promise<void> {
    if (!grid) return;
    active = { row: 2, col: 1 };
    await new Promise((r) => setTimeout(r, 200));
    await nextFrames(2);
    await benchApi.snapshot('snapshot-origin', grid.snapshot());

    const { width, height } = grid.contentSize();
    active = { row: 1_048_575, col: 16_383 };
    await grid.runScrollBench(1, () => ({ x: width, y: height }));
    await new Promise((r) => setTimeout(r, 200));
    await grid.runScrollBench(1, () => ({ x: width, y: height }));
    await benchApi.snapshot('snapshot-end', grid.snapshot());
    active = { row: 0, col: 0 };
    grid.scrollToOrigin();
  }

  onMount(async () => {
    info = await sheetApi.info(SHEET);
    if (await benchApi.benchMode()) {
      // Let the first frames settle before measuring.
      await new Promise((r) => setTimeout(r, 1000));
      const md = await bench();
      await saveSnapshots();
      await benchApi.report(md);
    }
  });
</script>

<main class="app">
  <header class="formula-bar">
    <div class="name-box">{cellAddress(active.row, active.col)}</div>
    <div class="fx">fx</div>
    <input class="formula" value={activeText} readonly placeholder="v0.2 起可編輯" />
  </header>

  {#if info}
    <Grid bind:this={grid} sheet={SHEET} {info} {active} onselect={(pos) => (active = pos)} />
    <StatusBar sheetName={info.name} cache={grid?.cache} drawTimes={grid?.drawTimes} {benchBusy} onbench={onBench} />
  {/if}

  {#if report}
    <div class="report">
      <div class="report-head">
        <strong>效能量測結果</strong>
        <button onclick={() => navigator.clipboard.writeText(report)}>複製 Markdown</button>
        <button onclick={() => (report = '')}>關閉</button>
      </div>
      <pre>{report}</pre>
    </div>
  {/if}
</main>

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .formula-bar {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 32px;
    padding: 0 8px;
    background: var(--chrome-bg);
    border-bottom: 1px solid var(--chrome-border);
  }
  .name-box {
    width: 90px;
    padding: 3px 6px;
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
    font-variant-numeric: tabular-nums;
  }
  .fx {
    color: var(--chrome-muted);
    font-style: italic;
    padding: 0 4px;
  }
  .formula {
    flex: 1;
    font: inherit;
    padding: 3px 6px;
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
    background: var(--grid-bg);
    color: var(--chrome-text);
  }
  .report {
    position: absolute;
    right: 16px;
    bottom: 40px;
    width: min(760px, calc(100% - 32px));
    max-height: 70%;
    display: flex;
    flex-direction: column;
    background: var(--chrome-bg);
    border: 1px solid var(--chrome-border);
    border-radius: 8px;
    box-shadow: 0 8px 30px rgb(0 0 0 / 0.2);
  }
  .report-head {
    display: flex;
    gap: 8px;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--chrome-border);
  }
  .report-head strong {
    flex: 1;
  }
  .report pre {
    margin: 0;
    padding: 12px;
    overflow: auto;
    font: 11px ui-monospace, Menlo, monospace;
    -webkit-user-select: text;
    user-select: text;
  }
</style>
