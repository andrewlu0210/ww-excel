<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { sheetApi, type SheetInfo } from '../bridge/sheet';
  import { AxisLayout } from '../grid/layout';
  import { Samples, type ScrollBenchResult } from '../grid/perf';
  import { GridRenderer, HEADER_H, HEADER_W, SelectionRenderer, type CellPos } from '../grid/renderer';
  import { ScrollAxis } from '../grid/scroll';
  import { BlockCache } from '../stores/viewportCache';

  interface Props {
    sheet: number;
    info: SheetInfo;
    active: CellPos;
    onselect?: (pos: CellPos) => void;
  }

  let { sheet, info, active, onselect }: Props = $props();

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
    drawTimes.push(gridLayer.draw(sx, sy, active));
    selLayer.draw(sx, sy, active);
  }

  function resize(): void {
    // Exclude space taken by non-overlay scrollbars.
    const w = scroller.clientWidth;
    const h = scroller.clientHeight;
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

  function onPointerDown(e: PointerEvent): void {
    const rect = host.getBoundingClientRect();
    const px = e.clientX - rect.left;
    const py = e.clientY - rect.top;
    // Clicks on the native scrollbars land outside the client area.
    if (px >= scroller.clientWidth || py >= scroller.clientHeight) return;
    if (px < HEADER_W || py < HEADER_H) return; // header selection comes in v0.2
    onselect?.({ row: rows.indexAt(py - HEADER_H + sy), col: cols.indexAt(px - HEADER_W + sx) });
  }

  // Redraw when the active cell changes.
  $effect(() => {
    void active.row;
    void active.col;
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
    };
  });

  // Drives scrolling programmatically, one step per animation frame, and
  // records frame intervals and draw cost. Used by the bench harness.
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
        sx = clamp(pos.x, cols.total - (scroller.clientWidth - HEADER_W));
        sy = clamp(pos.y, rows.total - (scroller.clientHeight - HEADER_H));
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

  function clamp(v: number, max: number): number {
    return Math.max(0, Math.min(v, max));
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
  >
    <div style:width="{spacerW}px" style:height="{spacerH}px"></div>
  </div>
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
