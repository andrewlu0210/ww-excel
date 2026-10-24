<script lang="ts">
  import { onMount } from 'svelte';
  import { runBench } from './bench/runBench';
  import { runSelfTest } from './bench/selfTest';
  import { benchApi, documentApi, sheetApi, type DocState, type FormatPatch, type SheetInfo, type StyleDTO } from './bridge/sheet';
  import NumberFormatDialog from './components/NumberFormatDialog.svelte';
  import Sheet from './components/Sheet.svelte';
  import SheetTabs from './components/SheetTabs.svelte';
  import StatusBar from './components/StatusBar.svelte';
  import Toolbar from './components/Toolbar.svelte';
  import { single, type Selection } from './grid/selection';
  import type { MenuAction, SheetMode } from './input/keymap';
  import { styleStore } from './stores/styles';

  // Bench and self-test always run on the first sheet.
  const SHEET = 0;

  let info = $state<SheetInfo>();
  let sheets = $state<string[]>([]);
  let active = $state(0);
  // Bumped when the sheet list or document changes: remounts the sheet
  // view, whose block cache is keyed by sheet index.
  let generation = $state(0);
  // Remembered selection per sheet name, restored when switching back.
  const selections = new Map<string, Selection>();

  let sheetView = $state<ReturnType<typeof Sheet>>();
  let mode = $state<SheetMode>('ready');
  let activeStyle = $state<StyleDTO | null>(null);
  let activeMerged = $state(false);
  let circular = $state('');
  let numberDialog = $state(false);
  // A short message, e.g. why a merge was refused.
  let toast = $state<string | null>(null);
  let toastTimer: ReturnType<typeof setTimeout> | undefined;
  // Sheet to show after the next sheet-list change (set before adding a
  // sheet: the event and the call's result arrive in either order).
  let preferIndex: number | null = null;
  let benchBusy = $state(false);
  let report = $state('');

  let grid = $derived(sheetView?.getGrid());

  async function showSheet(index: number): Promise<void> {
    const i = Math.max(0, Math.min(index, sheets.length - 1));
    info = await sheetApi.info(i);
    active = i;
    generation++;
  }

  function onStructure(st: DocState): void {
    const current = sheets[active];
    sheets = st.sheets ?? [];
    if (preferIndex !== null) {
      const i = preferIndex;
      preferIndex = null;
      void showSheet(i);
      return;
    }
    const keep = sheets.indexOf(current);
    void showSheet(keep >= 0 ? keep : active);
  }

  function notify(message: string | null | undefined): void {
    if (!message) return;
    toast = message;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (toast = null), 4000);
  }

  async function onReset(st: DocState): Promise<void> {
    selections.clear();
    styleStore.reset();
    sheets = st.sheets ?? [];
    await showSheet(0);
  }

  function onAppAction(action: MenuAction): void {
    if (action === 'insertSheet') void addSheet();
    if (action === 'formatCells') numberDialog = true;
  }

  async function addSheet(): Promise<void> {
    // Go inserts after the active sheet; sheets:changed then shows it.
    preferIndex = Math.min(active + 1, sheets.length);
    await documentApi.addSheet(preferIndex);
  }

  async function format(patch: FormatPatch): Promise<void> {
    notify(await sheetView?.applyFormat(patch));
  }

  async function previewFormat(code: string): Promise<string> {
    const pos = sheetView?.getSelection().active ?? { row: 0, col: 0 };
    return sheetApi.previewFormat(active, pos.row, pos.col, code);
  }

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
      documentApi.onMenu((action) => {
        if (action === 'insertSheet' || action === 'formatCells') onAppAction(action);
        else void sheetView?.runMenu(action);
      }),
      documentApi.onState((st) => (circular = st.circular)),
      documentApi.onSheetsChanged(onStructure),
      documentApi.onReset((st) => void onReset(st)),
    ];

    void (async () => {
      const st = await documentApi.state();
      sheets = st.sheets ?? [];
      circular = st.circular;
      await showSheet(SHEET);

      if (await benchApi.benchMode()) {
        await wait(1000); // let the first frames settle
        const md = await bench();
        await saveBenchSnapshots();
        await benchApi.report(md);
      } else if (await benchApi.selfTestMode()) {
        // Report JS errors right away so a broken step does not look like
        // a hang.
        const fail = (what: string) => void benchApi.report(`# Self-test\n\n❌ JS 錯誤：${what}\n`);
        window.addEventListener('error', (e) => fail(`${e.message} @ ${e.filename}:${e.lineno}`));
        window.addEventListener('unhandledrejection', (e) => fail(String(e.reason?.stack ?? e.reason)));
        await wait(500);
        // Watchdog: a hung step must still produce a report and quit.
        const timeout = wait(60_000).then(() => ({ markdown: '# Self-test\n\n❌ 逾時（60 秒）：有步驟沒有完成。\n', failed: 1 }));
        // Sheet operations remount the sheet view; the driver always
        // talks to the current one.
        const driver = new Proxy({} as NonNullable<typeof sheetView>, {
          get: (_, key) => {
            const v = sheetView as unknown as Record<string | symbol, unknown>;
            const member = v[key];
            return typeof member === 'function' ? member.bind(v) : member;
          },
        });
        const result = await Promise.race([runSelfTest(driver, SHEET), timeout]);
        const g = sheetView!.getGrid()!;
        await benchApi.snapshot('selftest', g.snapshot());
        await benchApi.report(result.markdown);
      }
    })();

    return () => offs.forEach((off) => off());
  });
</script>

<main class="app">
  {#if info}
    <Toolbar
      style={activeStyle}
      merged={activeMerged}
      onformat={(p) => void format(p)}
      onmerge={(center) => void sheetView?.mergeCells(center).then(notify)}
      onclear={(m) => void sheetView?.clearRanges(m)}
      onnumberdialog={() => (numberDialog = true)}
    />
    {#key generation}
      <Sheet
        bind:this={sheetView}
        sheet={active}
        {info}
        initialSelection={selections.get(sheets[active])}
        bind:mode
        bind:activeStyle
        bind:activeMerged
        onselectionchange={(sel) => selections.set(sheets[active], sel)}
        onappaction={onAppAction}
      />
    {/key}
    <StatusBar {mode} {circular} cache={grid?.cache} drawTimes={grid?.drawTimes} {benchBusy} onbench={onBench}>
      {#snippet tabs()}
        <SheetTabs
          {sheets}
          {active}
          onselect={(i) => i !== active && void showSheet(i)}
          onadd={() => void addSheet()}
          onrename={(i, name) => documentApi.renameSheet(i, name)}
          ondelete={(i) => void documentApi.deleteSheet(i)}
          onmove={(from, to) => void documentApi.moveSheet(from, to)}
          ondone={() => sheetView?.focus()}
        />
      {/snippet}
    </StatusBar>
  {/if}

  {#if toast}
    <div class="toast" role="status">{toast}</div>
  {/if}

  {#if numberDialog}
    <NumberFormatDialog
      code={activeStyle?.numFmt ?? ''}
      preview={previewFormat}
      onapply={(code) => {
        numberDialog = false;
        void format({ numFmt: code, label: '數值格式' });
      }}
      onclose={() => {
        numberDialog = false;
        sheetView?.focus();
      }}
    />
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
  .toast {
    position: absolute;
    left: 50%;
    bottom: 48px;
    transform: translateX(-50%);
    z-index: 40;
    padding: 8px 14px;
    color: var(--chrome-text);
    background: var(--popup-bg);
    border: 1px solid var(--error-fg);
    border-radius: 8px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.2);
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
