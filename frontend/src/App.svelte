<script lang="ts">
  import { t, language } from './i18n';
  import { languageSelfTest } from './bench/languageSelfTest';
  import { onMount, tick } from 'svelte';
  import { runBench } from './bench/runBench';
  import { runSelfTest } from './bench/selfTest';
  import { workbookCompatibilityTest } from './bench/workbookCompatibilityTest';
  import {
    benchApi,
    documentApi,
    sheetApi,
    type ChartDTO,
    type DocState,
    type PageSetupDTO,
    type FindOptionsDTO,
    type FormatPatch,
    type HitDTO,
    type RangeDTO,
    type SheetInfo,
    type StyleDTO,
    type ValidationDTO,
  } from './bridge/sheet';
  import ChartDialog from './components/ChartDialog.svelte';
  import CondFormatDialog from './components/CondFormatDialog.svelte';
  import PageSetupDialog from './components/PageSetupDialog.svelte';
  import PrintPreview from './components/PrintPreview.svelte';
  import FindPanel from './components/FindPanel.svelte';
  import NameManager from './components/NameManager.svelte';
  import TableDialog from './components/TableDialog.svelte';
  import ValidationDialog from './components/ValidationDialog.svelte';
  import { absoluteRef } from './formula/refs';
  import NumberFormatDialog from './components/NumberFormatDialog.svelte';
  import PasteSpecialDialog from './components/PasteSpecialDialog.svelte';
  import ShiftDialog from './components/ShiftDialog.svelte';
  import PreferencesDialog from './components/PreferencesDialog.svelte';
  import { settings } from './stores/settings';
  import SortDialog from './components/SortDialog.svelte';
  import Sheet from './components/Sheet.svelte';
  import SheetTabs from './components/SheetTabs.svelte';
  import StatusBar from './components/StatusBar.svelte';
  import ZoomDialog from './components/ZoomDialog.svelte';
  import { normalizeZoom, SheetZooms } from './grid/zoom';
  import Toolbar from './components/Toolbar.svelte';
  import Ribbon from './components/Ribbon.svelte';
  import { current, fromRange, single, type CrossEdit, type Range, type Selection } from './grid/selection';
  import type { MenuAction, SheetMode } from './input/keymap';
  import { styleStore } from './stores/styles';

  // Bench and self-test always run on the first sheet.
  const SHEET = 0;

  let info = $state<SheetInfo>();
  let sheets = $state<string[]>([]);
  let active = $state(0);
  let hiddenSheets = $state<number[]>([]);
  // Bumped when the sheet list or document changes: remounts the sheet
  // view, whose block cache is keyed by sheet index.
  let generation = $state(0);
  // Remembered selection per sheet name, restored when switching back.
  const selections = new Map<string, Selection>();
  const zooms = new SheetZooms();
  let zoom = $state(100);
  let zoomDialog = $state(false);
  let paintMode = $state<'off' | 'once' | 'repeat'>('off');
  let paintSource = $state<{ name: string; range: Range } | null>(null);
  let paintRequest = 0;

  function cancelPainter(): void {
    paintRequest++;
    paintMode = 'off';
    paintSource = null;
  }

  async function startPainter(repeat: boolean): Promise<void> {
    if (!repeat && paintMode !== 'off') { cancelPainter(); return; }
    if (repeat && paintMode !== 'off') { paintMode = 'repeat'; return; }
    const request = ++paintRequest;
    const source = await sheetView?.captureFormats();
    if (request !== paintRequest || !source) return;
    paintSource = { name: sheets[active], range: source };
    paintMode = repeat ? 'repeat' : 'once';
  }

  function changeZoom(value: number): void {
    zoom = normalizeZoom(value);
    zooms.set(sheets[active], zoom);
  }

  let sheetView = $state<ReturnType<typeof Sheet>>();
  let mode = $state<SheetMode>('ready');
  let activeStyle = $state<StyleDTO | null>(null);
  let activeMerged = $state(false);
  let canUndo = $state(false);
  let canRedo = $state(false);
  let circular = $state('');
  let numberDialog = $state(false);
  let findPanel = $state<{ replace: boolean } | null>(null);
  let sortDialog = $state<{ range: RangeDTO; header: boolean } | null>(null);
  let pasteDialog = $state(false);
  let prefsDialog = $state(false);
  let shiftDialog = $state<{ insert: boolean } | null>(null);
  // v0.6 dialogs.
  let names = $state<string[]>([]);
  // v0.7 dialogs.
  let chartDialog = $state<{ editing: boolean; initial: ChartDTO; id: number; at: { row: number; col: number } } | null>(null);
  let pageSetupDialog = $state<{ initial: PageSetupDTO; reopenPreview: boolean } | null>(null);
  let preview = $state<{ url: string; pages: number } | null>(null);
  // A formula being edited while visiting other sheets.
  let crossEdit = $state<CrossEdit | null>(null);
  let nameDialog = $state<{ startNew: boolean; ref: string } | null>(null);
  let validationDialog = $state<{ current: ValidationDTO | null } | null>(null);
  let cfDialog = $state<{ manage: boolean } | null>(null);
  let tableDialog = $state<
    { range: RangeDTO; header: boolean; styles: string[] } | { renaming: string } | null
  >(null);
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
    const requested = Math.max(0, Math.min(index, sheets.length - 1));
    const i = hiddenSheets.includes(requested) ? sheets.findIndex((_, i) => !hiddenSheets.includes(i)) : requested;
    if (i < 0) return;
    info = await sheetApi.info(i);
    active = i;
    zoom = zooms.get(sheets[i], info.zoom || 100);
    generation++;
  }

  function onStructure(st: DocState): void {
    cancelPainter();
    const current = sheets[active];
    zooms.reconcile(sheets, st.sheets ?? []);
    sheets = st.sheets ?? [];
    hiddenSheets = st.hiddenSheets ?? [];
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
    cancelPainter();
    selections.clear();
    zooms.clear();
    styleStore.reset();
    sheets = st.sheets ?? [];
    hiddenSheets = st.hiddenSheets ?? [];
    await showSheet(0);
  }

  function onAppAction(action: MenuAction): void {
    switch (action) {
      case 'insertSheet':
        void addSheet();
        break;
      case 'formatCells':
        numberDialog = true;
        break;
      case 'find':
      case 'replace':
        findPanel = { replace: action === 'replace' };
        break;
      case 'pasteSpecial':
        pasteDialog = true;
        break;
      case 'preferences':
        if (settings()) prefsDialog = true;
        break;
      case 'insertCells':
      case 'deleteCells':
        shiftDialog = { insert: action === 'insertCells' };
        break;
      case 'sort':
        void openSort();
        break;
      case 'defineName':
      case 'nameManager': {
        const r = sheetView ? current(sheetView.getSelection()) : { r0: 0, c0: 0, r1: 0, c1: 0 };
        nameDialog = { startNew: action === 'defineName', ref: absoluteRef(sheets[active], r.r0, r.c0, r.r1, r.c1) };
        break;
      }
      case 'dataValidation':
        void openValidation();
        break;
      case 'condFormat':
      case 'condFormatManager':
        cfDialog = { manage: action === 'condFormatManager' };
        break;
      case 'insertTable':
        void openTable();
        break;
      case 'insertChart':
        void openChart();
        break;
      case 'editChart': {
        const id = sheetView?.getSelectedObject();
        const o = sheetView?.getObjects().find((x) => x.id === id);
        if (o?.chart) chartDialog = { editing: true, initial: o.chart, id: o.id, at: { row: o.row, col: o.col } };
        break;
      }
      case 'pageSetup':
        void openPageSetup(false);
        break;
      case 'printPreview':
        void openPreview();
        break;
      case 'print':
        void printSheet();
        break;
      case 'exportPDF':
        void exportPDF();
        break;
      case 'renameTable': {
        const t = sheetView?.getActiveTable();
        if (t) tableDialog = { renaming: t.name };
        break;
      }
    }
  }

  function selectedRanges(): RangeDTO[] {
    return (sheetView?.getSelection().ranges ?? []).map((r) => ({ r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 }));
  }

  async function openValidation(): Promise<void> {
    const pos = sheetView?.getSelection().active ?? { row: 0, col: 0 };
    validationDialog = { current: await sheetApi.validation(active, pos.row, pos.col) };
  }

  async function openTable(): Promise<void> {
    const sel = sheetView?.getSelection();
    if (!sel) return;
    const r = current(sel);
    const info = await sheetApi.tableRange(active, { r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 });
    if (!info.ok) return notify(t('請選取資料範圍中的儲存格再建立表格。'));
    tableDialog = { range: info.range, header: info.header, styles: await sheetApi.tableStyles() };
  }

  async function openChart(): Promise<void> {
    const sel = sheetView?.getSelection();
    if (!sel) return;
    const r = current(sel);
    const res = await sheetApi.chartFromSelection(active, { r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 }, 'column');
    if (res.error || !res.value) return notify(res.error ?? t('請先選取要做成圖表的資料。'));
    // Excel puts a new chart next to its data.
    chartDialog = { editing: false, initial: res.value, id: 0, at: { row: r.r0, col: r.c1 + 2 } };
  }

  async function suggestChart(type: string): Promise<ChartDTO | null> {
    const sel = sheetView?.getSelection();
    if (!sel) return null;
    const r = current(sel);
    const res = await sheetApi.chartFromSelection(active, { r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 }, type);
    return res.value;
  }

  async function applyChart(chart: ChartDTO): Promise<string | null> {
    const d = chartDialog;
    if (!d) return null;
    let err: string | null;
    if (d.editing) {
      const o = sheetView?.getObjects().find((x) => x.id === d.id);
      if (!o) return t('圖表不存在');
      err = await sheetApi.updateObject(active, { ...o, chart }, t('編輯圖表'));
    } else {
      const res = await sheetApi.addChart(active, d.at.row, d.at.col, 480, 288, chart);
      err = res.error;
      if (!err && res.value) sheetView?.selectObject(res.value);
    }
    if (!err) {
      chartDialog = null;
      sheetView?.focus();
    }
    return err;
  }

  async function openPageSetup(reopenPreview: boolean): Promise<void> {
    pageSetupDialog = { initial: await sheetApi.pageSetup(active), reopenPreview };
  }

  async function openPreview(): Promise<void> {
    const res = await sheetApi.printPreview(active, sheetView?.chartImages() ?? {});
    if (res.error || !res.value) return notify(res.error);
    preview = res.value;
  }

  async function printSheet(): Promise<void> {
    notify(await documentApi.printSheet(active, sheetView?.chartImages() ?? {}));
  }

  async function exportPDF(): Promise<void> {
    const res = await documentApi.exportPDF(active, sheetView?.chartImages() ?? {});
    if (res.error) notify(res.error);
    else if (res.value) notify(t('已匯出 PDF。'));
  }

  // The name box went to another sheet.
  async function goToSheet(index: number, range: Range): Promise<void> {
    if (hiddenSheets.includes(index)) return notify(t('工作表已隱藏'));
    selections.set(sheets[index], fromRange(range));
    await showSheet(index);
  }

  async function openSort(): Promise<void> {
    const res = await sheetView?.sortInfo();
    if (!res?.ok) return notify(t('請選取資料範圍中的儲存格再排序。'));
    sortDialog = { range: res.range, header: res.header };
  }

  // The new sheet view has taken the cross-sheet edit at mount.
  async function handOff(): Promise<void> {
    await tick();
    crossEdit = null;
  }

  function closeTable(): void {
    tableDialog = null;
    sheetView?.focus();
  }

  function closeDialogs(): void {
    sortDialog = null;
    pasteDialog = false;
    shiftDialog = null;
    sheetView?.focus();
  }

  // ---- Find & replace ----

  async function gotoHit(hit: HitDTO): Promise<void> {
    const pos = { row: hit.row, col: hit.col };
    if (hit.sheet !== active) {
      selections.set(sheets[hit.sheet], single(pos));
      await showSheet(hit.sheet);
    } else {
      sheetView?.goTo(pos);
    }
  }

  async function findNext(opt: FindOptionsDTO, forward: boolean): Promise<string | null> {
    const pos = sheetView?.getSelection().active ?? { row: 0, col: 0 };
    const hit = await sheetApi.findNext(active, pos.row, pos.col, opt, forward);
    if (!hit) return t('找不到符合的資料。');
    await gotoHit(hit);
    return null;
  }

  async function replace(opt: FindOptionsDTO, withText: string, all: boolean): Promise<string | null> {
    await sheetView?.whenIdle();
    const pos = sheetView?.getSelection().active ?? { row: 0, col: 0 };
    const res = await sheetApi.replace(active, opt, withText, all, pos.row, pos.col);
    if (res.error) return res.error;
    if (all) return res.value ? t('已完成取代 %d 項。', res.value) : t('找不到符合的資料。');
    // 取代: replace the current match, then go to the next one.
    const next = await findNext(opt, true);
    return res.value ? null : next;
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
        if (['preferences', 'insertSheet', 'formatCells', 'sort', 'defineName', 'nameManager', 'dataValidation', 'condFormat', 'condFormatManager', 'insertTable'].includes(action))
          onAppAction(action);
        else void sheetView?.runMenu(action);
      }),
      documentApi.onState((st) => {
        canUndo = st.canUndo; canRedo = st.canRedo;
        circular = st.circular;
        names = st.names ?? [];
      }),
      documentApi.onSheetsChanged(onStructure),
      documentApi.onReset((st) => void onReset(st)),
    ];

    void (async () => {
      const st = await documentApi.state();
      canUndo = st.canUndo; canRedo = st.canRedo;
      sheets = st.sheets ?? [];
      hiddenSheets = st.hiddenSheets ?? [];
      circular = st.circular;
      names = st.names ?? [];
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
        // A run takes about 45s; the first run after a build is slower.
        const timeout = wait(120_000).then(() => ({ markdown: '# Self-test\n\n❌ 逾時（120 秒）：有步驟沒有完成。\n', failed: 1 }));
        // Sheet operations remount the sheet view; the driver always
        // talks to the current one.
        const driver = new Proxy({} as NonNullable<typeof sheetView>, {
          get: (_, key) => {
            const v = sheetView as unknown as Record<string | symbol, unknown>;
            const member = v[key];
            return typeof member === 'function' ? member.bind(v) : member;
          },
        });
        const fixture = await benchApi.fixturePath();
        const result = await Promise.race([fixture ? workbookCompatibilityTest(driver, fixture) : language() !== 'zh-TW' ? languageSelfTest(() => onAppAction('preferences')) : runSelfTest(driver, SHEET), timeout]);
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
    <Ribbon {canUndo} {canRedo} onaction={(action) => {
      if (action === 'preferences') onAppAction(action);
      else void sheetView?.runMenu(action);
    }}>
    <Toolbar
      onaction={(action) => {
        void sheetView?.runMenu(action);
      }}
      painting={paintMode !== 'off'}
      onpaint={(repeat) => void startPainter(repeat)}
      style={activeStyle}
      merged={activeMerged}
      onformat={(p) => void format(p)}
      onmerge={(center) => void sheetView?.mergeCells(center).then(notify)}
      onclear={(m) => void sheetView?.clearRanges(m)}
      onnumberdialog={() => (numberDialog = true)}
    />
    </Ribbon>
    {#key generation}
      <Sheet
        bind:this={sheetView}
        sheet={active}
        {zoom}
        {info}
        {paintMode}
        paintSource={paintSource?.name === sheets[active] ? paintSource.range : null}
        oncancelpaint={cancelPainter}
        onpainted={() => { if (paintMode === 'once') cancelPainter(); }}
        initialSelection={selections.get(sheets[active])}
        bind:mode
        bind:activeStyle
        bind:activeMerged
        onselectionchange={(sel) => selections.set(sheets[active], sel)}
        onappaction={onAppAction}
        onnotify={notify}
        sheetName={sheets[active]}
        {names}
        ongoto={(i, r) => void goToSheet(i, r)}
        {crossEdit}
        oncrossdone={(origin, pos, retry) => {
          crossEdit = retry ? { originSheet: origin, pos, text: retry, caret: retry.length } : null;
          selections.set(sheets[origin], single(pos));
          void showSheet(origin).then(handOff);
        }}
      />
    {/key}
    <StatusBar {zoom} onzoom={changeZoom} onzoomdialog={() => (zoomDialog = true)} {mode} {circular} cache={grid?.cache} drawTimes={grid?.drawTimes} {benchBusy} onbench={onBench}>
      {#snippet tabs()}
        <SheetTabs
          {sheets}
          hidden={hiddenSheets}
          {active}
          onselect={(i) => {
            if (i === active) return;
            // Editing a formula: keep editing it on the other sheet.
            crossEdit = sheetView?.formulaState() ?? null;
            void showSheet(i).then(handOff);
          }}
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

  {#if zoomDialog}
    <ZoomDialog {zoom}
      onapply={async (value) => {
        const g = sheetView?.getGrid();
        changeZoom(value ?? g?.zoomForSelection() ?? zoom);
        zoomDialog = false;
        await tick();
        if (value === null) g?.revealSelection();
        sheetView?.focus();
      }}
      onclose={() => {
        zoomDialog = false;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if numberDialog}
    <NumberFormatDialog
      code={activeStyle?.numFmt ?? ''}
      preview={previewFormat}
      onapply={(code) => {
        numberDialog = false;
        void format({ numFmt: code, label: t('數值格式') });
      }}
      onclose={() => {
        numberDialog = false;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if findPanel}
    <FindPanel
      bind:replaceMode={findPanel.replace}
      {sheets}
      onfind={findNext}
      onfindall={(opt) => sheetApi.findAll(active, opt)}
      onreplace={replace}
      ongoto={(h) => void gotoHit(h)}
      onclose={() => {
        findPanel = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if sortDialog}
    <SortDialog
      range={sortDialog.range}
      header={sortDialog.header}
      columns={(header) => sheetView?.columnLabels(sortDialog!.range, header) ?? []}
      onapply={(rg, keys, header) => {
        sortDialog = null;
        void sheetView?.sortRange(rg, keys, header);
      }}
      onclose={closeDialogs}
    />
  {/if}

  {#if pasteDialog}
    <PasteSpecialDialog
      onapply={(mode, transpose, skip) => {
        pasteDialog = false;
        void sheetView?.pasteSpecial(mode, transpose, skip);
      }}
      onclose={closeDialogs}
    />
  {/if}

  {#if prefsDialog && settings()}
    <PreferencesDialog initial={settings()!} onclose={() => (prefsDialog = false)} />
  {/if}
  {#if shiftDialog}
    <ShiftDialog
      insert={shiftDialog.insert}
      onapply={(rows) => {
        const insert = shiftDialog!.insert;
        shiftDialog = null;
        void sheetView?.shiftSelection(rows, insert).then(() => sheetView?.focus());
      }}
      onclose={closeDialogs}
    />
  {/if}

  {#if nameDialog}
    <NameManager
      {sheets}
      selectionRef={nameDialog.ref}
      startNew={nameDialog.startNew}
      onclose={() => {
        nameDialog = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if validationDialog}
    <ValidationDialog
      current={validationDialog.current}
      onapply={async (v) => {
        const err = await sheetApi.setValidation(active, selectedRanges(), v);
        if (!err) {
          validationDialog = null;
          sheetView?.focus();
        }
        return err;
      }}
      onclose={() => {
        validationDialog = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if cfDialog}
    <CondFormatDialog
      manage={cfDialog.manage}
      rules={() => sheetApi.condFormats(active)}
      onadd={(rule) => sheetApi.addCondFormat(active, selectedRanges(), rule)}
      ondelete={(i) => sheetApi.deleteCondFormat(active, i)}
      onclose={() => {
        cfDialog = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if tableDialog}
    {#if 'renaming' in tableDialog}
      <TableDialog
        renaming={tableDialog.renaming}
        onrename={async (name) => {
          const old = tableDialog && 'renaming' in tableDialog ? tableDialog.renaming : '';
          const err = await sheetApi.renameTable(active, old, name);
          if (!err) closeTable();
          return err;
        }}
        onclose={closeTable}
      />
    {:else}
      <TableDialog
        range={tableDialog.range}
        header={tableDialog.header}
        styles={tableDialog.styles}
        oncreate={async (header, style) => {
          const rg = tableDialog && 'range' in tableDialog ? tableDialog.range : { r0: 0, c0: 0, r1: 0, c1: 0 };
          const res = await sheetApi.createTable(active, rg, header, style);
          if (!res.error) closeTable();
          return res.error;
        }}
        onclose={closeTable}
      />
    {/if}
  {/if}

  {#if chartDialog}
    <ChartDialog
      editing={chartDialog.editing}
      initial={chartDialog.initial}
      suggest={suggestChart}
      preview={(c) => sheetApi.chartPreview(active, c)}
      onapply={applyChart}
      onclose={() => {
        chartDialog = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if pageSetupDialog}
    <PageSetupDialog
      initial={pageSetupDialog.initial}
      selection={(() => {
        const r = sheetView ? current(sheetView.getSelection()) : { r0: 0, c0: 0, r1: 0, c1: 0 };
        return { r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 };
      })()}
      onapply={async (p) => {
        const err = await sheetApi.setPageSetup(active, p);
        if (!err) {
          const reopen = pageSetupDialog?.reopenPreview;
          pageSetupDialog = null;
          if (reopen) await openPreview();
          else sheetView?.focus();
        }
        return err;
      }}
      onclose={() => {
        const reopen = pageSetupDialog?.reopenPreview;
        pageSetupDialog = null;
        if (reopen) void openPreview();
        else sheetView?.focus();
      }}
    />
  {/if}

  {#if preview}
    <PrintPreview
      url={preview.url}
      pages={preview.pages}
      onprint={() => {
        preview = null;
        void printSheet();
      }}
      onexport={() => void exportPDF()}
      onsetup={() => {
        preview = null;
        void openPageSetup(true);
      }}
      onclose={() => {
        preview = null;
        sheetView?.focus();
      }}
    />
  {/if}

  {#if report}
    <div class="report">
      <div class="report-head">
        <strong>{t('效能量測結果')}</strong>
        <button onclick={() => navigator.clipboard.writeText(report)}>{t('複製 Markdown')}</button>
        <button
          onclick={() => {
            report = '';
            sheetView?.focus();
          }}>{t('關閉')}</button
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
