<script lang="ts">
  import { onMount, untrack, type Snippet } from 'svelte';
  import { sheetApi, type SheetInfo } from '../bridge/sheet';
  import { AxisLayout } from '../grid/layout';
  import { Samples, type ScrollBenchResult } from '../grid/perf';
  import { GridRenderer, HEADER_H, HEADER_W, rangeRect, SelectionRenderer, type Highlight, type Rect } from '../grid/renderer';
  import { ScrollAxis } from '../grid/scroll';
  import type { CellPos, Selection } from '../grid/selection';
  import type { GridPointer, PointerArea } from '../input/pointer';
  import { BlockCache } from '../stores/viewportCache';

  interface Props {
    sheet: number;
    info: SheetInfo;
    selection: Selection;
    // Reference boxes while editing a formula.
    highlights?: Highlight[];
    onpointer?: (e: GridPointer) => void;
    // Called after every redraw, e.g. to keep the editor on its cell.
    onviewport?: () => void;
    children?: Snippet;
  }

  let { sheet, info, selection, highlights = [], onpointer, onviewport, children }: Props = $props();

  const DEFAULT_ROW_H = 20;
  const DEFAULT_COL_W = 80;

  // Layout and cache are created once per mounted sheet; later prop
  // changes do not rebuild them.
  const geo = untrack(() => ({ ...info, sheet }));
  const rows = new AxisLayout(geo.rows, DEFAULT_ROW_H);
  const cols = new AxisLayout(geo.cols, DEFAULT_COL_W);
  export const cache = new BlockCache(
    (refs) => sheetApi.getBlocks(geo.sheet, refs),
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
  let sx = 0;
  let sy = 0;
  let raf = 0;
  let benchRunning = false;

  function schedule(): void {
    if (!raf) raf = requestAnimationFrame(frame);
  }

  function frame(): void {
    raf = 0;
    drawNow();
  }

  function drawNow(): void {
    if (!gridLayer || !selLayer) return;
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

  function resize(): void {
    // Exclude space taken by non-overlay scrollbars.
    const w = viewW();
    const h = viewH();
    const dpr = window.devicePixelRatio || 1;
    gridLayer?.resize(w, h, dpr);
    selLayer?.resize(w, h, dpr);
    scrollY.update(HEADER_H + rows.total, h);
    scrollX.update(HEADER_W + cols.total, w);
    spacerH = scrollY.spacer;
    spacerW = scrollX.spacer;
    drawNow();
  }

  function onScroll(): void {
    if (benchRunning) return;
    sx = scrollX.toContent(scroller.scrollLeft);
    sy = scrollY.toContent(scroller.scrollTop);
    schedule();
  }

  function setScroll(x: number, y: number): void {
    sx = clamp(x, cols.total - (viewW() - HEADER_W));
    sy = clamp(y, rows.total - (viewH() - HEADER_H));
    scroller.scrollLeft = scrollX.toScroll(sx);
    scroller.scrollTop = scrollY.toScroll(sy);
    schedule();
  }

  function clamp(v: number, max: number): number {
    return Math.max(0, Math.min(v, max));
  }

  // ---- Pointer input ----

  let drag: { area: PointerArea; pointerId: number } | null = null;
  let lastPointer = { x: 0, y: 0, shift: false, meta: false };
  let autoRaf = 0;

  function local(e: { clientX: number; clientY: number }): { x: number; y: number } {
    const rect = host.getBoundingClientRect();
    return { x: e.clientX - rect.left, y: e.clientY - rect.top };
  }

  function areaAt(x: number, y: number): PointerArea {
    if (x < HEADER_W && y < HEADER_H) return 'corner';
    if (y < HEADER_H) return 'col';
    if (x < HEADER_W) return 'row';
    return 'cell';
  }

  // Cell under a point, clamped into the content area (for drags that
  // leave the grid).
  function posAt(x: number, y: number): CellPos {
    const cx = Math.max(HEADER_W, Math.min(x, viewW() - 1));
    const cy = Math.max(HEADER_H, Math.min(y, viewH() - 1));
    return { row: rows.indexAt(cy - HEADER_H + sy), col: cols.indexAt(cx - HEADER_W + sx) };
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
    const area = areaAt(p.x, p.y);
    drag = { area, pointerId: e.pointerId };
    scroller.setPointerCapture(e.pointerId);
    emit('down', area);
  }

  function onPointerMove(e: PointerEvent): void {
    const p = local(e);
    if (!drag) {
      scroller.style.cursor = areaAt(p.x, p.y) === 'cell' ? 'cell' : 'default';
      return;
    }
    lastPointer = { ...p, shift: lastPointer.shift, meta: lastPointer.meta };
    emit('drag', drag.area);
    if (!autoRaf && outside(p)) autoRaf = requestAnimationFrame(autoScroll);
  }

  function onPointerUp(e: PointerEvent): void {
    if (!drag || e.pointerId !== drag.pointerId) return;
    const area = drag.area;
    drag = null;
    cancelAnimationFrame(autoRaf);
    autoRaf = 0;
    if (scroller.hasPointerCapture(e.pointerId)) scroller.releasePointerCapture(e.pointerId);
    emit('up', area);
  }

  function onDblClick(e: MouseEvent): void {
    const p = local(e);
    if (areaAt(p.x, p.y) !== 'cell') return;
    lastPointer = { ...p, shift: e.shiftKey, meta: e.metaKey };
    emit('dbl', 'cell');
  }

  function outside(p: { x: number; y: number }): boolean {
    return p.x < HEADER_W || p.y < HEADER_H || p.x >= viewW() || p.y >= viewH();
  }

  // While dragging past an edge, scroll proportionally to the overshoot.
  function autoScroll(): void {
    autoRaf = 0;
    if (!drag) return;
    const p = lastPointer;
    const speed = (over: number) => Math.sign(over) * Math.min(60, Math.abs(over) / 2 + 4);
    let dx = 0;
    let dy = 0;
    if (drag.area !== 'row') {
      if (p.x < HEADER_W) dx = speed(p.x - HEADER_W);
      else if (p.x >= viewW()) dx = speed(p.x - viewW());
    }
    if (drag.area !== 'col') {
      if (p.y < HEADER_H) dy = speed(p.y - HEADER_H);
      else if (p.y >= viewH()) dy = speed(p.y - viewH());
    }
    if (dx === 0 && dy === 0) return;
    setScroll(sx + dx, sy + dy);
    emit('drag', drag.area);
    autoRaf = requestAnimationFrame(autoScroll);
  }

  // ---- Public API ----

  // Cell rectangle in host coordinates (may lie outside the viewport).
  export function cellRect(pos: CellPos): Rect {
    return rangeRect(rows, cols, { r0: pos.row, c0: pos.col, r1: pos.row, c1: pos.col }, sx, sy);
  }

  export function viewport(): { width: number; height: number } {
    return { width: viewW(), height: viewH() };
  }

  // Scrolls the minimum needed to show pos.
  export function scrollIntoView(pos: CellPos): void {
    const contentW = viewW() - HEADER_W;
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
      cols: Math.max(1, cols.indexAt(sx + viewW() - HEADER_W) - cols.indexAt(sx)),
    };
  }

  export function scrollByCells(dRows: number, dCols: number): void {
    setScroll(sx + dCols * DEFAULT_COL_W, sy + dRows * DEFAULT_ROW_H);
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
    cache.onUpdate = schedule;

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
        sx = clamp(pos.x, cols.total - (viewW() - HEADER_W));
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
