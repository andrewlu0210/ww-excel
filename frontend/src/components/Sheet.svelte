<script lang="ts">
  // Sheet controller: selection, keyboard, the cell editor, the formula
  // bar and the link to Go. Rendering lives in Grid.
  //
  // The cell editor is a single <textarea> that always holds keyboard
  // focus, invisible while idle and positioned on the active cell. Typing
  // (or an IME composition) therefore starts in the textarea itself, so
  // the first keystroke of 注音/倉頡 is captured and the candidate window
  // appears next to the cell. The formula bar is a second input showing
  // the same text; whichever one has focus is the "active input".
  import { t } from '../i18n';
  import { onMount, untrack } from 'svelte';
  import {
    documentApi,
    sheetApi,
    type CellsChanged,
    type ChartDataDTO,
    type ObjectDTO,
    type CopyResult,
    type FilterValueDTO,
    type FormatPatch,
    type FuncInfo,
    type LayoutDTO,
    type PasteMode,
    type RangeDTO,
    type SortInfo,
    type SortKeyDTO,
    type TableDTO,
    type ValidationDTO,
    type SheetInfo,
    type StyleDTO,
  } from '../bridge/sheet';
  import { absoluteRef, callAt, canPoint, findRefs, quoteSheet, refText, toggleAbsolute, wordAt } from '../formula/refs';
  import { fillCopy, fillTarget, sameRange } from '../grid/fill';
  import { cellAddress, colName } from '../grid/layout';
  import { areaOf, expand, mergeAt, snap, stepStart } from '../grid/merges';
  import { HEADER_H, metrics, REF_COLORS, type Highlight } from '../grid/renderer';
  import { LINE_HEIGHT, TextCache } from '../grid/text';
  import { styleStore } from '../stores/styles';
  import { drawShape } from '../shapes/render';
  import {
    clampPos,
    current,
    cycleActive,
    extendTo,
    contains,
    fromRange,
    isFullCols,
    isFullRows,
    isSingleCell,
    rangeOf,
    selectAll,
    selectCols,
    selectRows,
    single,
    type CellPos,
    type CrossEdit,
    type Range,
    type Selection,
  } from '../grid/selection';
  import {
    arrowCommit,
    editAction,
    enterThen,
    idleAction,
    type Action,
    type EditMode,
    type MenuAction,
    type SheetMode,
  } from '../input/keymap';
  import type { ContextRequest, GridPointer, ResizeRequest } from '../input/pointer';
  import { drawChart, LIGHT_THEME } from '../charts/render';
  import FilterMenu from './FilterMenu.svelte';
  import ObjectLayer, { toChartData } from './ObjectLayer.svelte';
  import Grid from './Grid.svelte';

  interface Props {
    paintMode?: 'off' | 'once' | 'repeat';
    paintSource?: Range | null;
    oncancelpaint?: () => void;
    onpainted?: () => void;
    zoom?: number;
    sheet: number;
    info: SheetInfo;
    initialSelection?: Selection;
    // Outputs for the status bar and the toolbar.
    mode?: SheetMode;
    activeStyle?: StyleDTO | null;
    activeMerged?: boolean;
    onselectionchange?: (sel: Selection) => void;
    // Menu actions handled by the app (sheet tabs, dialogs).
    onappaction?: (action: MenuAction) => void;
    // A message for the user, e.g. why a paste was refused.
    onnotify?: (message: string) => void;
    // v0.6: the sheet's name (for absolute references), the defined
    // names (autocomplete), and going to another sheet from the name box.
    sheetName?: string;
    names?: string[];
    ongoto?: (sheet: number, range: Range) => void;
    // A formula being edited on another sheet (cross-sheet point mode),
    // or one to resume on this sheet. Read once, at mount.
    crossEdit?: CrossEdit | null;
    // The cross-sheet edit ended: committed (retry null) or refused
    // (retry is the text to edit again on the origin sheet).
    oncrossdone?: (origin: number, pos: CellPos, retry: string | null) => void;
  }

  let {
    paintMode = 'off',
    paintSource = null,
    oncancelpaint,
    onpainted,
    zoom = 100,
    sheet,
    info,
    initialSelection,
    mode = $bindable('ready'),
    activeStyle = $bindable(null),
    activeMerged = $bindable(false),
    onselectionchange,
    onappaction,
    onnotify,
    sheetName = '',
    names = [],
    ongoto,
    crossEdit = null,
    oncrossdone,
  }: Props = $props();

  let scale = $derived(zoom / 100);

  // Fixed for the lifetime of the component: the grid is always full size.
  const lim = untrack(() => ({ rows: info.rows, cols: info.cols }));

  let grid = $state<ReturnType<typeof Grid>>();
  let root: HTMLDivElement;
  let editor: HTMLTextAreaElement;
  let bar: HTMLTextAreaElement;
  let selection = $state<Selection>(untrack(() => initialSelection) ?? single({ row: 0, col: 0 }));
  let layout = $state<LayoutDTO | null>(null);
  let merges = $derived((layout?.merges ?? []).map((m) => ({ r0: m.r0, c0: m.c0, r1: m.r1, c1: m.c1 })));
  // Bumped when cells, styles or the layout change, to refresh the
  // toolbar state.
  let styleTick = $state(0);
  let editing = $state<{ mode: EditMode; pos: CellPos } | null>(null);
  let activeInput: 'cell' | 'bar' = 'cell';
  // Mirror of the text being edited, for highlights and popups.
  let editText = $state('');
  let used = untrack(() => ({ rows: info.usedRows, cols: info.usedCols }));
  // Column where a run of Tabs started; Enter returns to it (Excel).
  let tabStartCol: number | null = null;

  // Point mode: the reference being inserted by arrows or the mouse.
  let point = $state<{ start: number; end: number; anchor: CellPos; focus: CellPos } | null>(null);

  // Function autocomplete and parameter hint. Defined names are offered
  // too, marked with this category.
  const NAME_CATEGORY = t('名稱');
  let functions: FuncInfo[] = [];
  let funcByName = new Map<string, FuncInfo>();
  let suggestions = $state<FuncInfo[]>([]);
  let suggestIndex = $state(0);
  let hint = $state<{ info: FuncInfo; arg: number } | null>(null);
  let editError = $state<string | null>(null);
  let popupPos = $state({ x: 0, y: 0 });

  // Copied or cut range, shown with marching ants until Esc or an edit.
  let copyRange = $state<Range | null>(null);
  // Fill-handle drag: the source and the target being previewed.
  let fillSrc: Range | null = null;
  let fillPreview = $state<Range | null>(null);
  // Right-click menu and filter dropdown.
  let contextMenu = $state<{ x: number; y: number; area: ContextRequest['area'] } | null>(null);
  let filterMenu = $state<{ col: number; x: number; y: number; values: FilterValueDTO[]; title: string } | null>(null);

  function notify(message: string | null | undefined): void {
    if (message) onnotify?.(message);
  }

  // ---- v0.7: floating objects ----

  let selectedObject = $state<number | null>(null);
  // Bumped on every redraw so object positions follow scrolling.
  let viewTick = $state(0);
  let chartData = $state(new Map<number, ChartDataDTO>());
  let images = $state(new Map<string, string>());
  let objects = $derived(layout?.objects ?? []);

  function objectRect(o: ObjectDTO): { x: number; y: number; w: number; h: number } {
    const rc = grid?.areaRect({ r0: o.row, c0: o.col, r1: o.row, c1: o.col }) ?? { x: 0, y: 0, w: 0, h: 0 };
    return { x: rc.x + o.dx * scale, y: rc.y + o.dy * scale, w: o.w * scale, h: o.h * scale };
  }

  // Chart data and pictures for the objects on this sheet.
  let objectsToken = 0;
  async function refreshObjects(): Promise<void> {
    const token = ++objectsToken;
    const list = objects;
    const data = new Map<number, ChartDataDTO>();
    for (const o of list) {
      if (o.kind === 'chart') data.set(o.id, await sheetApi.chartData(sheet, o.id));
      else if (o.imageKey && !images.has(o.imageKey)) {
        const url = await sheetApi.objectImage(sheet, o.id);
        if (url) images = new Map(images).set(o.imageKey, url);
      }
    }
    if (token === objectsToken) chartData = data;
  }

  let chartTimer: ReturnType<typeof setTimeout> | undefined;
  function scheduleChartRefresh(): void {
    if (!objects.some((o) => o.kind === 'chart')) return;
    clearTimeout(chartTimer);
    chartTimer = setTimeout(() => void refreshObjects(), 80);
  }

  // A dragged object was dropped: anchor it to the cell under its
  // top-left corner.
  function commitObject(o: ObjectDTO, rect: { x: number; y: number; w: number; h: number }, label: string): void {
    if (!grid) return;
    const at = grid.cellAtPoint(rect.x + 0.5, rect.y + 0.5);
    const next = { ...o, row: at.pos.row, col: at.pos.col, dx: Math.max(0, (rect.x - at.rect.x) / scale), dy: Math.max(0, (rect.y - at.rect.y) / scale), w: rect.w / scale, h: rect.h / scale, chart: null };
    void serial(() => sheetApi.updateObject(sheet, next, label)).then(notify);
  }

  // Composite visible DOM objects as well as the grid for compatibility snapshots.
  export function snapshotWithObjects(): string {
    return grid?.snapshot((ctx, dpr) => {
      ctx.save(); ctx.scale(dpr, dpr);
      const vp = grid!.viewport();
      ctx.beginPath(); ctx.rect(metrics.headerW * scale, HEADER_H * scale, vp.width - metrics.headerW * scale, vp.height - HEADER_H * scale); ctx.clip();
      for (const o of objects) {
        const element = root.querySelector<HTMLCanvasElement | HTMLImageElement>(`[data-object-id="${o.id}"] canvas, [data-object-id="${o.id}"] img`);
        if (!element || element instanceof HTMLImageElement && (!element.complete || !element.naturalWidth)) continue;
        const rect = objectRect(o);
        ctx.drawImage(element, rect.x, rect.y, rect.w, rect.h);
      }
      ctx.restore();
    }) ?? '';
  }

  export function getObjects(): ObjectDTO[] {
    return objects;
  }

  export function getSelectedObject(): number | null {
    return selectedObject;
  }

  export function selectObject(id: number | null): void {
    selectedObject = id;
  }

  // Charts as PNG data URLs (2x), for printing.
  export function chartImages(): Record<number, string> {
    const out: Record<number, string> = {};
    for (const o of objects) {
      if (!o.chart && !o.shape) continue;
      const canvas = document.createElement('canvas');
      canvas.width = Math.round(o.w * 2);
      canvas.height = Math.round(o.h * 2);
      const ctx = canvas.getContext('2d');
      if (!ctx) continue;
      ctx.scale(2, 2);
      if (o.shape) drawShape(ctx, o.w, o.h, o.shape);
      else if (o.chart) drawChart(ctx, o.w, o.h, o.chart, toChartData(chartData.get(o.id)), LIGHT_THEME);
      out[o.id] = canvas.toDataURL('image/png');
    }
    return out;
  }

  // ---- v0.6 state ----

  // Editing a formula of a cell on another sheet: clicks here insert
  // references with this sheet's name.
  let remote = $state<{ sheet: number } | null>(null);

  // Data validation of the active cell (dropdown button, input prompt).
  let validation = $state<ValidationDTO | null>(null);
  let dropdown = $state<{ values: string[]; index: number } | null>(null);
  // The table containing the active cell (context menu items).
  let activeTable = $state<TableDTO | null>(null);
  // Note shown while hovering a cell, and the note being edited.
  let hoverNote = $state<{ row: number; col: number; author: string; text: string } | null>(null);
  let noteEdit = $state<{ row: number; col: number; text: string } | null>(null);
  let noteInput = $state<HTMLTextAreaElement>();
  let nameBox: HTMLInputElement;
  let nameBoxText = $state('');
  let nameBoxFocused = $state(false);

  let notes = $derived(new Map((layout?.notes ?? []).map((n) => [n.row * 16384 + n.col, n])));

  // Refresh what depends on the active cell when it moves.
  let cellInfoToken = 0;
  $effect(() => {
    const { row, col } = selection.active;
    void styleTick;
    const token = ++cellInfoToken;
    dropdown = null;
    void Promise.all([sheetApi.validation(sheet, row, col), sheetApi.tableAt(sheet, row, col)])
      .then(([v, t]) => {
        if (token !== cellInfoToken) return;
        validation = v;
        activeTable = t;
      })
      .catch(() => {});
  });

  $effect(() => {
    mode = !editing ? 'ready' : point ? 'point' : editing.mode;
  });

  // The active cell's style, for the toolbar.
  $effect(() => {
    void styleTick;
    const { row, col } = selection.active;
    const id = grid?.styleAt(row, col) ?? 0;
    activeStyle = styleStore.get(id)?.dto ?? null;
    activeMerged = mergeAt(merges, selection.active) !== null;
  });

  $effect(() => {
    onselectionchange?.(selection);
  });

  let address = $derived(cellAddress(selection.active.row, selection.active.col));

  // Colored boxes for the references in the formula being edited. The
  // same reference text gets the same color, like Excel.
  let highlights = $derived.by((): Highlight[] => {
    if (!editing || !editText.startsWith('=')) return [];
    const colors = new Map<string, number>();
    const out: Highlight[] = [];
    for (const ref of findRefs(editText)) {
      if (ref.sheet !== null) continue; // other sheets are not on screen
      const key = editText.slice(ref.start, ref.end).replace(/\$/g, '').toUpperCase();
      if (!colors.has(key)) colors.set(key, colors.size % REF_COLORS);
      out.push({ range: ref.range, color: colors.get(key)!, dashed: point?.start === ref.start });
    }
    return out;
  });

  // Backend calls run strictly in order (edits, then dependent moves).
  let queue: Promise<void> = Promise.resolve();
  function enqueue(fn: () => Promise<void> | void): Promise<void> {
    queue = queue.then(fn).catch((err) => console.error(err));
    return queue;
  }

  export function whenIdle(): Promise<void> {
    return queue;
  }

  export function focus(): void {
    editor?.focus({ preventScroll: true });
  }

  function input(): HTMLTextAreaElement {
    return activeInput === 'bar' ? bar : editor;
  }

  // ---- Selection ----

  // ---- VoiceOver ----
  // The editor textarea keeps the focus, so screen readers learn about
  // the active cell from a live region: its address and value, or the
  // selected range. Updates are debounced so holding an arrow key does
  // not queue an announcement per cell.
  let announcement = $state('');
  let announceTimer: ReturnType<typeof setTimeout> | undefined;
  $effect(() => {
    const sel = selection;
    clearTimeout(announceTimer);
    announceTimer = setTimeout(() => {
      announcement = describeSelection(sel);
    }, 150);
    return () => clearTimeout(announceTimer);
  });

  function describeSelection(sel: Selection): string {
    const a = sel.active;
    const addr = cellAddress(a.row, a.col);
    const value = grid?.cache.cell(a.row, a.col)?.text ?? '';
    const cell = value ? `${addr} ${value}` : t('%s 空白', addr);
    if (isSingleCell(sel)) return cell;
    const rg = sel.ranges[sel.ranges.length - 1];
    const range = `${cellAddress(rg.r0, rg.c0)}:${cellAddress(rg.r1, rg.c1)}`;
    return t('已選取 %s，%s', range, cell);
  }

  export function getAnnouncement(): string {
    return announcement;
  }

  function setSelection(sel: Selection, reveal: CellPos = sel.active): void {
    selection = sel;
    grid?.scrollIntoView(reveal);
    refreshFormula();
  }

  // A single-cell selection; a merged area selects as one cell.
  function singleAt(pos: CellPos): Selection {
    const anchor = snap(merges, pos);
    const area = areaOf(merges, anchor);
    return { active: anchor, anchor, focus: anchor, ranges: [area] };
  }

  function withMerges(sel: Selection): Selection {
    if (merges.length === 0) return sel;
    return { ...sel, ranges: sel.ranges.map((r) => expand(merges, r)) };
  }

  function moved(pos: CellPos, extend: boolean): void {
    const p = clampPos(pos, lim);
    if (extend) setSelection(withMerges(extendTo(selection, p)), p);
    else setSelection(singleAt(p));
  }

  // While idle the formula bar shows the active cell's edit text.
  let formulaToken = 0;
  function refreshFormula(): void {
    if (editing || !bar) return;
    const token = ++formulaToken;
    const { row, col } = selection.active;
    sheetApi
      .editText(sheet, row, col)
      .then((text) => {
        if (token === formulaToken && !editing && bar) bar.value = text;
      })
      // Best effort: the call is dropped when this view unmounts.
      .catch(() => {});
  }

  const toDTO = (r: Range): RangeDTO => ({ r0: r.r0, c0: r.c0, r1: r.r1, c1: r.c1 });

  // ---- Idle actions ----

  async function perform(a: Action): Promise<void> {
    const sel = selection;
    // Shift-extensions move the focus end; plain moves start at active.
    const base = 'extend' in a && a.extend ? sel.focus : sel.active;
    if (a.type !== 'tab' && a.type !== 'enter') tabStartCol = null;

    switch (a.type) {
      case 'move':
        if (a.jump) {
          const p = await sheetApi.dataEdge(sheet, base.row, base.col, a.dr, a.dc);
          moved(p, a.extend);
        } else {
          const from = stepStart(merges, base, a.dr, a.dc);
          moved({ row: from.row + a.dr, col: from.col + a.dc }, a.extend);
        }
        break;
      case 'page': {
        const page = grid!.pageSize();
        const n = (a.horizontal ? page.cols : page.rows) * a.dir;
        if (a.horizontal) grid!.scrollByCells(0, n);
        else grid!.scrollByCells(n, 0);
        moved(a.horizontal ? { row: base.row, col: base.col + n } : { row: base.row + n, col: base.col }, a.extend);
        break;
      }
      case 'rowStart':
        moved({ row: base.row, col: 0 }, a.extend);
        break;
      case 'docStart':
        moved({ row: 0, col: 0 }, a.extend);
        break;
      case 'docEnd':
        moved({ row: Math.max(0, used.rows - 1), col: Math.max(0, used.cols - 1) }, a.extend);
        break;
      case 'enter': {
        const then = enterThen(a.backward);
        if (then !== 'stay') advance(then === 'down' || then === 'up' ? 'down' : 'right', then === 'up' || then === 'left');
        break;
      }
      case 'tab':
        advance('right', a.backward);
        break;
      case 'beginEdit':
        await beginEdit('edit');
        break;
      case 'clear':
        copyRange = null;
        await sheetApi.clear(sheet, sel.ranges.map(toDTO));
        break;
      case 'escape':
        if (copyRange) {
          copyRange = null;
          await sheetApi.clearClip();
        }
        break;
      case 'selectRow': {
        const r = current(sel);
        setSelection(selectRows(sel, r.r0, r.r1, lim, false, sel.active.col), sel.active);
        break;
      }
      case 'selectCol': {
        const r = current(sel);
        setSelection(selectCols(sel, r.c0, r.c1, lim, false, sel.active.row), sel.active);
        break;
      }
    }
  }

  // Enter/Tab movement: cycles inside a multi-cell selection, otherwise
  // moves one cell (Enter after Tabs returns to the starting column).
  function advance(dir: 'down' | 'right', backward: boolean): void {
    const sel = selection;
    if (!isSingleCell(sel)) {
      setSelection(cycleActive(sel, dir === 'down', backward));
      return;
    }
    const { row, col } = stepStart(merges, sel.active, dir === 'down' ? (backward ? -1 : 1) : 0, dir === 'right' ? (backward ? -1 : 1) : 0);
    const step = backward ? -1 : 1;
    if (dir === 'right') {
      if (tabStartCol === null) tabStartCol = col;
      moved({ row, col: col + step }, false);
    } else {
      const targetCol = !backward && tabStartCol !== null ? tabStartCol : col;
      tabStartCol = null;
      moved({ row: row + step, col: targetCol }, false);
    }
  }

  // ---- Menu / shortcuts ----

  // Shortcuts can arrive both as keydown and as a menu event; ignore the
  // second copy.
  let lastMenu = { action: '', at: 0 };

  // Entry point for menu items and ⌘ shortcuts. Must not be called from
  // inside the queue.
  export async function runMenu(action: MenuAction, pasteText: string | null = null): Promise<void> {
    if (paintMode !== 'off') oncancelpaint?.();
    const now = performance.now();
    if (lastMenu.action === action && now - lastMenu.at < 150) return;
    lastMenu = { action, at: now };
    contextMenu = null;

    if (await runEditingMenu(action, pasteText)) return;

    switch (action) {
      case 'undo':
      case 'redo':
        if (editing) {
          document.execCommand(action);
        } else {
          await enqueue(async () => {
            await (action === 'undo' ? sheetApi.undo() : sheetApi.redo());
          });
        }
        return;
      case 'selectAll':
        if (editing) input().select();
        else await selectRegionOrAll();
        return;
      case 'bold':
      case 'italic':
      case 'underline':
        // Excel formats the cell; rich text inside the editor comes later.
        if (!editing) await toggleFont(action);
        return;
      case 'autofitCols':
        await autofit(false);
        return;
      case 'autofitRows':
        await autofit(true);
        return;
      case 'formatCells':
      case 'insertSheet':
      case 'find':
      case 'replace':
        if (editing) commit('stay');
        onappaction?.(action);
        return;
    }

    // File actions commit any edit first, like Excel.
    if (editing) commit('stay');
    await enqueue(async () => {
      switch (action) {
        case 'new':
          await documentApi.newDocument();
          break;
        case 'open':
          await documentApi.open();
          break;
        case 'save':
          await documentApi.save();
          break;
        case 'saveAs':
          await documentApi.saveAs();
          break;
        case 'close':
          await documentApi.requestClose();
          break;
      }
    });
    focus();
  }

  // ---- Editing ----

  async function beginEdit(m: EditMode): Promise<void> {
    const pos = selection.active;
    const text = await sheetApi.editText(sheet, pos.row, pos.col);
    startEditing(m, pos, text, text.length);
  }

  function startEditing(m: EditMode, pos: CellPos, text?: string, caret?: number): void {
    editing = { mode: m, pos };
    editError = null;
    point = null;
    if (text !== undefined) setText(text, caret ?? text.length);
    else syncFrom(input());
    if (!remote) grid?.scrollIntoView(pos);
    layoutEditor();
  }

  function endEditing(): void {
    editing = null;
    point = null;
    suggestions = [];
    hint = null;
    editor.value = '';
    editText = '';
    if (activeInput === 'bar') {
      activeInput = 'cell';
      focus();
    }
    layoutEditor();
    refreshFormula();
  }

  // Replaces the edited text in both inputs and places the caret in the
  // active one.
  function setText(text: string, caret: number): void {
    editor.value = text;
    bar.value = text;
    input().setSelectionRange(caret, caret);
    editText = text;
    layoutEditor();
    updateAssist();
  }

  // Copies the active input's text to the other one.
  function syncFrom(el: HTMLTextAreaElement): void {
    const other = el === editor ? bar : editor;
    if (other.value !== el.value) other.value = el.value;
    editText = el.value;
  }

  // Go may reject the input (formula errors): go back to editing it and
  // explain, like Excel's "There's a problem with this formula" dialog.
  function restoreEdit(pos: CellPos, text: string, message: string): void {
    setSelection(single(pos));
    activeInput = 'cell';
    focus();
    startEditing('edit', pos, text, text.length);
    editError = message;
  }

  function commit(then: 'down' | 'up' | 'right' | 'left' | 'stay'): void {
    if (!editing) return;
    if (remote) return commitRemote();
    copyRange = null;
    const { pos } = editing;
    const text = input().value;
    endEditing();
    const written = enqueue(async () => {
      const res = await sheetApi.setCell(sheet, pos.row, pos.col, text);
      if (res.error) restoreEdit(pos, text, res.error);
      else notify(res.notice);
    });
    // Follow-ups run after the write, outside the queue (they enqueue
    // their own calls). A line break turns on wrapping: grow the row.
    // A number too wide for a default-width column widens it.
    if (text.includes('\n')) {
      const row = pos.row;
      void written.then(() => autofitRowsOf([{ r0: row, c0: 0, r1: row, c1: 0 }], true));
    } else if (text !== '' && !text.startsWith('=') && !text.startsWith("'")) {
      void written.then(() => widenForNumber(pos));
    }
    switch (then) {
      case 'down':
      case 'up':
        advance('down', then === 'up');
        break;
      case 'right':
      case 'left':
        advance('right', then === 'left');
        break;
    }
  }

  // Ends a cross-sheet edit: writes the cell on its own sheet, then the
  // app goes back there.
  function commitRemote(): void {
    if (!editing || !remote) return;
    const origin = remote.sheet;
    const { pos } = editing;
    const text = input().value;
    endEditing();
    remote = null;
    void enqueue(async () => {
      const res = await sheetApi.setCell(origin, pos.row, pos.col, text);
      if (res.error) notify(res.error);
      oncrossdone?.(origin, pos, res.error ? text : null);
    });
  }

  // For the app: the formula being edited, to continue on another sheet.
  export function formulaState(): CrossEdit | null {
    if (!editing || !input().value.startsWith('=')) return null;
    const el = input();
    return { originSheet: remote?.sheet ?? sheet, pos: editing.pos, text: el.value, caret: el.selectionStart };
  }

  function commitFill(): void {
    if (!editing) return;
    const { pos } = editing;
    const text = input().value;
    const ranges = selection.ranges.map(toDTO);
    const sel = selection;
    endEditing();
    enqueue(async () => {
      const err = await sheetApi.fill(sheet, ranges, text, pos.row, pos.col);
      if (err) {
        restoreEdit(pos, text, err);
        selection = sel;
      }
    });
  }

  // Point mode applies when editing a formula and either a reference is
  // already being pointed or the caret sits after an operator.
  function pointable(): boolean {
    const el = input();
    return !!editing && el.value.startsWith('=') && (point !== null || canPoint(el.value, el.selectionStart));
  }

  function placePoint(anchor: CellPos, focusPos: CellPos): void {
    const el = input();
    const start = point ? point.start : el.selectionStart;
    const end = point ? point.end : el.selectionEnd;
    const ref = (remote ? quoteSheet(sheetName) + '!' : '') + refText(anchor, focusPos);
    const text = el.value.slice(0, start) + ref + el.value.slice(end);
    point = { start, end: start + ref.length, anchor, focus: focusPos };
    setText(text, start + ref.length);
    grid?.scrollIntoView(focusPos);
  }

  function pointArrow(a: { dr: number; dc: number; extend: boolean }): boolean {
    if (!editing || !pointable()) return false;
    // In Edit mode (F2) arrows move the caret unless already pointing.
    if (!point && editing.mode === 'edit') return false;
    const base = point ? point.focus : remote ? selection.active : editing.pos;
    const focusPos = clampPos({ row: base.row + a.dr, col: base.col + a.dc }, lim);
    const anchor = a.extend ? (point ? point.anchor : editing.pos) : focusPos;
    placePoint(anchor, focusPos);
    return true;
  }

  // Returns whether the action was handled (and the key consumed).
  function performEdit(a: Action): boolean {
    switch (a.type) {
      case 'arrow': {
        if (pointArrow(a)) return true;
        const c = editing ? arrowCommit(a, editing.mode) : null;
        if (!c || c.type !== 'commit') return false; // move the caret
        commit(c.then);
        return true;
      }
      case 'commit':
        commit(a.then);
        return true;
      case 'commitFill':
        commitFill();
        return true;
      case 'cancel':
        if (remote) {
          const origin = remote.sheet;
          const pos = editing!.pos;
          endEditing();
          remote = null;
          oncrossdone?.(origin, pos, null);
          return true;
        }
        endEditing();
        return true;
      case 'newline': {
        const el = input();
        el.setRangeText('\n', el.selectionStart, el.selectionEnd, 'end');
        onInput();
        return true;
      }
      case 'toggleMode':
        if (editing) editing = { ...editing, mode: editing.mode === 'enter' ? 'edit' : 'enter' };
        return true;
      case 'toggleAbs': {
        const el = input();
        const next = toggleAbsolute(el.value, el.selectionStart);
        if (next) {
          point = null;
          setText(next.text, next.caret);
        }
        return true;
      }
      case 'menu':
        void runMenu(a.action);
        return true;
    }
    return false;
  }

  function onKeyDown(e: KeyboardEvent): void {
    if (paintMode !== 'off' && e.key === 'Escape') {
      e.preventDefault();
      oncancelpaint?.();
      return;
    }
    if (editing) {
      if (suggestions.length > 0 && handleSuggestionKey(e)) {
        e.preventDefault();
        return;
      }
      const a = editAction(e);
      if (a && performEdit(a)) e.preventDefault();
      return;
    }
    if (selectedObject !== null) {
      if (e.key === 'Delete' || e.key === 'Backspace') {
        e.preventDefault();
        const id = selectedObject;
        selectedObject = null;
        void serial(() => sheetApi.deleteObjects(sheet, [id])).then(notify);
        return;
      }
      if (e.key === 'Escape') {
        e.preventDefault();
        selectedObject = null;
        return;
      }
      selectedObject = null;
    }
    if (onDropdownKey(e)) {
      e.preventDefault();
      return;
    }
    if (e.altKey && e.key === 'ArrowDown' && validation?.type === 'list' && validation.dropdown) {
      e.preventDefault();
      void openDropdown();
      return;
    }
    if (e.key === 'F5') {
      // 到 (Go To): type an address or name in the name box.
      e.preventDefault();
      nameBox.focus();
      return;
    }
    if (e.shiftKey && e.key === 'F2') {
      e.preventDefault();
      editNote();
      return;
    }
    const a = idleAction(e);
    if (!a) return; // printable keys flow into the textarea -> onInput
    e.preventDefault();
    // runMenu enqueues its own backend work; running it inside the queue
    // would wait on itself.
    if (a.type === 'menu') void runMenu(a.action);
    else void enqueue(() => perform(a));
  }

  // Typing while idle starts Enter mode with the typed text, replacing
  // the cell content. Typing while pointing fixes the pointed reference.
  function onInput(): void {
    if (!editing) {
      startEditing('enter', selection.active);
      updateAssist();
      return;
    }
    point = null;
    editError = null;
    syncFrom(input());
    layoutEditor();
    updateAssist();
  }

  function onCompositionStart(): void {
    if (!editing) startEditing('enter', selection.active);
  }

  function onEditorFocus(): void {
    activeInput = 'cell';
  }

  // Clicking the formula bar edits the active cell there (Edit mode).
  function onBarFocus(): void {
    activeInput = 'bar';
    if (!editing) startEditing('edit', selection.active, bar.value, bar.selectionStart);
  }

  // ---- Autocomplete and parameter hints ----

  function updateAssist(): void {
    if (!editing) {
      suggestions = [];
      hint = null;
      return;
    }
    const el = input();
    const text = el.value;
    const caret = el.selectionStart;
    // Moving the caret away fixes the pointed reference.
    if (point && caret !== point.end) point = null;

    const w = wordAt(text, caret);
    const prefix = w?.word.toUpperCase() ?? '';
    const nameItems: FuncInfo[] = names
      .filter((n) => n.toUpperCase().startsWith(prefix))
      .map((n) => ({ name: n, sig: '', desc: t('名稱'), category: NAME_CATEGORY }));
    const next = w ? [...nameItems, ...functions.filter((f) => f.name.startsWith(prefix))].slice(0, 10) : [];
    const exact = next.length === 1 && next[0].name.toUpperCase() === prefix;
    suggestions = exact ? [] : next;
    suggestIndex = 0;

    const call = text.startsWith('=') ? callAt(text, caret) : null;
    const info = call ? funcByName.get(call.name) : undefined;
    hint = info && call ? { info, arg: call.arg } : null;
    positionPopups();
  }

  function handleSuggestionKey(e: KeyboardEvent): boolean {
    if (e.metaKey || e.altKey || e.ctrlKey) return false;
    switch (e.key) {
      case 'ArrowDown':
        suggestIndex = (suggestIndex + 1) % suggestions.length;
        return true;
      case 'ArrowUp':
        suggestIndex = (suggestIndex - 1 + suggestions.length) % suggestions.length;
        return true;
      case 'Tab':
        if (e.shiftKey) return false;
        acceptSuggestion(suggestIndex);
        return true;
      case 'Escape':
        suggestions = [];
        return true;
    }
    return false;
  }

  function acceptSuggestion(i: number): void {
    const el = input();
    const w = wordAt(el.value, el.selectionStart);
    const f = suggestions[i];
    if (!w || !f) return;
    // Functions get an opening parenthesis; defined names do not.
    const insert = f.category === NAME_CATEGORY ? f.name : f.name + '(';
    const text = el.value.slice(0, w.start) + insert + el.value.slice(el.selectionStart);
    point = null;
    setText(text, w.start + insert.length);
  }

  // Splits "number1, [number2], ..." so the current argument can be bold.
  function hintParts(sig: string, arg: number): { text: string; active: boolean }[] {
    const parts = sig ? sig.split(', ') : [];
    const variadic = parts.at(-1) === '...';
    let active = Math.min(arg, parts.length - 1);
    if (variadic && arg >= parts.length - 1) active = parts.length - 2;
    return parts.map((text, i) => ({ text, active: i === active }));
  }

  function positionPopups(): void {
    if (!root) return;
    const el = input();
    const base = root.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    popupPos = { x: Math.max(4, r.left - base.left), y: r.bottom - base.top + 2 };
  }

  // ---- Editor layout ----

  let measureCtx: CanvasRenderingContext2D | null = null;
  function textWidth(text: string): number {
    measureCtx ??= document.createElement('canvas').getContext('2d');
    if (!measureCtx) return 0;
    measureCtx.font = getComputedStyle(editor).font;
    return Math.max(0, ...text.split('\n').map((line) => measureCtx!.measureText(line).width));
  }

  // Positions the textarea on its cell. While editing it grows to the
  // right to fit the text, then wraps and grows downward, like Excel.
  function layoutEditor(): void {
    if (!grid || !editor) return;
    if (remote) {
      // The edited cell is on another sheet: only the formula bar shows.
      editor.style.left = '-10000px';
      return;
    }
    const pos = editing?.pos ?? selection.active;
    const rc = grid.areaRect(areaOf(merges, pos));
    const vp = grid.viewport();
    const st = editor.style;
    // The editor uses the cell's font.
    const cellStyle = styleStore.get(grid.styleAt(pos.row, pos.col));
    if (cellStyle) {
      st.font = cellStyle.font;
      st.fontSize = `${cellStyle.fontPx * scale}px`;
      st.lineHeight = `${Math.round(cellStyle.fontPx * LINE_HEIGHT * scale)}px`;
      st.color = cellStyle.color ?? '';
    }
    st.padding = `${scale}px ${3 * scale}px 0 ${4 * scale}px`;
    st.left = `${Math.max(rc.x, metrics.headerW * scale) - 1}px`;
    st.top = `${rc.y - 1}px`;
    if (!editing) {
      st.width = `${rc.w + 1}px`;
      st.height = `${rc.h + 1}px`;
      return;
    }
    const maxW = Math.max(rc.w + 1, vp.width - rc.x - 2);
    const want = textWidth(editor.value) + 12 * scale;
    st.width = `${Math.min(maxW, Math.max(rc.w + 1, want))}px`;
    st.height = 'auto';
    st.height = `${Math.max(rc.h + 1, editor.scrollHeight + 2)}px`;
    if (activeInput === 'cell') positionPopups();
  }

  // ---- Pointer ----

  let dragBase: Range[] = [];
  let dragAnchor = 0;
  let pointDrag: CellPos | null = null;

  function onPointer(e: GridPointer): void {
    // Clicking cells while editing a formula inserts references.
    if (editing && e.area === 'cell' && (pointDrag || (e.kind === 'down' && pointable()))) {
      if (e.kind === 'down') {
        pointDrag = e.pos;
        placePoint(e.pos, e.pos);
      } else if (e.kind === 'drag' && pointDrag) {
        placePoint(pointDrag, e.pos);
      } else if (e.kind === 'up') {
        pointDrag = null;
      }
      return;
    }
    if (e.kind === 'down' && editing) commit('stay');
    if (e.kind === 'down') selectedObject = null;
    tabStartCol = null;
    contextMenu = null;
    const sel = selection;

    switch (e.area) {
      case 'fill':
        onFillPointer(e);
        break;
      case 'corner':
        if (e.kind === 'down') setSelection(selectAll(lim), sel.active);
        break;
      case 'cell':
        if (e.kind === 'dbl') {
          void enqueue(() => beginEdit('edit'));
        } else if (e.kind === 'down') {
          if (e.shift) {
            dragBase = sel.ranges.slice(0, -1);
            setSelection(withMerges(extendTo(sel, e.pos)), e.pos);
          } else {
            dragBase = e.meta ? sel.ranges : [];
            const s = singleAt(e.pos);
            setSelection({ ...s, ranges: [...dragBase, ...s.ranges] });
          }
        } else if (e.kind === 'drag') {
          const r = expand(merges, rangeOf(sel.anchor, e.pos));
          setSelection({ ...sel, focus: e.pos, ranges: [...dragBase, r] }, e.pos);
        }
        break;
      case 'col':
      case 'row': {
        const isCol = e.area === 'col';
        const idx = isCol ? e.pos.col : e.pos.row;
        if (e.kind === 'down') {
          dragBase = e.meta ? sel.ranges : [];
          dragAnchor = e.shift ? (isCol ? sel.active.col : sel.active.row) : idx;
        }
        if (e.kind === 'down' || e.kind === 'drag') {
          const s = isCol ? selectCols(sel, dragAnchor, idx, lim) : selectRows(sel, dragAnchor, idx, lim);
          const active = e.shift || e.kind === 'drag' ? sel.active : s.active;
          setSelection(
            { ...s, active, ranges: [...dragBase, ...s.ranges] },
            isCol ? { row: sel.active.row, col: idx } : { row: idx, col: sel.active.col },
          );
        }
        break;
      }
    }
    if (e.kind === 'up' && paintMode !== 'off') void paintFormats();
  }

  export async function captureFormats(): Promise<Range | null> {
    if (editing) commit('stay');
    await whenIdle();
    const source = { ...current(selection) };
    const error = await serial(() => sheetApi.captureFormats(sheet, toDTO(source)));
    if (error) { notify(error); return null; }
    focus();
    return source;
  }

  let paintBusy = false;
  async function paintFormats(): Promise<void> {
    if (paintBusy) return;
    paintBusy = true;
    const ranges = selection.ranges.map(toDTO);
    try {
      const error = await serial(() => sheetApi.paintFormats(sheet, ranges));
      if (error) notify(error);
      else onpainted?.();
    } finally { paintBusy = false; }
  }

  // ---- Backend events ----

  function onCellsChanged(d: CellsChanged): void {
    styleStore.merge(d.styles);
    // Charts may show data from any sheet.
    scheduleChartRefresh();
    if (d.sheet !== sheet) return;
    grid?.cache.applyDiff(d);
    styleTick++;
    used = { rows: d.usedRows, cols: d.usedCols };
    if (d.select) setSelection(fromRange(d.select));
    else refreshFormula();
  }

  async function onReset(): Promise<void> {
    if (editing) endEditing();
    const fresh = await sheetApi.info(sheet);
    used = { rows: fresh.usedRows, cols: fresh.usedCols };
    grid?.cache.reset();
    grid?.scrollToOrigin();
    setSelection(single({ row: 0, col: 0 }));
  }

  async function loadLayout(): Promise<void> {
    layout = await sheetApi.layout(sheet, styleStore.known);
    styleTick++;
    if (selectedObject !== null && !objects.some((o) => o.id === selectedObject)) selectedObject = null;
    void refreshObjects();
  }

  onMount(() => {
    const offs = [
      sheetApi.onCellsChanged(onCellsChanged),
      sheetApi.onLayoutChanged((e) => {
        if (e.sheet === sheet) void loadLayout();
      }),
      documentApi.onReset(() => void onReset()),
      styleStore.subscribe(() => styleTick++),
    ];
    for (const el of [editor, bar]) {
      for (const type of ['copy', 'cut', 'paste']) el.addEventListener(type, onClipboardEvent as EventListener);
      for (const type of ['beforecopy', 'beforecut']) el.addEventListener(type, onBeforeClipboard);
    }
    void loadLayout();
    sheetApi.functions().then((list) => {
      functions = list;
      funcByName = new Map(list.map((f) => [f.name, f]));
    });
    focus();
    refreshFormula();
    const ce = untrack(() => crossEdit);
    if (ce) {
      if (ce.originSheet !== sheet) {
        remote = { sheet: ce.originSheet };
        activeInput = 'bar';
        startEditing('edit', ce.pos, ce.text, ce.caret);
        bar.focus();
      } else {
        startEditing('edit', ce.pos, ce.text, ce.caret);
      }
    }
    return () => offs.forEach((off) => off());
  });

  // ---- Clipboard, fill, structure, sort & filter (v0.5) ----

  // Menu actions that act on cells. Returns false for other actions.
  async function runEditingMenu(action: MenuAction, pasteText: string | null): Promise<boolean> {
    switch (action) {
      case 'copy':
      case 'cut':
        if (editing) document.execCommand(action);
        else await copySelection(action === 'cut');
        return true;
      case 'paste':
        if (editing) return true; // the text field pastes natively
        await paste(pasteText);
        return true;
      case 'pasteSpecial':
        if (editing) commit('stay');
        if (await sheetApi.hasClip()) onappaction?.('pasteSpecial');
        else notify(t('請先複製儲存格，再使用選擇性貼上。'));
        return true;
      case 'fillDown':
      case 'fillRight': {
        if (editing) return true;
        const f = fillCopy(current(selection), action === 'fillDown');
        if (f) await fill(f.src, f.dst, true);
        return true;
      }
      case 'insertCells':
      case 'deleteCells': {
        if (editing) commit('stay');
        const r = current(selection);
        const insert = action === 'insertCells';
        if (isFullRows(r, lim)) await shiftSelection(true, insert);
        else if (isFullCols(r, lim)) await shiftSelection(false, insert);
        else onappaction?.(action); // ask: whole rows or columns
        return true;
      }
      case 'insertRows':
      case 'deleteRows':
      case 'insertCols':
      case 'deleteCols':
        if (editing) commit('stay');
        await shiftSelection(action.endsWith('Rows'), action.startsWith('insert'));
        return true;
      case 'hideRows':
      case 'unhideRows':
      case 'hideCols':
      case 'unhideCols': {
        if (editing) return true;
        const rows = action.endsWith('Rows');
        const hidden = action.startsWith('hide');
        for (const r of selection.ranges) {
          const err = await serial(() => sheetApi.hide(sheet, rows, rows ? r.r0 : r.c0, rows ? r.r1 : r.c1, hidden));
          if (err) return notify(err), true;
        }
        return true;
      }
      case 'freezePanes':
      case 'freezeTopRow':
      case 'freezeFirstCol':
      case 'unfreeze':
        if (editing) commit('stay');
        await freezePanes(action);
        return true;
      case 'sortAsc':
      case 'sortDesc':
        if (editing) commit('stay');
        await quickSort(action === 'sortDesc');
        return true;
      case 'sort':
        if (editing) commit('stay');
        onappaction?.('sort');
        return true;
      case 'filter':
        if (editing) commit('stay');
        notify(await serial(() => sheetApi.toggleFilter(sheet, toDTO(current(selection)))));
        return true;
      case 'insertNote':
        editNote();
        return true;
      case 'deleteNote': {
        const { row, col } = selection.active;
        notify(await serial(() => sheetApi.setNote(sheet, row, col, '')));
        return true;
      }
      case 'tableTotals':
        await tableAction('totals');
        return true;
      case 'convertTable':
        await tableAction('convert');
        return true;
      case 'clearValidation':
        if (editing) commit('stay');
        notify(await serial(() => sheetApi.setValidation(sheet, selection.ranges.map(toDTO), null)));
        return true;
      case 'clearCondFormats':
        if (editing) commit('stay');
        notify(await serial(() => sheetApi.clearCondFormats(sheet, selection.ranges.map(toDTO))));
        return true;
      case 'clearAllCondFormats':
        if (editing) commit('stay');
        notify(await serial(() => sheetApi.clearCondFormats(sheet, [])));
        return true;
      case 'insertPicture': {
        if (editing) commit('stay');
        const { row, col } = selection.active;
        notify(await documentApi.insertPicture(sheet, row, col));
        return true;
      }
      case 'insertChart':
      case 'editChart':
      case 'pageSetup':
      case 'printPreview':
      case 'print':
      case 'exportPDF':
        if (editing) commit('stay');
        onappaction?.(action);
        return true;
      case 'defineName':
      case 'nameManager':
      case 'dataValidation':
      case 'condFormat':
      case 'condFormatManager':
      case 'insertTable':
      case 'renameTable':
        if (editing) commit('stay');
        onappaction?.(action);
        return true;
      case 'clearAll':
      case 'clearFormats':
      case 'clearContents':
        if (editing) return true;
        await clearRanges(action === 'clearAll' ? 'all' : action === 'clearFormats' ? 'formats' : 'contents');
        return true;
    }
    return false;
  }

  // Runs a backend call in the queue and returns its result.
  function serial<T>(fn: () => Promise<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      void enqueue(async () => {
        try {
          resolve(await fn());
        } catch (err) {
          reject(err);
        }
      });
    });
  }

  function single1(): Range | null {
    if (selection.ranges.length > 1) {
      notify(t('無法對多重選取範圍執行此命令。'));
      return null;
    }
    return current(selection);
  }

  async function copySelection(cut: boolean): Promise<void> {
    const r = single1();
    if (!r) return;
    // Offer an HTML table too (formats for other apps) while still inside
    // the user's gesture; Go has already put the text on the clipboard.
    let done: (v: CopyResult | null) => void = () => {};
    const result = new Promise<CopyResult | null>((res) => (done = res));
    writeRichClipboard(result);
    const res = await serial(() => sheetApi.copy(sheet, toDTO(r), cut));
    done(res.value);
    if (res.error) return notify(res.error);
    copyRange = r;
  }

  function writeRichClipboard(result: Promise<CopyResult | null>): void {
    if (typeof ClipboardItem === 'undefined' || !navigator.clipboard?.write) return;
    const blob = (type: string, pick: (r: CopyResult) => string) =>
      result.then((r) => {
        if (!r) throw new Error('copy failed');
        return new Blob([pick(r)], { type });
      });
    try {
      void navigator.clipboard
        .write([
          new ClipboardItem({
            'text/plain': blob('text/plain', (r) => r.text),
            'text/html': blob('text/html', (r) => r.html),
          }),
        ])
        .catch(() => {});
    } catch {
      // Not allowed here; the plain text from Go stays.
    }
  }

  // Pastes the clipboard at the selection (text from a paste event, or
  // read by Go).
  export async function paste(text: string | null): Promise<void> {
    const r = single1();
    if (!r) return;
    const res = await serial(() => sheetApi.paste(sheet, toDTO(r), text));
    if (res.error) return notify(res.error);
    if (res.value) {
      if (res.value.moved) copyRange = null;
      setSelection(fromRange(res.value.area));
    }
  }

  export async function pasteSpecial(mode: PasteMode, transpose: boolean, skipBlanks: boolean): Promise<void> {
    const r = single1();
    if (!r) return;
    const res = await serial(() => sheetApi.pasteSpecial(sheet, toDTO(r), mode, transpose, skipBlanks));
    if (res.error) notify(res.error);
    else if (res.value) setSelection(fromRange(res.value));
    focus();
  }

  async function fill(src: Range, dst: Range, copyOnly: boolean): Promise<void> {
    copyRange = null;
    const err = await serial(() => sheetApi.fillSeries(sheet, toDTO(src), toDTO(dst), copyOnly));
    if (err) return notify(err);
    setSelection({ ...fromRange(dst), active: selection.active });
  }

  function onFillPointer(e: GridPointer): void {
    const src = current(selection);
    switch (e.kind) {
      case 'down':
        fillSrc = src;
        fillPreview = src;
        break;
      case 'drag':
        if (fillSrc) {
          fillPreview = fillTarget(fillSrc, e.pos);
          grid?.scrollIntoView(e.pos);
        }
        break;
      case 'up': {
        const from = fillSrc;
        const to = fillPreview;
        fillSrc = null;
        fillPreview = null;
        if (from && to && !sameRange(from, to)) void fill(from, to, false);
        break;
      }
      case 'dbl':
        void fillToRegion(src);
        break;
    }
  }

  // Double-clicking the fill handle fills down as far as the data
  // around it goes.
  async function fillToRegion(src: Range): Promise<void> {
    const region = await sheetApi.currentRegion(sheet, src.r1, src.c0);
    if (region.r1 > src.r1) await fill(src, { ...src, r1: region.r1 }, false);
  }

  export async function shiftSelection(rows: boolean, insert: boolean): Promise<void> {
    const r = single1();
    if (!r) return;
    const at = rows ? r.r0 : r.c0;
    const count = rows ? r.r1 - r.r0 + 1 : r.c1 - r.c0 + 1;
    copyRange = null;
    notify(await serial(() => sheetApi.shift(sheet, rows, at, insert ? count : -count)));
  }

  async function freezePanes(action: MenuAction): Promise<void> {
    const fz = grid?.freeze() ?? { rows: 0, cols: 0 };
    let rows = 0;
    let cols = 0;
    if (action === 'freezeTopRow') rows = 1;
    else if (action === 'freezeFirstCol') cols = 1;
    else if (action === 'freezePanes' && fz.rows === 0 && fz.cols === 0) {
      ({ row: rows, col: cols } = selection.active);
      if (rows === 0 && cols === 0) return notify(t('請選取要凍結的列下方、欄右方的儲存格。'));
    }
    grid?.scrollToOrigin();
    notify(await serial(() => sheetApi.freeze(sheet, rows, cols)));
  }

  export function sortInfo(): Promise<SortInfo> {
    return sheetApi.sortInfo(sheet, toDTO(current(selection)));
  }

  export async function sortRange(rg: RangeDTO, keys: SortKeyDTO[], header: boolean): Promise<void> {
    copyRange = null;
    notify(await serial(() => sheetApi.sort(sheet, rg, keys, header)));
    focus();
  }

  async function quickSort(desc: boolean): Promise<void> {
    const info = await sortInfo();
    if (!info.ok) return notify(t('請選取資料範圍中的儲存格再排序。'));
    const col = Math.max(info.range.c0, Math.min(selection.active.col, info.range.c1));
    await sortRange(info.range, [{ col, desc }], info.header);
  }

  // Column labels for the sort dialog: header texts or 欄 A.
  export function columnLabels(rg: RangeDTO, header: boolean): { col: number; label: string }[] {
    const out = [];
    for (let c = rg.c0; c <= rg.c1 && out.length < 200; c++) {
      const text = header ? grid?.cache.cell(rg.r0, c)?.text : '';
      out.push({ col: c, label: text ? text : t('欄 %s', colName(c)) });
    }
    return out;
  }

  // ⌘A: the data region around the active cell first, then everything.
  async function selectRegionOrAll(): Promise<void> {
    const sel = selection;
    if (isSingleCell(sel)) {
      const r = await sheetApi.currentRegion(sheet, sel.active.row, sel.active.col);
      if (r.r0 !== r.r1 || r.c0 !== r.c1) {
        setSelection({ ...fromRange(r), active: sel.active }, sel.active);
        return;
      }
    }
    setSelection(selectAll(lim), sel.active);
  }

  export function goTo(pos: CellPos): void {
    if (editing) commit('stay');
    setSelection(singleAt(pos));
    focus();
  }

  async function onFilterButton(e: { col: number; x: number; y: number }): Promise<void> {
    if (editing) commit('stay');
    const values = await sheetApi.filterValues(sheet, e.col);
    const base = root.getBoundingClientRect();
    filterMenu = { col: e.col, x: e.x - base.left, y: e.y - base.top + 2, values, title: t('欄 %s', colName(e.col)) };
  }

  async function applyFilter(shown: string[] | null): Promise<void> {
    const col = filterMenu?.col;
    filterMenu = null;
    if (col === undefined) return;
    notify(await serial(() => sheetApi.setFilter(sheet, col, shown ?? [], shown === null)));
    focus();
  }

  async function sortByFilter(desc: boolean): Promise<void> {
    const col = filterMenu?.col;
    filterMenu = null;
    if (col === undefined) return;
    notify(await serial(() => sheetApi.sortFilter(sheet, col, desc)));
    focus();
  }

  function onContext(e: ContextRequest): void {
    if (editing) commit('stay');
    filterMenu = null;
    // Right-clicking outside the selection selects what was clicked.
    const inside = selection.ranges.some((r) => contains(r, e.pos.row, e.pos.col));
    if (!inside || e.area === 'corner') {
      if (e.area === 'row') setSelection(selectRows(selection, e.pos.row, e.pos.row, lim, false, selection.active.col));
      else if (e.area === 'col') setSelection(selectCols(selection, e.pos.col, e.pos.col, lim, false, selection.active.row));
      else if (e.area === 'cell') setSelection(singleAt(e.pos));
    }
    const base = root.getBoundingClientRect();
    contextMenu = { x: e.x - base.left, y: e.y - base.top, area: e.area };
  }

  type MenuItem = { label: string; action: MenuAction; key?: string } | null;
  let contextItems = $derived.by((): MenuItem[] => {
    if (!contextMenu) return [];
    const clip: MenuItem[] = [
      { label: t('剪下'), action: 'cut', key: '⌘X' },
      { label: t('拷貝'), action: 'copy', key: '⌘C' },
      { label: t('貼上'), action: 'paste', key: '⌘V' },
      { label: t('選擇性貼上…'), action: 'pasteSpecial', key: '⌃⌘V' },
      null,
    ];
    if (contextMenu.area === 'row' || contextMenu.area === 'col') {
      const rows = contextMenu.area === 'row';
      return [
        ...clip,
        { label: rows ? t('插入列') : t('插入欄'), action: rows ? 'insertRows' : 'insertCols' },
        { label: rows ? t('刪除列') : t('刪除欄'), action: rows ? 'deleteRows' : 'deleteCols' },
        { label: t('清除內容'), action: 'clearContents' },
        null,
        { label: rows ? t('自動調整列高') : t('自動調整欄寬'), action: rows ? 'autofitRows' : 'autofitCols' },
        { label: t('隱藏'), action: rows ? 'hideRows' : 'hideCols' },
        { label: t('取消隱藏'), action: rows ? 'unhideRows' : 'unhideCols' },
      ];
    }
    const hasNote = notes.has(selection.active.row * 16384 + selection.active.col);
    const noteItems: MenuItem[] = hasNote
      ? [
          { label: t('編輯註解'), action: 'insertNote' },
          { label: t('刪除註解'), action: 'deleteNote' },
        ]
      : [{ label: t('插入註解'), action: 'insertNote', key: '⇧F2' }];
    const tableItems: MenuItem[] = activeTable
      ? [
          null,
          { label: activeTable.totals ? t('移除合計列') : t('加入合計列'), action: 'tableTotals' },
          { label: t('重新命名表格…'), action: 'renameTable' },
          { label: t('轉換為範圍'), action: 'convertTable' },
        ]
      : [];
    return [
      ...clip,
      ...noteItems,
      null,
      { label: t('插入…'), action: 'insertCells', key: '⇧⌘=' },
      { label: t('刪除…'), action: 'deleteCells', key: '⌘-' },
      { label: t('清除內容'), action: 'clearContents' },
      null,
      { label: t('遞增排序'), action: 'sortAsc' },
      { label: t('遞減排序'), action: 'sortDesc' },
      { label: t('篩選'), action: 'filter', key: '⇧⌘F' },
      null,
      { label: t('儲存格格式…'), action: 'formatCells', key: '⌘1' },
      ...tableItems,
    ];
  });

  // Clipboard events reach the focused textarea from the Edit menu
  // (native roles). While idle the grid handles them.
  function onClipboardEvent(e: ClipboardEvent): void {
    if (editing) return;
    e.preventDefault();
    if (e.type === 'paste') void runMenu('paste', e.clipboardData?.getData('text/plain') ?? null);
    else void runMenu(e.type === 'cut' ? 'cut' : 'copy');
  }

  // WebKit enables Copy/Cut in the menu only when there is a selection
  // or the page handles beforecopy/beforecut.
  function onBeforeClipboard(e: Event): void {
    if (!editing) e.preventDefault();
  }

  // ---- Names, validation, notes, tables (v0.6) ----

  // Name box: go to an address or name; an unknown valid name defines
  // a name for the selection, as in Excel.
  export async function goToName(text: string): Promise<void> {
    const t = text.trim();
    if (!t) return focus();
    const target = await documentApi.resolve(sheet, t);
    if (target.ok) {
      if (target.sheet !== sheet) ongoto?.(target.sheet, target.range);
      else setSelection(withMerges(fromRange(target.range)));
      return focus();
    }
    const r = current(selection);
    const err = await documentApi.setName('', -1, {
      name: t,
      refersTo: '=' + absoluteRef(sheetName, r.r0, r.c0, r.r1, r.c1),
      scope: -1,
      comment: '',
    });
    notify(err);
    focus();
  }

  async function openDropdown(): Promise<void> {
    const { row, col } = selection.active;
    const values = await sheetApi.listValues(sheet, row, col);
    if (values.length > 0) dropdown = { values, index: 0 };
  }

  function pickDropdown(value: string): void {
    const pos = selection.active;
    dropdown = null;
    void enqueue(async () => {
      const res = await sheetApi.setCell(sheet, pos.row, pos.col, value);
      notify(res.error ?? res.notice);
    });
    focus();
  }

  function onDropdownKey(e: KeyboardEvent): boolean {
    if (!dropdown) return false;
    const n = dropdown.values.length;
    switch (e.key) {
      case 'ArrowDown':
        dropdown.index = (dropdown.index + 1) % n;
        return true;
      case 'ArrowUp':
        dropdown.index = (dropdown.index - 1 + n) % n;
        return true;
      case 'Enter':
        pickDropdown(dropdown.values[dropdown.index]);
        return true;
      case 'Escape':
        dropdown = null;
        return true;
    }
    return false;
  }

  export function editNote(): void {
    if (editing) commit('stay');
    const { row, col } = selection.active;
    noteEdit = { row, col, text: notes.get(row * 16384 + col)?.text ?? '' };
    hoverNote = null;
    queueMicrotask(() => noteInput?.focus());
  }

  async function saveNote(text: string | null): Promise<void> {
    const n = noteEdit;
    noteEdit = null;
    focus();
    if (!n || text === null) return;
    notify(await serial(() => sheetApi.setNote(sheet, n.row, n.col, text)));
  }

  function onHover(pos: CellPos | null): void {
    const n = pos ? notes.get(pos.row * 16384 + pos.col) : undefined;
    hoverNote = n && !noteEdit ? n : null;
  }

  // Where a popup next to a cell goes (sheet-relative coordinates).
  function besideCell(pos: CellPos): { x: number; y: number; bottom: number } {
    const rc = grid?.areaRect(areaOf(merges, pos)) ?? { x: 0, y: 0, w: 0, h: 0 };
    const top = (grid ? 32 : 0) + rc.y; // below the formula bar
    return { x: rc.x + rc.w + 4, y: top, bottom: top + rc.h };
  }

  async function tableAction(action: 'totals' | 'convert' | 'rename'): Promise<void> {
    const t = activeTable;
    if (!t) return;
    if (action === 'totals') notify(await serial(() => sheetApi.toggleTableTotals(sheet, t.name)));
    else if (action === 'convert') notify(await serial(() => sheetApi.convertTable(sheet, t.name)));
    else onappaction?.('renameTable');
    focus();
  }

  export function getActiveTable(): TableDTO | null {
    return activeTable;
  }

  // ---- Formatting ----

  // Applies a toolbar change to the selection.
  export async function applyFormat(patch: FormatPatch): Promise<string | null> {
    if (editing) commit('stay');
    const ranges = selection.ranges.map(toDTO);
    let err: string | null = null;
    await enqueue(async () => {
      err = await sheetApi.format(sheet, ranges, patch);
    });
    // Bigger fonts and wrapping make rows taller, as in Excel.
    if (!err && (patch.fontSize || patch.wrap || patch.fontName)) await autofitRowsOf(selection.ranges, true);
    focus();
    return err;
  }

  async function toggleFont(attr: 'bold' | 'italic' | 'underline'): Promise<void> {
    const on = !(activeStyle?.font[attr] ?? false);
    const label = { bold: t('粗體'), italic: t('斜體'), underline: t('底線') }[attr];
    await applyFormat({ [attr]: on, label });
  }

  export async function mergeCells(center: boolean): Promise<string | null> {
    if (editing) commit('stay');
    const r = current(selection);
    if (activeMerged) {
      await enqueue(() => sheetApi.unmerge(sheet, selection.ranges.map(toDTO)));
      focus();
      return null;
    }
    let err: string | null = null;
    await enqueue(async () => {
      err = await sheetApi.merge(sheet, toDTO(r), center);
    });
    focus();
    return err;
  }

  export async function clearRanges(mode: 'contents' | 'formats' | 'all'): Promise<void> {
    if (editing) commit('stay');
    copyRange = null;
    await enqueue(() => sheetApi.clear(sheet, selection.ranges.map(toDTO), mode));
    focus();
  }

  // Header border dragged: resize that row/column, or all selected whole
  // rows/columns when it is one of them (Excel).
  function onResize(r: ResizeRequest): void {
    const indices = selectedFull(r.rows).includes(r.index) ? selectedFull(r.rows) : [r.index];
    void enqueue(() => sheetApi.setSizes(sheet, r.rows, indices, r.px));
  }

  // Indices of whole rows/columns in the selection (capped).
  function selectedFull(rows: boolean): number[] {
    const out: number[] = [];
    for (const r of selection.ranges) {
      const full = rows ? r.c0 === 0 && r.c1 === lim.cols - 1 : r.r0 === 0 && r.r1 === lim.rows - 1;
      if (!full) continue;
      const [a, b] = rows ? [r.r0, r.r1] : [r.c0, r.c1];
      for (let i = a; i <= b && out.length < 1000; i++) out.push(i);
    }
    return out;
  }

  let textCache: TextCache | null = null;
  function measurer(): TextCache {
    textCache ??= new TextCache(document.createElement('canvas').getContext('2d')!);
    return textCache;
  }

  // Auto-fit selected columns (or rows) to their content.
  async function autofit(rows: boolean): Promise<void> {
    const list = selectedFull(rows);
    const targets = list.length > 0 ? list : rows ? spanOf(current(selection), true) : spanOf(current(selection), false);
    for (const i of targets.slice(0, 200)) await autofitOne(rows, i);
  }

  function spanOf(r: Range, rows: boolean): number[] {
    const [a, b] = rows ? [r.r0, r.r1] : [r.c0, r.c1];
    return Array.from({ length: Math.min(200, b - a + 1) }, (_, k) => a + k);
  }

  async function autofitRowsOf(ranges: Range[], onlyAuto: boolean): Promise<void> {
    const custom = new Set((layout?.rows ?? []).map((s) => s.i));
    for (const r of ranges) {
      if (r.r1 - r.r0 > 500) continue;
      for (let row = r.r0; row <= r.r1; row++) {
        if (!onlyAuto || !custom.has(row)) await autofitOne(true, row);
      }
    }
  }

  // Excel widens a column of default width when a typed number or date
  // does not fit (instead of showing ####).
  async function widenForNumber(pos: CellPos): Promise<void> {
    if ((layout?.cols ?? []).some((s) => s.i === pos.col)) return; // custom width
    const res = await sheetApi.getBlocks(sheet, [{ row: Math.floor(pos.row / 64), col: Math.floor(pos.col / 32) }], styleStore.known);
    styleStore.merge(res.styles);
    const cell = res.blocks?.[0]?.cells?.find((c) => c.r === pos.row % 64 && c.c === pos.col % 32);
    const st = styleStore.get(cell?.st ?? 0);
    if (!cell?.n || !st) return;
    const want = Math.ceil(measurer().width(st.font, cell.v) + 10);
    if (want > (grid?.colWidth(pos.col) ?? 0)) await enqueue(() => sheetApi.setSizes(sheet, false, [pos.col], want));
  }

  async function autofitOne(rows: boolean, index: number): Promise<void> {
    const m = measurer();
    let size: number;
    if (rows) {
      const samples = await sheetApi.rowTexts(sheet, index);
      size = layout?.defaultRow ?? 20;
      for (const t of samples) {
        const st = styleStore.get(t.st);
        if (!st) continue;
        const lineH = st.fontPx * LINE_HEIGHT;
        const lines = st.dto.wrap ? m.wrap(st.font, t.v, Math.max(1, (grid?.colWidth(t.col) ?? 64) - 6)).length : 1;
        size = Math.max(size, Math.ceil(lines * lineH + 5));
      }
      if (size === grid?.rowHeight(index)) return;
    } else {
      const samples = await sheetApi.autoFitColumn(sheet, index);
      if (samples.length === 0) {
        size = -1; // back to the default width
      } else {
        size = 0;
        for (const t of samples) {
          const st = styleStore.get(t.st);
          if (!st) continue;
          const widest = Math.max(...t.v.split('\n').map((line) => m.width(st.font, line)));
          size = Math.max(size, Math.ceil(widest + 10));
        }
      }
    }
    await enqueue(() => sheetApi.setSizes(sheet, rows, [index], size));
  }

  export function getSelection(): Selection {
    return selection;
  }

  // ---- Test / bench hooks ----

  export function getGrid() {
    return grid;
  }

  export function getEditing() {
    return editing;
  }

  export function getAssist() {
    return { suggestions: suggestions.map((s) => s.name), hint: hint?.info.name ?? null, error: editError, highlights };
  }

  export function getBarText(): string {
    return bar.value;
  }

  // Simulated input for the self-test: key events go through the same
  // handlers as real ones; text is inserted like a keyboard would.
  export function pressKey(key: string, mods: Partial<KeyboardEventInit> = {}): void {
    input().dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...mods }));
  }

  export function typeText(text: string): void {
    const el = input();
    for (const ch of text) {
      el.setRangeText(ch, el.selectionStart, el.selectionEnd, 'end');
      el.dispatchEvent(new InputEvent('input', { data: ch, inputType: 'insertText', bubbles: true }));
    }
  }

  export function clickCell(pos: CellPos, kind: GridPointer['kind'] = 'down'): void {
    onPointer({ kind, area: 'cell', pos, shift: false, meta: false });
  }

  export function focusBar(): void {
    bar.focus();
  }

  export function setSelectionForTest(sel: Selection): void {
    setSelection(sel);
  }

  export function getCopyRange(): Range | null {
    return copyRange;
  }

  export function getLayout(): LayoutDTO | null {
    return layout;
  }

  // Drives the fill handle like a mouse drag from the selection corner.
  export function dragFillHandle(to: CellPos): void {
    onPointer({ kind: 'down', area: 'fill', pos: selection.active, shift: false, meta: false });
    onPointer({ kind: 'drag', area: 'fill', pos: to, shift: false, meta: false });
    onPointer({ kind: 'up', area: 'fill', pos: to, shift: false, meta: false });
  }
