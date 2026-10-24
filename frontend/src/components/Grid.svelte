<script lang="ts">
  import { onMount, untrack, type Snippet } from 'svelte';
  import { sheetApi, type LayoutDTO, type SheetInfo } from '../bridge/sheet';
  import { AxisLayout } from '../grid/layout';
  import { Samples, type ScrollBenchResult } from '../grid/perf';
  import {
    emptyLook,
    GridRenderer,
    HEADER_H,
    headerWidthFor,
    metrics,
    rangeRect,
    SelectionRenderer,
    type Highlight,
    type Rect,
    type SheetLook,
  } from '../grid/renderer';
  import { ScrollAxis } from '../grid/scroll';
  import type { CellPos, Range, Selection } from '../grid/selection';
  import type { GridPointer, PointerArea, ResizeRequest } from '../input/pointer';
  import { styleStore } from '../stores/styles';
  import { BlockCache } from '../stores/viewportCache';

  interface Props {
    sheet: number;
    info: SheetInfo;
    layout: LayoutDTO | null;
    selection: Selection;
    // Reference boxes while editing a formula.
    highlights?: Highlight[];
    onpointer?: (e: GridPointer) => void;
    // Header border dragged (resize) or double-clicked (auto-fit).
    onresize?: (r: ResizeRequest) => void;
    onautofit?: (r: { rows: boolean; index: number }) => void;
    // Called after every redraw, e.g. to keep the editor on its cell.
    onviewport?: () => void;
    children?: Snippet;
  }

  let { sheet, info, layout, selection, highlights = [], onpointer, onresize, onautofit, onviewport, children }: Props =
    $props();

  // Layout and cache are created once per mounted sheet; later prop
  // changes do not rebuild them.
  const geo = untrack(() => ({ ...info, sheet }));
  const rows = new AxisLayout(geo.rows, 20);
  const cols = new AxisLayout(geo.cols, 64);
  export const cache = new BlockCache(
    async (refs) => {
      const res = await sheetApi.getBlocks(geo.sheet, refs, styleStore.known);
      styleStore.merge(res.styles);
      return res;
    },
    geo.blockRows,
    geo.blockCols,
    Math.ceil(geo.rows / geo.blockRows) - 1,
    Math.ceil(geo.cols / geo.blockCols) - 1,
  );
  // Draw cost of the grid layer per frame (ms).
  export const drawTimes = new Samples(240);

  const scrollY = new ScrollAxis();
  const scrollX = new ScrollAxis();

  let host: HTMLDivElement;
  let scroller: HTMLDivElement;
  let gridCanvas: HTMLCanvasElement;
  let selCanvas: HTMLCanvasElement;
  let spacerW = $state(0);
  let spacerH = $state(0);

  let gridLayer: GridRenderer | undefined;
  let selLayer: SelectionRenderer | undefined;
  let look: SheetLook = emptyLook();
  let sx = 0;
  let sy = 0;
  let raf = 0;
  let benchRunning = false;
  // Set on unmount: fetches that finish later must not draw.
  let destroyed = false;

  function schedule(): void {
    if (!raf && !destroyed) raf = requestAnimationFrame(frame);
  }

  function frame(): void {
    raf = 0;
    drawNow();
  }

  function drawNow(): void {
    if (!gridLayer || !selLayer || destroyed) return;
    // Widen the row header for the row numbers on screen.
    const hw = headerWidthFor(rows.indexAt(sy + viewH() - HEADER_H));
    if (hw !== metrics.headerW) {
      metrics.headerW = hw;
      updateSpacer();
    }
    drawTimes.push(gridLayer.draw(sx, sy, selection));
    selLayer.draw(sx, sy, selection, highlights);
    onviewport?.();
  }

  function viewW(): number {
    return scroller.clientWidth;
  }

  function viewH(): number {
    return scroller.clientHeight;
  }

  function updateSpacer(): void {
    scrollY.update(HEADER_H + rows.total, viewH());
    scrollX.update(metrics.headerW + cols.total, viewW());
    spacerH = scrollY.spacer;
    spacerW = scrollX.spacer;
  }

  function resize(): void {
    // Exclude space taken by non-overlay scrollbars.
    const w = viewW();
    const h = viewH();
    const dpr = window.devicePixelRatio || 1;
    gridLayer?.resize(w, h, dpr);
    selLayer?.resize(w, h, dpr);
    updateSpacer();
    drawNow();
  }

  function onScroll(): void {
    if (benchRunning) return;
    sx = scrollX.toContent(scroller.scrollLeft);
    sy = scrollY.toContent(scroller.scrollTop);
    schedule();
  }

  function setScroll(x: number, y: number): void {
    sx = clamp(x, cols.total - (viewW() - metrics.headerW));
    sy = clamp(y, rows.total - (viewH() - HEADER_H));
    scroller.scrollLeft = scrollX.toScroll(sx);
    scroller.scrollTop = scrollY.toScroll(sy);
    schedule();
  }

  function clamp(v: number, max: number): number {
    return Math.max(0, Math.min(v, max));
  }

  // Applies the sheet layout from Go: sizes, row/column styles, merges.
  function applyLayout(l: LayoutDTO): void {
    styleStore.merge(l.styles);
    rows.reset(l.defaultRow, (l.rows ?? []).map((s): [number, number] => [s.i, s.hidden ? 0 : s.px]));
    cols.reset(l.defaultCol, (l.cols ?? []).map((s): [number, number] => [s.i, s.hidden ? 0 : s.px]));
    look = {
      rowStyles: new Map((l.rowStyles ?? []).map((s) => [s.i, s.s])),
      colStyles: new Map((l.colStyles ?? []).map((s) => [s.i, s.s])),
      merges: (l.merges ?? []).map((m) => ({ r0: m.r0, c0: m.c0, r1: m.r1, c1: m.c1 })),
    };
    if (gridLayer) gridLayer.look = look;
    if (selLayer) selLayer.merges = look.merges;
    if (scroller) updateSpacer();
    schedule();
  }

  $effect(() => {
    const l = layout;
    // Untracked: applying the layout notifies style listeners, which
    // update other state and must not become dependencies here.
    if (l) untrack(() => applyLayout(l));
  });

  // ---- Pointer input ----

  type Drag =
    | { kind: 'select'; area: PointerArea; pointerId: number }
    | { kind: 'resize'; rows: boolean; index: number; start: number; startSize: number; pointerId: number };
  let drag: Drag | null = null;
  let lastPointer = { x: 0, y: 0, shift: false, meta: false };
  let autoRaf = 0;

  function local(e: { clientX: number; clientY: number }): { x: number; y: number } {
    const rect = host.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  function areaAt(x: number, y: number): PointerArea {
    if (x < metrics.headerW && y < HEADER_H) return 'corner';
    if (y < HEADER_H) return 'col';
    if (x < metrics.headerW) return 'row';
    return 'cell';
  }

  // A header border within a few pixels of the pointer: the row or
  // column whose trailing edge it is.
  function borderAt(x: number, y: number): { rows: boolean; index: number } | null {
    const area = areaAt(x, y);
    if (area === 'col') {
      const c = cols.indexAt(x - metrics.headerW + sx);
      const right = metrics.headerW + cols.offset(c + 1) - sx;
      const left = metrics.headerW + cols.offset(c) - sx;
      if (Math.abs(x - right) <= 4) return { rows: false, index: c };
      if (Math.abs(x - left) <= 4 && c > 0) return { rows: false, index: prevVisible(cols, c) };
    } else if (area === 'row') {
      const r = rows.indexAt(y - HEADER_H + sy);
      const bottom = HEADER_H + rows.offset(r + 1) - sy;
      const top = HEADER_H + rows.offset(r) - sy;
      if (Math.abs(y - bottom) <= 3) return { rows: true, index: r };
      if (Math.abs(y - top) <= 3 && r > 0) return { rows: true, index: prevVisible(rows, r) };
    }
    return null;
  }

  function prevVisible(axis: AxisLayout, i: number): number {
    let k = i - 1;
    while (k > 0 && axis.size(k) === 0) k--;
    return k;
  }

  // Cell under a point, clamped into the content area (for drags that
  // leave the grid).
  function posAt(x: number, y: number): CellPos {
    const cx = Math.max(metrics.headerW, Math.min(x, viewW() - 1));
    const cy = Math.max(HEADER_H, Math.min(y, viewH() - 1));
    return { row: rows.indexAt(cy - HEADER_H + sy), col: cols.indexAt(cx - metrics.headerW + sx) };
  }

  function emit(kind: GridPointer['kind'], area: PointerArea): void {
    onpointer?.({
      kind,
      area,
      pos: posAt(lastPointer.x, lastPointer.y),
      shift: lastPointer.shift,
      meta: lastPointer.meta,
    });
  }

  function onPointerDown(e: PointerEvent): void {
    const p = local(e);
    // Clicks on non-overlay scrollbars land outside the client area.
    if (p.x >= viewW() || p.y >= viewH() || e.button !== 0) return;
    // Keep keyboard focus on the cell editor.
    e.preventDefault();
    lastPointer = { ...p, shift: e.shiftKey, meta: e.metaKey };
    scroller.setPointerCapture(e.pointerId);
    const border = borderAt(p.x, p.y);
    if (border) {
      const axis = border.rows ? rows : cols;
      drag = {
        kind: 'resize',
        rows: border.rows,
        index: border.index,
        start: border.rows ? p.y : p.x,
        startSize: axis.size(border.index),
        pointerId: e.pointerId,
      };
      return;
    }
    const area = areaAt(p.x, p.y);
    drag = { kind: 'select', area, pointerId: e.pointerId };
    emit('down', area);
  }

  function onPointerMove(e: PointerEvent): void {
    const p = local(e);
    if (!drag) {
      const border = borderAt(p.x, p.y);
      scroller.style.cursor = border
        ? border.rows
          ? 'row-resize'
          : 'col-resize'
        : areaAt(p.x, p.y) === 'cell'
          ? 'cell'
          : 'default';
      return;
    }
    if (drag.kind === 'resize') {
      const at = drag.rows ? p.y : p.x;
      const size = Math.max(0, Math.round(drag.startSize + at - drag.start));
      // Live preview; Go's layout event makes it permanent.
      (drag.rows ? rows : cols).setSize(drag.index, size);
      if (selLayer) {
        const edge = drag.rows
          ? HEADER_H + rows.offset(drag.index + 1) - sy
          : metrics.headerW + cols.offset(drag.index + 1) - sx;
        selLayer.guide = { vertical: !drag.rows, at: edge };
      }
      updateSpacer();
      schedule();
      return;
    }
    lastPointer = { ...p, shift: lastPointer.shift, meta: lastPointer.meta };
    emit('drag', drag.area);
    if (!autoRaf && outside(p)) autoRaf = requestAnimationFrame(autoScroll);
  }

  function onPointerUp(e: PointerEvent): void {
    if (!drag || e.pointerId !== drag.pointerId) return;
    const d = drag;
    drag = null;
    cancelAnimationFrame(autoRaf);
    autoRaf = 0;
    if (scroller.hasPointerCapture(e.pointerId)) scroller.releasePointerCapture(e.pointerId);
    if (d.kind === 'resize') {
      if (selLayer) selLayer.guide = null;
      const size = (d.rows ? rows : cols).size(d.index);
      if (size !== d.startSize) onresize?.({ rows: d.rows, index: d.index, px: size });
      schedule();
      return;
    }
    emit('up', d.area);
  }

  function onDblClick(e: MouseEvent): void {
    const p = local(e);
    const border = borderAt(p.x, p.y);
    if (border) {
      onautofit?.(border);
      return;
    }
    if (areaAt(p.x, p.y) !== 'cell') return;
    lastPointer = { ...p, shift: e.shiftKey, meta: e.metaKey };
    emit('dbl', 'cell');
  }

  function outside(p: { x: number; y: number }): boolean {
    return p.x < metrics.headerW || p.y < HEADER_H || p.x >= viewW() || p.y >= viewH();
  }

  // While dragging past an edge, scroll proportionally to the overshoot.
  function autoScroll(): void {
    autoRaf = 0;
    if (!drag || drag.kind !== 'select') return;
    const area = drag.area;
    const p = lastPointer;
    const speed = (over: number) => Math.sign(over) * Math.min(60, Math.abs(over) / 2 + 4);
    let dx = 0;
    let dy = 0;
    if (area !== 'row') {
      if (p.x < metrics.headerW) dx = speed(p.x - metrics.headerW);
      else if (p.x >= viewW()) dx = speed(p.x - viewW());
    }
    if (area !== 'col') {
      if (p.y < HEADER_H) dy = speed(p.y - HEADER_H);
      else if (p.y >= viewH()) dy = speed(p.y - viewH());
    }
    if (dx === 0 && dy === 0) return;
    setScroll(sx + dx, sy + dy);
    emit('drag', area);
    autoRaf = requestAnimationFrame(autoScroll);
  }

  // ---- Public API ----

  // Rectangle of a range in host coordinates (may lie off screen).
  export function areaRect(r: Range): Rect {
    return rangeRect(rows, cols, r, sx, sy);
  }

  export function cellRect(pos: CellPos): Rect {
    return areaRect({ r0: pos.row, c0: pos.col, r1: pos.row, c1: pos.col });
  }

  export function viewport(): { width: number; height: number } {
    return { width: viewW(), height: viewH() };
  }

  export function rowHeight(row: number): number {
    return rows.size(row);
  }

  export function colWidth(col: number): number {
    return cols.size(col);
  }

  // Style ID in effect at a position (cell, row or column style).
  export function styleAt(row: number, col: number): number {
    return gridLayer?.styleAt(row, col) ?? 0;
  }

  // Scrolls the minimum needed to show pos.
  export function scrollIntoView(pos: CellPos): void {
    const contentW = viewW() - metrics.headerW;
    const contentH = viewH() - HEADER_H;
    let x = sx;
    let y = sy;
    const left = cols.offset(pos.col);
    const right = left + cols.size(pos.col);
    const top = rows.offset(pos.row);
    const bottom = top + rows.size(pos.row);
    if (left < x) x = left;
    else if (right > x + contentW) x = Math.min(left, right - contentW);
    if (top < y) y = top;
    else if (bottom > y + contentH) y = Math.min(top, bottom - contentH);
    if (x !== sx || y !== sy) setScroll(x, y);
  }

  // Rows / columns that fit in the viewport, for Page Up/Down.
  export function pageSize(): { rows: number; cols: number } {
    return {
      rows: Math.max(1, rows.indexAt(sy + viewH() - HEADER_H) - rows.indexAt(sy)),
      cols: Math.max(1, cols.indexAt(sx + viewW() - metrics.headerW) - cols.indexAt(sx)),
    };
  }

  export function scrollByCells(dRows: number, dCols: number): void {
    setScroll(sx + dCols * cols.defaultSize, sy + dRows * rows.defaultSize);
  }

  export function redraw(): void {
    schedule();
  }

  // Redraw when the selection or the reference highlights change.
  $effect(() => {
    void selection;
    void highlights;
    schedule();
  });

  onMount(() => {
    gridLayer = new GridRenderer(gridCanvas, rows, cols, cache);
    selLayer = new SelectionRenderer(selCanvas, rows, cols);
    gridLayer.look = look;
    selLayer.merges = look.merges;
    cache.onUpdate = schedule;
    const offStyles = styleStore.subscribe(schedule);

    const ro = new ResizeObserver(resize);
    ro.observe(host);

    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const onTheme = () => {
      gridLayer?.refreshTheme();
      selLayer?.refreshTheme();
      schedule();
    };
    media.addEventListener('change', onTheme);

    return () => {
      destroyed = true;
      cache.onUpdate = () => {};
      offStyles();
      ro.disconnect();
      media.removeEventListener('change', onTheme);
      cancelAnimationFrame(raf);
      cancelAnimationFrame(autoRaf);
    };
  });

  // ---- Bench harness ----

  // Drives scrolling programmatically, one step per animation frame, and
  // records frame intervals and draw cost.
  export function runScrollBench(
    frames: number,
    step: (i: number) => { x: number; y: number },
  ): Promise<ScrollBenchResult> {
    benchRunning = true;
    const frameMs: number[] = [];
    const drawMs: number[] = [];
    let last = 0;
    return new Promise((resolve) => {
      const tick = (t: number) => {
        if (last > 0) frameMs.push(t - last);
        last = t;
        const i = frameMs.length;
        if (i >= frames) {
          benchRunning = false;
          onScroll();
          resolve({ frameMs, drawMs });
          return;
        }
        const pos = step(i);
        sx = clamp(pos.x, cols.total - (viewW() - metrics.headerW));
        sy = clamp(pos.y, rows.total - (viewH() - HEADER_H));
        scroller.scrollLeft = scrollX.toScroll(sx);
        scroller.scrollTop = scrollY.toScroll(sy);
        const t0 = performance.now();
        drawNow();
        drawMs.push(performance.now() - t0);
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
  }

  // Composites both layers into a PNG data URL.
  export function snapshot(): string {
    const out = document.createElement('canvas');
    out.width = gridCanvas.width;
    out.height = gridCanvas.height;
    const ctx = out.getContext('2d')!;
    ctx.drawImage(gridCanvas, 0, 0);
    ctx.drawImage(selCanvas, 0, 0);
    return out.toDataURL('image/png');
  }

  export function scrollToOrigin(): void {
    scroller.scrollTo(0, 0);
  }

  export function contentSize(): { width: number; height: number } {
    return { width: cols.total, height: rows.total };
  }
</script>

<div class="grid-host" bind:this={host}>
  <canvas class="layer" bind:this={gridCanvas}></canvas>
  <canvas class="layer" bind:this={selCanvas}></canvas>
  <!-- Transparent native scroller on top: receives wheel, scrollbar and pointer input. -->
  <div
    class="scroller"
    role="application"
    aria-label="工作表格線"
    bind:this={scroller}
    onscroll={onScroll}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerUp}
    ondblclick={onDblClick}
  >
    <div style:width="{spacerW}px" style:height="{spacerH}px"></div>
  </div>
  {@render children?.()}
</div>

<style>
  .grid-host {
    position: relative;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    background: var(--grid-bg);
  }
  .layer {
    position: absolute;
    inset: 0;
    pointer-events: none;
  }
  .scroller {
    position: absolute;
    inset: 0;
    overflow: scroll;
    cursor: cell;
  }
</style>
