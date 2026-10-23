<script lang="ts">
  import { onMount } from 'svelte';
  import { runBench } from './bench/runBench';
  import { runSelfTest } from './bench/selfTest';
  import { benchApi, documentApi, sheetApi, type SheetInfo } from './bridge/sheet';
  import Sheet from './components/Sheet.svelte';
  import StatusBar from './components/StatusBar.svelte';
  import { cellAddress } from './grid/layout';
  import { single, type CellPos } from './grid/selection';
  import type { SheetMode } from './input/keymap';

  const SHEET = 0;

  let info = $state<SheetInfo>();
  let sheetView = $state<ReturnType<typeof Sheet>>();
  let mode = $state<SheetMode>('ready');
  let formulaText = $state('');
  let activeCell = $state<CellPos>({ row: 0, col: 0 });
  let sheetName = $state('');
  let benchBusy = $state(false);
  let report = $state('');

  let grid = $derived(sheetView?.getGrid());

  async function bench(): Promise<string> {
    const g = sheetView?.getGrid();
    if (!g || !info) return '';
    benchBusy = true;
    try {
      const data = `${info.usedRows.toLocaleString()} used rows × ${info.usedCols.toLocaleString()} used cols（含邊界標記）`;
      return await runBench(g, g.cache, data);
    } finally {
      benchBusy = false;
      sheetView?.focus();
    }
  }

  async function onBench(): Promise<void> {
    report = await bench();
  }

  const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));

  // Visual checks for bench mode: top-left with a selection, and the
  // bottom-right corner of the grid.
  async function saveBenchSnapshots(): Promise<void> {
    const g = sheetView?.getGrid();
    if (!g || !sheetView) return;
    sheetView.setSelectionForTest(single({ row: 2, col: 1 }));
    g.scrollToOrigin();
    await wait(300);
    await benchApi.snapshot('snapshot-origin', g.snapshot());

    const { width, height } = g.contentSize();
    sheetView.setSelectionForTest(single({ row: 1_048_575, col: 16_383 }));
    await g.runScrollBench(1, () => ({ x: width, y: height }));
    await wait(200);
    await g.runScrollBench(1, () => ({ x: width, y: height }));
    await benchApi.snapshot('snapshot-end', g.snapshot());
    sheetView.setSelectionForTest(single({ row: 0, col: 0 }));
    g.scrollToOrigin();
  }

  onMount(() => {
    const offs = [
      documentApi.onMenu((action) => void sheetView?.runMenu(action)),
      documentApi.onState((st) => (sheetName = st.name)),
    ];

    void (async () => {
      info = await sheetApi.info(SHEET);
      sheetName = (await documentApi.state()).name;

      if (await benchApi.benchMode()) {
        await wait(1000); // let the first frames settle
        const md = await bench();
        await saveBenchSnapshots();
        await benchApi.report(md);
      } else if (await benchApi.selfTestMode()) {
        await wait(500);
        // Watchdog: a hung step must still produce a report and quit.
        const timeout = wait(60_000).then(() => ({ markdown: '# v0.2 Self-test\n\n❌ 逾時（60 秒）：有步驟沒有完成。\n', failed: 1 }));
        const result = await Promise.race([runSelfTest(sheetView!, SHEET), timeout]);
        const g = sheetView!.getGrid()!;
        await benchApi.snapshot('selftest', g.snapshot());
        await benchApi.report(result.markdown);
      }
    })();

    return () => offs.forEach((off) => off());
  });
</script>

<main class="app">
  <header class="formula-bar">
    <div class="name-box">{cellAddress(activeCell.row, activeCell.col)}</div>
    <div class="fx">fx</div>
    <!-- Read-only until formula-bar editing arrives with v0.3 point mode. -->
    <input class="formula" value={formulaText} readonly tabindex="-1" onfocus={() => sheetView?.focus()} />
  </header>

  {#if info}
    <Sheet bind:this={sheetView} sheet={SHEET} {info} bind:mode bind:formulaText bind:activeCell />
    <StatusBar {sheetName} {mode} cache={grid?.cache} drawTimes={grid?.drawTimes} {benchBusy} onbench={onBench} />
  {/if}

  {#if report}
    <div class="report">
      <div class="report-head">
        <strong>效能量測結果</strong>
        <button onclick={() => navigator.clipboard.writeText(report)}>複製 Markdown</button>
        <button
          onclick={() => {
            report = '';
            sheetView?.focus();
          }}>關閉</button
        >
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