</script>

<div class="sheet" bind:this={root}>
  <header class="formula-bar">
    <input
      bind:this={nameBox}
      class="name-box"
      aria-label={t('名稱方塊')}
      list="defined-names"
      spellcheck="false"
      value={nameBoxFocused ? nameBoxText : address}
      oninput={(e) => (nameBoxText = e.currentTarget.value)}
      onfocus={(e) => {
        nameBoxFocused = true;
        nameBoxText = address;
        if (editing) commit('stay');
        e.currentTarget.select();
      }}
      onblur={() => (nameBoxFocused = false)}
      onkeydown={(e) => {
        if (e.key === 'Enter' && !e.isComposing) {
          e.preventDefault();
          void goToName(nameBoxText);
        } else if (e.key === 'Escape') {
          e.preventDefault();
          focus();
        }
      }}
    />
    <datalist id="defined-names">
      {#each names as n (n)}<option value={n}></option>{/each}
    </datalist>
    <div class="fx">fx</div>
    <textarea
      bind:this={bar}
      class="formula"
      rows="1"
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      aria-label={t('公式列')}
      onfocus={onBarFocus}
      onkeydown={onKeyDown}
      oninput={onInput}
      onkeyup={updateAssist}
      onpointerup={updateAssist}
    ></textarea>
  </header>

  <Grid
    bind:this={grid}
    {sheet}
    {zoom}
    painting={paintMode !== 'off'}
    {info}
    {layout}
    {selection}
    {highlights}
    onpointer={onPointer}
    onresize={onResize}
    onautofit={(r) => void autofitOne(r.rows, r.index)}
    onviewport={() => {
      layoutEditor();
      viewTick++;
    }}
    copyRange={paintSource ?? copyRange}
    {fillPreview}
    onfilterbutton={(e) => void onFilterButton(e)}
    oncontext={onContext}
    onhover={onHover}
  >
    {#if objects.length > 0}
      <ObjectLayer
        {objects}
        {scale}
        rectOf={objectRect}
        tick={viewTick}
        left={metrics.headerW * scale}
        top={HEADER_H * scale}
        selected={selectedObject}
        {chartData}
        {images}
        onselect={(id) => {
          if (editing) commit('stay');
          selectedObject = id;
          focus();
        }}
        oncommit={commitObject}
        onedit={(id) => {
          selectedObject = id;
          onappaction?.('editChart');
        }}
      />
    {/if}
    <textarea
      bind:this={editor}
      class="cell-editor"
      class:active={editing !== null}
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      aria-label={t('儲存格編輯器')}
      onfocus={onEditorFocus}
      onkeydown={onKeyDown}
      oninput={onInput}
      onkeyup={updateAssist}
      onpointerup={updateAssist}
      oncompositionstart={onCompositionStart}
    ></textarea>
  </Grid>
  <div class="sr-only" role="status" aria-live="polite">{announcement}</div>

  {#if contextMenu}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="menu-backdrop"
      onpointerdown={() => {
        contextMenu = null;
        focus();
      }}
      oncontextmenu={(e) => {
        e.preventDefault();
        contextMenu = null;
      }}
    ></div>
    <ul class="context-menu" role="menu" style:left="{contextMenu.x}px" style:top="{contextMenu.y}px">
      {#each contextItems as item, i (i)}
        {#if item}
          <li role="none">
            <button
              role="menuitem"
              onpointerdown={(e) => e.preventDefault()}
              onclick={() => {
                const a = item.action;
                contextMenu = null;
                if (a === 'formatCells') onappaction?.(a);
                else void runMenu(a);
                if (a !== 'insertNote') focus();
              }}
            >
              <span>{item.label}</span>
              {#if item.key}<kbd>{item.key}</kbd>{/if}
            </button>
          </li>
        {:else}
          <li class="sep" role="separator"></li>
        {/if}
      {/each}
    </ul>
  {/if}

  {#if filterMenu}
    <FilterMenu
      x={filterMenu.x}
      y={filterMenu.y}
      title={filterMenu.title}
      values={filterMenu.values}
      onsort={(desc) => void sortByFilter(desc)}
      onapply={(shown) => void applyFilter(shown)}
      onclose={() => {
        filterMenu = null;
        focus();
      }}
    />
  {/if}

  {#if validation?.type === 'list' && validation.dropdown && !editing && isSingleCell(selection)}
    {@const at = besideCell(selection.active)}
    <button
      class="dropdown-button"
      style:left="{at.x - 2}px"
      style:top="{at.bottom - 18}px"
      aria-label={t('選擇清單中的值')}
      onpointerdown={(e) => {
        e.preventDefault();
        if (dropdown) dropdown = null;
        else void openDropdown();
      }}>▾</button
    >
  {/if}
  {#if dropdown}
    {@const at = besideCell(selection.active)}
    <ul class="dropdown-list" role="listbox" aria-label={t('清單')} style:left="{at.x - 120}px" style:top="{at.bottom + 1}px">
      {#each dropdown.values as v, i (i)}
        <li
          role="option"
          aria-selected={i === dropdown.index}
          class:selected={i === dropdown.index}
          onpointerdown={(e) => {
            e.preventDefault();
            pickDropdown(v);
          }}
        >
          {v || t('（空白）')}
        </li>
      {/each}
    </ul>
  {/if}
  {#if (validation?.prompt || validation?.promptTitle) && !editing && !dropdown && isSingleCell(selection)}
    {@const at = besideCell(selection.active)}
    <div class="cell-tip prompt" style:left="{at.x + 14}px" style:top="{at.bottom + 4}px">
      {#if validation.promptTitle}<strong>{validation.promptTitle}</strong>{/if}
      <span>{validation.prompt}</span>
    </div>
  {/if}
  {#if hoverNote && !noteEdit}
    {@const at = besideCell({ row: hoverNote.row, col: hoverNote.col })}
    <div class="cell-tip note" style:left="{at.x + 6}px" style:top="{at.y}px">
      {#if hoverNote.author}<strong>{hoverNote.author}：</strong>{/if}
      <span>{hoverNote.text}</span>
    </div>
  {/if}
  {#if noteEdit}
    {@const at = besideCell({ row: noteEdit.row, col: noteEdit.col })}
    <div class="cell-tip note editor" style:left="{at.x + 6}px" style:top="{at.y}px">
      <textarea
        bind:this={noteInput}
        bind:value={noteEdit.text}
        aria-label={t('註解')}
        rows="4"
        onkeydown={(e) => {
          if (e.key === 'Escape') {
            e.preventDefault();
            void saveNote(null);
          } else if (e.key === 'Enter' && e.metaKey) {
            e.preventDefault();
            void saveNote(noteEdit?.text ?? '');
          }
        }}
      ></textarea>
      <div class="note-actions">
        <button onclick={() => void saveNote(null)}>{t('取消')}</button>
        <button class="primary" onclick={() => void saveNote(noteEdit?.text ?? '')}>{t('儲存')}</button>
      </div>
    </div>
  {/if}

  {#if editing && (editError || suggestions.length > 0 || hint)}
    <div class="popups" style:left="{popupPos.x}px" style:top="{popupPos.y}px">
      {#if editError}
        <div class="popup error" role="alert">
          <strong>{editText.startsWith('=') ? t('公式有問題') : t('輸入的值無效')}</strong>
          <span>{editError}</span>
        </div>
      {/if}
      {#if hint && !editError}
        <div class="popup hint">
          <span class="fname">{hint.info.name}</span>(<!--
          -->{#each hintParts(hint.info.sig, hint.arg) as part, i (i)}{#if i > 0}, {/if}<span class:current={part.active}
              >{part.text}</span
            >{/each})
        </div>
      {/if}
      {#if suggestions.length > 0 && !editError}
        <ul class="popup suggestions" role="listbox" aria-label={t('函數')}>
          {#each suggestions as f, i (f.name)}
            <li
              role="option"
              aria-selected={i === suggestIndex}
              class:selected={i === suggestIndex}
              onpointerdown={(e) => {
                e.preventDefault();
                acceptSuggestion(i);
              }}
            >
              <span class="fname">{f.name}</span>
              <span class="desc">{f.desc}</span>
            </li>
          {/each}
        </ul>
      {/if}
    </div>
  {/if}
</div>

<style>
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  .sheet {
    position: relative;
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
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
    height: 22px;
    resize: none;
    overflow: hidden;
    white-space: pre;
    font: 13px var(--cell-font);
    line-height: 16px;
    padding: 2px 6px;
    border: 1px solid var(--chrome-border);
    border-radius: 4px;
    background: var(--grid-bg);
    color: var(--chrome-text);
    outline: none;
    -webkit-user-select: text;
    user-select: text;
  }
  .formula:focus {
    border-color: var(--accent);
  }
  .cell-editor {
    position: absolute;
    z-index: 2;
    margin: 0;
    padding: 1px 3px 0 4px;
    border: 2px solid var(--accent);
    border-radius: 0;
    outline: none;
    resize: none;
    overflow: hidden;
    background: var(--grid-bg);
    color: var(--cell-text);
    font: 13px var(--cell-font);
    line-height: 17px;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    /* Idle: invisible but focused, so typing and IME start here. */
    opacity: 0;
    caret-color: transparent;
    pointer-events: none;
  }
  .cell-editor.active {
    opacity: 1;
    caret-color: auto;
    pointer-events: auto;
    box-shadow: 0 1px 4px rgb(0 0 0 / 0.15);
  }
  .popups {
    position: absolute;
    z-index: 10;
    display: flex;
    flex-direction: column;
    gap: 4px;
    max-width: 420px;
    pointer-events: none;
  }
  .popup {
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    border-radius: 6px;
    box-shadow: 0 4px 14px rgb(0 0 0 / 0.18);
    font: 12px var(--ui-font);
    color: var(--chrome-text);
    pointer-events: auto;
  }
  .hint {
    padding: 4px 8px;
    white-space: nowrap;
  }
  .hint .current {
    font-weight: 700;
  }
  .error {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 6px 10px;
    border-color: var(--error-fg);
    color: var(--error-fg);
  }
  .suggestions {
    list-style: none;
    margin: 0;
    padding: 4px 0;
    max-height: 240px;
    overflow: auto;
  }
  .suggestions li {
    display: flex;
    gap: 10px;
    padding: 3px 10px;
    cursor: default;
  }
  .suggestions li.selected {
    background: var(--popup-active);
  }
  .fname {
    font-weight: 600;
    color: var(--accent);
  }
  .name-box {
    font: inherit;
    color: var(--chrome-text);
    background: var(--grid-bg);
    outline: none;
  }
  .name-box:focus {
    border-color: var(--accent);
  }
  .dropdown-button {
    position: absolute;
    z-index: 3;
    width: 18px;
    height: 18px;
    padding: 0;
    border: 1px solid var(--chrome-border);
    border-radius: 2px;
    background: var(--header-bg);
    color: var(--chrome-text);
    font-size: 11px;
    line-height: 16px;
  }
  .dropdown-list {
    position: absolute;
    z-index: 15;
    min-width: 136px;
    max-height: 220px;
    margin: 0;
    padding: 2px 0;
    overflow: auto;
    list-style: none;
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    box-shadow: 0 4px 14px rgb(0 0 0 / 0.18);
    font: 13px var(--ui-font);
  }
  .dropdown-list li {
    padding: 2px 10px;
    cursor: default;
  }
  .dropdown-list li.selected {
    background: var(--popup-active);
  }
  .cell-tip {
    position: absolute;
    z-index: 12;
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-width: 240px;
    padding: 6px 8px;
    font: 12px var(--ui-font);
    white-space: pre-wrap;
    color: #1f1f1f;
    background: #fff9c4;
    border: 1px solid #c9b458;
    box-shadow: 0 3px 10px rgb(0 0 0 / 0.18);
    pointer-events: none;
  }
  .cell-tip.prompt {
    background: #ffffe1;
  }
  .cell-tip.editor {
    pointer-events: auto;
    width: 220px;
  }
  .cell-tip textarea {
    width: 100%;
    box-sizing: border-box;
    resize: vertical;
    font: inherit;
    color: inherit;
    background: transparent;
    border: 0;
    outline: none;
    -webkit-user-select: text;
    user-select: text;
  }
  .note-actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }
  .note-actions button {
    font: inherit;
    padding: 1px 10px;
    border: 1px solid #c9b458;
    border-radius: 4px;
    background: #fffde7;
  }
  .note-actions .primary {
    background: var(--accent);
    border-color: var(--accent);
    color: #fff;
  }
  .menu-backdrop {
    position: fixed;
    inset: 0;
    z-index: 20;
  }
  .context-menu {
    position: absolute;
    z-index: 21;
    min-width: 200px;
    margin: 0;
    padding: 4px;
    list-style: none;
    background: var(--popup-bg);
    border: 1px solid var(--popup-border);
    border-radius: 7px;
    box-shadow: 0 6px 20px rgb(0 0 0 / 0.2);
    font: 13px var(--ui-font);
  }
  .context-menu button {
    display: flex;
    width: 100%;
    justify-content: space-between;
    gap: 16px;
    padding: 3px 10px;
    border: 0;
    border-radius: 4px;
    background: none;
    color: var(--chrome-text);
    font: inherit;
    text-align: left;
  }
  .context-menu button:hover {
    background: var(--accent);
    color: #fff;
  }
  .context-menu kbd {
    font: inherit;
    opacity: 0.6;
  }
  .context-menu .sep {
    height: 1px;
    margin: 4px 6px;
    background: var(--popup-border);
  }
  .desc {
    color: var(--chrome-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
