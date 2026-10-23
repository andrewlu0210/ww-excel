<script lang="ts">
  // Sheet controller: selection, keyboard, the cell editor and the link
  // to Go. Rendering lives in Grid.
  //
  // The editor is a single <textarea> that always holds keyboard focus,
  // invisible while idle and positioned on the active cell. Typing (or an
  // IME composition) therefore starts in the textarea itself, so the very
  // first keystroke of 注音/倉頡 is captured and the candidate window
  // appears next to the cell.
  import { onMount, untrack } from 'svelte';
  import { documentApi, sheetApi, type CellsChanged, type RangeDTO, type SheetInfo } from '../bridge/sheet';
  import { HEADER_W } from '../grid/renderer';
  import {
    clampPos,
    current,
    cycleActive,
    extendTo,
    fromRange,
    isSingleCell,
    rangeOf,
    selectAll,
    selectCols,
    selectRows,
    single,
    type CellPos,
    type Range,
    type Selection,
  } from '../grid/selection';
  import { editAction, idleAction, type Action, type EditMode, type MenuAction, type SheetMode } from '../input/keymap';
  import type { GridPointer } from '../input/pointer';
  import Grid from './Grid.svelte';

  interface Props {
    sheet: number;
    info: SheetInfo;
    // Outputs for the formula bar and status bar.
    mode?: SheetMode;
    formulaText?: string;
    activeCell?: CellPos;
  }

  let {
    sheet,
    info,
    mode = $bindable('ready'),
    formulaText = $bindable(''),
    activeCell = $bindable({ row: 0, col: 0 }),
  }: Props = $props();

  // Fixed for the lifetime of the component: the grid is always full size.
  const lim = untrack(() => ({ rows: info.rows, cols: info.cols }));

  let grid = $state<ReturnType<typeof Grid>>();
  let editor: HTMLTextAreaElement;
  let selection = $state<Selection>(single({ row: 0, col: 0 }));
  let editing = $state<{ mode: EditMode; pos: CellPos } | null>(null);
  let used = untrack(() => ({ rows: info.usedRows, cols: info.usedCols }));
  // Column where a run of Tabs started; Enter returns to it (Excel).
  let tabStartCol: number | null = null;

  $effect(() => {
    mode = editing ? editing.mode : 'ready';
  });
  $effect(() => {
    activeCell = selection.active;
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

  // ---- Selection ----

  function setSelection(sel: Selection, reveal: CellPos = sel.active): void {
    selection = sel;
    grid?.scrollIntoView(reveal);
    refreshFormula();
  }

  function moved(pos: CellPos, extend: boolean): void {
    const p = clampPos(pos, lim);
    if (extend) setSelection(extendTo(selection, p), p);
    else setSelection(single(p));
  }

  let formulaToken = 0;
  function refreshFormula(): void {
    if (editing) {
      formulaText = editor.value;
      return;
    }
    const token = ++formulaToken;
    const { row, col } = selection.active;
    sheetApi.editText(sheet, row, col).then((text) => {
      if (token === formulaToken && !editing) formulaText = text;
    });
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
          moved({ row: base.row + a.dr, col: base.col + a.dc }, a.extend);
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
      case 'enter':
        advance('down', a.backward);
        break;
      case 'tab':
        advance('right', a.backward);
        break;
      case 'beginEdit':
        await beginEdit('edit');
        break;
      case 'clear':
        await sheetApi.clear(sheet, sel.ranges.map(toDTO));
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
    const { row, col } = sel.active;
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
  export async function runMenu(action: MenuAction): Promise<void> {
    const now = performance.now();
    if (lastMenu.action === action && now - lastMenu.at < 150) return;
    lastMenu = { action, at: now };

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
        if (editing) editor.select();
        else setSelection(selectAll(lim), selection.active);
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
    editor.value = text;
    editor.setSelectionRange(text.length, text.length);
    startEditing(m, pos);
  }

  function startEditing(m: EditMode, pos: CellPos): void {
    editing = { mode: m, pos };
    grid?.scrollIntoView(pos);
    layoutEditor();
    refreshFormula();
  }

  function endEditing(): void {
    editing = null;
    editor.value = '';
    layoutEditor();
    refreshFormula();
  }

  function commit(then: 'down' | 'up' | 'right' | 'left' | 'stay'): void {
    if (!editing) return;
    const { pos } = editing;
    const text = editor.value;
    endEditing();
    enqueue(() => sheetApi.setCell(sheet, pos.row, pos.col, text));
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

  function performEdit(a: Action): void {
    switch (a.type) {
      case 'commit':
        commit(a.then);
        break;
      case 'commitFill': {
        const text = editor.value;
        const ranges = selection.ranges.map(toDTO);
        endEditing();
        enqueue(async () => {
          try {
            await sheetApi.fill(sheet, ranges, text);
          } catch (err) {
            console.warn('fill rejected', err);
          }
        });
        break;
      }
      case 'cancel':
        endEditing();
        break;
      case 'newline':
        editor.setRangeText('\n', editor.selectionStart, editor.selectionEnd, 'end');
        onInput();
        break;
      case 'toggleMode':
        if (editing) editing = { ...editing, mode: editing.mode === 'enter' ? 'edit' : 'enter' };
        break;
      case 'menu':
        void runMenu(a.action);
        break;
    }
  }

  function onKeyDown(e: KeyboardEvent): void {
    if (editing) {
      const a = editAction(e, editing.mode);
      if (!a) return; // normal text editing
      e.preventDefault();
      performEdit(a);
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
  // the cell content.
  function onInput(): void {
    if (!editing) {
      startEditing('enter', selection.active);
      return;
    }
    layoutEditor();
    formulaText = editor.value;
  }

  function onCompositionStart(): void {
    if (!editing) startEditing('enter', selection.active);
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
    const pos = editing?.pos ?? selection.active;
    const rc = grid.cellRect(pos);
    const vp = grid.viewport();
    const st = editor.style;
    st.left = `${Math.max(rc.x, HEADER_W) - 1}px`;
    st.top = `${rc.y - 1}px`;
    if (!editing) {
      st.width = `${rc.w + 1}px`;
      st.height = `${rc.h + 1}px`;
      return;
    }
    const maxW = Math.max(rc.w + 1, vp.width - rc.x - 2);
    const want = textWidth(editor.value) + 12;
    st.width = `${Math.min(maxW, Math.max(rc.w + 1, want))}px`;
    st.height = 'auto';
    st.height = `${Math.max(rc.h + 1, editor.scrollHeight + 2)}px`;
  }

  // ---- Pointer ----

  let dragBase: Range[] = [];
  let dragAnchor = 0;

  function onPointer(e: GridPointer): void {
    if (e.kind === 'down' && editing) commit('stay');
    tabStartCol = null;
    const sel = selection;

    switch (e.area) {
      case 'corner':
        if (e.kind === 'down') setSelection(selectAll(lim), sel.active);
        break;
      case 'cell':
        if (e.kind === 'dbl') {
          void enqueue(() => beginEdit('edit'));
        } else if (e.kind === 'down') {
          if (e.shift) {
            dragBase = sel.ranges.slice(0, -1);
            setSelection(extendTo(sel, e.pos), e.pos);
          } else {
            dragBase = e.meta ? sel.ranges : [];
            const s = single(e.pos);
            setSelection({ ...s, ranges: [...dragBase, ...s.ranges] });
          }
        } else if (e.kind === 'drag') {
          setSelection({ ...sel, focus: e.pos, ranges: [...dragBase, rangeOf(sel.anchor, e.pos)] }, e.pos);
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
          setSelection({ ...s, active, ranges: [...dragBase, ...s.ranges] }, isCol ? { row: sel.active.row, col: idx } : { row: idx, col: sel.active.col });
        }
        break;
      }
    }
  }

  // ---- Backend events ----

  function onCellsChanged(d: CellsChanged): void {
    if (d.sheet !== sheet) return;
    grid?.cache.applyDiff(d);
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

  onMount(() => {
    const offs = [sheetApi.onCellsChanged(onCellsChanged), documentApi.onReset(() => void onReset())];
    focus();
    refreshFormula();
    return () => offs.forEach((off) => off());
  });

  // ---- Test / bench hooks ----

  export function getGrid() {
    return grid;
  }

  export function getSelection(): Selection {
    return selection;
  }

  export function getEditing() {
    return editing;
  }

  // Simulated input for the self-test: key events go through the same
  // handlers as real ones; text is inserted like a keyboard would.
  export function pressKey(key: string, mods: Partial<KeyboardEventInit> = {}): void {
    editor.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...mods }));
  }

  export function typeText(text: string): void {
    for (const ch of text) {
      editor.setRangeText(ch, editor.selectionStart, editor.selectionEnd, 'end');
      editor.dispatchEvent(new InputEvent('input', { data: ch, inputType: 'insertText', bubbles: true }));
    }
  }

  export function setSelectionForTest(sel: Selection): void {
    setSelection(sel);
  }
</script>

<Grid bind:this={grid} {sheet} {info} {selection} onpointer={onPointer} onviewport={layoutEditor}>
  <textarea
    bind:this={editor}
    class="cell-editor"
    class:active={editing !== null}
    spellcheck="false"
    autocomplete="off"
    autocapitalize="off"
    aria-label="儲存格編輯器"
    onkeydown={onKeyDown}
    oninput={onInput}
    oncompositionstart={onCompositionStart}
  ></textarea>
</Grid>

<style>
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
</style>
