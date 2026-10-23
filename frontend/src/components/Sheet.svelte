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
  import { onMount, untrack } from 'svelte';
  import { documentApi, sheetApi, type CellsChanged, type FuncInfo, type RangeDTO, type SheetInfo } from '../bridge/sheet';
  import { callAt, canPoint, findRefs, refText, toggleAbsolute, wordAt } from '../formula/refs';
  import { cellAddress } from '../grid/layout';
  import { HEADER_W, REF_COLORS, type Highlight } from '../grid/renderer';
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
  import {
    arrowCommit,
    editAction,
    idleAction,
    type Action,
    type EditMode,
    type MenuAction,
    type SheetMode,
  } from '../input/keymap';
  import type { GridPointer } from '../input/pointer';
  import Grid from './Grid.svelte';

  interface Props {
    sheet: number;
    info: SheetInfo;
    // Output for the status bar.
    mode?: SheetMode;
  }

  let { sheet, info, mode = $bindable('ready') }: Props = $props();

  // Fixed for the lifetime of the component: the grid is always full size.
  const lim = untrack(() => ({ rows: info.rows, cols: info.cols }));

  let grid = $state<ReturnType<typeof Grid>>();
  let root: HTMLDivElement;
  let editor: HTMLTextAreaElement;
  let bar: HTMLTextAreaElement;
  let selection = $state<Selection>(single({ row: 0, col: 0 }));
  let editing = $state<{ mode: EditMode; pos: CellPos } | null>(null);
  let activeInput: 'cell' | 'bar' = 'cell';
  // Mirror of the text being edited, for highlights and popups.
  let editText = $state('');
  let used = untrack(() => ({ rows: info.usedRows, cols: info.usedCols }));
  // Column where a run of Tabs started; Enter returns to it (Excel).
  let tabStartCol: number | null = null;

  // Point mode: the reference being inserted by arrows or the mouse.
  let point = $state<{ start: number; end: number; anchor: CellPos; focus: CellPos } | null>(null);

  // Function autocomplete and parameter hint.
  let functions: FuncInfo[] = [];
  let funcByName = new Map<string, FuncInfo>();
  let suggestions = $state<FuncInfo[]>([]);
  let suggestIndex = $state(0);
  let hint = $state<{ info: FuncInfo; arg: number } | null>(null);
  let editError = $state<string | null>(null);
  let popupPos = $state({ x: 0, y: 0 });

  $effect(() => {
    mode = !editing ? 'ready' : point ? 'point' : editing.mode;
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

  // While idle the formula bar shows the active cell's edit text.
  let formulaToken = 0;
  function refreshFormula(): void {
    if (editing || !bar) return;
    const token = ++formulaToken;
    const { row, col } = selection.active;
    sheetApi.editText(sheet, row, col).then((text) => {
      if (token === formulaToken && !editing) bar.value = text;
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
        if (editing) input().select();
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
    startEditing(m, pos, text, text.length);
  }

  function startEditing(m: EditMode, pos: CellPos, text?: string, caret?: number): void {
    editing = { mode: m, pos };
    editError = null;
    point = null;
    if (text !== undefined) setText(text, caret ?? text.length);
    else syncFrom(input());
    grid?.scrollIntoView(pos);
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
    const { pos } = editing;
    const text = input().value;
    endEditing();
    enqueue(async () => {
      const err = await sheetApi.setCell(sheet, pos.row, pos.col, text);
      if (err) restoreEdit(pos, text, err);
    });
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
    const ref = refText(anchor, focusPos);
    const text = el.value.slice(0, start) + ref + el.value.slice(end);
    point = { start, end: start + ref.length, anchor, focus: focusPos };
    setText(text, start + ref.length);
    grid?.scrollIntoView(focusPos);
  }

  function pointArrow(a: { dr: number; dc: number; extend: boolean }): boolean {
    if (!editing || !pointable()) return false;
    // In Edit mode (F2) arrows move the caret unless already pointing.
    if (!point && editing.mode === 'edit') return false;
    const base = point ? point.focus : editing.pos;
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
    if (editing) {
      if (suggestions.length > 0 && handleSuggestionKey(e)) {
        e.preventDefault();
        return;
      }
      const a = editAction(e);
      if (a && performEdit(a)) e.preventDefault();
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
    const next = w ? functions.filter((f) => f.name.startsWith(prefix)).slice(0, 10) : [];
    const exact = next.length === 1 && next[0].name === prefix;
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
    const text = el.value.slice(0, w.start) + f.name + '(' + el.value.slice(el.selectionStart);
    point = null;
    setText(text, w.start + f.name.length + 1);
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
          setSelection(
            { ...s, active, ranges: [...dragBase, ...s.ranges] },
            isCol ? { row: sel.active.row, col: idx } : { row: idx, col: sel.active.col },
          );
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
    sheetApi.functions().then((list) => {
      functions = list;
      funcByName = new Map(list.map((f) => [f.name, f]));
    });
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
</script>

<div class="sheet" bind:this={root}>
  <header class="formula-bar">
    <div class="name-box">{address}</div>
    <div class="fx">fx</div>
    <textarea
      bind:this={bar}
      class="formula"
      rows="1"
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      aria-label="公式列"
      onfocus={onBarFocus}
      onkeydown={onKeyDown}
      oninput={onInput}
      onkeyup={updateAssist}
      onpointerup={updateAssist}
    ></textarea>
  </header>

  <Grid bind:this={grid} {sheet} {info} {selection} {highlights} onpointer={onPointer} onviewport={layoutEditor}>
    <textarea
      bind:this={editor}
      class="cell-editor"
      class:active={editing !== null}
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      aria-label="儲存格編輯器"
      onfocus={onEditorFocus}
      onkeydown={onKeyDown}
      oninput={onInput}
      onkeyup={updateAssist}
      onpointerup={updateAssist}
      oncompositionstart={onCompositionStart}
    ></textarea>
  </Grid>

  {#if editing && (editError || suggestions.length > 0 || hint)}
    <div class="popups" style:left="{popupPos.x}px" style:top="{popupPos.y}px">
      {#if editError}
        <div class="popup error" role="alert">
          <strong>公式有問題</strong>
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
        <ul class="popup suggestions" role="listbox" aria-label="函數">
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
  .desc {
    color: var(--chrome-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
