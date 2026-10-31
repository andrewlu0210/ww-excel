<script lang="ts">
  import { t } from '../i18n';
  import { onMount, untrack, type Snippet } from 'svelte';
  import { sheetApi, type LayoutDTO, type SheetInfo } from '../bridge/sheet';
  import { AxisLayout } from '../grid/layout';
  import { Samples, type ScrollBenchResult } from '../grid/perf';
  import {
    emptyLook,
    filterButton,
    frozenRect,
    GridRenderer,
    HEADER_H,
    headerWidthFor,
    metrics,
    SelectionRenderer,
    type Highlight,
    type Rect,
    type SheetLook,
  } from '../grid/renderer';
  import { ScrollAxis } from '../grid/scroll';
  import { fitZoom, ZoomTransform } from '../grid/zoom';
  import type { CellPos, Range, Selection } from '../grid/selection';
  import type { ContextRequest, GridPointer, PointerArea, ResizeRequest } from '../input/pointer';
  import { styleStore } from '../stores/styles';
  import { BlockCache } from '../stores/viewportCache';

  interface Props {
    painting?: boolean;
    zoom?: number;
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
    // Copied range (marching ants) and fill-handle drag target.
    copyRange?: Range | null;
    fillPreview?: Range | null;
    // A filter button was clicked: its column and where to open the menu.
    onfilterbutton?: (e: { row:number; col: number; x: number; y: number }) => void;
    oncontext?: (e: ContextRequest) => void;
    // The cell under the idle pointer (null outside cells), with its
    // rectangle in host coordinates.
    onhover?: (pos: CellPos | null) => void;
    children?: Snippet;
  }

  let {
    painting = false,
    zoom = 100,
    sheet,
    info,
    layout,
    selection,
    highlights = [],
    onpointer,
    onresize,
    onautofit,
    onviewport,
    copyRange = null,
    fillPreview = null,
    onfilterbutton,
    oncontext,
    onhover,
    children,
  }: Props = $props();

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

  let transform = $derived(new ZoomTransform(zoom));

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
    drawSelection();
    onviewport?.();
  }

  let antsPhase = 0;
  function drawSelection(): void {
    selLayer?.draw(sx, sy, selection, highlights, { copy: copyRange, antsPhase, fill: fillPreview });
  }

  // Marching ants: redraw only the selection layer.
  $effect(() => {
    if (!copyRange) return;
    const timer = setInterval(() => {
      antsPhase = (antsPhase + 1) % 7;
      if (!destroyed) drawSelection();
    }, 90);
    return () => clearInterval(timer);
  });

  // Frozen panes: positions left of / above the freeze lines do not
  // scroll.
  function frozenW(): number {
    return look.freeze.cols > 0 ? cols.offset(look.freeze.cols) : 0;
  }
  function frozenH(): number {
    return look.freeze.rows > 0 ? rows.offset(look.freeze.rows) : 0;
  }
  function scrollXAt(x: number): number {
    return x < metrics.headerW + frozenW() ? 0 : sx;
  }
  function scrollYAt(y: number): number {
    return y < HEADER_H + frozenH() ? 0 : sy;
  }

  function viewW(): number {
    return transform.toLogical(scroller.clientWidth);
  }

  function viewH(): number {
    return transform.toLogical(scroller.clientHeight);
  }

  function updateSpacer(): void {
    scrollY.update(transform.toScreen(HEADER_H + rows.total), scroller.clientHeight);
    scrollX.update(transform.toScreen(metrics.headerW + cols.total), scroller.clientWidth);
    spacerH = scrollY.spacer;
    spacerW = scrollX.spacer;
  }

  function resize(): void {
    // Exclude space taken by non-overlay scrollbars.
    const w = scroller.clientWidth;
    const h = scroller.clientHeight;
    const dpr = window.devicePixelRatio || 1;
    gridLayer?.resize(w, h, dpr, transform.scale);
    selLayer?.resize(w, h, dpr, transform.scale);
    updateSpacer();
    drawNow();
  }

  function onScroll(): void {
    if (benchRunning || destroyed || !scroller) return;
    sx = transform.toLogical(scrollX.toContent(scroller.scrollLeft));
    sy = transform.toLogical(scrollY.toContent(scroller.scrollTop));
    schedule();
  }

  function setScroll(x: number, y: number): void {
    // Calls that arrive after unmount (pending edits, selections) do
    // nothing.
    if (destroyed || !scroller) return;
    sx = clamp(x, cols.total - (viewW() - metrics.headerW));
    sy = clamp(y, rows.total - (viewH() - HEADER_H));
    scroller.scrollLeft = scrollX.toScroll(transform.toScreen(sx));
    scroller.scrollTop = scrollY.toScroll(transform.toScreen(sy));
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
      hideGridLines:l.hideGridLines ?? false,
      tableFilters:(l.tables ?? []).filter(t=>!t.noFilter).map(t=>({r0:t.range.r0,c0:t.range.c0,r1:t.range.r1,c1:t.range.c1})),
      rowStyles: new Map((l.rowStyles ?? []).map((s) => [s.i, s.s])),
      colStyles: new Map((l.colStyles ?? []).map((s) => [s.i, s.s])),
      merges: (l.merges ?? []).map((m) => ({ r0: m.r0, c0: m.c0, r1: m.r1, c1: m.c1 })),
      freeze: { rows: l.freezeRows ?? 0, cols: l.freezeCols ?? 0 },
      filter: l.filter ? { r0: l.filter.r0, c0: l.filter.c0, r1: l.filter.r1, c1: l.filter.c1 } : null,
      filtered: new Set(l.filteredCols ?? []),
      notes: new Set((l.notes ?? []).map((n) => n.row * 16384 + n.col)),
    };
    if (gridLayer) gridLayer.look = look;
    if (selLayer) {
      selLayer.merges = look.merges;
      selLayer.freeze = look.freeze;
    }
    if (scroller) updateSpacer();
    schedule();
  }

  $effect(() => {
    const l = layout;
    // Untracked: applying the layout notifies style listeners, which
    // update other state and must not become dependencies here.
    if (l) untrack(() => applyLayout(l));
  });

  $effect(() => {
    void transform;
    untrack(() => {
      if (!gridLayer || !scroller || destroyed) return;
      resize();
      setScroll(sx, sy);
    });
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
    return { x: transform.toLogical(e.clientX - rect.left), y: transform.toLogical(e.clientY - rect.top) };
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
      const s = scrollXAt(x);
      const c = cols.indexAt(x - metrics.headerW + s);
      const right = metrics.headerW + cols.offset(c + 1) - s;
      const left = metrics.headerW + cols.offset(c) - s;
      if (Math.abs(x - right) <= transform.toLogical(4)) return { rows: false, index: c };
      if (Math.abs(x - left) <= transform.toLogical(4) && c > 0) return { rows: false, index: prevVisible(cols, c) };
    } else if (area === 'row') {
      const s = scrollYAt(y);
      const r = rows.indexAt(y - HEADER_H + s);
      const bottom = HEADER_H + rows.offset(r + 1) - s;
      const top = HEADER_H + rows.offset(r) - s;
      if (Math.abs(y - bottom) <= transform.toLogical(3)) return { rows: true, index: r };
      if (Math.abs(y - top) <= transform.toLogical(3) && r > 0) return { rows: true, index: prevVisible(rows, r) };
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
    return { row: rows.indexAt(cy - HEADER_H + scrollYAt(cy)), col: cols.indexAt(cx - metrics.headerW + scrollXAt(cx)) };
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
    if (p.x >= viewW() || p.y >= viewH() || e.button !== 0 || e.ctrlKey) return;
    // Keep keyboard focus on the cell editor.
    e.preventDefault();
    lastPointer = { ...p, shift: e.shiftKey, meta: e.metaKey };
    scroller.setPointerCapture(e.pointerId);
    const border = painting ? null : borderAt(p.x, p.y);
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
    if (!painting && onHandle(p.x, p.y)) {
      drag = { kind: 'select', area: 'fill', pointerId: e.pointerId };
      emit('down', 'fill');
      return;
    }
    const area = areaAt(p.x, p.y);
    if (!painting && area === 'cell') {
      const btn = filterButtonAt(p.x, p.y);
      if (btn) {
        scroller.releasePointerCapture(e.pointerId);
        const rect = host.getBoundingClientRect();
        onfilterbutton?.({ row:btn.row,col: btn.col, x: rect.left + transform.toScreen(btn.x), y: rect.top + transform.toScreen(btn.y) });
        return;
      }
    }
    drag = { kind: 'select', area, pointerId: e.pointerId };
    emit('down', area);
  }

  function onHandle(x: number, y: number): boolean {
    const h = selLayer?.handle;
    return !!h && x >= h.x && x <= h.x + h.w && y >= h.y && y <= h.y + h.h;
  }

  // The filter dropdown button under a point, if any.
  function filterButtonAt(x: number, y: number): { row:number; col: number; x: number; y: number } | null {
    const pos = posAt(x, y);
    const f=[...(look.tableFilters ?? []),...(look.filter ? [look.filter] : [])].find(r=>pos.row===r.r0 && pos.col>=r.c0 && pos.col<=r.c1);
    if (!f) return null;
    if (pos.row !== f.r0 || pos.col < f.c0 || pos.col > f.c1) return null;
    const b = filterButton(frozenRect(rows, cols, { r0: pos.row, c0: pos.col, r1: pos.row, c1: pos.col }, look.freeze, sx, sy));
    if (x < b.x || x > b.x + b.w || y < b.y || y > b.y + b.h) return null;
    return { row:pos.row,col: pos.col, x: b.x, y: b.y + b.h };
  }

  function onContextMenu(e: MouseEvent): void {
    e.preventDefault();
    const p = local(e);
    if (p.x >= viewW() || p.y >= viewH()) return;
    oncontext?.({ area: areaAt(p.x, p.y), pos: posAt(p.x, p.y), x: e.clientX, y: e.clientY });
  }

  let hovered = -1;
  function onPointerMove(e: PointerEvent): void {
    const p = local(e);
    if (!drag) {
      const over = areaAt(p.x, p.y) === 'cell' ? posAt(p.x, p.y) : null;
      const key = over ? over.row * 16384 + over.col : -1;
      if (key !== hovered) {
        hovered = key;
        onhover?.(over);
      }
      const border = borderAt(p.x, p.y);
      scroller.style.cursor = painting ? 'copy' : border
        ? border.rows
          ? 'row-resize'
          : 'col-resize'
        : onHandle(p.x, p.y)
          ? 'crosshair'
          : areaAt(p.x, p.y) === 'cell' && !filterButtonAt(p.x, p.y)
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
    if (painting) return;
    const p = local(e);
    const border = painting ? null : borderAt(p.x, p.y);
    if (border) {
      onautofit?.(border);
      return;
    }
    if (!painting && onHandle(p.x, p.y)) {
      emit('dbl', 'fill');
      return;
    }
    if (areaAt(p.x, p.y) !== 'cell' || filterButtonAt(p.x, p.y)) return;
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
    const frozen = { x: metrics.headerW + frozenW(), y: HEADER_H + frozenH() };
    const p = lastPointer;
    const speed = (over: number) => Math.sign(over) * Math.min(60, Math.abs(over) / 2 + 4);
    let dx = 0;
    let dy = 0;
    if (area !== 'row') {
      if (p.x < frozen.x && sx > 0) dx = speed(p.x - frozen.x);
      else if (p.x >= viewW()) dx = speed(p.x - viewW());
    }
    if (area !== 'col') {
      if (p.y < frozen.y && sy > 0) dy = speed(p.y - frozen.y);
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
    return transform.rect(frozenRect(rows, cols, r, look.freeze, sx, sy));
  }

  export function cellRect(pos: CellPos): Rect {
    return areaRect({ r0: pos.row, c0: pos.col, r1: pos.row, c1: pos.col });
  }

  function selectionBounds(): Range {
    return selection.ranges.reduce((a, b) => ({
      r0: Math.min(a.r0, b.r0), c0: Math.min(a.c0, b.c0),
      r1: Math.max(a.r1, b.r1), c1: Math.max(a.c1, b.c1),
    }));
  }

  export function zoomForSelection(): number {
    const r = selectionBounds();
    const width = frozenW() + Math.max(0, cols.offset(r.c1 + 1) - cols.offset(Math.max(r.c0, look.freeze.cols)));
    const height = frozenH() + Math.max(0, rows.offset(r.r1 + 1) - rows.offset(Math.max(r.r0, look.freeze.rows)));
    return fitZoom(scroller.clientWidth, scroller.clientHeight, metrics.headerW + width, HEADER_H + height);
  }

  export function revealSelection(): void {
    const r = selectionBounds();
    setScroll(Math.max(0, cols.offset(r.c0) - frozenW()), Math.max(0, rows.offset(r.r0) - frozenH()));
  }

  export function viewport(): { width: number; height: number } {
    return { width: scroller.clientWidth, height: scroller.clientHeight };
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
    if (destroyed || !scroller) return;
    const contentW = viewW() - metrics.headerW;
    const contentH = viewH() - HEADER_H;
    // Frozen cells are always visible; the others must clear the frozen
    // panes.
    const fw = frozenW();
    const fh = frozenH();
    let x = sx;
    let y = sy;
    const left = cols.offset(pos.col);
    const right = left + cols.size(pos.col);
    const top = rows.offset(pos.row);
    const bottom = top + rows.size(pos.row);
    if (pos.col >= look.freeze.cols) {
      if (left < x + fw) x = left - fw;
      else if (right > x + contentW) x = Math.min(left - fw, right - contentW);
    }
    if (pos.row >= look.freeze.rows) {
      if (top < y + fh) y = top - fh;
      else if (bottom > y + contentH) y = Math.min(top - fh, bottom - contentH);
    }
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
    void copyRange;
    void fillPreview;
    schedule();
  });

  onMount(() => {
    gridLayer = new GridRenderer(gridCanvas, rows, cols, cache);
    selLayer = new SelectionRenderer(selCanvas, rows, cols);
    gridLayer.look = look;
    selLayer.merges = look.merges;
    selLayer.freeze = look.freeze;
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
    window.addEventListener('ww-theme', onTheme);

    return () => {
      destroyed = true;
      cache.onUpdate = () => {};
      offStyles();
      ro.disconnect();
      media.removeEventListener('change', onTheme);
      window.removeEventListener('ww-theme', onTheme);
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
        scroller.scrollLeft = scrollX.toScroll(transform.toScreen(sx));
        scroller.scrollTop = scrollY.toScroll(transform.toScreen(sy));
        const t0 = performance.now();
        drawNow();
        drawMs.push(performance.now() - t0);
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
  }

  // Composites both layers into a PNG data URL.
  export function snapshot(objects?: (ctx: CanvasRenderingContext2D, dpr: number) => void): string {
    const out = document.createElement('canvas');
    out.width = gridCanvas.width;
    out.height = gridCanvas.height;
    const ctx = out.getContext('2d')!;
    ctx.drawImage(gridCanvas, 0, 0);
    ctx.drawImage(selCanvas, 0, 0);
    objects?.(ctx, window.devicePixelRatio || 1);
    return out.toDataURL('image/png');
  }

  export function scrollToOrigin(): void {
    if (destroyed || !scroller) return;
    scroller.scrollTo(0, 0);
  }

  // The cell under a point in host coordinates, and that cell's
  // rectangle (for anchoring dragged objects).
  export function cellAtPoint(x: number, y: number): { pos: CellPos; rect: Rect } {
    const pos = posAt(transform.toLogical(x), transform.toLogical(y));
    return { pos, rect: cellRect(pos) };
  }

  export function freeze(): { rows: number; cols: number } {
    return look.freeze;
  }

  // Visible top-left scrollable cell, for freezing panes.
  export function topLeft(): CellPos {
    return { row: rows.indexAt(sy + frozenH()), col: cols.indexAt(sx + frozenW()) };
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
    style:cursor={painting ? 'copy' : undefined}
    role="application"
    aria-label={t('工作表格線')}
    bind:this={scroller}
    onscroll={onScroll}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerUp}
    ondblclick={onDblClick}
    oncontextmenu={onContextMenu}
    onpointerleave={() => {
      hovered = -1;
      onhover?.(null);
    }}
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
